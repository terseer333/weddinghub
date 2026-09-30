package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolate clears the named variables for the duration of the test and restores them afterwards,
// so a developer's own environment cannot decide the result.
func isolate(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		previous, existed := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		t.Cleanup(func() {
			if !existed {
				os.Unsetenv(key)
				return
			}
			os.Setenv(key, previous)
		})
	}
}

func writeEnvFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	return path
}

func TestLoadEnvFileReadsValues(t *testing.T) {
	path := writeEnvFile(t, strings.Join([]string{
		"# a comment",
		"",
		"   ",
		"WEDDINGHUB_TEST_PLAIN=plain",
		" WEDDINGHUB_TEST_QUOTED = \"a value with spaces\" ",
		"export WEDDINGHUB_TEST_EXPORTED=exported",
		"WEDDINGHUB_TEST_SINGLE='single quoted'",
		"WEDDINGHUB_TEST_HASH=postgres://user:pass@host/db?sslmode=disable#fragment",
		"WEDDINGHUB_TEST_BROKEN",
	}, "\n"))
	keys := []string{"WEDDINGHUB_ENV_FILE", "WEDDINGHUB_TEST_PLAIN", "WEDDINGHUB_TEST_QUOTED",
		"WEDDINGHUB_TEST_EXPORTED", "WEDDINGHUB_TEST_SINGLE", "WEDDINGHUB_TEST_HASH", "WEDDINGHUB_TEST_BROKEN"}
	isolate(t, keys...)
	t.Setenv("WEDDINGHUB_ENV_FILE", path)

	loaded, err := LoadEnvFile()
	if err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if loaded != path {
		t.Fatalf("loaded = %q, want %q", loaded, path)
	}
	for key, want := range map[string]string{
		"WEDDINGHUB_TEST_PLAIN":    "plain",
		"WEDDINGHUB_TEST_QUOTED":   "a value with spaces",
		"WEDDINGHUB_TEST_EXPORTED": "exported",
		"WEDDINGHUB_TEST_SINGLE":   "single quoted",
		"WEDDINGHUB_TEST_HASH":     "postgres://user:pass@host/db?sslmode=disable#fragment",
	} {
		if got := os.Getenv(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if _, exists := os.LookupEnv("WEDDINGHUB_TEST_BROKEN"); exists {
		t.Error("a line without = must be ignored")
	}
}

func TestLoadEnvFileNeverOverridesTheEnvironment(t *testing.T) {
	isolate(t, "WEDDINGHUB_ENV_FILE", "WEDDINGHUB_TEST_KEPT")
	t.Setenv("WEDDINGHUB_ENV_FILE", writeEnvFile(t, "WEDDINGHUB_TEST_KEPT=from-file\n"))
	t.Setenv("WEDDINGHUB_TEST_KEPT", "from-environment")

	if _, err := LoadEnvFile(); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if got := os.Getenv("WEDDINGHUB_TEST_KEPT"); got != "from-environment" {
		t.Fatalf("WEDDINGHUB_TEST_KEPT = %q, want the environment value", got)
	}
}

func TestLoadEnvFileWithoutFileIsNotAnError(t *testing.T) {
	isolate(t, "WEDDINGHUB_ENV_FILE")
	dir := t.TempDir()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(working) })

	loaded, err := LoadEnvFile()
	if err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if loaded != "" {
		t.Fatalf("loaded = %q, want no file", loaded)
	}
}

func TestLoadEnvFileReportsAnExplicitMissingFile(t *testing.T) {
	isolate(t, "WEDDINGHUB_ENV_FILE")
	t.Setenv("WEDDINGHUB_ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))

	if _, err := LoadEnvFile(); err == nil {
		t.Fatal("expected an error for an explicitly configured file that does not exist")
	}
}

func TestLoadEnvFileDiscoversTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("WEDDINGHUB_TEST_DISCOVERED=yes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	isolate(t, "WEDDINGHUB_ENV_FILE", "WEDDINGHUB_TEST_DISCOVERED")
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(working) })

	loaded, err := LoadEnvFile()
	if err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if loaded != ".env" {
		t.Fatalf("loaded = %q, want %q", loaded, ".env")
	}
	if got := os.Getenv("WEDDINGHUB_TEST_DISCOVERED"); got != "yes" {
		t.Fatalf("WEDDINGHUB_TEST_DISCOVERED = %q, want yes", got)
	}
}

func TestListenAddress(t *testing.T) {
	isolate(t, "WEDDINGHUB_ADDR", "PORT")
	if got := ListenAddress(); got != ":8080" {
		t.Fatalf("default = %q, want :8080", got)
	}
	t.Setenv("PORT", "10000")
	if got := ListenAddress(); got != "0.0.0.0:10000" {
		t.Fatalf("with PORT = %q, want 0.0.0.0:10000", got)
	}
	t.Setenv("WEDDINGHUB_ADDR", "127.0.0.1:9000")
	if got := ListenAddress(); got != "127.0.0.1:9000" {
		t.Fatalf("with WEDDINGHUB_ADDR = %q, want 127.0.0.1:9000", got)
	}
}
