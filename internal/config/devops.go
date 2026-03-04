package config

type DevopsConfig struct {
	IAC      IACConfig       `yaml:"iac"`
	Projects []ProjectConfig `yaml:"projects"`
}

type IACConfig struct {
	Terraform TerraformIACConfig `yaml:"terraform"`
	K8s       K8sIACConfig       `yaml:"k8s"`
}

type TerraformIACConfig struct {
	GitHub         string                        `yaml:"github"`
	CloudProviders TerraformCloudProvidersConfig `yaml:"cloudProviders"`
}

type TerraformCloudProvidersConfig struct {
	AWS AWSProviderConfig `yaml:"aws"`
	GCP GCPProviderConfig `yaml:"gcp"`
}

type AWSProviderConfig struct {
	Prod AWSEnvConfig `yaml:"prod"`
	Dev  AWSEnvConfig `yaml:"dev"`
}

type AWSEnvConfig struct {
	Region string `yaml:"region"`
}

type GCPProviderConfig struct {
	Prod GCPEnvConfig `yaml:"prod"`
	Dev  GCPEnvConfig `yaml:"dev"`
}

type GCPEnvConfig struct {
	SecretsFile string `yaml:"secretsFile"`
	GCPProject  string `yaml:"gcpProject"`
}

type K8sIACConfig struct {
	GitHub         string `yaml:"github"`
	ManagedAppsDir string `yaml:"managedAppsDir"`
	EnvVarKey      string `yaml:"envVarKey"`
	SecretsKey     string `yaml:"secretsKey"`
}

type ProjectConfig struct {
	Slugs              []string             `yaml:"slugs"`
	GitHub             string               `yaml:"github"`
	HealthURL          string               `yaml:"healthUrl"`
	HealthURLDev       string               `yaml:"healthUrlDev"`
	GitHubEnvironments map[string]string    `yaml:"githubEnvironments"`
	Secrets            ProjectSecretsConfig `yaml:"secrets"`
	EnvVars            ProjectEnvVarsConfig `yaml:"envVars"`
}

type ProjectSecretsConfig struct {
	IAC                 string                    `yaml:"iac"`
	CloudProvider       string                    `yaml:"cloudProvider"`
	AppName             string                    `yaml:"appName"`
	CloudProviderConfig *AWSSecretsProviderConfig `yaml:"cloudProviderConfig,omitempty"`
}

type AWSSecretsProviderConfig struct {
	SecretsKey string `yaml:"secretsKey"`
	EnvFile    string `yaml:"envFile"`
	DevEnvFile string `yaml:"devEnvFile"`
}

type ProjectEnvVarsConfig struct {
	IAC                 string                `yaml:"iac"`
	AppName             string                `yaml:"appName"`
	CloudProvider       string                `yaml:"cloudProvider"`
	CloudProviderConfig *AWSEnvProviderConfig `yaml:"cloudProviderConfig,omitempty"`
}

type AWSEnvProviderConfig struct {
	EnvKey     string `yaml:"envKey"`
	EnvFile    string `yaml:"envFile"`
	DevEnvFile string `yaml:"devEnvFile"`
}
