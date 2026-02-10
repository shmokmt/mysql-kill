package mysqlkill

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func TestIsSecretsManagerARN(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "valid ARN",
			value: "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-AbCdEf",
			want:  true,
		},
		{
			name:  "valid ARN with json key",
			value: "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-AbCdEf:password::",
			want:  true,
		},
		{
			name:  "plain password",
			value: "mysecretpassword",
			want:  false,
		},
		{
			name:  "empty string",
			value: "",
			want:  false,
		},
		{
			name:  "other AWS service ARN",
			value: "arn:aws:s3:::my-bucket",
			want:  false,
		},
		{
			name:  "IAM ARN",
			value: "arn:aws:iam::123456789012:role/MyRole",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSecretsManagerARN(tt.value)
			if got != tt.want {
				t.Fatalf("isSecretsManagerARN(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseSecretRef(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		wantARN      string
		wantJSONKey  string
		wantStage    string
		wantVersion  string
	}{
		{
			name:    "ARN only (7 parts)",
			value:   "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-AbCdEf",
			wantARN: "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-AbCdEf",
		},
		{
			name:        "with json key and empty stage/version",
			value:       "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-Ab:password::",
			wantARN:     "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-Ab",
			wantJSONKey: "password",
		},
		{
			name:        "with json key only (8 parts)",
			value:       "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-Ab:password",
			wantARN:     "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-Ab",
			wantJSONKey: "password",
		},
		{
			name:        "with all fields",
			value:       "arn:aws:secretsmanager:us-east-1:123456789012:secret:mydb-Ab:password:AWSPREVIOUS:v1",
			wantARN:     "arn:aws:secretsmanager:us-east-1:123456789012:secret:mydb-Ab",
			wantJSONKey: "password",
			wantStage:   "AWSPREVIOUS",
			wantVersion: "v1",
		},
		{
			name:      "with version stage only",
			value:     "arn:aws:secretsmanager:us-east-1:123456789012:secret:mydb-Ab::AWSPREVIOUS:",
			wantARN:   "arn:aws:secretsmanager:us-east-1:123456789012:secret:mydb-Ab",
			wantStage: "AWSPREVIOUS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arn, jsonKey, stage, version := parseSecretRef(tt.value)
			if arn != tt.wantARN {
				t.Errorf("arn = %q, want %q", arn, tt.wantARN)
			}
			if jsonKey != tt.wantJSONKey {
				t.Errorf("jsonKey = %q, want %q", jsonKey, tt.wantJSONKey)
			}
			if stage != tt.wantStage {
				t.Errorf("versionStage = %q, want %q", stage, tt.wantStage)
			}
			if version != tt.wantVersion {
				t.Errorf("versionID = %q, want %q", version, tt.wantVersion)
			}
		})
	}
}

type mockSMClient struct {
	output *secretsmanager.GetSecretValueOutput
	err    error
}

func (m *mockSMClient) GetSecretValue(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	return m.output, m.err
}

func strPtr(s string) *string { return &s }

func TestResolveSecretValue(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		output  *secretsmanager.GetSecretValueOutput
		err     error
		want    string
		wantErr string
	}{
		{
			name: "plain text secret",
			ref:  "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-Ab",
			output: &secretsmanager.GetSecretValueOutput{
				SecretString: strPtr("mypassword"),
			},
			want: "mypassword",
		},
		{
			name: "json secret with key",
			ref:  "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-Ab:password::",
			output: &secretsmanager.GetSecretValueOutput{
				SecretString: strPtr(`{"username":"admin","password":"secret123"}`),
			},
			want: "secret123",
		},
		{
			name: "json secret key not found",
			ref:  "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-Ab:missing_key::",
			output: &secretsmanager.GetSecretValueOutput{
				SecretString: strPtr(`{"username":"admin","password":"secret123"}`),
			},
			wantErr: `key "missing_key" not found in secret JSON`,
		},
		{
			name: "binary secret not supported",
			ref:  "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-Ab",
			output: &secretsmanager.GetSecretValueOutput{
				SecretBinary: []byte("binary-data"),
			},
			wantErr: "no SecretString",
		},
		{
			name:    "API error",
			ref:     "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-Ab",
			err:     fmt.Errorf("access denied"),
			wantErr: "get secret value: access denied",
		},
		{
			name: "json secret with non-string value",
			ref:  "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-Ab:port::",
			output: &secretsmanager.GetSecretValueOutput{
				SecretString: strPtr(`{"port":3306}`),
			},
			wantErr: `key "port" in secret JSON is not a string`,
		},
		{
			name: "invalid json with key",
			ref:  "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:mydb-Ab:password::",
			output: &secretsmanager.GetSecretValueOutput{
				SecretString: strPtr("not-json"),
			},
			wantErr: "parse secret JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockSMClient{output: tt.output, err: tt.err}
			got, err := resolveSecretValue(context.Background(), client, tt.ref)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if got := err.Error(); !contains(got, tt.wantErr) {
					t.Fatalf("error %q does not contain %q", got, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsAt(s, substr)
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
