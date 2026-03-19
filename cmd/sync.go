package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gh "github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Clone or fetch the connectors and collectors repositories",
	Long: `Ensures the OpenCTI connectors and collectors repositories are available locally.

If a repository has not been cloned yet, it is cloned from GitHub into the
local repositories/ directory. If it already exists, it is synced from the
source GitHub repository using gh.

Repositories managed:
  - OpenCTI-Platform/connectors  -> repositories/connectors
  - OpenAEV-Platform/collectors  -> repositories/collectors`,
	Run: func(cmd *cobra.Command, args []string) {
		repos := []repoTarget{
			{Name: "connectors", Slug: "OpenCTI-Platform/connectors"},
			{Name: "collectors", Slug: "OpenAEV-Platform/collectors"},
		}

		for _, repo := range repos {
			if err := syncRepository(repo); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

		client, err := api.DefaultRESTClient()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		response := struct{ Login string }{}
		if err = client.Get("user", &response); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("running as %s\n", response.Login)
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// syncCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// syncCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

type repoTarget struct {
	Name string
	Slug string
}

var runRepositoryCommand = runGHCommand

func syncRepository(repository repoTarget) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	repositoriesDir := filepath.Join(workingDir, "repositories")
	repositoryPath := filepath.Join(repositoriesDir, repository.Name)
	gitPath := filepath.Join(repositoryPath, ".git")

	repositoryInfo, repositoryErr := os.Stat(repositoryPath)
	if repositoryErr == nil {
		if !repositoryInfo.IsDir() {
			return fmt.Errorf("%s exists but is not a directory", repositoryPath)
		}

		gitInfo, gitErr := os.Stat(gitPath)
		if gitErr != nil {
			if os.IsNotExist(gitErr) {
				return fmt.Errorf("%s exists but is not a git repository", repositoryPath)
			}
			return fmt.Errorf("inspect %s: %w", gitPath, gitErr)
		}
		if !gitInfo.IsDir() {
			return fmt.Errorf("%s is not a directory", gitPath)
		}

		fmt.Printf("found %s, fetching updates...\n", repository.Slug)
		return runRepositoryCommand(repositoryPath, "repo", "sync", "--source", repository.Slug)
	}
	if !os.IsNotExist(repositoryErr) {
		return fmt.Errorf("inspect %s: %w", repositoryPath, repositoryErr)
	}

	fmt.Printf("%s not found, cloning...\n", repository.Slug)
	if err = os.MkdirAll(repositoriesDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", repositoriesDir, err)
	}
	return runRepositoryCommand("", "repo", "clone", repository.Slug, repositoryPath)
}

func runGHCommand(dir string, args ...string) error {
	originalDir := ""
	if dir != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve working directory: %w", err)
		}
		originalDir = cwd
		if err = os.Chdir(dir); err != nil {
			return fmt.Errorf("change directory to %s: %w", dir, err)
		}
		defer func() {
			_ = os.Chdir(originalDir)
		}()
	}

	stdout, stderr, err := gh.Exec(args...)
	if stdout.Len() > 0 {
		fmt.Print(stdout.String())
	}
	if stderr.Len() > 0 {
		fmt.Fprint(os.Stderr, stderr.String())
	}
	if err != nil {
		return fmt.Errorf("gh %s failed: %w", strings.Join(args, " "), err)
	}
	return nil
}
