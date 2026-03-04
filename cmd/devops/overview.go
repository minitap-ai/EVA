package devops

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/minitap-ai/eva/internal/devops"
	"github.com/spf13/cobra"
)

func newOverviewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "overview",
		Short: "Show deployment status of all configured projects",
		Run:   runOverview,
	}
}

var semverRe = regexp.MustCompile(`^v\d+\.\d+\.\d+`)

func runOverview(cmd *cobra.Command, args []string) {
	cfg := mustLoadConfig()
	token := githubToken(cfg)
	if token == "" {
		fmt.Println("⚠  No GitHub token found. Set GITHUB_TOKEN env var or github_token in config.")
	}

	t := table.NewWriter()
	t.SetOutputMirror(cmd.OutOrStdout())
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false
	t.Style().Options.SeparateHeader = false
	t.Style().Color.Header = text.Colors{text.Bold}
	t.AppendHeader(table.Row{"PROJECT", "ENV", "DEPLOYED", "UP TO DATE", "TO DEPLOY", "HEALTH"})

	for _, project := range cfg.Devops.Projects {
		name := devops.RepoNameFromSlug(project.GitHub)

		latestTag := "?"
		if tag, err := devops.GetLatestTagRemote(project.GitHub); err == nil {
			latestTag = tag
		}

		defaultBranch := ""
		if token != "" {
			if branch, err := devops.GetDefaultBranch(token, project.GitHub); err == nil {
				defaultBranch = branch
			}
		}

		if len(project.GitHubEnvironments) == 0 {
			healthIcon := healthStatus(project.HealthURL)
			t.AppendRow(table.Row{name, "-", "-", latestTag, "-", healthIcon})
			continue
		}

		// Sort envs so order is deterministic (prod before dev)
		envOrder := sortedEnvKeys(project.GitHubEnvironments)
		for i, envKey := range envOrder {
			ghEnv := project.GitHubEnvironments[envKey]
			deployedRef := "?"
			upToDateIcon := "?"
			toDeployCol := "-"

			if token != "" {
				info, err := devops.GetLatestDeployment(token, project.GitHub, ghEnv)
				if err == nil {
					deployedRef = info.Ref
					isBranchDeploy := !semverRe.MatchString(deployedRef)

					// UP TO DATE column
					if isBranchDeploy {
						latestSHA, shaErr := devops.GetLatestCommitSHA(token, project.GitHub, deployedRef)
						if shaErr == nil && strings.HasPrefix(latestSHA, info.SHA) {
							upToDateIcon = "✅"
						} else {
							upToDateIcon = fmt.Sprintf("⚠  (behind %s)", deployedRef)
						}
					} else if deployedRef == latestTag {
						upToDateIcon = "✅"
					} else {
						upToDateIcon = fmt.Sprintf("⚠  (latest: %s)", latestTag)
					}

					// TO DEPLOY column: commits on default branch ahead of what's deployed
					if defaultBranch != "" {
						base := info.SHA // for branch deploy, compare from deployed SHA
						if !isBranchDeploy {
							base = deployedRef // for tag deploy, compare from the tag
						}
						if n, aheadErr := devops.GetAheadBy(token, project.GitHub, base, defaultBranch); aheadErr == nil {
							if n == 0 {
								toDeployCol = "✅"
							} else {
								toDeployCol = fmt.Sprintf("⚠  %d commits", n)
							}
						}
					}
				} else {
					deployedRef = "error"
					upToDateIcon = "❓"
				}
			}

			healthURL := ""
			switch envKey {
			case "prod":
				healthURL = project.HealthURL
			case "dev":
				healthURL = project.HealthURLDev
			}
			healthIcon := healthStatus(healthURL)

			// Show project name only on first env row
			rowName := ""
			if i == 0 {
				rowName = name
			}
			t.AppendRow(table.Row{rowName, envKey, deployedRef, upToDateIcon, toDeployCol, healthIcon})
		}
		t.AppendSeparator()
	}

	t.Render()
}

func sortedEnvKeys(envs map[string]string) []string {
	order := []string{"prod", "dev"}
	seen := make(map[string]bool)
	result := []string{}
	for _, k := range order {
		if _, ok := envs[k]; ok {
			result = append(result, k)
			seen[k] = true
		}
	}
	for k := range envs {
		if !seen[k] {
			result = append(result, k)
		}
	}
	return result
}

func healthStatus(url string) string {
	if url == "" {
		return "-"
	}
	if devops.CheckHealth(url) {
		return "✅ up"
	}
	return "❌ down"
}
