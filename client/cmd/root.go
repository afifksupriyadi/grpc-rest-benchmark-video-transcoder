// Package cmd contains the Cobra command definitions for the client CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the base command when the CLI is invoked without a subcommand.
var rootCmd = &cobra.Command{
	Use:   "client",
	Short: "CLI client for benchmarking REST and gRPC video transcoding",
	Long: `client sends a video file to the Gateway service for transcoding,
either via REST or gRPC, and measures end-to-end timing for research purposes.`,
}

// Execute runs the root command. This is the single entry point called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
