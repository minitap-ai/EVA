package devops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func RepoDir(githubSlug string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}
	parts := strings.Split(githubSlug, "/")
	repoName := parts[len(parts)-1]
	return filepath.Join(home, ".eva", "repos", repoName), nil
}

// CloneOrPull ensures the repo is cloned under ~/.eva/repos/<name> and up to date.
func CloneOrPull(githubSlug string) (string, error) {
	dir, err := RepoDir(githubSlug)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
			return "", fmt.Errorf("failed to create repos dir: %w", err)
		}
		cloneURL := fmt.Sprintf("https://github.com/%s.git", githubSlug)
		fmt.Printf("📥 Cloning %s...\n", githubSlug)
		cmd := exec.Command("git", "clone", cloneURL, dir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git clone failed: %w", err)
		}
	} else {
		fmt.Printf("🔄 Pulling latest changes for %s...\n", githubSlug)
		for _, args := range [][]string{
			{"fetch", "origin"},
			{"reset", "--hard", "origin/HEAD"},
		} {
			cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
			}
		}
	}

	return dir, nil
}

// CommitAndPush commits all changes in the repo and pushes to origin.
func CommitAndPush(repoDir, message string) error {
	addCmd := exec.Command("git", "-C", repoDir, "add", "-A")
	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	commitCmd := exec.Command("git", "-C", repoDir, "commit", "-m", message)
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		if isNothingToCommit(repoDir) {
			return nil
		}
		return fmt.Errorf("git commit failed: %w", err)
	}

	pushCmd := exec.Command("git", "-C", repoDir, "push")
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}
	return nil
}

func isNothingToCommit(repoDir string) bool {
	out, err := exec.Command("git", "-C", repoDir, "status", "--porcelain").Output()
	return err == nil && strings.TrimSpace(string(out)) == ""
}
