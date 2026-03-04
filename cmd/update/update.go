package update

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update eva to the latest version (via install.sh)",
		Run: func(cmd *cobra.Command, args []string) {
			installURL := "https://raw.githubusercontent.com/minitap-ai/eva/main/install.sh"
			fmt.Println("⬇️  Updating eva via:", installURL)

			c := exec.Command("sh", "-c", fmt.Sprintf("curl -sSfL %s | sh", installURL))
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Stdin = os.Stdin

			if err := c.Run(); err != nil {
				fmt.Println("❌ Update failed:", err)
				os.Exit(1)
			}

			fmt.Println("✅ eva updated successfully.")
		},
	}
}
