package devops

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/minitap-ai/eva/internal/config"
)

func resolveProject(cfg *config.Config, slug string) *config.ProjectConfig {
	for i, p := range cfg.Devops.Projects {
		for _, s := range p.Slugs {
			if s == slug {
				return &cfg.Devops.Projects[i]
			}
		}
	}
	return nil
}

func mustLoadConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}
	return cfg
}

func mustResolveProject(cfg *config.Config, slug string) *config.ProjectConfig {
	p := resolveProject(cfg, slug)
	if p == nil {
		fmt.Printf("❌ Project %q not found in config\n", slug)
		os.Exit(1)
	}
	return p
}

func promptLine(prompt string) string {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(prompt)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func promptSecret(prompt string) string {
	return promptLine(prompt)
}

// parseKVFlags parses a slice of "KEY=VALUE" strings into a map.
func parseKVFlags(kvs []string) map[string]string {
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		key, val, _ := strings.Cut(kv, "=")
		if key != "" {
			m[key] = val
		}
	}
	return m
}

// resolveEnvs returns the list of environments to process, constrained to those
// present in the project's githubEnvironments config. If --env is specified it
// is further filtered to that single environment.
func resolveEnvs(envFlag string, configuredEnvs map[string]string) []string {
	candidates := []string{"prod", "dev"}
	if envFlag != "" {
		candidates = []string{envFlag}
	}

	var result []string
	for _, env := range candidates {
		if _, ok := configuredEnvs[env]; ok {
			result = append(result, env)
		}
	}
	return result
}

// ensureAWSAuth verifies that the AWS CLI is authenticated. Exits with a helpful
// message if credentials are missing or expired.
func ensureAWSAuth() {
	cmd := exec.Command("aws", "sts", "get-caller-identity")
	if err := cmd.Run(); err != nil {
		fmt.Println("❌ AWS credentials are missing or expired. Please run 'aws login' and try again.")
		os.Exit(1)
	}
}

// ensureGCPAuth verifies that gcloud application-default credentials are valid.
// Exits with a helpful message if credentials are missing or expired.
func ensureGCPAuth() {
	cmd := exec.Command("gcloud", "auth", "print-access-token")
	if err := cmd.Run(); err != nil {
		fmt.Println("❌ GCP credentials are missing or expired. Please run 'gcloud auth login' and try again.")
		os.Exit(1)
	}
}

// confirmDiff shows the git diff for repoDir and prompts the user to confirm.
// Returns true if the user confirms (or if yes=true). Exits if user declines.
// Does nothing if there are no changes to commit.
func confirmDiff(repoDir string, yes bool) {
	out, err := exec.Command("git", "-C", repoDir, "status", "--porcelain").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return
	}

	cmd := exec.Command("git", "-C", repoDir, "diff")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	if yes {
		return
	}

	answer := promptLine("👆 Commit these changes? [y/N]: ")
	if !strings.EqualFold(answer, "y") {
		fmt.Println("❌ Aborted.")
		os.Exit(1)
	}
}

func githubToken(cfg *config.Config) string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return cfg.GitHubToken
}
