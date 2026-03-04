package devops

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/minitap-ai/eva/internal/config"
	"github.com/minitap-ai/eva/internal/devops"
	"github.com/spf13/cobra"
)

func newReleaseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "release [project]",
		Short: "Tag a new release for a project",
		Args:  cobra.ExactArgs(1),
		Run:   runRelease,
	}
}

func runRelease(cmd *cobra.Command, args []string) {
	slug := args[0]

	cfg, err := config.Load()
	if err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}

	project := resolveProject(cfg, slug)
	if project == nil {
		fmt.Printf("❌ Project %q not found in config\n", slug)
		os.Exit(1)
	}

	repoDir, err := devops.CloneOrPull(project.GitHub)
	if err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}

	if err := devops.FetchTags(repoDir); err != nil {
		fmt.Println("⚠️  Could not fetch tags:", err)
	}

	latestTag, err := devops.GetLatestTag(repoDir)
	if err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}
	fmt.Printf("🏷️  Latest tag: %s\n\n", latestTag)

	commits, err := devops.GetCommitsSinceTag(repoDir, latestTag)
	if err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}

	if len(commits) == 0 {
		fmt.Println("ℹ️  No commits since last tag.")
	} else {
		fmt.Println("📝 Commits since last tag:")
		for _, c := range commits {
			fmt.Println("  ", c)
		}
		fmt.Println()
	}

	current, err := devops.ParseSemVer(latestTag)
	if err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}

	proposed := devops.BumpVersion(current, commits)
	fmt.Printf("💡 Proposed tag: %s\n", proposed)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Accept? [Y/n or enter custom tag]: ")
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())

	var newTag string
	switch strings.ToLower(input) {
	case "", "y", "yes":
		newTag = proposed.String()
	default:
		custom, err := devops.ParseSemVer(input)
		if err != nil {
			fmt.Printf("❌ Invalid semver %q: %v\n", input, err)
			os.Exit(1)
		}
		if !custom.GreaterThan(current) {
			fmt.Printf("❌ Tag %s must be greater than current %s\n", custom, current)
			os.Exit(1)
		}
		newTag = custom.String()
	}

	fmt.Printf("🚀 Creating and pushing tag %s...\n", newTag)
	if err := devops.CreateAndPushTag(repoDir, newTag); err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}

	releaseURL := fmt.Sprintf("https://github.com/%s/releases/new?tag=%s", project.GitHub, newTag)
	fmt.Printf("✅ Tag %s pushed!\n", newTag)
	devops.OpenBrowser(releaseURL)
}
