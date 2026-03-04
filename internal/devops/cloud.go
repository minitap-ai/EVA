package devops

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type CloudProvider string

const (
	CloudGCP CloudProvider = "gcp"
	CloudAWS CloudProvider = "aws"
)

func isAuthError(stderr string) bool {
	authPhrases := []string{
		"ExpiredToken",
		"AuthFailure",
		"InvalidClientTokenId",
		"not logged in",
		"Your current credentials",
		"re-authenticate",
		"Token has been expired",
		"credentialsExpired",
	}
	for _, phrase := range authPhrases {
		if strings.Contains(stderr, phrase) {
			return true
		}
	}
	return false
}

func waitForUserReauth(provider CloudProvider) {
	var instructions string
	switch provider {
	case CloudGCP:
		instructions = "Run: gcloud auth login\nThen press Enter to continue..."
	case CloudAWS:
		instructions = "Run: aws login\nThen press Enter to continue..."
	default:
		instructions = "Please re-authenticate and press Enter to continue..."
	}
	fmt.Printf("\n⚠️  Authentication expired. %s\n", instructions)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
}

// RunWithAuthRetry runs a command, detects auth expiry, prompts re-auth, and retries once.
func RunWithAuthRetry(provider CloudProvider, name string, args ...string) error {
	for attempt := 0; attempt < 2; attempt++ {
		cmd := exec.Command(name, args...)
		cmd.Stdout = os.Stdout
		var stderrBuf strings.Builder
		cmd.Stderr = &stderrBuf
		err := cmd.Run()
		if err == nil {
			return nil
		}
		stderr := stderrBuf.String()
		fmt.Fprint(os.Stderr, stderr)
		if attempt == 0 && isAuthError(stderr) {
			waitForUserReauth(provider)
			continue
		}
		return fmt.Errorf("command %s failed: %w", name, err)
	}
	return fmt.Errorf("command %s failed after re-authentication", name)
}

// GCloudSecretCreate creates a GCP secret if it doesn't already exist.
func GCloudSecretCreate(secretID, gcpProject string) error {
	checkCmd := exec.Command("gcloud", "secrets", "describe", secretID, "--project", gcpProject)
	if checkCmd.Run() == nil {
		return nil // already exists
	}
	return RunWithAuthRetry(CloudGCP,
		"gcloud", "secrets", "create", secretID,
		"--project", gcpProject,
		"--replication-policy=automatic",
	)
}

// GCloudSecretAddVersion adds a new version to a GCP secret.
func GCloudSecretAddVersion(secretID, gcpProject, value string) error {
	args := []string{"secrets", "versions", "add", secretID, "--project", gcpProject, "--data-file=-"}
	cmd := exec.Command("gcloud", args...)
	cmd.Stdin = strings.NewReader(value)
	cmd.Stdout = os.Stdout
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	if err == nil {
		return nil
	}
	stderr := stderrBuf.String()
	fmt.Fprint(os.Stderr, stderr)
	if isAuthError(stderr) {
		waitForUserReauth(CloudGCP)
		cmd2 := exec.Command("gcloud", args...)
		cmd2.Stdin = strings.NewReader(value)
		cmd2.Stdout = os.Stdout
		cmd2.Stderr = os.Stderr
		return cmd2.Run()
	}
	return fmt.Errorf("gcloud secrets versions add failed: %w", err)
}

// GCloudSecretDelete deletes a GCP secret and all its versions.
func GCloudSecretDelete(secretID, gcpProject string) error {
	return RunWithAuthRetry(CloudGCP,
		"gcloud", "secrets", "delete", secretID,
		"--project", gcpProject,
		"--quiet",
	)
}

// AWSSecretDelete deletes an AWS Secrets Manager secret immediately (no recovery window).
func AWSSecretDelete(secretID, region string) error {
	return RunWithAuthRetry(CloudAWS,
		"aws", "secretsmanager", "delete-secret",
		"--secret-id", secretID,
		"--region", region,
		"--force-delete-without-recovery",
	)
}

// AWSSecretUpsert creates or updates an AWS Secrets Manager secret in the given region.
func AWSSecretUpsert(secretID, region, value string) error {
	checkCmd := exec.Command("aws", "secretsmanager", "describe-secret", "--secret-id", secretID, "--region", region)
	if checkCmd.Run() == nil {
		return RunWithAuthRetry(CloudAWS,
			"aws", "secretsmanager", "put-secret-value",
			"--secret-id", secretID,
			"--region", region,
			"--secret-string", value,
		)
	}
	return RunWithAuthRetry(CloudAWS,
		"aws", "secretsmanager", "create-secret",
		"--name", secretID,
		"--region", region,
		"--secret-string", value,
	)
}
