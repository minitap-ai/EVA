package devops

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type Deployment struct {
	ID  int    `json:"id"`
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type DeploymentStatus struct {
	State string `json:"state"`
}

type DeploymentInfo struct {
	Ref   string
	SHA   string
	State string
}

func githubAPIGet(token, url string, out interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitHub API %s returned %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// GetLatestDeployment fetches the latest deployment and its status for a given GitHub environment.
func GetLatestDeployment(token, githubSlug, environment string) (*DeploymentInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/deployments?environment=%s&per_page=1",
		githubSlug, environment)

	var deployments []Deployment
	if err := githubAPIGet(token, url, &deployments); err != nil {
		return nil, err
	}
	if len(deployments) == 0 {
		return nil, fmt.Errorf("no deployments found for environment %q", environment)
	}

	d := deployments[0]
	statusURL := fmt.Sprintf("https://api.github.com/repos/%s/deployments/%d/statuses?per_page=1",
		githubSlug, d.ID)

	var statuses []DeploymentStatus
	if err := githubAPIGet(token, statusURL, &statuses); err != nil {
		return nil, err
	}

	state := "unknown"
	if len(statuses) > 0 {
		state = statuses[0].State
	}

	return &DeploymentInfo{
		Ref:   d.Ref,
		SHA:   d.SHA,
		State: state,
	}, nil
}

// GetLatestCommitSHA returns the SHA of the latest commit on the given branch.
func GetLatestCommitSHA(token, githubSlug, branch string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", githubSlug, branch)
	var commit struct {
		SHA string `json:"sha"`
	}
	if err := githubAPIGet(token, url, &commit); err != nil {
		return "", err
	}
	return commit.SHA, nil
}

// GetDefaultBranch returns the default branch name of a repository.
func GetDefaultBranch(token, githubSlug string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s", githubSlug)
	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := githubAPIGet(token, url, &repo); err != nil {
		return "", err
	}
	return repo.DefaultBranch, nil
}

// GetAheadBy returns how many commits `head` is ahead of `base` on the given repo.
// base and head can be SHAs, tags, or branch names.
func GetAheadBy(token, githubSlug, base, head string) (int, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/compare/%s...%s", githubSlug, base, head)
	var result struct {
		AheadBy int `json:"ahead_by"`
	}
	if err := githubAPIGet(token, url, &result); err != nil {
		return 0, err
	}
	return result.AheadBy, nil
}

// CheckHealth performs an HTTP GET on the healthURL and returns true if 2xx.
func CheckHealth(healthURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// OpenBrowser opens a URL in the default browser.
func OpenBrowser(url string) {
	for _, cmd := range []string{"open", "xdg-open", "start"} {
		if exec.Command(cmd, url).Start() == nil {
			return
		}
	}
	fmt.Printf("🔗 Open manually: %s\n", url)
}

// RepoNameFromSlug returns the repo name from a GitHub slug (e.g. "org/repo" → "repo").
func RepoNameFromSlug(slug string) string {
	parts := strings.Split(slug, "/")
	return parts[len(parts)-1]
}

// RepoHTTPSURL returns the HTTPS clone URL for a GitHub slug.
func RepoHTTPSURL(slug string) string {
	return fmt.Sprintf("https://github.com/%s", slug)
}
