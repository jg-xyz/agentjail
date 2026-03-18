package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

// arrayFlags allows setting multiple flags of the same name.
type arrayFlags []string

func (i *arrayFlags) String() string {
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	// Pre-scan os.Args for --config which takes an optional argument:
	//   agentjail --config             → print clean config and exit
	//   agentjail --config /path/file  → load config from that path
	var configFlagArg *string // nil = not provided; &"" = print mode; &"path" = load path
	{
		newArgs := []string{os.Args[0]}
		for i := 1; i < len(os.Args); i++ {
			arg := os.Args[i]
			if arg == "--config" || arg == "-config" {
				empty := ""
				configFlagArg = &empty
				if i+1 < len(os.Args) && !strings.HasPrefix(os.Args[i+1], "-") {
					path := os.Args[i+1]
					configFlagArg = &path
					i++
				}
			} else {
				newArgs = append(newArgs, arg)
			}
		}
		os.Args = newArgs
	}

	if configFlagArg != nil && *configFlagArg == "" {
		printCleanConfig()
		os.Exit(0)
	}

	// 0. Load Global Config
	var globalConfig *GlobalConfig
	var err error
	if configFlagArg != nil && *configFlagArg != "" {
		globalConfig, err = loadGlobalConfigFromPath(*configFlagArg)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
	} else {
		globalConfig, err = loadGlobalConfig()
	}
	if err != nil {
		fmt.Printf("Warning: Could not load global config: %v. Using defaults.\n", err)
		globalConfig = &GlobalConfig{
			DefaultEditor:        "micro",
			DefaultShell:         "zsh",
			MountSystemGitconfig: true,
			MountGhConfig:        true,
			AgentFrameworks: AgentFrameworksConfig{
				Copilot: FrameworkConfig{Enabled: true},
			},
		}
	}

	// Flags
	// -d: Directory to mount (Project Root)
	// -C: Path to opencode.json (Config)
	// -E: Path to editor config (e.g. .vimrc)
	// -D: Path to Dockerfile
	// -e: Editor binary name (e.g. micro, vim)
	// -v: Additional volumes (can be repeated)
	// -n: Docker network

	dirPtr := flag.String("d", ".", "Directory to mount as project folder")
	configPtr := flag.String("C", "opencode.json", "Path to opencode.json")
	editorConfigPtr := flag.String("E", "", "Path to editor config (mounted to /root/<filename>)")
	dockerfilePtr := flag.String("D", "", "Path to Dockerfile")
	editorPtr := flag.String("e", globalConfig.DefaultEditor, "Editor to use")
	shellPtr := flag.String("s", globalConfig.DefaultShell, "Shell to use (bash, zsh)")
	networkPtr := flag.String("n", "", "Docker network to connect to")
	buildPtr := flag.Bool("build", false, "Build/rebuild the agentjail image (uses cache)")
	flag.BoolVar(buildPtr, "b", false, "Build/rebuild the agentjail image (uses cache)")
	buildNoCachePtr := flag.Bool("build-no-cache", false, "Build/rebuild the agentjail image without cache")
	privilegedPtr := flag.Bool("P", false, "Run container in privileged mode with Docker daemon exposed")
	autoStartPtr := flag.Bool("A", false, "Automatically start the preferred agent when the container starts")

	var volumeFlags arrayFlags
	flag.Var(&volumeFlags, "v", "Additional volume mounts (e.g. /host:/container)")

	var portFlags arrayFlags
	flag.Var(&portFlags, "p", "Publish container port(s) to the host (e.g. 8080:8080)")

	flag.Parse()

	// Check if no arguments were provided (except flags)
	if len(os.Args) == 1 {
		// Auto-exec: try to find existing container for current directory
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Printf("Error getting current working directory: %v\n", err)
			os.Exit(1)
		}

		containerName, err := getContainerForDirectory(cwd)
		if err != nil {
			fmt.Printf("Error checking for existing containers: %v\n", err)
			os.Exit(1)
		}

		if containerName != "" {
			fmt.Printf("Found existing container '%s' for current directory. Executing into it...\n", containerName)

			// Exec into the existing container
			execArgs := []string{"exec", "-it", containerName, "/bin/bash"}
			execCmd := exec.Command("docker", execArgs...)
			execCmd.Stdin = os.Stdin
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr

			if err := execCmd.Run(); err != nil {
				fmt.Printf("Error executing into container: %v\n", err)
				os.Exit(1)
			}
			return
		}

		fmt.Println("No existing container found for current directory. Continuing with normal startup...")
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current working directory: %v\n", err)
		os.Exit(1)
	}

	absDir, err := filepath.Abs(*dirPtr)
	if err != nil {
		fmt.Printf("Error resolving directory path: %v\n", err)
		os.Exit(1)
	}

	// 1. Ensure ./opencode.json exists only when opencode agent is enabled
	defaultConfigPath := filepath.Join(cwd, "opencode.json")
	if *configPtr == "opencode.json" && globalConfig.AgentFrameworks.OpenCode.Enabled {
		if _, err := os.Stat(defaultConfigPath); os.IsNotExist(err) {
			// Try to ensure it exists from template
			if err := ensureFileFromTemplate(defaultConfigPath, "configs/opencode/opencode.json"); err != nil {
				fmt.Printf("Warning: Could not ensure default opencode.json: %v\n", err)
			}
		}
	}

	// 2. Resolve Config File to use
	absConfig, err := filepath.Abs(*configPtr)
	if err != nil {
		fmt.Printf("Error resolving config path: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(absConfig); os.IsNotExist(err) {
		if *configPtr != "opencode.json" {
			fmt.Printf("Error: Configuration file not found at %s\n", absConfig)
			os.Exit(1)
		}
		// If it's the default and not found, we just won't mount it to /project/opencode_config.json
		absConfig = ""
	}

	// 3. Docker Image Logic
	imageName := "agentjail"
	needsBuild := !imageExists(imageName) || *buildPtr || *buildNoCachePtr

	if needsBuild {
		if *buildPtr || *buildNoCachePtr {
			fmt.Println("Rebuilding 'agentjail' image...")
		} else {
			fmt.Println("Docker image 'agentjail' not found. Preparing to build...")
		}

		var dockerfilePath string
		usingTemp := false

		// Determine Dockerfile to use
		if *dockerfilePtr != "" {
			// User provided
			dockerfilePath, _ = filepath.Abs(*dockerfilePtr)
		} else {
			// Check local
			localDf := filepath.Join(cwd, "Dockerfile")
			localDfLower := filepath.Join(cwd, "dockerfile")

			if _, err := os.Stat(localDf); err == nil {
				dockerfilePath = localDf
			} else if _, err := os.Stat(localDfLower); err == nil {
				dockerfilePath = localDfLower
			} else {
				// Create temp
				fmt.Println("No local Dockerfile found. Using template.")
				tmpPath, err := createTempDockerfile()
				if err != nil {
					fmt.Printf("Error creating temp Dockerfile: %v\n", err)
					os.Exit(1)
				}
				dockerfilePath = tmpPath
				usingTemp = true
			}
		}

		fmt.Printf("Building with Dockerfile: %s\n", dockerfilePath)
		buildArgs := []string{
			"build", "-f", dockerfilePath, "-t", imageName,
			"--build-arg", fmt.Sprintf("SHELL=%s", *shellPtr),
			"--build-arg", fmt.Sprintf("EDITOR=%s", *editorPtr),
			"--build-arg", fmt.Sprintf("USE_OPENCODE=%t", globalConfig.AgentFrameworks.OpenCode.Enabled),
			"--build-arg", fmt.Sprintf("USE_COPILOT=%t", globalConfig.AgentFrameworks.Copilot.Enabled),
			"--build-arg", "HOSTNAME=agentjail",
		}

		if *buildNoCachePtr {
			buildArgs = append(buildArgs, "--no-cache")
		}

		buildArgs = append(buildArgs, ".")
		buildCmd := exec.Command("docker", buildArgs...)
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr

		if err := buildCmd.Run(); err != nil {
			fmt.Printf("Error building Docker image: %v\n", err)
			if usingTemp {
				os.Remove(dockerfilePath)
			}
			os.Exit(1)
		}

		if usingTemp {
			os.Remove(dockerfilePath)
		}
	} else {
		// Image exists
		// We don't check Dockerfile args here because we aren't building
		fmt.Println("Docker image 'agentjail' found.")
	}

	// 4. Run Container
	fmt.Println("\nStarting container...")

	// Generate container name: "agentjail." + first 5 chars of project directory name
	dirName := filepath.Base(absDir)
	prefix := dirName
	if len(prefix) > 5 {
		prefix = prefix[:5]
	}
	containerName := fmt.Sprintf("agentjail.%s", prefix)

	// Create .agentjail folder and update gitignore
	agentJailDir, err := createAgentJailFolder(absDir)
	if err != nil {
		fmt.Printf("Error creating .agentjail folder: %v\n", err)
		os.Exit(1)
	}

	if err := updateGitignore(absDir); err != nil {
		fmt.Printf("Warning: Could not update .gitignore: %v\n", err)
	}

	// Check for version updates
	currentVersion := "1.0.0" // This should be updated with actual version
	if err := checkVersionUpdate(agentJailDir, currentVersion); err != nil {
		fmt.Printf("Warning: Version check failed: %v\n", err)
	}

	// Prepare environment variables for metadata
	envVars := map[string]string{
		"EDITOR":       *editorPtr,
		"SHELL":        *shellPtr,
		"CONTAINER_ID": containerName,
	}

	// Copy template configs to .agentjail
	if err := copyTemplateConfigs(agentJailDir, globalConfig); err != nil {
		fmt.Printf("Warning: Could not copy template configs: %v\n", err)
	}

	// Collect all volumes for metadata
	volumes := []string{
		fmt.Sprintf("%s:/project", absDir),
		fmt.Sprintf("%s:/root/.agentjail", agentJailDir),
	}

	if absConfig != "" {
		volumes = append(volumes, fmt.Sprintf("%s:/project/opencode_config.json", absConfig))
	}

	runArgs := []string{
		"run", "-it", "--rm",
		"--name", containerName,
		"--hostname", "agentjail",
		"-v", volumes[0],
		"-v", volumes[1],
	}
	if absConfig != "" {
		runArgs = append(runArgs, "-v", volumes[2]) // index 2 is opencode_config.json
	}
	runArgs = append(runArgs,
		"-e", fmt.Sprintf("EDITOR=%s", *editorPtr),
		"-e", fmt.Sprintf("SHELL=%s", *shellPtr),
		"-e", fmt.Sprintf("CONTAINER_ID=%s", containerName),
		"-e", fmt.Sprintf("HISTFILE=/root/.agentjail/%s_history", *shellPtr),
	)

	// Mount rovr config (always)
	rovrMount := fmt.Sprintf("%s/rovr:/root/.config/rovr", agentJailDir)
	runArgs = append(runArgs, "-v", rovrMount)
	volumes = append(volumes, rovrMount)

	// Mount opencode config if enabled
	if globalConfig.AgentFrameworks.OpenCode.Enabled {
		opencodeMount := fmt.Sprintf("%s/opencode/opencode.json:/root/.config/opencode/config.json", agentJailDir)
		runArgs = append(runArgs, "-v", opencodeMount)
		volumes = append(volumes, opencodeMount)
		// Mount host opencode data dir for auth persistence
		usr, _ := user.Current()
		hostOpencodePath := filepath.Join(usr.HomeDir, ".config", "opencode")
		if _, err := os.Stat(hostOpencodePath); err == nil {
			hostOpencodeMount := fmt.Sprintf("%s:/root/.local/share/opencode", hostOpencodePath)
			runArgs = append(runArgs, "-v", hostOpencodeMount)
			volumes = append(volumes, hostOpencodeMount)
		}
	}

	// Mount copilot config if enabled
	if globalConfig.AgentFrameworks.Copilot.Enabled {
		usr, _ := user.Current()
		hostCopilotPath := filepath.Join(usr.HomeDir, ".config", "github-copilot")
		if _, err := os.Stat(hostCopilotPath); err == nil {
			// Mount host credentials so container doesn't re-authenticate
			copilotMount := fmt.Sprintf("%s:/root/.config/github-copilot", hostCopilotPath)
			runArgs = append(runArgs, "-v", copilotMount)
			volumes = append(volumes, copilotMount)
		} else {
			// Fall back to project-local dir (will require auth on first use)
			copilotMount := fmt.Sprintf("%s/copilot:/root/.config/github-copilot", agentJailDir)
			runArgs = append(runArgs, "-v", copilotMount)
			volumes = append(volumes, copilotMount)
		}

		// Mount gh CLI config (primary auth store used by gh copilot)
		if globalConfig.MountGhConfig {
			hostGhPath := filepath.Join(usr.HomeDir, ".config", "gh")
			if _, err := os.Stat(hostGhPath); err == nil {
				ghMount := fmt.Sprintf("%s:/root/.config/gh", hostGhPath)
				runArgs = append(runArgs, "-v", ghMount)
				volumes = append(volumes, ghMount)
				fmt.Println("Mounting host gh CLI config for Copilot auth.")
			}
		}

		// Mount copilot config files (config.json, mcp.json) as targeted mounts so
		// they are present even when the host credential directory is bind-mounted.
		for _, cfgFile := range []string{"config.json", "mcp.json"} {
			cfgPath := filepath.Join(agentJailDir, "copilot", cfgFile)
			if _, err := os.Stat(cfgPath); err == nil {
				mount := fmt.Sprintf("%s:/root/.config/github-copilot/%s", cfgPath, cfgFile)
				runArgs = append(runArgs, "-v", mount)
				volumes = append(volumes, mount)
			}
		}

		// Pass API token if configured or present in host environment
		token := globalConfig.GithubToken
		if token == "" {
			token = os.Getenv("GH_TOKEN")
		}
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}
		if token != "" {
			runArgs = append(runArgs, "-e", fmt.Sprintf("GH_TOKEN=%s", token))
			fmt.Println("Passing GH_TOKEN to container for Copilot auth.")
		}
	}

	// Mount system gitconfig if enabled
	if globalConfig.MountSystemGitconfig {
		usr, _ := user.Current()
		gitconfigPath := filepath.Join(usr.HomeDir, ".gitconfig")
		if _, err := os.Stat(gitconfigPath); err == nil {
			gitconfigMount := fmt.Sprintf("%s:/root/.gitconfig", gitconfigPath)
			runArgs = append(runArgs, "-v", gitconfigMount)
			volumes = append(volumes, gitconfigMount)
		}
	}

	// Handle -P (Privileged + Docker daemon)
	if *privilegedPtr {
		runArgs = append(runArgs, "--privileged")
		runArgs = append(runArgs, "-v", "/var/run/docker.sock:/var/run/docker.sock")
	}

	// Handle -n (Network)
	if *networkPtr != "" {
		runArgs = append(runArgs, "--network", *networkPtr)
	}

	// Handle -E (Editor Config)
	if *editorConfigPtr != "" {
		absEditorConfig, err := filepath.Abs(*editorConfigPtr)
		if err != nil {
			fmt.Printf("Error resolving editor config path: %v\n", err)
			os.Exit(1)
		}

		if _, err := os.Stat(absEditorConfig); os.IsNotExist(err) {
			fmt.Printf("Error: Editor config not found at %s\n", absEditorConfig)
			os.Exit(1)
		}

		baseName := filepath.Base(absEditorConfig)
		// Mount to /root/<filename>
		mountArg := fmt.Sprintf("%s:/root/%s", absEditorConfig, baseName)
		runArgs = append(runArgs, "-v", mountArg)
		volumes = append(volumes, mountArg)
		fmt.Printf("Mounting editor config: %s -> /root/%s\n", absEditorConfig, baseName)
	}

	// Handle -v (Additional Volumes)
	for _, v := range volumeFlags {
		runArgs = append(runArgs, "-v", v)
		volumes = append(volumes, v)
	}

	// Handle port mappings: merge config-defined mappings with -p flags
	// Build a new slice to avoid mutating globalConfig.PortMappings via append.
	allPorts := make([]string, 0, len(globalConfig.PortMappings)+len(portFlags))
	allPorts = append(allPorts, globalConfig.PortMappings...)
	allPorts = append(allPorts, portFlags...)
	for _, p := range allPorts {
		pTrimmed := strings.TrimSpace(p)
		if pTrimmed == "" {
			// Skip empty/whitespace-only port mappings to avoid emitting `-p ""`.
			fmt.Println("Warning: skipping empty port mapping entry")
			continue
		}
		runArgs = append(runArgs, "-p", pTrimmed)
	}

	// Handle container_env_vars from config
	for key, val := range globalConfig.ContainerEnvVars {
		resolvedVal := val
		if strings.HasPrefix(val, "env:") {
			hostVarName := strings.TrimPrefix(val, "env:")
			resolvedVal = os.Getenv(hostVarName)
			if resolvedVal == "" {
				// Host variable is not set; skip to avoid clobbering file-based auth fallbacks
				fmt.Printf("Warning: host environment variable %q is not set; skipping %q\n", hostVarName, key)
				continue
			}
		}
		runArgs = append(runArgs, "-e", fmt.Sprintf("%s=%s", key, resolvedVal))
	}

	// Inject GITHUB_TOKEN into the container if inject_gh_auth_token is enabled.
	// The token is resolved using the same fallback chain as GH_TOKEN injection:
	//   github_token config > GH_TOKEN env > GITHUB_TOKEN env > gh auth token (CLI).
	// This block is skipped entirely when GITHUB_TOKEN is already explicitly
	// configured via container_env_vars (user config takes precedence).
	var injectedGitHubToken string
	if globalConfig.InjectGhAuthToken {
		if _, alreadyConfigured := globalConfig.ContainerEnvVars["GITHUB_TOKEN"]; !alreadyConfigured {
			// Walk the standard fallback chain to find an effective token.
			effectiveToken := globalConfig.GithubToken
			if effectiveToken == "" {
				effectiveToken = os.Getenv("GH_TOKEN")
			}
			if effectiveToken == "" {
				effectiveToken = os.Getenv("GITHUB_TOKEN")
			}
			if effectiveToken != "" {
				// A token was resolved from the fallback chain; inject it silently.
				injectedGitHubToken = effectiveToken
				runArgs = append(runArgs, "-e", "GITHUB_TOKEN")
			} else {
				// No token found in the fallback chain; attempt to obtain one from the gh CLI.
				out, err := exec.Command("gh", "auth", "token").Output()
				if err != nil {
					fmt.Printf("Warning: failed to obtain GitHub auth token from gh CLI; GITHUB_TOKEN will not be injected: %v\n", err)
				} else {
					if token := strings.TrimSpace(string(out)); token != "" {
						injectedGitHubToken = token
						runArgs = append(runArgs, "-e", "GITHUB_TOKEN")
						fmt.Println("Injecting GITHUB_TOKEN from gh CLI auth token.")
					} else {
						fmt.Println("Warning: gh auth token returned empty output; GITHUB_TOKEN will not be injected.")
					}
				}
			}
		}
	}

	// Create and save metadata
	metadata := &AgentJailMetadata{
		ContainerName:    containerName,
		Network:          *networkPtr,
		Volumes:          volumes,
		EnvironmentVars:  envVars,
		ImageVersion:     imageName,
		CreatedAt:        time.Now(),
		LastUsed:         time.Now(),
		AgentJailVersion: currentVersion,
	}

	if err := saveMetadata(agentJailDir, metadata); err != nil {
		fmt.Printf("Warning: Could not save metadata: %v\n", err)
	}

	runArgs = append(runArgs, imageName)

	// Handle -A (auto-start agent)
	if *autoStartPtr {
		agent := globalConfig.PreferredAgent
		if agent == "" {
			agent = chooseEnabledAgent(globalConfig)
		}
		if agent != "" {
			cmd := agentCommand(agent)
			shell := *shellPtr
			// Use -i so the shell rc file is sourced (enables mise PATH activation),
			// run mise trust/install, then launch the agent, then drop into an interactive shell.
			initCmd := fmt.Sprintf("mise trust && mise install; %s; exec %s", cmd, shell)
			runArgs = append(runArgs, shell, "-i", "-c", initCmd)
			fmt.Printf("Auto-starting agent: %s\n", agent)
		}
	}

	fmt.Printf("Exec: docker %v\n", runArgs)

	runCmd := exec.Command("docker", runArgs...)
	// Ensure the injected GitHub token, if any, is available to docker via its environment.
	if injectedGitHubToken != "" {
		runCmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", injectedGitHubToken))
	}
	runCmd.Stdin = os.Stdin
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr

	if err := runCmd.Run(); err != nil {
		fmt.Printf("\nError running Docker container: %v\n", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Printf("Container exited with code: %d\n", exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
