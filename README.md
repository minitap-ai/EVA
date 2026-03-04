# 🤖 EVA

**EVA** is a CLI tool to automate dev workflows — from creating Git branches off Notion tickets to managing releases, environment variables, and secrets across your infrastructure.

## 📦 Features

### Core
- `eva branch TASK-123` — Create a Git branch from a Notion task (and move it to "Doing")
- `eva open` — Open the current GitHub branch in your browser
- `eva init` — Set up your local config interactively
- `eva update` — Upgrade to the latest version

### DevOps
- `eva devops overview` — Deployment status dashboard across all projects
- `eva devops release [project]` — Tag a new semver release with conventional-commit bumping
- `eva devops env add [project]` — Add environment variables to k8s/terraform IAC repos
- `eva devops env rm [project]` — Remove environment variables
- `eva devops secrets add [project]` — Create secrets in cloud providers and IAC
- `eva devops secrets rm [project]` — Remove secrets from cloud providers and IAC

All configuration lives in `~/.eva/config.yaml` (see [`config.example.yaml`](config.example.yaml)).

## Tech Stack

- Go
- Cobra CLI
- Notion API
- GitHub + Make + YAML

## ⚙️ Installation

### 🧩 One-line install (recommended)

```bash
curl -sSfL https://raw.githubusercontent.com/minitap-ai/eva/main/install.sh | sh
```

### With Go

```bash
make install
```

Make sure `$HOME/go/bin` is in your `$PATH`

## 📦 Updating

```bash
eva update
```

## ⚙️ Configuration

The config file is required to use eva with Notion.

### 📥 Option 1 — Automatic (recommended)

```bash
eva init
```

This will:

- Ask for your Notion API Key and Database ID
- Create `~/.eva/config.yaml` for you

### ✍️ Option 2 — Manual

```bash
mkdir -p ~/.eva
nano ~/.eva/config.yaml
```

Paste this:

```yaml
notion_api_key: "your_notion_secret_here"
notion_database_id: "your_notion_database_id"
```

## 🛠 DevOps Commands

The `devops` command group provides infrastructure automation for teams managing multiple projects across cloud providers.

### Prerequisites

DevOps commands require authenticated cloud CLIs:
- **AWS**: `aws configure` or SSO session
- **GCP**: `gcloud auth login`

EVA checks authentication on each run and prompts to re-authenticate if needed.

### Overview

```bash
eva devops overview
```

Displays a deployment status table for all configured projects showing:
- Current deployed version and commit
- Whether each environment is up to date with the default branch
- Number of commits ahead (if any)
- Health check status

### Release

```bash
eva devops release [project]
```

Tags a new semver release on a project repository:
1. Fetches the latest tags and shows commits since the last release
2. Proposes a version bump based on conventional commits (`feat!` → major, `feat` → minor, else → patch)
3. Confirms or allows a custom version
4. Pushes the tag and opens the GitHub release page

### Environment Variables

```bash
eva devops env add [project]          # Add or update env vars
eva devops env rm [project]           # Remove env vars
```

Manages environment variables in your IAC repositories (Kubernetes `values.yaml` and/or Terraform files):
- `--env prod|dev|both` — target environment (default: interactive prompt)
- `--set KEY=VALUE` — set a production value (repeatable)
- `--set-dev KEY=VALUE` — set a dev value (repeatable)
- `--yes` — skip confirmation prompts

The command clones/pulls the IAC repo, edits the relevant files, shows a diff for confirmation, then commits and pushes.

### Secrets

```bash
eva devops secrets add [project]      # Create secrets
eva devops secrets rm [project]       # Remove secrets
```

Manages secrets across cloud providers (GCP Secret Manager, AWS Secrets Manager) **and** Terraform configuration:
- `--env prod|dev|both` — target environment
- `--secret NAME=VALUE` — set a secret (repeatable, prod)
- `--secret-dev NAME=VALUE` — set a dev secret (repeatable)
- `--name NAME` — secret name (for removal)
- `--yes` — skip confirmation prompts

For `add`: updates Terraform files, commits, then creates/updates the cloud secret.
For `rm`: removes from Terraform, commits, then deletes from the cloud provider.

## Development

```bash
make build       # Build binary locally
make run         # Run with args like CMD="branch TASK-123"
make clean       # Clean build artifacts
make snapshot    # Build binaries locally via GoReleaser
make release     # Publish a version (requires VERSION + token)
```
