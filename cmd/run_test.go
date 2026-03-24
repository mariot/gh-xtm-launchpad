package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadComposeEnvRequirementsParsesListEntries(t *testing.T) {
	workspace := t.TempDir()
	composePath := filepath.Join(workspace, "docker-compose.yml")
	content := `services:
  test:
    environment:
      - REQUIRED=${REQUIRED}
      - WITH_DEFAULT=${WITH_DEFAULT:-value}
      - LITERAL=abc
      - EMPTY=
`
	if err := os.WriteFile(composePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	requirements, err := readComposeEnvRequirements(composePath)
	if err != nil {
		t.Fatalf("readComposeEnvRequirements returned error: %v", err)
	}

	want := []envRequirement{
		{Key: "REQUIRED", Default: "", HasDefault: false},
		{Key: "WITH_DEFAULT", Default: "value", HasDefault: true},
		{Key: "LITERAL", Default: "abc", HasDefault: true},
		{Key: "EMPTY", Default: "", HasDefault: true},
	}
	if !reflect.DeepEqual(requirements, want) {
		t.Fatalf("unexpected requirements: want %#v, got %#v", want, requirements)
	}
}

func TestRunRepositoryTargetPromptsAndRunsDocker(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)

	targetPath := filepath.Join(workspace, "repositories", "collectors", "crowdstrike")
	writeDockerfile(t, targetPath)
	compose := `services:
  collector-crowdstrike:
    environment:
      - OPENAEV_URL=${OPENAEV_URL}
      - COLLECTOR_PERIOD=${COLLECTOR_PERIOD:-PT1M}
`
	if err := os.WriteFile(filepath.Join(targetPath, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatalf("write docker-compose file: %v", err)
	}

	originalInput := runPromptInput
	originalOutput := runPromptOutput
	originalRunner := runContainerCommand
	t.Cleanup(func() {
		runPromptInput = originalInput
		runPromptOutput = originalOutput
		runContainerCommand = originalRunner
	})

	runPromptInput = strings.NewReader("https://openaev.local\n\n")
	promptOutput := &bytes.Buffer{}
	runPromptOutput = promptOutput

	invocations := []commandInvocation{}
	runContainerCommand = func(dir string, args ...string) error {
		copiedArgs := make([]string, len(args))
		copy(copiedArgs, args)
		invocations = append(invocations, commandInvocation{Dir: dir, Args: copiedArgs})
		return nil
	}

	err := runRepositoryTarget("collectors/crowdstrike", ".env")
	if err != nil {
		t.Fatalf("runRepositoryTarget returned error: %v", err)
	}

	if !strings.Contains(promptOutput.String(), "OPENAEV_URL:") {
		t.Fatalf("expected OPENAEV_URL prompt, got %q", promptOutput.String())
	}
	if !strings.Contains(promptOutput.String(), "COLLECTOR_PERIOD [default: PT1M]:") {
		t.Fatalf("expected COLLECTOR_PERIOD prompt, got %q", promptOutput.String())
	}

	if len(invocations) != 1 {
		t.Fatalf("expected one docker invocation, got %d", len(invocations))
	}
	envPath := filepath.Join(targetPath, ".env")
	gotArgs := invocations[0].Args
	if len(gotArgs) != 5 {
		t.Fatalf("unexpected docker arg count: want 5, got %d (%v)", len(gotArgs), gotArgs)
	}
	if gotArgs[0] != "run" || gotArgs[1] != "--rm" || gotArgs[2] != "--env-file" || gotArgs[4] != "gh-xtm-launchpad/collector-crowdstrike:latest" {
		t.Fatalf("unexpected docker args: %v", gotArgs)
	}
	if !samePath(gotArgs[3], envPath) {
		t.Fatalf("unexpected env-file path: want %q, got %q", envPath, gotArgs[3])
	}
	for _, arg := range gotArgs {
		if strings.Contains(arg, "OPENAEV_TOKEN") || strings.Contains(arg, "OPENAEV_URL=") {
			t.Fatalf("expected secrets to stay in env file, got argument %q", arg)
		}
	}

	envData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	envContent := string(envData)
	if !strings.Contains(envContent, "OPENAEV_URL=https://openaev.local") {
		t.Fatalf("expected OPENAEV_URL in env file, got %q", envContent)
	}
	if !strings.Contains(envContent, "COLLECTOR_PERIOD=PT1M") {
		t.Fatalf("expected COLLECTOR_PERIOD in env file, got %q", envContent)
	}
}

func TestRunRepositoryTargetFailsWhenComposeMissing(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)

	targetPath := filepath.Join(workspace, "repositories", "collectors", "crowdstrike")
	writeDockerfile(t, targetPath)

	err := runRepositoryTarget("collectors/crowdstrike", ".env")
	if err == nil {
		t.Fatal("expected missing compose error, got nil")
	}
	if !strings.Contains(err.Error(), "docker-compose.yml") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRepositoryTargetLoadsValuesFromEnvFile(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)

	targetPath := filepath.Join(workspace, "repositories", "collectors", "crowdstrike")
	writeDockerfile(t, targetPath)
	compose := `services:
  collector-crowdstrike:
    environment:
      - OPENAEV_URL=${OPENAEV_URL}
      - OPENAEV_TOKEN=${OPENAEV_TOKEN}
`
	if err := os.WriteFile(filepath.Join(targetPath, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatalf("write docker-compose file: %v", err)
	}

	envPath := filepath.Join(targetPath, "collector.env")
	if err := os.WriteFile(envPath, []byte("OPENAEV_URL=https://loaded.example\nOPENAEV_TOKEN=secret\n"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	originalInput := runPromptInput
	originalOutput := runPromptOutput
	originalRunner := runContainerCommand
	t.Cleanup(func() {
		runPromptInput = originalInput
		runPromptOutput = originalOutput
		runContainerCommand = originalRunner
	})

	runPromptInput = strings.NewReader("")
	promptOutput := &bytes.Buffer{}
	runPromptOutput = promptOutput

	invocations := []commandInvocation{}
	runContainerCommand = func(dir string, args ...string) error {
		copiedArgs := make([]string, len(args))
		copy(copiedArgs, args)
		invocations = append(invocations, commandInvocation{Dir: dir, Args: copiedArgs})
		return nil
	}

	err := runRepositoryTarget("collectors/crowdstrike", "collector.env")
	if err != nil {
		t.Fatalf("runRepositoryTarget returned error: %v", err)
	}

	if promptOutput.Len() != 0 {
		t.Fatalf("expected no prompts when env file contains values, got %q", promptOutput.String())
	}

	gotArgs := invocations[0].Args
	if len(gotArgs) != 5 {
		t.Fatalf("unexpected docker arg count: want 5, got %d (%v)", len(gotArgs), gotArgs)
	}
	if gotArgs[0] != "run" || gotArgs[1] != "--rm" || gotArgs[2] != "--env-file" || gotArgs[4] != "gh-xtm-launchpad/collector-crowdstrike:latest" {
		t.Fatalf("unexpected docker args: %v", gotArgs)
	}
	if !samePath(gotArgs[3], envPath) {
		t.Fatalf("unexpected env-file path: want %q, got %q", envPath, gotArgs[3])
	}
	for _, arg := range gotArgs {
		if strings.Contains(arg, "OPENAEV_TOKEN") || strings.Contains(arg, "OPENAEV_URL=") {
			t.Fatalf("expected secrets to stay in env file, got argument %q", arg)
		}
	}
}
