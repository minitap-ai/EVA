package devops

import (
	"fmt"
	"os"
	"strings"

	"github.com/minitap-ai/eva/internal/config"
	"github.com/minitap-ai/eva/internal/devops"
	"github.com/spf13/cobra"
)

func newEnvCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environment variables for a project",
	}
	cmd.AddCommand(newEnvAddCommand())
	cmd.AddCommand(newEnvRmCommand())
	return cmd
}

// envEntry holds the prod and dev values for a single env var.
type envEntry struct {
	prod string
	dev  string
}

func newEnvAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [project]",
		Short: "Add or update environment variables for a project",
		Args:  cobra.ExactArgs(1),
		Run:   runEnvAdd,
	}
	cmd.Flags().String("env", "", "Target environment: prod or dev (default: both)")
	cmd.Flags().Bool("yes", false, "Skip confirmation prompt and commit immediately")
	cmd.Flags().StringArray("set", nil, "Set KEY=VALUE for prod (repeatable, e.g. --set FOO=bar)")
	cmd.Flags().StringArray("set-dev", nil, "Override KEY=VALUE for dev (repeatable); defaults to prod value")
	return cmd
}

func newEnvRmCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm [project]",
		Short: "Remove environment variables from a project",
		Args:  cobra.ExactArgs(1),
		Run:   runEnvRm,
	}
	cmd.Flags().String("env", "", "Target environment: prod or dev (default: both)")
	cmd.Flags().Bool("yes", false, "Skip confirmation prompt and commit immediately")
	cmd.Flags().StringArray("name", nil, "Env var name to remove (repeatable)")
	return cmd
}

func runEnvAdd(cmd *cobra.Command, args []string) {
	slug := args[0]
	envFlag, _ := cmd.Flags().GetString("env")
	yes, _ := cmd.Flags().GetBool("yes")
	setFlags, _ := cmd.Flags().GetStringArray("set")
	setDevFlags, _ := cmd.Flags().GetStringArray("set-dev")

	cfg := mustLoadConfig()
	project := mustResolveProject(cfg, slug)

	entries := map[string]envEntry{}

	if len(setFlags) > 0 {
		devOverrides := parseKVFlags(setDevFlags)
		for _, kv := range setFlags {
			key, val, ok := strings.Cut(kv, "=")
			if !ok {
				fmt.Printf("❌ Invalid --set value %q: expected KEY=VALUE\n", kv)
				os.Exit(1)
			}
			devVal := devOverrides[key]
			if devVal == "" {
				devVal = val
			}
			entries[key] = envEntry{prod: val, dev: devVal}
		}
	} else {
		_, hasDev := project.GitHubEnvironments["dev"]
		for {
			name := promptLine("🔑 Env var name (empty to finish): ")
			if name == "" {
				break
			}
			prodValue := promptLine("📦 PROD value: ")
			devValue := prodValue
			if hasDev {
				devValue = promptLine("📦 DEV value (leave empty to use PROD value): ")
				if devValue == "" {
					devValue = prodValue
				}
			}
			entries[name] = envEntry{prod: prodValue, dev: devValue}
		}
	}

	if len(entries) == 0 {
		fmt.Println("❌ No env vars provided")
		os.Exit(1)
	}

	envs := resolveEnvs(envFlag, project.GitHubEnvironments)

	switch project.EnvVars.IAC {
	case "k8s":
		if err := editK8sEnvVars(cfg, project, entries, envs, yes); err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}
	case "terraform":
		if err := editTerraformEnvVars(cfg, project, entries, envs, yes); err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("❌ Unknown IAC provider: %s\n", project.EnvVars.IAC)
		os.Exit(1)
	}

	fmt.Println("✅ Environment variable updated successfully!")
}

func runEnvRm(cmd *cobra.Command, args []string) {
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
			name := promptLine("🔑 Env var name to remove (empty to finish): ")
			if name == "" {
				break
			}
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		fmt.Println("❌ No env vars provided")
		os.Exit(1)
	}

	envs := resolveEnvs(envFlag, project.GitHubEnvironments)

	switch project.EnvVars.IAC {
	case "k8s":
		if err := removeK8sEnvVars(cfg, project, names, envs, yes); err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}
	case "terraform":
		if err := removeTerraformEnvVars(cfg, project, names, envs, yes); err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("❌ Unknown IAC provider: %s\n", project.EnvVars.IAC)
		os.Exit(1)
	}

	fmt.Println("✅ Environment variable removed successfully!")
}

func editK8sEnvVars(cfg *config.Config, project *config.ProjectConfig, entries map[string]envEntry, envs []string, yes bool) error {
	k8s := cfg.Devops.IAC.K8s
	repoDir, err := devops.CloneOrPull(k8s.GitHub)
	if err != nil {
		return err
	}

	for _, env := range envs {
		for name, entry := range entries {
			value := entry.prod
			if env == "dev" {
				value = entry.dev
			}
			fmt.Printf("✏️  Editing k8s values.yaml for %s/%s [%s]...\n", project.EnvVars.AppName, env, name)
			if err := devops.EditK8sEnvVar(repoDir, k8s.ManagedAppsDir, project.EnvVars.AppName, env, k8s.EnvVarKey, name, value); err != nil {
				return err
			}
		}
	}

	confirmDiff(repoDir, yes)
	names := envEntryNames(entries)
	return devops.CommitAndPush(repoDir, fmt.Sprintf("chore: add/update env vars %s for %s", names, project.EnvVars.AppName))
}

func editTerraformEnvVars(cfg *config.Config, project *config.ProjectConfig, entries map[string]envEntry, envs []string, yes bool) error {
	tfCfg := project.EnvVars.CloudProviderConfig
	if tfCfg == nil {
		return fmt.Errorf("missing cloudProviderConfig for terraform envVars")
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
		for name, entry := range entries {
			value := entry.prod
			if env == "dev" {
				value = entry.dev
			}
			fmt.Printf("✏️  Editing terraform %s for %s [%s]...\n", filePath, env, name)
			if err := devops.EditTFEnvVar(absPath, tfCfg.EnvKey, name, value); err != nil {
				return err
			}
		}
	}

	confirmDiff(tfRepoDir, yes)
	names := envEntryNames(entries)
	return devops.CommitAndPush(tfRepoDir, fmt.Sprintf("chore: add/update env vars %s for %s", names, project.GitHub))
}

func removeK8sEnvVars(cfg *config.Config, project *config.ProjectConfig, names []string, envs []string, yes bool) error {
	k8s := cfg.Devops.IAC.K8s
	repoDir, err := devops.CloneOrPull(k8s.GitHub)
	if err != nil {
		return err
	}

	for _, env := range envs {
		for _, name := range names {
			fmt.Printf("🗑️  Removing env var %s from k8s values.yaml for %s/%s...\n", name, project.EnvVars.AppName, env)
			if err := devops.RemoveK8sEnvVar(repoDir, k8s.ManagedAppsDir, project.EnvVars.AppName, env, k8s.EnvVarKey, name); err != nil {
				return err
			}
		}
	}

	confirmDiff(repoDir, yes)
	return devops.CommitAndPush(repoDir, fmt.Sprintf("chore: remove env vars %s for %s", strings.Join(names, ", "), project.EnvVars.AppName))
}

func removeTerraformEnvVars(cfg *config.Config, project *config.ProjectConfig, names []string, envs []string, yes bool) error {
	tfCfg := project.EnvVars.CloudProviderConfig
	if tfCfg == nil {
		return fmt.Errorf("missing cloudProviderConfig for terraform envVars")
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
			fmt.Printf("🗑️  Removing env var %s from terraform %s (%s)...\n", name, filePath, env)
			if err := devops.RemoveTFEnvVar(absPath, tfCfg.EnvKey, name); err != nil {
				return err
			}
		}
	}

	confirmDiff(tfRepoDir, yes)
	return devops.CommitAndPush(tfRepoDir, fmt.Sprintf("chore: remove env vars %s for %s", strings.Join(names, ", "), project.GitHub))
}

func envEntryNames(entries map[string]envEntry) string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}
