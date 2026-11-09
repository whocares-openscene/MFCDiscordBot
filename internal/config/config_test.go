package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "" +
		"# a comment\n" +
		"\n" +
		"MFCBOT_TEST_NEW=from_file\n" +
		"export MFCBOT_TEST_EXPORTED=exported value\n" +
		"MFCBOT_TEST_QUOTED=\"quoted value\"\n" +
		"MFCBOT_TEST_SINGLE='single value'\n" +
		"MFCBOT_TEST_EXISTING=from_file\n" +
		"not a valid line\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// Existing environment variables must win over the file.
	t.Setenv("MFCBOT_TEST_EXISTING", "from_env")

	for _, key := range []string{
		"MFCBOT_TEST_NEW", "MFCBOT_TEST_EXPORTED",
		"MFCBOT_TEST_QUOTED", "MFCBOT_TEST_SINGLE",
	} {
		os.Unsetenv(key)
		defer os.Unsetenv(key)
	}

	loadDotEnv(path)

	cases := map[string]string{
		"MFCBOT_TEST_NEW":      "from_file",
		"MFCBOT_TEST_EXPORTED": "exported value",
		"MFCBOT_TEST_QUOTED":   "quoted value",
		"MFCBOT_TEST_SINGLE":   "single value",
		"MFCBOT_TEST_EXISTING": "from_env",
	}
	for key, want := range cases {
		if got := os.Getenv(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestLoadDotEnvMissingFileIsIgnored(t *testing.T) {
	// Must not panic or block when no .env exists.
	loadDotEnv(filepath.Join(t.TempDir(), "does-not-exist"))
}

func TestListenAddressDefaultsToDisabled(t *testing.T) {
	t.Setenv("LISTEN_ADDRESS", "")
	if got := Load().ListenAddress; got != "" {
		t.Fatalf("ListenAddress = %q, want empty (HTTP disabled)", got)
	}
}

func TestListenAddressFromEnvironment(t *testing.T) {
	t.Setenv("LISTEN_ADDRESS", "127.0.0.1:9090")
	if got := Load().ListenAddress; got != "127.0.0.1:9090" {
		t.Fatalf("ListenAddress = %q, want 127.0.0.1:9090", got)
	}
}
