package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version information
	version = "0.1.0"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "qa",
	Short: "DreamUp QA Agent - Automated game testing tool",
	Long: `DreamUp QA Agent is a browser automation tool for testing web-based games.
It uses Chrome DevTools Protocol to interact with games, capture screenshots,
and evaluate game functionality using LLM analysis.`,
	Version: version,
}

func init() {
	// Add subcommands here
	rootCmd.AddCommand(testCmd)
}
