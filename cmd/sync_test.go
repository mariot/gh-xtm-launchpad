package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type commandInvocation struct {
	Dir  string
	Args []string
}

func TestSyncRepositoryClonesWhenMissing(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)
	invocations := stubRepositoryRunner(t, func(invocation commandInvocation) error {
		if len(invocation.Args) == 4 && invocation.Args[0] == "repo" && invocation.Args[1] == "clone" {
			target := invocation.Args[3]
			return os.MkdirAll(filepath.Join(target, ".git"), 0o755)
		}
		return nil
	})

	repository := repoTarget{Name: "connectors", Slug: "OpenCTI-Platform/connectors"}
	err := syncRepository(repository)
	if err != nil {
		t.Fatalf("syncRepository returned error: %v", err)
	}

	repositoryPath := filepath.Join(workspace, "repositories", "connectors")
	if len(*invocations) != 1 {
		t.Fatalf("expected one gh invocation, got %d", len(*invocations))
	}
	got := (*invocations)[0]
	if got.Dir != "" {
		t.Fatalf("expected clone to run from current working directory, got %q", got.Dir)
	}
	if len(got.Args) != 4 {
		t.Fatalf("expected 4 clone args, got %d (%v)", len(got.Args), got.Args)
	}
	wantPrefix := []string{"repo", "clone", "OpenCTI-Platform/connectors"}
	if !reflect.DeepEqual(got.Args[:3], wantPrefix) {
		t.Fatalf("unexpected clone args prefix: want %v, got %v", wantPrefix, got.Args[:3])
	}
	if !samePath(got.Args[3], repositoryPath) {
		t.Fatalf("unexpected clone target path: want %q, got %q", repositoryPath, got.Args[3])
	}

	if _, statErr := os.Stat(filepath.Join(repositoryPath, ".git")); statErr != nil {
		t.Fatalf("expected cloned repository to contain .git directory: %v", statErr)
	}
}

func TestSyncRepositoryFetchesWhenAlreadyCloned(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)
	invocations := stubRepositoryRunner(t, func(invocation commandInvocation) error {
		return nil
	})

	repositoryPath := filepath.Join(workspace, "repositories", "collectors")
	err := os.MkdirAll(filepath.Join(repositoryPath, ".git"), 0o755)
	if err != nil {
		t.Fatalf("create repository fixture: %v", err)
	}

	repository := repoTarget{Name: "collectors", Slug: "OpenAEV-Platform/collectors"}
	err = syncRepository(repository)
	if err != nil {
		t.Fatalf("syncRepository returned error: %v", err)
	}

	if len(*invocations) != 1 {
		t.Fatalf("expected one gh invocation, got %d", len(*invocations))
	}
	got := (*invocations)[0]
	if !samePath(got.Dir, repositoryPath) {
		t.Fatalf("expected sync to run in %q, got %q", repositoryPath, got.Dir)
	}
	wantArgs := []string{"repo", "sync", "--source", "OpenAEV-Platform/collectors"}
	if !reflect.DeepEqual(got.Args, wantArgs) {
		t.Fatalf("unexpected sync args: want %v, got %v", wantArgs, got.Args)
	}
}

func TestSyncRepositoryErrorsWhenDirectoryIsNotGitRepo(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)
	invocations := stubRepositoryRunner(t, func(invocation commandInvocation) error {
		return nil
	})

	repositoryPath := filepath.Join(workspace, "repositories", "collectors")
	err := os.MkdirAll(repositoryPath, 0o755)
	if err != nil {
		t.Fatalf("create repository fixture: %v", err)
	}

	repository := repoTarget{Name: "collectors", Slug: "OpenAEV-Platform/collectors"}
	err = syncRepository(repository)
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "exists but is not a git repository") {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(*invocations) != 0 {
		t.Fatalf("expected no gh command to run, got %d invocations", len(*invocations))
	}
}

func stubRepositoryRunner(t *testing.T, fn func(commandInvocation) error) *[]commandInvocation {
	t.Helper()

	originalRunner := runRepositoryCommand
	invocations := []commandInvocation{}
	runRepositoryCommand = func(dir string, args ...string) error {
		copiedArgs := make([]string, len(args))
		copy(copiedArgs, args)
		invocation := commandInvocation{Dir: dir, Args: copiedArgs}
		invocations = append(invocations, invocation)
		return fn(invocation)
	}
	t.Cleanup(func() {
		runRepositoryCommand = originalRunner
	})

	return &invocations
}

func setWorkingDir(t *testing.T, target string) {
	t.Helper()

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("get original working dir: %v", err)
	}
	err = os.Chdir(target)
	if err != nil {
		t.Fatalf("change working dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})
}

func samePath(left string, right string) bool {
	cleanLeft := filepath.Clean(left)
	cleanRight := filepath.Clean(right)
	return cleanLeft == cleanRight || cleanLeft == "/private"+cleanRight || "/private"+cleanLeft == cleanRight
}
