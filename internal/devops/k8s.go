package devops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// resolveYAMLMapping traverses a dot-notation path through nested YAML mapping nodes,
// creating intermediate mapping nodes as needed. Returns the deepest mapping node.
// e.g. "env" → returns root mapping under that key
// e.g. "maas-api.env" → descends into "maas-api" then "env"
func resolveYAMLMapping(doc *yaml.Node, dotKey string) (*yaml.Node, error) {
	parts := strings.Split(dotKey, ".")
	current := doc
	for _, part := range parts {
		if current.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("expected mapping node at %q, got kind %d", part, current.Kind)
		}
		var next *yaml.Node
		for i := 0; i < len(current.Content)-1; i += 2 {
			if current.Content[i].Value == part {
				next = current.Content[i+1]
				break
			}
		}
		if next == nil {
			newMap := &yaml.Node{Kind: yaml.MappingNode}
			current.Content = append(current.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: part},
				newMap,
			)
			next = newMap
		}
		current = next
	}
	return current, nil
}

func parseYAMLFile(filePath string) (*yaml.Node, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}
	if root.Kind == 0 {
		root = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{
			{Kind: yaml.MappingNode},
		}}
	}
	if root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("unexpected root node kind in %s", filePath)
	}
	return &root, nil
}

func writeYAMLFile(filePath string, root *yaml.Node) error {
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return fmt.Errorf("marshal %s: %w", filePath, err)
	}
	return enc.Close()
}

// editValuesYAML reads a values.yaml, sets name = value under the given dotKey path, and writes it back.
// dotKey supports N-level nesting via dot notation, e.g. "env" or "maas-api.env".
func editValuesYAML(filePath, dotKey, name, value string) error {
	root, err := parseYAMLFile(filePath)
	if err != nil {
		return err
	}

	targetMap, err := resolveYAMLMapping(root.Content[0], dotKey)
	if err != nil {
		return err
	}

	found := false
	for i := 0; i < len(targetMap.Content)-1; i += 2 {
		if targetMap.Content[i].Value == name {
			targetMap.Content[i+1].Value = value
			found = true
			break
		}
	}
	if !found {
		targetMap.Content = append(targetMap.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: name},
			&yaml.Node{Kind: yaml.ScalarNode, Value: value, Style: yaml.DoubleQuotedStyle},
		)
	}

	return writeYAMLFile(filePath, root)
}

// addToYAMLList appends a string to a YAML sequence under the given dotKey path if not already present.
// dotKey supports N-level nesting via dot notation, e.g. "secrets" or "maas-api.secrets".
func addToYAMLList(filePath, dotKey, value string) error {
	root, err := parseYAMLFile(filePath)
	if err != nil {
		return err
	}

	parts := strings.Split(dotKey, ".")
	parentKey := strings.Join(parts[:len(parts)-1], ".")
	leafKey := parts[len(parts)-1]

	var parentMap *yaml.Node
	if parentKey == "" {
		parentMap = root.Content[0]
	} else {
		parentMap, err = resolveYAMLMapping(root.Content[0], parentKey)
		if err != nil {
			return err
		}
	}

	var seqNode *yaml.Node
	for i := 0; i < len(parentMap.Content)-1; i += 2 {
		if parentMap.Content[i].Value == leafKey {
			seqNode = parentMap.Content[i+1]
			break
		}
	}
	if seqNode == nil {
		seqNode = &yaml.Node{Kind: yaml.SequenceNode}
		parentMap.Content = append(parentMap.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: leafKey},
			seqNode,
		)
	}

	for _, item := range seqNode.Content {
		if item.Value == value {
			return nil
		}
	}
	seqNode.Content = append(seqNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: value})

	return writeYAMLFile(filePath, root)
}

// K8sValuesPath returns the path to the values.yaml for a given app and environment.
func K8sValuesPath(repoDir, managedAppsDir, appName, env string) string {
	return filepath.Join(repoDir, managedAppsDir, appName, env, "values.yaml")
}

// EditK8sEnvVar updates or inserts an env var in the k8s values.yaml.
// For dev environments the key is prefixed with the app name (e.g. "<app-name>.env").
func EditK8sEnvVar(repoDir, managedAppsDir, appName, env, envVarKey, name, value string) error {
	path := K8sValuesPath(repoDir, managedAppsDir, appName, env)
	dotKey := envVarKey
	if env == "dev" {
		dotKey = appName + "." + envVarKey
	}
	if err := editValuesYAML(path, dotKey, name, value); err != nil {
		return fmt.Errorf("k8s env edit (%s/%s): %w", appName, env, err)
	}
	return nil
}

// RemoveK8sEnvVar removes an env var from the k8s values.yaml under the given dotKey path.
func RemoveK8sEnvVar(repoDir, managedAppsDir, appName, env, envVarKey, name string) error {
	path := K8sValuesPath(repoDir, managedAppsDir, appName, env)
	dotKey := envVarKey
	if env == "dev" {
		dotKey = appName + "." + envVarKey
	}
	root, err := parseYAMLFile(path)
	if err != nil {
		return fmt.Errorf("k8s env remove (%s/%s): %w", appName, env, err)
	}

	targetMap, err := resolveYAMLMapping(root.Content[0], dotKey)
	if err != nil {
		return fmt.Errorf("k8s env remove (%s/%s): %w", appName, env, err)
	}

	newContent := make([]*yaml.Node, 0, len(targetMap.Content))
	for i := 0; i < len(targetMap.Content)-1; i += 2 {
		if targetMap.Content[i].Value == name {
			continue
		}
		newContent = append(newContent, targetMap.Content[i], targetMap.Content[i+1])
	}
	targetMap.Content = newContent

	return writeYAMLFile(path, root)
}

// EditK8sSecret appends a secret name to the secrets list in the k8s values.yaml.
func EditK8sSecret(repoDir, managedAppsDir, appName, env, secretsKey, secretName string) error {
	path := K8sValuesPath(repoDir, managedAppsDir, appName, env)
	if err := addToYAMLList(path, secretsKey, secretName); err != nil {
		return fmt.Errorf("k8s secret edit (%s/%s): %w", appName, env, err)
	}
	return nil
}
