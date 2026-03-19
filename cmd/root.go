package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gh-xtm-launchpad",
	Short: "Fetch, build and run OpenCTI connectors and collectors",
	Long: `gh-xtm-launchpad is a GitHub CLI extension that keeps the OpenCTI
connectors and collectors repositories up to date, then builds and runs them.

Step 1: clone or fetch the connector/collector repos into the local
        repositories/ directory.`,
}

// Execute is the entrypoint called by main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gh-xtm-launchpad.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
