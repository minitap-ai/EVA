package devops

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type SemVer struct {
	Major int
	Minor int
	Patch int
}

func ParseSemVer(tag string) (SemVer, error) {
	s := strings.TrimPrefix(tag, "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return SemVer{}, fmt.Errorf("invalid semver: %s", tag)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return SemVer{}, fmt.Errorf("invalid major in %s", tag)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return SemVer{}, fmt.Errorf("invalid minor in %s", tag)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return SemVer{}, fmt.Errorf("invalid patch in %s", tag)
	}
	return SemVer{Major: major, Minor: minor, Patch: patch}, nil
}

func (s SemVer) String() string {
	return fmt.Sprintf("v%d.%d.%d", s.Major, s.Minor, s.Patch)
}

func (s SemVer) GreaterThan(other SemVer) bool {
	if s.Major != other.Major {
		return s.Major > other.Major
	}
	if s.Minor != other.Minor {
		return s.Minor > other.Minor
	}
	return s.Patch > other.Patch
}

func GetLatestTag(repoDir string) (string, error) {
	cmd := exec.Command("git", "-C", repoDir, "tag", "--list", "v*", "--sort=-version:refname")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git tag list failed: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		tag := strings.TrimSpace(line)
		if tag != "" {
			return tag, nil
		}
	}
	return "", fmt.Errorf("no v* tags found in repo")
}

func GetLatestTagRemote(githubSlug string) (string, error) {
	url := fmt.Sprintf("https://github.com/%s.git", githubSlug)
	cmd := exec.Command("git", "ls-remote", "--tags", "--sort=-version:refname", url)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git ls-remote failed: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	re := regexp.MustCompile(`refs/tags/(v\d+\.\d+\.\d+)$`)
	for _, line := range lines {
		if m := re.FindStringSubmatch(line); m != nil {
			return m[1], nil
		}
	}
	return "", fmt.Errorf("no semver tags found for %s", githubSlug)
}

func GetCommitsSinceTag(repoDir, tag string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoDir, "log", tag+"..HEAD", "--oneline")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return []string{}, nil
	}
	return strings.Split(raw, "\n"), nil
}

func BumpVersion(current SemVer, commits []string) SemVer {
	breakingRe := regexp.MustCompile(`feat(\(.+\))?!:`)
	featRe := regexp.MustCompile(`feat(\(.+\))?:`)

	for _, c := range commits {
		if breakingRe.MatchString(c) {
			return SemVer{Major: current.Major + 1, Minor: 0, Patch: 0}
		}
	}
	for _, c := range commits {
		if featRe.MatchString(c) {
			return SemVer{Major: current.Major, Minor: current.Minor + 1, Patch: 0}
		}
	}
	return SemVer{Major: current.Major, Minor: current.Minor, Patch: current.Patch + 1}
}

func CreateAndPushTag(repoDir, tag string) error {
	for _, args := range [][]string{
		{"tag", tag},
		{"push", "origin", tag},
	} {
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

func FetchTags(repoDir string) error {
	cmd := exec.Command("git", "-C", repoDir, "fetch", "--tags")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
