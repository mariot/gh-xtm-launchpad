package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveBuildTargetFindsConnector(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)

	targetPath := filepath.Join(workspace, "repositories", "connectors", "external-import", "crowdstrike")
	writeDockerfile(t, targetPath)

	target, err := resolveBuildTarget("connectors/external-import/crowdstrike")
	if err != nil {
		t.Fatalf("resolveBuildTarget returned error: %v", err)
	}
	if target.Kind != "connector" {
		t.Fatalf("expected kind connector, got %q", target.Kind)
	}
	if target.Name != "crowdstrike" {
		t.Fatalf("expected name crowdstrike, got %q", target.Name)
	}
	if !samePath(target.Path, targetPath) {
		t.Fatalf("expected path %q, got %q", targetPath, target.Path)
	}
	if !samePath(target.Dockerfile, filepath.Join(targetPath, "Dockerfile")) {
		t.Fatalf("unexpected Dockerfile path %q", target.Dockerfile)
	}
}

func TestResolveBuildTargetRejectsInvalidPrefix(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)

	_, err := resolveBuildTarget("sources/crowdstrike")
	if err == nil {
		t.Fatal("expected invalid target error, got nil")
	}
	if !strings.Contains(err.Error(), "use collectors/... or connectors/...") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveBuildTargetRejectsTraversal(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)

	_, err := resolveBuildTarget("collectors/../crowdstrike")
	if err == nil {
		t.Fatal("expected traversal error, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal is not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRepositoryTargetRunsDockerBuild(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)

	targetPath := filepath.Join(workspace, "repositories", "collectors", "crowdstrike")
	dockerfilePath := writeDockerfile(t, targetPath)

	invocations := stubBuildRunner(t, func(invocation commandInvocation) error {
		return nil
	})

	err := buildRepositoryTarget("collectors/crowdstrike", "", 0)
	if err != nil {
		t.Fatalf("buildRepositoryTarget returned error: %v", err)
	}

	if len(*invocations) != 1 {
		t.Fatalf("expected one docker invocation, got %d", len(*invocations))
	}
	got := (*invocations)[0]
	if got.Dir != "" {
		t.Fatalf("expected docker command to run from current working directory, got %q", got.Dir)
	}
	if len(got.Args) != 6 {
		t.Fatalf("expected 6 docker args, got %d (%v)", len(got.Args), got.Args)
	}
	wantPrefix := []string{"build", "-t", "gh-xtm-launchpad/collector-crowdstrike:latest", "-f"}
	if !reflect.DeepEqual(got.Args[:4], wantPrefix) {
		t.Fatalf("unexpected docker args prefix: want %v, got %v", wantPrefix, got.Args[:4])
	}
	if !samePath(got.Args[4], dockerfilePath) {
		t.Fatalf("unexpected Dockerfile path: want %q, got %q", dockerfilePath, got.Args[4])
	}
	if !samePath(got.Args[5], targetPath) {
		t.Fatalf("unexpected context path: want %q, got %q", targetPath, got.Args[5])
	}
}

func TestBuildRepositoryTargetWithBranchFetchesAndChecksOutThenBuilds(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)

	targetPath := filepath.Join(workspace, "repositories", "connectors", "external-import", "crowdstrike")
	writeDockerfile(t, targetPath)

	gitInvocations := stubBuildGitRunner(t, func(invocation commandInvocation) error {
		return nil
	})
	dockerInvocations := stubBuildRunner(t, func(invocation commandInvocation) error {
		return nil
	})

	err := buildRepositoryTarget("connectors/external-import/crowdstrike", "fix/159-add-docs", 0)
	if err != nil {
		t.Fatalf("buildRepositoryTarget returned error: %v", err)
	}

	repoRoot := filepath.Join(workspace, "repositories", "connectors")
	if len(*gitInvocations) != 2 {
		t.Fatalf("expected two git invocations, got %d", len(*gitInvocations))
	}
	if !samePath((*gitInvocations)[0].Dir, repoRoot) {
		t.Fatalf("expected git fetch in %q, got %q", repoRoot, (*gitInvocations)[0].Dir)
	}
	wantFetch := []string{"fetch", "--all", "--prune", "--tags"}
	if !reflect.DeepEqual((*gitInvocations)[0].Args, wantFetch) {
		t.Fatalf("unexpected git fetch args: want %v, got %v", wantFetch, (*gitInvocations)[0].Args)
	}
	if !samePath((*gitInvocations)[1].Dir, repoRoot) {
		t.Fatalf("expected git checkout in %q, got %q", repoRoot, (*gitInvocations)[1].Dir)
	}
	wantCheckout := []string{"checkout", "--detach", "origin/fix/159-add-docs"}
	if !reflect.DeepEqual((*gitInvocations)[1].Args, wantCheckout) {
		t.Fatalf("unexpected git checkout args: want %v, got %v", wantCheckout, (*gitInvocations)[1].Args)
	}

	if len(*dockerInvocations) != 1 {
		t.Fatalf("expected one docker invocation, got %d", len(*dockerInvocations))
	}
}

func TestBuildRepositoryTargetWithPRFetchesAndChecksOutThenBuilds(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)

	targetPath := filepath.Join(workspace, "repositories", "collectors", "crowdstrike")
	writeDockerfile(t, targetPath)

	gitInvocations := stubBuildGitRunner(t, func(invocation commandInvocation) error {
		return nil
	})
	dockerInvocations := stubBuildRunner(t, func(invocation commandInvocation) error {
		return nil
	})

	err := buildRepositoryTarget("collectors/crowdstrike", "", 123)
	if err != nil {
		t.Fatalf("buildRepositoryTarget returned error: %v", err)
	}

	repoRoot := filepath.Join(workspace, "repositories", "collectors")
	if len(*gitInvocations) != 3 {
		t.Fatalf("expected three git invocations, got %d", len(*gitInvocations))
	}
	wantFetchAll := []string{"fetch", "--all", "--prune", "--tags"}
	if !reflect.DeepEqual((*gitInvocations)[0].Args, wantFetchAll) {
		t.Fatalf("unexpected git fetch args: want %v, got %v", wantFetchAll, (*gitInvocations)[0].Args)
	}
	wantFetchPR := []string{"fetch", "origin", "pull/123/head"}
	if !reflect.DeepEqual((*gitInvocations)[1].Args, wantFetchPR) {
		t.Fatalf("unexpected git fetch pr args: want %v, got %v", wantFetchPR, (*gitInvocations)[1].Args)
	}
	wantCheckout := []string{"checkout", "--detach", "FETCH_HEAD"}
	if !reflect.DeepEqual((*gitInvocations)[2].Args, wantCheckout) {
		t.Fatalf("unexpected git checkout args: want %v, got %v", wantCheckout, (*gitInvocations)[2].Args)
	}
	for i, invocation := range *gitInvocations {
		if !samePath(invocation.Dir, repoRoot) {
			t.Fatalf("expected git invocation %d in %q, got %q", i, repoRoot, invocation.Dir)
		}
	}

	if len(*dockerInvocations) != 1 {
		t.Fatalf("expected one docker invocation, got %d", len(*dockerInvocations))
	}
}

func TestBuildRepositoryTargetRejectsBranchAndPRTogether(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDir(t, workspace)

	targetPath := filepath.Join(workspace, "repositories", "collectors", "crowdstrike")
	writeDockerfile(t, targetPath)

	err := buildRepositoryTarget("collectors/crowdstrike", "master", 123)
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildImageTagSanitizesName(t *testing.T) {
	target := resolvedBuildTarget{Kind: "connector", Name: "CrowdStrike Falcon"}
	tag := buildImageTag(target)
	if tag != "gh-xtm-launchpad/connector-crowdstrike-falcon:latest" {
		t.Fatalf("unexpected image tag %q", tag)
	}
}

func stubBuildRunner(t *testing.T, fn func(commandInvocation) error) *[]commandInvocation {
	t.Helper()

	originalRunner := runBuildCommand
	invocations := []commandInvocation{}
	runBuildCommand = func(dir string, args ...string) error {
		copiedArgs := make([]string, len(args))
		copy(copiedArgs, args)
		invocation := commandInvocation{Dir: dir, Args: copiedArgs}
		invocations = append(invocations, invocation)
		return fn(invocation)
	}
	t.Cleanup(func() {
		runBuildCommand = originalRunner
	})

	return &invocations
}

func stubBuildGitRunner(t *testing.T, fn func(commandInvocation) error) *[]commandInvocation {
	t.Helper()

	originalRunner := runBuildGitCommand
	invocations := []commandInvocation{}
	runBuildGitCommand = func(dir string, args ...string) error {
		copiedArgs := make([]string, len(args))
		copy(copiedArgs, args)
		invocation := commandInvocation{Dir: dir, Args: copiedArgs}
		invocations = append(invocations, invocation)
		return fn(invocation)
	}
	t.Cleanup(func() {
		runBuildGitCommand = originalRunner
	})

	return &invocations
}

func writeDockerfile(t *testing.T, targetPath string) string {
	t.Helper()

	err := os.MkdirAll(targetPath, 0o755)
	if err != nil {
		t.Fatalf("create target path: %v", err)
	}
	dockerfilePath := filepath.Join(targetPath, "Dockerfile")
	err = os.WriteFile(dockerfilePath, []byte("FROM scratch\n"), 0o644)
	if err != nil {
		t.Fatalf("write dockerfile: %v", err)
	}

	return dockerfilePath
}
