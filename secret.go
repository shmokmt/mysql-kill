package mysqlkill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

const secretsManagerARNPrefix = "arn:aws:secretsmanager:"

// secretsManagerClient abstracts the Secrets Manager API for testing.
type secretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// isSecretsManagerARN reports whether value looks like a Secrets Manager ARN.
func isSecretsManagerARN(value string) bool {
	return strings.HasPrefix(value, secretsManagerARNPrefix)
}

// parseSecretRef splits an ECS-compatible secret reference into its components.
// Format: arn:aws:secretsmanager:region:account-id:secret:secret-name:json-key:version-stage:version-id
// The base ARN has 7 colon-separated parts. Parts 8-10 are optional ECS extensions.
func parseSecretRef(value string) (arn, jsonKey, versionStage, versionID string) {
	parts := strings.Split(value, ":")

	// A standard Secrets Manager ARN has 7 parts:
	// arn : aws : secretsmanager : region : account-id : secret : secret-name
	if len(parts) <= 7 {
		return value, "", "", ""
	}

	arn = strings.Join(parts[:7], ":")

	if len(parts) > 7 {
		jsonKey = parts[7]
	}
	if len(parts) > 8 {
		versionStage = parts[8]
	}
	if len(parts) > 9 {
		versionID = parts[9]
	}

	return arn, jsonKey, versionStage, versionID
}

// resolveSecretValue fetches a secret from Secrets Manager and optionally extracts a JSON key.
func resolveSecretValue(ctx context.Context, client secretsManagerClient, ref string) (string, error) {
	arn, jsonKey, versionStage, versionID := parseSecretRef(ref)

	input := &secretsmanager.GetSecretValueInput{
		SecretId: &arn,
	}
	if versionStage != "" {
		input.VersionStage = &versionStage
	}
	if versionID != "" {
		input.VersionId = &versionID
	}

	out, err := client.GetSecretValue(ctx, input)
	if err != nil {
		return "", fmt.Errorf("get secret value: %w", err)
	}

	if out.SecretString == nil {
		return "", fmt.Errorf("secret %q has no SecretString (binary secrets are not supported)", arn)
	}

	secret := *out.SecretString

	if jsonKey == "" {
		return secret, nil
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(secret), &m); err != nil {
		return "", fmt.Errorf("parse secret JSON: %w", err)
	}

	v, ok := m[jsonKey]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret JSON", jsonKey)
	}

	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("key %q in secret JSON is not a string", jsonKey)
	}

	return s, nil
}

// newSecretsManagerClient creates a real Secrets Manager client using the region from the ARN.
func newSecretsManagerClient(ctx context.Context, ref string) (secretsManagerClient, error) {
	arn, _, _, _ := parseSecretRef(ref)
	parts := strings.Split(arn, ":")
	if len(parts) < 4 || parts[3] == "" {
		return nil, fmt.Errorf("cannot extract region from ARN: %s", arn)
	}
	region := parts[3]

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return secretsmanager.NewFromConfig(cfg), nil
}

// resolvePassword resolves a password value. If it is a Secrets Manager ARN,
// the secret is fetched; otherwise the value is returned as-is.
func resolvePassword(ctx context.Context, password string) (string, error) {
	if !isSecretsManagerARN(password) {
		return password, nil
	}

	client, err := newSecretsManagerClient(ctx, password)
	if err != nil {
		return "", err
	}

	return resolveSecretValue(ctx, client, password)
}
