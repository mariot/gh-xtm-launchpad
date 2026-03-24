package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Environment any `yaml:"environment"`
}

type envRequirement struct {
	Key        string
	Default    string
	HasDefault bool
}

var runContainerCommand = runDockerCommand
var runPromptInput io.Reader = os.Stdin
var runPromptOutput io.Writer = os.Stdout
var runEnvFile string

var runCmd = &cobra.Command{
	Use:   "run <target>",
	Short: "Run a built connector or collector Docker image",
	Long: `Runs a previously built image for an explicit target path under repositories/.

The command reads docker-compose.yml in the target directory, prompts for every
environment variable listed there, then starts the image with docker run.

Example:
  go run . run collectors/crowdstrike
  go run . run connectors/external-import/crowdstrike`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runRepositoryTarget(args[0], runEnvFile); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&runEnvFile, "env-file", ".env", "env file to load/store values for the target (default: .env)")
}

func runRepositoryTarget(targetRef string, envFileRef string) error {
	target, err := resolveBuildTarget(targetRef)
	if err != nil {
		return err
	}

	composePath := filepath.Join(target.Path, "docker-compose.yml")
	requirements, err := readComposeEnvRequirements(composePath)
	if err != nil {
		return err
	}

	envFilePath := resolveEnvFilePath(target.Path, envFileRef)
	existingValues, err := readEnvFile(envFilePath)
	if err != nil {
		return err
	}

	envValues, err := resolveEnvValues(requirements, existingValues)
	if err != nil {
		return err
	}

	if err = writeEnvFile(envFilePath, requirements, envValues, existingValues); err != nil {
		return err
	}

	tag := buildImageTag(target)
	fmt.Printf("running %s from %s\n", tag, target.Path)

	args := []string{"run", "--rm"}
	for _, requirement := range requirements {
		args = append(args, "-e", requirement.Key+"="+envValues[requirement.Key])
	}
	args = append(args, tag)

	return runContainerCommand("", args...)
}

func resolveEnvFilePath(targetPath string, envFileRef string) string {
	if envFileRef == "" {
		envFileRef = ".env"
	}
	if filepath.IsAbs(envFileRef) {
		return envFileRef
	}
	return filepath.Join(targetPath, envFileRef)
}

func readEnvFile(envFilePath string) (map[string]string, error) {
	data, err := os.ReadFile(envFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(filepath.Dir(envFilePath), 0o755); mkErr != nil {
				return nil, fmt.Errorf("create env file directory %s: %w", filepath.Dir(envFilePath), mkErr)
			}
			if writeErr := os.WriteFile(envFilePath, []byte(""), 0o644); writeErr != nil {
				return nil, fmt.Errorf("create env file %s: %w", envFilePath, writeErr)
			}
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read env file %s: %w", envFilePath, err)
	}

	values := map[string]string{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		value := ""
		if len(parts) == 2 {
			value = strings.TrimSpace(parts[1])
		}
		values[key] = parseLiteralDefault(value)
	}

	return values, nil
}

func writeEnvFile(envFilePath string, requirements []envRequirement, resolvedValues map[string]string, existingValues map[string]string) error {
	requiredKeys := map[string]bool{}
	lines := []string{}

	for _, requirement := range requirements {
		requiredKeys[requirement.Key] = true
		lines = append(lines, requirement.Key+"="+resolvedValues[requirement.Key])
	}

	extraKeys := make([]string, 0, len(existingValues))
	for key := range existingValues {
		if requiredKeys[key] {
			continue
		}
		extraKeys = append(extraKeys, key)
	}
	sort.Strings(extraKeys)
	for _, key := range extraKeys {
		lines = append(lines, key+"="+existingValues[key])
	}

	content := ""
	if len(lines) > 0 {
		content = strings.Join(lines, "\n") + "\n"
	}

	if err := os.WriteFile(envFilePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write env file %s: %w", envFilePath, err)
	}
	return nil
}

func readComposeEnvRequirements(composePath string) ([]envRequirement, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s was not found", composePath)
		}
		return nil, fmt.Errorf("read %s: %w", composePath, err)
	}

	parsed := composeFile{}
	if err = yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", composePath, err)
	}

	seen := map[string]bool{}
	requirements := []envRequirement{}
	for _, service := range parsed.Services {
		serviceRequirements, serviceErr := readServiceEnvRequirements(service)
		if serviceErr != nil {
			return nil, fmt.Errorf("parse environment in %s: %w", composePath, serviceErr)
		}
		for _, requirement := range serviceRequirements {
			if requirement.Key == "" || seen[requirement.Key] {
				continue
			}
			seen[requirement.Key] = true
			requirements = append(requirements, requirement)
		}
	}

	return requirements, nil
}

func readServiceEnvRequirements(service composeService) ([]envRequirement, error) {
	switch raw := service.Environment.(type) {
	case nil:
		return nil, nil
	case []any:
		requirements := []envRequirement{}
		for _, item := range raw {
			entry, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("unsupported list environment entry type %T", item)
			}
			requirement, err := parseEnvEntry(entry)
			if err != nil {
				return nil, err
			}
			requirements = append(requirements, requirement)
		}
		return requirements, nil
	case map[string]any:
		requirements := []envRequirement{}
		for key, value := range raw {
			requirement := envRequirement{Key: strings.TrimSpace(key)}
			if value != nil {
				requirement.Default = parseLiteralDefault(fmt.Sprint(value))
				requirement.HasDefault = true
			}
			requirements = append(requirements, requirement)
		}
		return requirements, nil
	default:
		return nil, fmt.Errorf("unsupported environment type %T", service.Environment)
	}
}

func parseEnvEntry(entry string) (envRequirement, error) {
	parts := strings.SplitN(strings.TrimSpace(entry), "=", 2)
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return envRequirement{}, fmt.Errorf("invalid environment entry %q", entry)
	}
	if len(parts) == 1 {
		return envRequirement{Key: key}, nil
	}

	defaultValue, hasDefault := parseComposeDefault(parts[1])
	return envRequirement{
		Key:        key,
		Default:    defaultValue,
		HasDefault: hasDefault,
	}, nil
}

func parseComposeDefault(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		body := value[2 : len(value)-1]
		if index := strings.Index(body, ":-"); index >= 0 {
			return parseLiteralDefault(body[index+2:]), true
		}
		if index := strings.Index(body, "-"); index >= 0 {
			return parseLiteralDefault(body[index+1:]), true
		}
		return "", false
	}
	return parseLiteralDefault(value), true
}

func parseLiteralDefault(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 2 {
		if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
			return strings.Trim(trimmed, "\"")
		}
		if strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'") {
			return strings.Trim(trimmed, "'")
		}
	}
	return trimmed
}

func resolveEnvValues(requirements []envRequirement, existingValues map[string]string) (map[string]string, error) {
	envValues := map[string]string{}
	scanner := bufio.NewScanner(runPromptInput)

	for _, requirement := range requirements {
		if currentValue, hasValue := existingValues[requirement.Key]; hasValue {
			envValues[requirement.Key] = currentValue
			continue
		}

		prompt := requirement.Key + ": "
		if requirement.HasDefault {
			prompt = fmt.Sprintf("%s [default: %s]: ", requirement.Key, requirement.Default)
		}
		if _, err := fmt.Fprint(runPromptOutput, prompt); err != nil {
			return nil, fmt.Errorf("write prompt: %w", err)
		}

		value := ""
		if scanner.Scan() {
			value = strings.TrimSpace(scanner.Text())
		} else if scanErr := scanner.Err(); scanErr != nil {
			return nil, fmt.Errorf("read input: %w", scanErr)
		}
		if value == "" && requirement.HasDefault {
			value = requirement.Default
		}
		envValues[requirement.Key] = value
	}

	return envValues, nil
}
