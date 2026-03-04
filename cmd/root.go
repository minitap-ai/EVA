package cmd

import (
	"fmt"
	"os"

	"github.com/minitap-ai/eva/cmd/branch"
	devopscmd "github.com/minitap-ai/eva/cmd/devops"
	initcmd "github.com/minitap-ai/eva/cmd/init"
	"github.com/minitap-ai/eva/cmd/open"
	"github.com/minitap-ai/eva/cmd/update"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "eva",
	Short: "🤖 EVA - CLI tool to automate dev workflows",
}

func Execute() {
	rootCmd.AddCommand(branch.NewCommand())
	rootCmd.AddCommand(open.NewCommand())
	rootCmd.AddCommand(update.NewCommand())
	rootCmd.AddCommand(initcmd.NewCommand())
	rootCmd.AddCommand(devopscmd.NewCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
