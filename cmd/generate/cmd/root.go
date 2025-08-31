package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate code for Agent Client Protocol Go library",
	Long:  `A code generation tool that creates Go types and constants from the Agent Client Protocol JSON schema.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
