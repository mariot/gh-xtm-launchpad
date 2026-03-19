package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type resolvedBuildTarget struct {
	Kind       string
	Name       string
	Path       string
	Dockerfile string
	RepoRoot   string
}

var runBuildCommand = runDockerCommand
var runBuildGitCommand = runGitForBuild
var buildBranch string
var buildPR int

var buildCmd = &cobra.Command{
	Use:   "build <target>",
	Short: "Build a connector or collector Docker image",
	Long: `Builds a Docker image from an explicit target path under repositories/.

The target must start with collectors/ or connectors/ and point to a directory
containing a Dockerfile.

Example:
  go run . build collectors/crowdstrike
  go run . build collectors/crowdstrike --branch master
  go run . build collectors/crowdstrike --pr 123
  go run . build connectors/external-import/crowdstrike --pull-request 123`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := buildRepositoryTarget(args[0], buildBranch, buildPR); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().StringVar(&buildBranch, "branch", "", "branch to checkout before building (example: master)")
	buildCmd.Flags().IntVar(&buildPR, "pr", 0, "pull request number to checkout before building")
	buildCmd.Flags().IntVar(&buildPR, "pull-request", 0, "pull request number to checkout before building")

	// ...existing code...
}

func buildRepositoryTarget(targetRef string, branch string, pr int) error {
	target, err := resolveBuildTarget(targetRef)
	if err != nil {
		return err
	}
	if branch != "" && pr > 0 {
		return fmt.Errorf("--branch and --pr/--pull-request cannot be used together")
	}
	if pr < 0 {
		return fmt.Errorf("invalid pull request number %d", pr)
	}

	if branch != "" {
		if err = checkoutOriginBranch(target, branch); err != nil {
			return err
		}
	}
	if pr > 0 {
		if err = checkoutPullRequest(target, pr); err != nil {
			return err
		}
	}

	tag := buildImageTag(target)
	fmt.Printf("building %s from %s as %s\n", target.Name, target.Path, tag)

	return runBuildCommand("", "build", "-t", tag, "-f", target.Dockerfile, target.Path)
}

func checkoutOriginBranch(target resolvedBuildTarget, branch string) error {
	fmt.Printf("updating %s and checking out origin/%s\n", target.RepoRoot, branch)
	if err := runBuildGitCommand(target.RepoRoot, "fetch", "--all", "--prune", "--tags"); err != nil {
		return err
	}
	if err := runBuildGitCommand(target.RepoRoot, "checkout", "--detach", "origin/"+branch); err != nil {
		return err
	}
	return nil
}

func checkoutPullRequest(target resolvedBuildTarget, pr int) error {
	fmt.Printf("updating %s and checking out origin pull request %d\n", target.RepoRoot, pr)
	if err := runBuildGitCommand(target.RepoRoot, "fetch", "--all", "--prune", "--tags"); err != nil {
		return err
	}
	if err := runBuildGitCommand(target.RepoRoot, "fetch", "origin", fmt.Sprintf("pull/%d/head", pr)); err != nil {
		return err
	}
	if err := runBuildGitCommand(target.RepoRoot, "checkout", "--detach", "FETCH_HEAD"); err != nil {
		return err
	}
	return nil
}

func resolveBuildTarget(targetRef string) (resolvedBuildTarget, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return resolvedBuildTarget{}, fmt.Errorf("resolve working directory: %w", err)
	}

	rawTarget := filepath.ToSlash(targetRef)
	if strings.HasPrefix(rawTarget, "/") {
		return resolvedBuildTarget{}, fmt.Errorf("invalid target %q: use collectors/... or connectors/...", targetRef)
	}
	for _, segment := range strings.Split(rawTarget, "/") {
		if segment == ".." {
			return resolvedBuildTarget{}, fmt.Errorf("invalid target %q: path traversal is not allowed", targetRef)
		}
	}

	normalizedTarget := filepath.ToSlash(filepath.Clean(rawTarget))
	if normalizedTarget == "." || strings.HasPrefix(normalizedTarget, "../") {
		return resolvedBuildTarget{}, fmt.Errorf("invalid target %q: use collectors/... or connectors/...", targetRef)
	}

	parts := strings.Split(normalizedTarget, "/")
	if len(parts) < 2 {
		return resolvedBuildTarget{}, fmt.Errorf("invalid target %q: use collectors/... or connectors/...", targetRef)
	}

	kind := ""
	switch parts[0] {
	case "connectors":
		kind = "connector"
	case "collectors":
		kind = "collector"
	default:
		return resolvedBuildTarget{}, fmt.Errorf("invalid target %q: use collectors/... or connectors/...", targetRef)
	}

	repositoriesDir := filepath.Join(workingDir, "repositories")
	repositoryPath := filepath.Join(repositoriesDir, filepath.FromSlash(normalizedTarget))
	repositoryInfo, repositoryErr := os.Stat(repositoryPath)
	if repositoryErr != nil {
		if os.IsNotExist(repositoryErr) {
			return resolvedBuildTarget{}, fmt.Errorf("build target %q was not found at %s", targetRef, repositoryPath)
		}
		return resolvedBuildTarget{}, fmt.Errorf("inspect %s: %w", repositoryPath, repositoryErr)
	}
	if !repositoryInfo.IsDir() {
		return resolvedBuildTarget{}, fmt.Errorf("build target %q is not a directory", targetRef)
	}

	dockerfile := filepath.Join(repositoryPath, "Dockerfile")
	dockerfileInfo, dockerfileErr := os.Stat(dockerfile)
	if dockerfileErr != nil {
		if os.IsNotExist(dockerfileErr) {
			return resolvedBuildTarget{}, fmt.Errorf("build target %q does not contain a Dockerfile", targetRef)
		}
		return resolvedBuildTarget{}, fmt.Errorf("inspect %s: %w", dockerfile, dockerfileErr)
	}
	if dockerfileInfo.IsDir() {
		return resolvedBuildTarget{}, fmt.Errorf("build target %q has an invalid Dockerfile path", targetRef)
	}

	return resolvedBuildTarget{
		Kind:       kind,
		Name:       parts[len(parts)-1],
		Path:       repositoryPath,
		Dockerfile: dockerfile,
		RepoRoot:   filepath.Join(repositoriesDir, parts[0]),
	}, nil
}

func buildImageTag(target resolvedBuildTarget) string {
	return fmt.Sprintf("gh-xtm-launchpad/%s-%s:latest", target.Kind, sanitizeForTag(target.Name))
}

func sanitizeForTag(value string) string {
	lower := strings.ToLower(value)
	b := strings.Builder{}
	b.Grow(len(lower))
	for _, ch := range lower {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '.' || ch == '_' || ch == '-' {
			b.WriteRune(ch)
			continue
		}
		b.WriteByte('-')
	}
	return b.String()
}

func runDockerCommand(dir string, args ...string) error {
	cmd := exec.Command("docker", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker %s failed: %w", strings.Join(args, " "), err)
	}

	return nil

}

func runGitForBuild(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}

	return nil
}
