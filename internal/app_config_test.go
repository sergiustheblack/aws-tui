package internal

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// setupAWSEnv points the AWS config files at a temp dir and clears the
// environment variables that affect profile and region resolution.
func setupAWSEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config")
	credentialsFile := filepath.Join(dir, "credentials")
	config := "[default]\n[profile with-region]\nregion = ap-south-1\n[profile no-region]\n"
	credentials := "[creds-only]\naws_access_key_id = x\naws_secret_access_key = y\n"
	if err := os.WriteFile(configFile, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credentialsFile, []byte(credentials), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", configFile)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsFile)
	for _, v := range []string{"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION"} {
		t.Setenv(v, "")
		os.Unsetenv(v)
	}
}

func TestLoadConfigRegion(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		profile  string
		region   string
		expected string
	}{
		{name: "default", expected: "us-east-1"},
		{name: "profile without region", profile: "no-region", expected: "us-east-1"},
		{name: "profile with region", profile: "with-region", expected: "ap-south-1"},
		{name: "AWS_PROFILE", env: map[string]string{"AWS_PROFILE": "with-region"}, expected: "ap-south-1"},
		{name: "AWS_DEFAULT_PROFILE", env: map[string]string{"AWS_DEFAULT_PROFILE": "with-region"}, expected: "ap-south-1"},
		{name: "AWS_REGION", env: map[string]string{"AWS_REGION": "eu-west-1"}, expected: "eu-west-1"},
		{name: "AWS_DEFAULT_REGION", env: map[string]string{"AWS_DEFAULT_REGION": "eu-west-2"}, expected: "eu-west-2"},
		{name: "AWS_REGION over profile", env: map[string]string{"AWS_REGION": "eu-west-1"}, profile: "with-region", expected: "eu-west-1"},
		{name: "explicit region", env: map[string]string{"AWS_REGION": "eu-west-1"}, profile: "with-region", region: "us-west-2", expected: "us-west-2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAWSEnv(t)
			for k, v := range test.env {
				t.Setenv(k, v)
			}
			cfg, err := loadConfig(test.profile, test.region)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Region != test.expected {
				t.Errorf("expected region %v, got %v", test.expected, cfg.Region)
			}
		})
	}
}

func TestEnvProfile(t *testing.T) {
	setupAWSEnv(t)
	if got := envProfile(); got != "" {
		t.Errorf("expected no profile, got %q", got)
	}
	t.Setenv("AWS_DEFAULT_PROFILE", "b")
	if got := envProfile(); got != "b" {
		t.Errorf("expected AWS_DEFAULT_PROFILE, got %q", got)
	}
	t.Setenv("AWS_PROFILE", "a")
	if got := envProfile(); got != "a" {
		t.Errorf("expected AWS_PROFILE to take precedence, got %q", got)
	}
}

func TestReadAWSProfiles(t *testing.T) {
	setupAWSEnv(t)
	expected := []string{"default", "with-region", "no-region", "creds-only"}
	if got := readAWSProfiles(); !reflect.DeepEqual(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}
