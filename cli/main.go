package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed templates/*
var templatesFS embed.FS

// ensureFileFromTemplate checks if targetPath exists. If not, it writes the template content to it.
func ensureFileFromTemplate(targetPath string, templateName string) error {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		content, err := templatesFS.ReadFile("templates/" + templateName)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", templateName, err)
		}

		if err := os.WriteFile(targetPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", targetPath, err)
		}
		fmt.Printf("Created %s from template.\n", targetPath)
	}
	return nil
}

// createTempDockerfile creates a temporary Dockerfile from the template and returns its path.
func createTempDockerfile() (string, error) {
	content, err := templatesFS.ReadFile("templates/Dockerfile")
	if err != nil {
		return "", fmt.Errorf("failed to read Dockerfile template: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "Dockerfile-agentjail-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp Dockerfile: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(content); err != nil {
		return "", fmt.Errorf("failed to write to temp Dockerfile: %w", err)
	}

	return tmpFile.Name(), nil
}

func imageExists(imageName string) bool {
	cmd := exec.Command("docker", "image", "inspect", imageName)
	cmd.Stdout = nil // specific output not needed, just exit code
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// getContainerForDirectory finds a running container that has the current directory mounted
func getContainerForDirectory(dir string) (string, error) {
	// Get all running containers
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}\t{{.Mounts}}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	// Split by lines and check each container
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		containerName := parts[0]
		mounts := parts[1]

		// Check if the current directory is in the mounts
		if strings.Contains(mounts, dir) {
			return containerName, nil
		}
	}

	return "", nil
}

// Metadata structure for .agentjail/metadata.json
type AgentJailMetadata struct {
	ContainerName    string            `json:"container_name"`
	Network          string            `json:"network,omitempty"`
	Volumes          []string          `json:"volumes"`
	EnvironmentVars  map[string]string `json:"environment_vars"`
	ImageVersion     string            `json:"image_version"`
	CreatedAt        time.Time         `json:"created_at"`
	LastUsed         time.Time         `json:"last_used"`
	AgentJailVersion string            `json:"agentjail_version"`
}

// createAgentJailFolder creates the .agentjail folder and ensures it's set up properly
func createAgentJailFolder(baseDir string) (string, error) {
	agentJailDir := filepath.Join(baseDir, ".agentjail")

	if err := os.MkdirAll(agentJailDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .agentjail directory: %w", err)
	}

	// Create history file if it doesn't exist
	historyFile := filepath.Join(agentJailDir, "bash_history")
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		if err := os.WriteFile(historyFile, []byte{}, 0644); err != nil {
			return "", fmt.Errorf("failed to create history file: %w", err)
		}
	}

	return agentJailDir, nil
}

// GlobalConfig structure for ~/.config/agentjail/config.yaml
type GlobalConfig struct {
	DefaultEditor        string                `yaml:"default_editor"`
	DefaultShell         string                `yaml:"default_shell"`
	MountSystemGitconfig bool                  `yaml:"mount_system_gitconfig"`
	MountGhConfig        bool                  `yaml:"mount_gh_config"`
	GithubToken          string                `yaml:"github_token"`
	PreferredAgent       string                `yaml:"preferred_agent"`
	AgentFrameworks      AgentFrameworksConfig `yaml:"agent_frameworks"`
	ContainerEnvVars     map[string]string     `yaml:"container_env_vars"`
}

type AgentFrameworksConfig struct {
	OpenCode FrameworkConfig `yaml:"opencode"`
	Copilot  FrameworkConfig `yaml:"copilot"`
}

type FrameworkConfig struct {
	Enabled bool     `yaml:"enabled"`
	Plugins []string `yaml:"plugins"`
}

func getGlobalConfigPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, ".config", "agentjail", "config.yaml"), nil
}

func loadGlobalConfig() (*GlobalConfig, error) {
	configPath, err := getGlobalConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		config := &GlobalConfig{
			DefaultEditor:        "micro",
			DefaultShell:         "zsh",
			MountSystemGitconfig: true,
			MountGhConfig:        true,
			AgentFrameworks: AgentFrameworksConfig{
				Copilot: FrameworkConfig{
					Enabled: true,
				},
			},
		}
		if err := saveGlobalConfig(config); err != nil {
			return nil, err
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config GlobalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func saveGlobalConfig(config *GlobalConfig) error {
	configPath, err := getGlobalConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// copyTemplateConfigs copies tool-specific configs from templates to .agentjail
func copyTemplateConfigs(agentJailDir string, config *GlobalConfig) error {
	// Always copy rovr
	rovrDir := filepath.Join(agentJailDir, "rovr")
	if err := os.MkdirAll(rovrDir, 0755); err != nil {
		return err
	}

	for _, file := range []string{"config.toml", "pins.json"} {
		content, err := templatesFS.ReadFile("templates/configs/rovr/" + file)
		if err != nil {
			continue // Might not exist
		}
		if err := os.WriteFile(filepath.Join(rovrDir, file), content, 0644); err != nil {
			return err
		}
	}

	// Copy opencode if enabled
	if config.AgentFrameworks.OpenCode.Enabled {
		opencodeDir := filepath.Join(agentJailDir, "opencode")
		if err := os.MkdirAll(opencodeDir, 0755); err != nil {
			return err
		}
		content, err := templatesFS.ReadFile("templates/configs/opencode/opencode.json")
		if err == nil {
			if err := os.WriteFile(filepath.Join(opencodeDir, "opencode.json"), content, 0644); err != nil {
				return err
			}
		}
	}

	// Copy copilot if enabled
	if config.AgentFrameworks.Copilot.Enabled {
		copilotDir := filepath.Join(agentJailDir, "copilot")
		if err := os.MkdirAll(copilotDir, 0755); err != nil {
			return err
		}
		// No templates for copilot yet, but we create the dir
	}

	return nil
}

// saveMetadata saves the container metadata to .agentjail/metadata.json
func saveMetadata(agentJailDir string, metadata *AgentJailMetadata) error {
	metadata.LastUsed = time.Now()

	metadataFile := filepath.Join(agentJailDir, "metadata.json")

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// loadMetadata loads existing metadata from .agentjail/metadata.json
func loadMetadata(agentJailDir string) (*AgentJailMetadata, error) {
	metadataFile := filepath.Join(agentJailDir, "metadata.json")

	data, err := os.ReadFile(metadataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No existing metadata
		}
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata AgentJailMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// updateGitignore updates .gitignore to ignore .agentjail folder
func updateGitignore(baseDir string) error {
	gitignoreFile := filepath.Join(baseDir, ".gitignore")

	gitignoreContent := ""
	if data, err := os.ReadFile(gitignoreFile); err == nil {
		gitignoreContent = string(data)
	}

	// Check if .agentjail is already ignored
	if strings.Contains(gitignoreContent, ".agentjail") {
		return nil // Already present
	}

	// Add .agentjail to gitignore
	newContent := gitignoreContent
	if newContent != "" && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += ".agentjail/\n"

	if err := os.WriteFile(gitignoreFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to update .gitignore: %w", err)
	}

	fmt.Println("Added .agentjail/ to .gitignore")
	return nil
}

// checkVersionUpdate checks if agentjail version changed and updates metadata
func checkVersionUpdate(agentJailDir string, currentVersion string) error {
	existingMetadata, err := loadMetadata(agentJailDir)
	if err != nil {
		return fmt.Errorf("failed to load existing metadata: %w", err)
	}

	if existingMetadata != nil && existingMetadata.AgentJailVersion != currentVersion {
		fmt.Printf("AgentJail version updated from %s to %s\n", existingMetadata.AgentJailVersion, currentVersion)
		existingMetadata.AgentJailVersion = currentVersion
		existingMetadata.LastUsed = time.Now()

		if err := saveMetadata(agentJailDir, existingMetadata); err != nil {
			return fmt.Errorf("failed to update metadata with new version: %w", err)
		}
	}

	return nil
}

// loadGlobalConfigFromPath loads a GlobalConfig from a specific file path.
func loadGlobalConfigFromPath(path string) (*GlobalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	var config GlobalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}
	return &config, nil
}

// printCleanConfig prints a clean, commented config template to stdout.
func printCleanConfig() {
	fmt.Print(`# AgentJail configuration file
# Default location: ~/.config/agentjail/config.yaml

# Default editor to use inside the container (e.g. micro, vim, nano)
default_editor: micro

# Default shell to use inside the container (bash or zsh)
default_shell: zsh

# Mount the host ~/.gitconfig into the container
mount_system_gitconfig: true

# Mount the host ~/.config/gh into the container (used for gh copilot auth)
mount_gh_config: true

# GitHub personal access token (optional; falls back to GH_TOKEN / GITHUB_TOKEN env vars)
github_token: ""

# Preferred agent to auto-start with -A. Must match an enabled agent framework name
# (e.g. "copilot" or "opencode"). Leave empty to be prompted when using -A.
preferred_agent: ""

# Agent framework settings
agent_frameworks:
  opencode:
    enabled: false
    plugins: []
  copilot:
    enabled: true
    plugins: []

# Environment variables to inject into the container.
# Supports two schemas:
#   CONT_VAR: value            # set to a literal value
#   CONT_VAR: env:HOST_VAR     # read from the host environment variable HOST_VAR
container_env_vars: {}
#   MY_TOKEN: env:MY_HOST_TOKEN
#   DEBUG: "true"
`)
}

// enabledAgents returns a list of agent names that are enabled in the config.
func enabledAgents(config *GlobalConfig) []string {
	var agents []string
	if config.AgentFrameworks.Copilot.Enabled {
		agents = append(agents, "copilot")
	}
	if config.AgentFrameworks.OpenCode.Enabled {
		agents = append(agents, "opencode")
	}
	return agents
}

// agentCommand returns the shell command string to launch the given agent.
func agentCommand(name string) string {
	switch name {
	case "opencode":
		return "opencode"
	case "copilot":
		return "copilot"
	default:
		return name
	}
}

// chooseEnabledAgent shows an interactive prompt and returns the chosen agent name.
func chooseEnabledAgent(config *GlobalConfig) string {
	agents := enabledAgents(config)
	if len(agents) == 0 {
		return ""
	}
	if len(agents) == 1 {
		return agents[0]
	}
	fmt.Println("Choose an agent to start:")
	for i, a := range agents {
		fmt.Printf("  %d. %s\n", i+1, a)
	}
	fmt.Print("Enter number: ")
	var choice int
	if _, err := fmt.Scan(&choice); err == nil && choice >= 1 && choice <= len(agents) {
		return agents[choice-1]
	}
	return agents[0]
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

	// 1. Ensure ./opencode.json exists (default behavior requirement) - OPTIONAL NOW
	// Only create if it doesn't exist and isn't provided via -C
	defaultConfigPath := filepath.Join(cwd, "opencode.json")
	if *configPtr == "opencode.json" {
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
