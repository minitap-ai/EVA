package branch

import (
	"fmt"
	"os"

	"github.com/minitap-ai/eva/internal/config"
	"github.com/minitap-ai/eva/internal/git"
	"github.com/minitap-ai/eva/internal/notion"
	"github.com/minitap-ai/eva/internal/utils"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "branch <TICKET_ID>",
		Short: "Create a Git branch from a Notion ticket",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ticketID := args[0]

			cfg, err := config.Load()
			if err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}
			if err := cfg.RequireNotion(); err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}

			ticketNumber := utils.ExtractTicketNumber(ticketID)

			task, err := notion.GetTaskTitle(ticketNumber, cfg)
			if err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}

			baseBranch := git.GetBaseBranch()
			branchName := fmt.Sprintf("TASK-%d/%s", ticketNumber, utils.FormatTitle(task.Title))

			if err := git.CreateAndPushBranch(branchName, baseBranch); err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}

			if err := notion.SetTaskStatusToDoing(task.PageID, cfg); err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}

			fmt.Println("✅ Branch created and task moved to Doing!")
		},
	}
}
