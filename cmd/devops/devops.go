package devops

import "github.com/spf13/cobra"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devops",
		Short: "DevOps utilities: release, env, secret, overview",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			ensureAWSAuth()
			ensureGCPAuth()
		},
	}

	cmd.AddCommand(newReleaseCommand())
	cmd.AddCommand(newEnvCommand())
	cmd.AddCommand(newSecretCommand())
	cmd.AddCommand(newOverviewCommand())

	return cmd
}
