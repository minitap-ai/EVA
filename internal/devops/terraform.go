package devops

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// resolveBlock navigates nested blocks using a dot-notation path where every segment
// except the last is a block label and the last is the attribute name.
// e.g. "aws_compute.control_plane_environment_variables" → block label "aws_compute", attr "control_plane_environment_variables"
// e.g. "a.b.c" → block "a" → block "b", attr "c"
// Returns the target body and the attribute name.
func resolveBlock(root *hclwrite.Body, dotKey string) (*hclwrite.Body, string, error) {
	parts := strings.Split(dotKey, ".")
	if len(parts) < 2 {
		return nil, "", fmt.Errorf("dotKey must have at least one block segment and one attribute: %q", dotKey)
	}

	body := root
	for _, label := range parts[:len(parts)-1] {
		var found *hclwrite.Body
		for _, block := range body.Blocks() {
			labels := block.Labels()
			if len(labels) > 0 && labels[0] == label {
				found = block.Body()
				break
			}
			if block.Type() == label {
				found = block.Body()
				break
			}
		}
		if found == nil {
			return nil, "", fmt.Errorf("block with label or type %q not found", label)
		}
		body = found
	}

	return body, parts[len(parts)-1], nil
}

func parseTFFile(filePath string) (*hclwrite.File, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}
	f, diags := hclwrite.ParseConfig(data, filePath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse %s: %s", filePath, diags.Error())
	}
	return f, nil
}

func writeTFFile(filePath string, f *hclwrite.File) error {
	if err := os.WriteFile(filePath, f.Bytes(), 0644); err != nil {
		return err
	}
	cmd := exec.Command("terraform", "fmt", filePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// attrTokensString returns the string representation of an attribute's value tokens.
func attrTokensString(body *hclwrite.Body, attrName string) (string, error) {
	attr := body.GetAttribute(attrName)
	if attr == nil {
		return "", fmt.Errorf("attribute %q not found", attrName)
	}
	var sb strings.Builder
	for _, tok := range attr.Expr().BuildTokens(nil) {
		sb.Write(tok.Bytes)
	}
	return sb.String(), nil
}

// EditTFEnvVar finds or updates a { name = "KEY", value = "VALUE" } entry in a TF list.
// dotKey supports N-level nesting: "block_label.attr" or "a.b.attr".
// If the key exists, updates the value in-place. If not, appends before the closing `]`.
func EditTFEnvVar(filePath, dotKey, envName, envValue string) error {
	f, err := parseTFFile(filePath)
	if err != nil {
		return err
	}

	body, attrName, err := resolveBlock(f.Body(), dotKey)
	if err != nil {
		return err
	}

	current, err := attrTokensString(body, attrName)
	if err != nil {
		return err
	}

	existingRe := regexp.MustCompile(
		`(\{\s*name\s*=\s*"` + regexp.QuoteMeta(envName) + `"\s*,\s*value\s*=\s*")([^"]*)(")`,
	)

	var updated string
	if existingRe.MatchString(current) {
		updated = existingRe.ReplaceAllString(current, `${1}`+envValue+`${3}`)
	} else {
		trimmed := strings.TrimRight(current, " \t\n")
		closing := strings.LastIndex(trimmed, "]")
		if closing < 0 {
			return fmt.Errorf("no closing ] found in attribute %q", attrName)
		}
		entry := fmt.Sprintf("    { name = %q, value = %q },\n", envName, envValue)
		updated = trimmed[:closing] + entry + "  " + trimmed[closing:]
	}

	body.SetAttributeRaw(attrName, hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(updated)},
	})

	return writeTFFile(filePath, f)
}

// EditTFSecretList adds a secret name to a simple string list argument.
// dotKey supports N-level nesting. No-op if already present.
func EditTFSecretList(filePath, dotKey, secretName string) error {
	f, err := parseTFFile(filePath)
	if err != nil {
		return err
	}

	body, attrName, err := resolveBlock(f.Body(), dotKey)
	if err != nil {
		return err
	}

	current, err := attrTokensString(body, attrName)
	if err != nil {
		return err
	}

	alreadyRe := regexp.MustCompile(`"` + regexp.QuoteMeta(secretName) + `"`)
	if alreadyRe.MatchString(current) {
		return nil
	}

	trimmed := strings.TrimRight(current, " \t\n")
	closing := strings.LastIndex(trimmed, "]")
	if closing < 0 {
		return fmt.Errorf("no closing ] found in attribute %q", attrName)
	}
	entry := fmt.Sprintf("    %q,\n", secretName)
	updated := trimmed[:closing] + entry + "  " + trimmed[closing:]

	body.SetAttributeRaw(attrName, hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(updated)},
	})

	return writeTFFile(filePath, f)
}

// RemoveTFEnvVar removes a { name = "KEY", value = "..." } entry from a TF list.
// No-op if the key is not found.
func RemoveTFEnvVar(filePath, dotKey, envName string) error {
	f, err := parseTFFile(filePath)
	if err != nil {
		return err
	}

	body, attrName, err := resolveBlock(f.Body(), dotKey)
	if err != nil {
		return err
	}

	current, err := attrTokensString(body, attrName)
	if err != nil {
		return err
	}

	entryRe := regexp.MustCompile(
		`\n?\s*\{[^}]*name\s*=\s*"` + regexp.QuoteMeta(envName) + `"[^}]*\},?\n?`,
	)
	updated := entryRe.ReplaceAllString(current, "\n")

	body.SetAttributeRaw(attrName, hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(updated)},
	})

	return writeTFFile(filePath, f)
}

// RemoveTFSecretList removes a secret name from a simple string list argument.
// No-op if not present.
func RemoveTFSecretList(filePath, dotKey, secretName string) error {
	f, err := parseTFFile(filePath)
	if err != nil {
		return err
	}

	body, attrName, err := resolveBlock(f.Body(), dotKey)
	if err != nil {
		return err
	}

	current, err := attrTokensString(body, attrName)
	if err != nil {
		return err
	}

	entryRe := regexp.MustCompile(`\n?\s*` + regexp.QuoteMeta(`"`+secretName+`"`) + `,?\n?`)
	updated := entryRe.ReplaceAllString(current, "\n")

	body.SetAttributeRaw(attrName, hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(updated)},
	})

	return writeTFFile(filePath, f)
}

// RemoveGCPSecretsFile removes a secret name from the locals.secrets[appName] list in a GCP secrets.tf file.
// No-op if not present.
func RemoveGCPSecretsFile(filePath, appName, secretName string) error {
	f, err := parseTFFile(filePath)
	if err != nil {
		return err
	}

	var localsBody *hclwrite.Body
	for _, block := range f.Body().Blocks() {
		if block.Type() == "locals" {
			localsBody = block.Body()
			break
		}
	}
	if localsBody == nil {
		return fmt.Errorf("no locals block found in %s", filePath)
	}

	secretsAttr := localsBody.GetAttribute("secrets")
	if secretsAttr == nil {
		return fmt.Errorf("no secrets attribute in locals block in %s", filePath)
	}

	var sb strings.Builder
	for _, tok := range secretsAttr.Expr().BuildTokens(nil) {
		sb.Write(tok.Bytes)
	}
	current := sb.String()

	appRe := regexp.MustCompile(
		`(?s)(` + regexp.QuoteMeta(appName) + `\s*=\s*\[)(.*?)(\])`,
	)
	if !appRe.MatchString(current) {
		return fmt.Errorf("app %q not found in secrets locals in %s", appName, filePath)
	}

	entryRe := regexp.MustCompile(`\n?\s*` + regexp.QuoteMeta(`"`+secretName+`"`) + `,?\n?`)
	updated := appRe.ReplaceAllStringFunc(current, func(match string) string {
		sub := appRe.FindStringSubmatch(match)
		newBody := entryRe.ReplaceAllString(sub[2], "\n")
		return sub[1] + newBody + sub[3]
	})

	localsBody.SetAttributeRaw("secrets", hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(updated)},
	})

	return writeTFFile(filePath, f)
}

// EditGCPSecretsFile adds a secret name to the locals.secrets[appName] list in a GCP secrets.tf file.
// It navigates: locals block → secrets attribute → appName key within the object.
func EditGCPSecretsFile(filePath, appName, secretName string) error {
	f, err := parseTFFile(filePath)
	if err != nil {
		return err
	}

	var localsBody *hclwrite.Body
	for _, block := range f.Body().Blocks() {
		if block.Type() == "locals" {
			localsBody = block.Body()
			break
		}
	}
	if localsBody == nil {
		return fmt.Errorf("no locals block found in %s", filePath)
	}

	secretsAttr := localsBody.GetAttribute("secrets")
	if secretsAttr == nil {
		return fmt.Errorf("no secrets attribute in locals block in %s", filePath)
	}

	var sb strings.Builder
	for _, tok := range secretsAttr.Expr().BuildTokens(nil) {
		sb.Write(tok.Bytes)
	}
	current := sb.String()

	appRe := regexp.MustCompile(
		`(?s)(` + regexp.QuoteMeta(appName) + `\s*=\s*\[)(.*?)(\])`,
	)
	if !appRe.MatchString(current) {
		return fmt.Errorf("app %q not found in secrets locals in %s", appName, filePath)
	}

	alreadyRe := regexp.MustCompile(`"` + regexp.QuoteMeta(secretName) + `"`)
	if alreadyRe.MatchString(current) {
		return nil
	}

	updated := appRe.ReplaceAllStringFunc(current, func(match string) string {
		sub := appRe.FindStringSubmatch(match)
		entry := fmt.Sprintf("\n      %q,", secretName)
		return sub[1] + strings.TrimRight(sub[2], " \t\n") + entry + "\n    " + sub[3]
	})

	localsBody.SetAttributeRaw("secrets", hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(updated)},
	})

	return writeTFFile(filePath, f)
}
