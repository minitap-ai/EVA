package devops

import (
	"fmt"
	"os"
	"strings"

	"github.com/minitap-ai/eva/internal/config"
	"github.com/minitap-ai/eva/internal/devops"
	"github.com/spf13/cobra"
)

func newSecretCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Manage secrets for a project",
	}
	cmd.AddCommand(newSecretAddCommand())
	cmd.AddCommand(newSecretRmCommand())
	return cmd
}

// secretEntry holds the prod and dev values for a single secret.
type secretEntry struct {
	prod string
	dev  string
}

func newSecretAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [project]",
		Short: "Add a secret for a project",
		Args:  cobra.ExactArgs(1),
		Run:   runSecretAdd,
	}
	cmd.Flags().String("env", "", "Target environment: prod or dev (default: both)")
	cmd.Flags().Bool("yes", false, "Skip confirmation prompt and commit immediately")
	cmd.Flags().StringArray("secret", nil, "Set NAME=VALUE for prod (repeatable, e.g. --secret DB_PASS=s3cr3t)")
	cmd.Flags().StringArray("secret-dev", nil, "Override NAME=VALUE for dev (repeatable); defaults to prod value")
	return cmd
}

func newSecretRmCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm [project]",
		Short: "Remove a secret from a project (IAC repo + cloud provider)",
		Args:  cobra.ExactArgs(1),
		Run:   runSecretRm,
	}
	cmd.Flags().String("env", "", "Target environment: prod or dev (default: both)")
	cmd.Flags().Bool("yes", false, "Skip confirmation prompt and commit immediately")
	cmd.Flags().StringArray("name", nil, "Secret name to remove (repeatable)")
	return cmd
}

func runSecretAdd(cmd *cobra.Command, args []string) {
	slug := args[0]
	envFlag, _ := cmd.Flags().GetString("env")
	yes, _ := cmd.Flags().GetBool("yes")
	secretFlags, _ := cmd.Flags().GetStringArray("secret")
	secretDevFlags, _ := cmd.Flags().GetStringArray("secret-dev")

	cfg := mustLoadConfig()
	project := mustResolveProject(cfg, slug)

	entries := map[string]secretEntry{}

	if len(secretFlags) > 0 {
		devOverrides := parseKVFlags(secretDevFlags)
		for _, kv := range secretFlags {
			key, val, ok := strings.Cut(kv, "=")
			if !ok {
				fmt.Printf("❌ Invalid --secret value %q: expected NAME=VALUE\n", kv)
				os.Exit(1)
			}
			devVal := devOverrides[key]
			if devVal == "" {
				devVal = val
			}
			entries[key] = secretEntry{prod: val, dev: devVal}
		}
	} else {
		_, hasDev := project.GitHubEnvironments["dev"]
		for {
			name := promptLine("🔑 Secret name (empty to finish): ")
			if name == "" {
				break
			}
			prodValue := promptSecret("🔒 PROD secret value: ")
			devValue := prodValue
			if hasDev {
				devValue = promptSecret("🔒 DEV secret value (leave empty to use PROD value): ")
				if devValue == "" {
					devValue = prodValue
				}
			}
			entries[name] = secretEntry{prod: prodValue, dev: devValue}
		}
	}

	if len(entries) == 0 {
		fmt.Println("❌ No secrets provided")
		os.Exit(1)
	}

	envs := resolveEnvs(envFlag, project.GitHubEnvironments)

	switch project.Secrets.IAC {
	case "terraform":
		switch project.Secrets.CloudProvider {
		case "gcp":
			if err := handleGCPSecret(cfg, project, entries, envs, yes); err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}
		case "aws":
			if err := handleAWSSecret(cfg, project, entries, envs, yes); err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}
		default:
			fmt.Printf("❌ Unknown cloud provider: %s\n", project.Secrets.CloudProvider)
			os.Exit(1)
		}
	default:
		fmt.Printf("❌ Unknown IAC provider: %s\n", project.Secrets.IAC)
		os.Exit(1)
	}

	fmt.Println("✅ Secret added successfully!")
}

func runSecretRm(cmd *cobra.Command, args []string) {
	slug := args[0]
	envFlag, _ := cmd.Flags().GetString("env")
	yes, _ := cmd.Flags().GetBool("yes")
	nameFlags, _ := cmd.Flags().GetStringArray("name")

	cfg := mustLoadConfig()
	project := mustResolveProject(cfg, slug)

	var names []string
	if len(nameFlags) > 0 {
		names = nameFlags
	} else {
		for {
			name := promptLine("🔑 Secret name to remove (empty to finish): ")
			if name == "" {
				break
			}
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		fmt.Println("❌ No secrets provided")
		os.Exit(1)
	}

	envs := resolveEnvs(envFlag, project.GitHubEnvironments)

	switch project.Secrets.IAC {
	case "terraform":
		switch project.Secrets.CloudProvider {
		case "gcp":
			if err := removeGCPSecret(cfg, project, names, envs, yes, yes); err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}
		case "aws":
			if err := removeAWSSecret(cfg, project, names, envs, yes, yes); err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}
		default:
			fmt.Printf("❌ Unknown cloud provider: %s\n", project.Secrets.CloudProvider)
			os.Exit(1)
		}
	default:
		fmt.Printf("❌ Unknown IAC provider: %s\n", project.Secrets.IAC)
		os.Exit(1)
	}

	fmt.Println("✅ Secret removed successfully!")
}

func handleGCPSecret(cfg *config.Config, project *config.ProjectConfig, entries map[string]secretEntry, envs []string, yes bool) error {
	gcp := cfg.Devops.IAC.Terraform.CloudProviders.GCP
	tfRepoDir, err := devops.CloneOrPull(cfg.Devops.IAC.Terraform.GitHub)
	if err != nil {
		return err
	}

	gcpEnvFiles := map[string]string{
		"prod": gcp.Prod.SecretsFile,
		"dev":  gcp.Dev.SecretsFile,
	}

	appName := project.Secrets.AppName
	if appName == "" {
		appName = devops.RepoNameFromSlug(project.GitHub)
	}

	for _, env := range envs {
		filePath := gcpEnvFiles[env]
		if filePath == "" {
			continue
		}
		absPath := tfRepoDir + "/" + filePath
		for name := range entries {
			fmt.Printf("✏️  Adding secret %s to GCP secrets.tf (%s)...\n", name, env)
			if err := devops.EditGCPSecretsFile(absPath, appName, name); err != nil {
				return err
			}
		}
	}

	confirmDiff(tfRepoDir, yes)
	names := secretEntryNames(entries)
	if err := devops.CommitAndPush(tfRepoDir, fmt.Sprintf("chore: add secrets %s for %s", names, appName)); err != nil {
		return err
	}

	// Update k8s-apps values.yaml with the new secrets
	k8sCfg := cfg.Devops.IAC.K8s
	if k8sCfg.GitHub != "" && k8sCfg.SecretsKey != "" {
		k8sRepoDir, err := devops.CloneOrPull(k8sCfg.GitHub)
		if err != nil {
			return err
		}
		for _, env := range envs {
			for name := range entries {
				fmt.Printf("✏️  Adding secret %s to k8s values.yaml (%s)...\n", name, env)
				if err := devops.EditK8sSecret(k8sRepoDir, k8sCfg.ManagedAppsDir, appName, env, k8sCfg.SecretsKey, name); err != nil {
					return err
				}
			}
		}
		confirmDiff(k8sRepoDir, yes)
		if err := devops.CommitAndPush(k8sRepoDir, fmt.Sprintf("chore: add secrets %s for %s", names, appName)); err != nil {
			return err
		}
	}

	gcpProjectForEnv := map[string]string{
		"prod": gcp.Prod.GCPProject,
		"dev":  gcp.Dev.GCPProject,
	}

	for name, entry := range entries {
		secretID := fmt.Sprintf("%s-%s", appName, strings.ToLower(strings.ReplaceAll(name, "_", "-")))
		for _, env := range envs {
			value := entry.prod
			if env == "dev" {
				value = entry.dev
			}
			gcpProject := gcpProjectForEnv[env]
			if gcpProject == "" {
				return fmt.Errorf("missing gcpProject for env %q in config (iac.terraform.cloudProviders.gcp.%s.gcpProject)", env, env)
			}
			fmt.Printf("☁️  Creating/updating GCP secret %s (%s)...\n", secretID, env)
			if err := devops.GCloudSecretCreate(secretID, gcpProject); err != nil {
				return fmt.Errorf("create GCP secret: %w", err)
			}
			if err := devops.GCloudSecretAddVersion(secretID, gcpProject, value); err != nil {
				return fmt.Errorf("add GCP secret version: %w", err)
			}
		}
	}

	return nil
}

func handleAWSSecret(cfg *config.Config, project *config.ProjectConfig, entries map[string]secretEntry, envs []string, yes bool) error {
	tfCfg := project.Secrets.CloudProviderConfig
	if tfCfg == nil {
		return fmt.Errorf("missing cloudProviderConfig for AWS secrets")
	}

	tfRepoDir, err := devops.CloneOrPull(cfg.Devops.IAC.Terraform.GitHub)
	if err != nil {
		return err
	}

	fileForEnv := map[string]string{
		"prod": tfCfg.EnvFile,
		"dev":  tfCfg.DevEnvFile,
	}

	for _, env := range envs {
		filePath := fileForEnv[env]
		if filePath == "" {
			continue
		}
		absPath := tfRepoDir + "/" + filePath
		for name := range entries {
			fmt.Printf("✏️  Adding secret %s to AWS terraform (%s)...\n", name, env)
			if err := devops.EditTFSecretList(absPath, tfCfg.SecretsKey, name); err != nil {
				return err
			}
		}
	}

	confirmDiff(tfRepoDir, yes)
	names := secretEntryNames(entries)
	if err := devops.CommitAndPush(tfRepoDir, fmt.Sprintf("chore: add secrets %s for %s", names, project.GitHub)); err != nil {
		return err
	}

	aws := cfg.Devops.IAC.Terraform.CloudProviders.AWS
	awsRegionForEnv := map[string]string{
		"prod": aws.Prod.Region,
		"dev":  aws.Dev.Region,
	}

	appName := devops.RepoNameFromSlug(project.GitHub)
	for name, entry := range entries {
		for _, env := range envs {
			value := entry.prod
			if env == "dev" {
				value = entry.dev
			}
			region := awsRegionForEnv[env]
			if region == "" {
				return fmt.Errorf("missing region for env %q in config (iac.terraform.cloudProviders.aws.%s.region)", env, env)
			}
			secretID := fmt.Sprintf("%s/%s/%s", appName, env, name)
			fmt.Printf("☁️  Creating/updating AWS secret %s...\n", secretID)
			if err := devops.AWSSecretUpsert(secretID, region, value); err != nil {
				return fmt.Errorf("AWS secret upsert: %w", err)
			}
		}
	}

	return nil
}

func removeGCPSecret(cfg *config.Config, project *config.ProjectConfig, names []string, envs []string, yes bool, confirmDelete bool) error {
	gcp := cfg.Devops.IAC.Terraform.CloudProviders.GCP
	tfRepoDir, err := devops.CloneOrPull(cfg.Devops.IAC.Terraform.GitHub)
	if err != nil {
		return err
	}

	gcpEnvFiles := map[string]string{
		"prod": gcp.Prod.SecretsFile,
		"dev":  gcp.Dev.SecretsFile,
	}

	appName := project.Secrets.AppName
	if appName == "" {
		appName = devops.RepoNameFromSlug(project.GitHub)
	}

	for _, env := range envs {
		filePath := gcpEnvFiles[env]
		if filePath == "" {
			continue
		}
		absPath := tfRepoDir + "/" + filePath
		for _, name := range names {
			fmt.Printf("🗑️  Removing secret %s from GCP secrets.tf (%s)...\n", name, env)
			if err := devops.RemoveGCPSecretsFile(absPath, appName, name); err != nil {
				return err
			}
		}
	}

	confirmDiff(tfRepoDir, yes)
	if err := devops.CommitAndPush(tfRepoDir, fmt.Sprintf("chore: remove secrets %s for %s", strings.Join(names, ", "), appName)); err != nil {
		return err
	}

	gcpProjectForEnv := map[string]string{
		"prod": gcp.Prod.GCPProject,
		"dev":  gcp.Dev.GCPProject,
	}

	fmt.Printf("\n⚠️  This will permanently delete the following secrets from GCP:\n")
	for _, name := range names {
		for _, env := range envs {
			secretID := fmt.Sprintf("%s-%s", appName, strings.ToLower(strings.ReplaceAll(name, "_", "-")))
			fmt.Printf("   • %s (project: %s)\n", secretID, gcpProjectForEnv[env])
		}
	}
	if !confirmDelete {
		answer := promptLine("🗑️  Confirm deletion? [y/N]: ")
		if !strings.EqualFold(answer, "y") {
			fmt.Println("⏭️  Skipping cloud secret deletion.")
			return nil
		}
	}

	for _, name := range names {
		secretID := fmt.Sprintf("%s-%s", appName, strings.ToLower(strings.ReplaceAll(name, "_", "-")))
		for _, env := range envs {
			gcpProject := gcpProjectForEnv[env]
			if gcpProject == "" {
				return fmt.Errorf("missing gcpProject for env %q in config", env)
			}
			fmt.Printf("☁️  Deleting GCP secret %s (%s)...\n", secretID, env)
			if err := devops.GCloudSecretDelete(secretID, gcpProject); err != nil {
				return fmt.Errorf("delete GCP secret: %w", err)
			}
		}
	}

	return nil
}

func removeAWSSecret(cfg *config.Config, project *config.ProjectConfig, names []string, envs []string, yes bool, confirmDelete bool) error {
	tfCfg := project.Secrets.CloudProviderConfig
	if tfCfg == nil {
		return fmt.Errorf("missing cloudProviderConfig for AWS secrets")
	}

	tfRepoDir, err := devops.CloneOrPull(cfg.Devops.IAC.Terraform.GitHub)
	if err != nil {
		return err
	}

	fileForEnv := map[string]string{
		"prod": tfCfg.EnvFile,
		"dev":  tfCfg.DevEnvFile,
	}

	for _, env := range envs {
		filePath := fileForEnv[env]
		if filePath == "" {
			continue
		}
		absPath := tfRepoDir + "/" + filePath
		for _, name := range names {
			fmt.Printf("🗑️  Removing secret %s from AWS terraform (%s)...\n", name, env)
			if err := devops.RemoveTFSecretList(absPath, tfCfg.SecretsKey, name); err != nil {
				return err
			}
		}
	}

	confirmDiff(tfRepoDir, yes)
	if err := devops.CommitAndPush(tfRepoDir, fmt.Sprintf("chore: remove secrets %s for %s", strings.Join(names, ", "), project.GitHub)); err != nil {
		return err
	}

	aws := cfg.Devops.IAC.Terraform.CloudProviders.AWS
	awsRegionForEnv := map[string]string{
		"prod": aws.Prod.Region,
		"dev":  aws.Dev.Region,
	}

	appName := devops.RepoNameFromSlug(project.GitHub)

	fmt.Printf("\n⚠️  This will permanently delete the following secrets from AWS:\n")
	for _, name := range names {
		for _, env := range envs {
			secretID := fmt.Sprintf("%s/%s/%s", appName, env, name)
			fmt.Printf("   • %s (region: %s)\n", secretID, awsRegionForEnv[env])
		}
	}
	if !confirmDelete {
		answer := promptLine("🗑️  Confirm deletion? [y/N]: ")
		if !strings.EqualFold(answer, "y") {
			fmt.Println("⏭️  Skipping cloud secret deletion.")
			return nil
		}
	}

	for _, name := range names {
		for _, env := range envs {
			region := awsRegionForEnv[env]
			if region == "" {
				return fmt.Errorf("missing region for env %q in config", env)
			}
			secretID := fmt.Sprintf("%s/%s/%s", appName, env, name)
			fmt.Printf("☁️  Deleting AWS secret %s...\n", secretID)
			if err := devops.AWSSecretDelete(secretID, region); err != nil {
				return fmt.Errorf("delete AWS secret: %w", err)
			}
		}
	}

	return nil
}

func secretEntryNames(entries map[string]secretEntry) string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}
