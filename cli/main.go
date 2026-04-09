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

	"golang.org/x/term"
)

// runWithTerminalRestore runs cmd and restores the terminal state and console
// code pages (on Windows) after it exits, regardless of success or failure.
func runWithTerminalRestore(cmd *exec.Cmd) error {
	savedState, _ := term.GetState(int(os.Stdin.Fd()))
	cpIn, cpOut := saveConsoleCP()
	defer func() {
		if savedState != nil {
			_ = term.Restore(int(os.Stdin.Fd()), savedState)
		}
		restoreConsoleCP(cpIn, cpOut)
	}()
	return cmd.Run()
}

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
	initLogger()

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

	// Subcommands (checked after --config pre-scan so os.Args is already clean).
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "update-config":
			if err := runConfigUpdate(); err != nil {
				log.Fatalf("%v", err)
			}
			return
		case "config-update":
			log.Warn("'config-update' is deprecated, use 'update-config' instead")
			if err := runConfigUpdate(); err != nil {
				log.Fatalf("%v", err)
			}
			return
		}
	}

	// 0. Load Global Config
	var globalConfig *GlobalConfig
	var err error
	if configFlagArg != nil && *configFlagArg != "" {
		globalConfig, err = loadGlobalConfigFromPath(*configFlagArg)
		if err != nil {
			log.Fatalf("loading config: %v", err)
		}
	} else {
		globalConfig, err = loadGlobalConfig()
	}
	if err != nil {
		log.Warnf("could not load global config: %v; using defaults", err)
		globalConfig = &GlobalConfig{
			DefaultEditor:        "micro",
			DefaultShell:         "zsh",
			MountSystemGitconfig: true,
			MountGhConfigDir:        true,
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
	nonInteractivePtr := flag.Bool("N", false, "Non-interactive mode for use as a process wrapper (e.g. claudeCode.claudeProcessWrapper)")
	flag.BoolVar(nonInteractivePtr, "noninteractive", false, "Non-interactive mode for use as a process wrapper")
	verbosePtr := flag.Bool("verbose", false, "Enable verbose/debug logging")

	var volumeFlags arrayFlags
	flag.Var(&volumeFlags, "v", "Additional volume mounts (e.g. /host:/container)")

	var portFlags arrayFlags
	flag.Var(&portFlags, "p", "Publish container port(s) to the host (e.g. 8080:8080)")

	flag.Parse()

	if *verbosePtr {
		enableVerboseLogging()
	}

	// Check if no arguments were provided (except flags)
	if len(os.Args) == 1 {
		// Auto-exec: try to find existing container for current directory
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("getting working directory: %v", err)
		}

		containerName, err := getContainerForDirectory(cwd)
		if err != nil {
			log.Fatalf("checking for existing containers: %v", err)
		}

		if containerName != "" {
			log.Infof("found existing container %q for current directory, re-attaching", containerName)

			// Exec into the existing container
			execArgs := []string{"exec", "-it", containerName, "/bin/bash"}
			execCmd := exec.Command("docker", execArgs...)
			execCmd.Stdin = os.Stdin
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr

			err = runWithTerminalRestore(execCmd)
			if err != nil {
				log.Fatalf("executing into container: %v", err)
			}
			return
		}

		log.Debug("no existing container found for current directory")
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("getting working directory: %v", err)
	}

	absDir, err := filepath.Abs(*dirPtr)
	if err != nil {
		log.Fatalf("resolving directory path: %v", err)
	}

	// 1. Ensure ./opencode.json exists only when opencode agent is enabled
	defaultConfigPath := filepath.Join(cwd, "opencode.json")
	if *configPtr == "opencode.json" && globalConfig.AgentFrameworks.OpenCode.Enabled {
		if _, err := os.Stat(defaultConfigPath); os.IsNotExist(err) {
			// Try to ensure it exists from template
			if err := ensureFileFromTemplate(defaultConfigPath, "configs/opencode/opencode.json"); err != nil {
				log.Warnf("could not ensure default opencode.json: %v", err)
			}
		}
	}

	// 2. Resolve Config File to use
	absConfig, err := filepath.Abs(*configPtr)
	if err != nil {
		log.Fatalf("resolving config path: %v", err)
	}

	if _, err := os.Stat(absConfig); os.IsNotExist(err) {
		if *configPtr != "opencode.json" {
			log.Fatalf("configuration file not found: %s", absConfig)
		}
		// If it's the default and not found, we just won't mount it to /project/opencode_config.json
		absConfig = ""
	}

	// 3a. Non-interactive early path: exec into an already-running container.
	// This is the fast path when VS Code spawns agentjail as a process wrapper
	// and an interactive agentjail session is already open for the same project.
	if *nonInteractivePtr {
		existingContainer, err := getContainerForDirectory(absDir)
		if err == nil && existingContainer != "" {
			log.Debugf("non-interactive: found running container %q, using docker exec", existingContainer)
			execArgs := nonInteractiveExecArgs(existingContainer, flag.Args())
			execCmd := exec.Command("docker", execArgs...)
			execCmd.Stdin = os.Stdin
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			if err := execCmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				log.Fatalf("exec into container: %v", err)
			}
			return
		}
	}

	// 3. Docker Image Logic
	imageName := "agentjail"
	needsBuild := !imageExists(imageName) || *buildPtr || *buildNoCachePtr

	if needsBuild {
		if *buildPtr || *buildNoCachePtr {
			log.Info("rebuilding agentjail image")
		} else {
			log.Info("docker image 'agentjail' not found, building")
		}

		var dockerfilePath string
		usingTemp := false

		// Determine Dockerfile to use
		if *dockerfilePtr != "" {
			// User provided via -D flag
			absPath, err := filepath.Abs(*dockerfilePtr)
			if err != nil {
				log.Fatalf("resolving Dockerfile path %q: %v", *dockerfilePtr, err)
			}
			if _, err := os.Stat(absPath); err != nil {
				if os.IsNotExist(err) {
					log.Fatalf("Dockerfile not found at %q (from -D flag)", absPath)
				}
				log.Fatalf("checking Dockerfile path %q: %v", absPath, err)
			}
			dockerfilePath = absPath
		} else {
			// Always use the embedded template unless -D is specified
			log.Info("using embedded Dockerfile template")
			tmpPath, err := createTempDockerfile()
			if err != nil {
				log.Fatalf("creating temp Dockerfile: %v", err)
			}
			dockerfilePath = tmpPath
			usingTemp = true
		}

		log.Infof("building with Dockerfile: %s", dockerfilePath)
		buildArgs := []string{
			"build", "-f", dockerfilePath, "-t", imageName,
			"--build-arg", fmt.Sprintf("SHELL=%s", *shellPtr),
			"--build-arg", fmt.Sprintf("EDITOR=%s", *editorPtr),
			"--build-arg", fmt.Sprintf("FILE_BROWSER=%s", globalConfig.FileBrowserCmd()),
			"--build-arg", fmt.Sprintf("USE_OPENCODE=%t", globalConfig.AgentFrameworks.OpenCode.Enabled),
			"--build-arg", fmt.Sprintf("USE_COPILOT=%t", globalConfig.AgentFrameworks.Copilot.Enabled),
			"--build-arg", fmt.Sprintf("USE_CLAUDE_CODE=%t", globalConfig.AgentFrameworks.ClaudeCode.Enabled),
			"--build-arg", "HOSTNAME=agentjail",
		}

		if *buildNoCachePtr {
			buildArgs = append(buildArgs, "--no-cache")
		}

		buildArgs = append(buildArgs, ".")
		buildCmd := exec.Command("docker", buildArgs...)
		if *nonInteractivePtr {
			buildCmd.Stdout = os.Stderr // keep stdout clean for the protocol stream
		} else {
			buildCmd.Stdout = os.Stdout
		}
		buildCmd.Stderr = os.Stderr

		if err := buildCmd.Run(); err != nil {
			if usingTemp {
				os.Remove(dockerfilePath)
			}
			log.Fatalf("building Docker image: %v", err)
		}

		if usingTemp {
			os.Remove(dockerfilePath)
		}
	} else {
		log.Debug("docker image 'agentjail' found")
	}

	// 4. Run Container
	log.Info("starting container")

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
		log.Fatalf("creating .agentjail folder: %v", err)
	}

	if err := updateGitignore(absDir); err != nil {
		log.Warnf("could not update .gitignore: %v", err)
	}

	// Check for version updates
	currentVersion := "1.0.0" // This should be updated with actual version
	if err := checkVersionUpdate(agentJailDir, currentVersion); err != nil {
		log.Warnf("version check failed: %v", err)
	}

	// Prepare environment variables for metadata
	envVars := map[string]string{
		"EDITOR":       *editorPtr,
		"VISUAL":       *editorPtr,
		"SHELL":        *shellPtr,
		"CONTAINER_ID": containerName,
	}

	// Copy template configs to .agentjail
	if err := copyTemplateConfigs(agentJailDir, globalConfig); err != nil {
		log.Warnf("could not copy template configs: %v", err)
	}

	// Resolve the preferred agent and write zellij files (only when zellij is enabled).
	if globalConfig.ZellijEnabled() {
		zellijAgentName := globalConfig.PreferredAgent
		if zellijAgentName == "" {
			if agents := enabledAgents(globalConfig); len(agents) > 0 {
				zellijAgentName = agents[0]
			}
		}
		zellijAgentCmd := ""
		if zellijAgentName != "" {
			zellijAgentCmd = agentCommand(zellijAgentName)
		}
		if err := writeZellijFiles(agentJailDir, globalConfig.ZellijThemeOrDefault(), zellijAgentName, zellijAgentCmd, globalConfig.FileBrowserCmd(), *shellPtr, globalConfig.ZellijPlugins); err != nil {
			log.Warnf("could not write zellij layout: %v", err)
		}
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
		"-e", fmt.Sprintf("VISUAL=%s", *editorPtr),
		"-e", fmt.Sprintf("FILE_BROWSER=%s", globalConfig.FileBrowserCmd()),
		"-e", fmt.Sprintf("SHELL=%s", *shellPtr),
		"-e", fmt.Sprintf("CONTAINER_ID=%s", containerName),
		"-e", fmt.Sprintf("HISTFILE=/root/.agentjail/%s_history", *shellPtr),
		"-e", fmt.Sprintf("AGENTJAIL_HOST_PATH=%s", absDir),
	)

	// Inject host UID/GID so the container can restore file ownership on exit.
	if hostUser, err := user.Current(); err == nil {
		runArgs = append(runArgs,
			"-e", fmt.Sprintf("HOST_UID=%s", hostUser.Uid),
			"-e", fmt.Sprintf("HOST_GID=%s", hostUser.Gid),
		)
	}

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
		if globalConfig.MountGhConfigDir {
			hostGhPath := filepath.Join(usr.HomeDir, ".config", "gh")
			if _, err := os.Stat(hostGhPath); err == nil {
				ghMount := fmt.Sprintf("%s:/root/.config/gh", hostGhPath)
				runArgs = append(runArgs, "-v", ghMount)
				volumes = append(volumes, ghMount)
				log.Info("mounting host gh CLI config for Copilot auth")
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
			log.Info("passing GH_TOKEN to container for Copilot auth")
		}
	}

	// Mount claude code config if enabled
	if globalConfig.AgentFrameworks.ClaudeCode.Enabled {
		usr, err := user.Current()
		if err != nil {
			log.Warnf("could not determine current user, skipping Claude Code host mounts: %v", err)
		} else {
			hostClaudePath := filepath.Join(usr.HomeDir, ".claude")
			if _, err := os.Stat(hostClaudePath); err == nil {
				claudeMount := fmt.Sprintf("%s:/root/.claude", hostClaudePath)
				runArgs = append(runArgs, "-v", claudeMount)
				volumes = append(volumes, claudeMount)
				log.Info("mounting host ~/.claude for Claude Code auth")
			}
			hostClaudeJSON := filepath.Join(usr.HomeDir, ".claude.json")
			if _, err := os.Stat(hostClaudeJSON); err == nil {
				claudeJSONMount := fmt.Sprintf("%s:/root/.claude.json", hostClaudeJSON)
				runArgs = append(runArgs, "-v", claudeJSONMount)
				volumes = append(volumes, claudeJSONMount)
			}
		}

		// Inject ANTHROPIC_API_KEY: config field takes priority, then host env var.
		// Skip if already configured via container_env_vars (user config takes precedence).
		if _, alreadyConfigured := globalConfig.ContainerEnvVars["ANTHROPIC_API_KEY"]; !alreadyConfigured {
			apiKey := globalConfig.AnthropicApiKey
			if apiKey == "" {
				apiKey = os.Getenv("ANTHROPIC_API_KEY")
			}
			if apiKey != "" {
				// Avoid exposing the key via command-line args (process listings, debug output).
				// Set in the process env and let Docker inherit it via a valueless -e flag.
				os.Setenv("ANTHROPIC_API_KEY", apiKey)
				runArgs = append(runArgs, "-e", "ANTHROPIC_API_KEY")
				log.Info("passing ANTHROPIC_API_KEY to container for Claude Code")
			}
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
			log.Fatalf("resolving editor config path: %v", err)
		}

		if _, err := os.Stat(absEditorConfig); os.IsNotExist(err) {
			log.Fatalf("editor config not found: %s", absEditorConfig)
		}

		baseName := filepath.Base(absEditorConfig)
		// Mount to /root/<filename>
		mountArg := fmt.Sprintf("%s:/root/%s", absEditorConfig, baseName)
		runArgs = append(runArgs, "-v", mountArg)
		volumes = append(volumes, mountArg)
		log.Infof("mounting editor config %s → /root/%s", absEditorConfig, baseName)
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
			log.Warn("skipping empty port mapping entry")
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
				log.Warnf("host environment variable %q is not set, skipping container var %q", hostVarName, key)
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
					log.Warnf("failed to obtain GitHub auth token from gh CLI, GITHUB_TOKEN will not be injected: %v", err)
				} else {
					if token := strings.TrimSpace(string(out)); token != "" {
						injectedGitHubToken = token
						runArgs = append(runArgs, "-e", "GITHUB_TOKEN")
						log.Info("injecting GITHUB_TOKEN from gh CLI auth token")
					} else {
						log.Warn("gh auth token returned empty output, GITHUB_TOKEN will not be injected")
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
		log.Warnf("could not save metadata: %v", err)
	}

	runArgs = append(runArgs, imageName)

	// When privileged mode is requested, ensure the Docker CLI is available inside
	// the container. The guard (command -v docker) makes this a no-op if it is
	// already installed in the image.
	dockerSetup := ""
	if *privilegedPtr {
		dockerSetup = "command -v docker >/dev/null 2>&1 || (apt-get update -qq && apt-get install -y -qq docker-ce-cli && apt-get clean && rm -rf /var/lib/apt/lists/*); "
		log.Info("privileged mode: will install Docker CLI on startup if not already present")
	}

	var niLockFile *os.File // held when we win the NI lock; released on exit/error
	if *nonInteractivePtr {
		// Non-interactive fallback: no running container found in the early path
		// above. Coordinate with any concurrently-spawned agentjail -N processes
		// (VS Code spawns the process wrapper twice when opening a session) so
		// that only one container starts and the other execs into it.
		niName := niContainerNameForPrefix(prefix)
		lockPath := filepath.Join(agentJailDir, "ni.lock")

		lockFile, won := tryNILock(lockPath)
		if !won {
			// Another process holds the lock and is starting a container.
			// Wait up to 10 s for it to appear, then exec into it.
			log.Debugf("non-interactive: waiting for concurrent container start (lock held by another process)")
			var found string
			for i := 0; i < 50; i++ {
				time.Sleep(200 * time.Millisecond)
				if c, _ := getContainerForDirectory(absDir); c != "" {
					found = c
					break
				}
			}
			if found != "" {
				log.Debugf("non-interactive: found container %q started by peer, exec-ing into it", found)
				execArgs := nonInteractiveExecArgs(found, flag.Args())
				execCmd := exec.Command("docker", execArgs...)
				execCmd.Stdin = os.Stdin
				execCmd.Stdout = os.Stdout
				execCmd.Stderr = os.Stderr
				if err := execCmd.Run(); err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						os.Exit(exitErr.ExitCode())
					}
					log.Fatalf("exec into container: %v", err)
				}
				return
			}
			// Timed out — start our own container without a name to avoid conflicts.
			log.Debugf("non-interactive: timed out waiting for peer container; starting own container")
			runArgs = adaptRunArgsForNonInteractive(runArgs, "")
		} else {
			// We hold the lock. Start the container with a fixed name so that
			// concurrently-spawned peers can find and exec into it. Release the
			// lock once the container is confirmed running.
			niLockFile = lockFile
			go func() {
				for i := 0; i < 75; i++ { // up to 15 s
					time.Sleep(200 * time.Millisecond)
					if isContainerRunning(niName) {
						break
					}
				}
				releaseNILock(lockFile)
			}()
			runArgs = adaptRunArgsForNonInteractive(runArgs, niName)
			// Update containerName and the CONTAINER_ID env var already in runArgs
			// so that metadata and the container's environment reflect the actual name.
			oldCID := fmt.Sprintf("CONTAINER_ID=%s", containerName)
			containerName = niName
			for i, arg := range runArgs {
				if arg == oldCID {
					runArgs[i] = fmt.Sprintf("CONTAINER_ID=%s", containerName)
					break
				}
			}
		}

		runArgs = append(runArgs, "claude")
		runArgs = append(runArgs, flag.Args()...)
	} else if globalConfig.ZellijEnabled() {
		if *autoStartPtr {
			log.Info("-A flag is no longer needed; the preferred agent launches automatically in the first zellij tab")
		}
		// Launch zellij with the 3-tab layout. mise trust/install runs first so all
		// tabs see the project's tools from the start.
		zellijEntrypoint := buildZellijEntrypoint(dirName)
		chownFix := `if [ -n "${HOST_UID}" ] && [ -n "${HOST_GID}" ]; then chown -R "${HOST_UID}:${HOST_GID}" /project /root/.agentjail 2>/dev/null || true; fi`
		runArgs = append(runArgs, "sh", "-c", dockerSetup+zellijEntrypoint+"; "+chownFix)
	} else {
		// Plain shell mode: restore the original -A behaviour.
		shell := *shellPtr
		if *autoStartPtr {
			agent := globalConfig.PreferredAgent
			if agent == "" {
				agent = chooseEnabledAgent(globalConfig)
			}
			if agent != "" {
				cmd := agentCommand(agent)
				initCmd := fmt.Sprintf("%smise trust --yes /project && mise install; %s; exec %s", dockerSetup, cmd, shell)
				runArgs = append(runArgs, shell, "-i", "-c", initCmd)
				log.Infof("auto-starting agent: %s", agent)
			} else {
				initCmd := fmt.Sprintf("%smise trust --yes /project && mise install; exec %s", dockerSetup, shell)
				runArgs = append(runArgs, shell, "-i", "-c", initCmd)
			}
		} else if dockerSetup != "" {
			// No -A and no zellij, but privileged: override the Dockerfile CMD so
			// we can prepend the Docker CLI install before dropping into the shell.
			initCmd := fmt.Sprintf("%smise trust --yes /project && mise install; exec %s", dockerSetup, shell)
			runArgs = append(runArgs, shell, "-i", "-c", initCmd)
		}
		// No -A, no zellij, no privileged: rely on the Dockerfile CMD (mise trust/install + shell).
	}

	log.Debugf("exec: docker %v", runArgs)

	runCmd := exec.Command("docker", runArgs...)
	// Ensure the injected GitHub token, if any, is available to docker via its environment.
	if injectedGitHubToken != "" {
		runCmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", injectedGitHubToken))
	}
	runCmd.Stdin = os.Stdin
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr

	if *nonInteractivePtr {
		if err := runCmd.Run(); err != nil {
			if niLockFile != nil {
				releaseNILock(niLockFile)
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			log.Fatalf("running Docker container: %v", err)
		}
	} else {
		err = runWithTerminalRestore(runCmd)
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				log.Fatalf("container exited with code %d", exitErr.ExitCode())
			}
			log.Fatalf("running Docker container: %v", err)
		}
	}
}
