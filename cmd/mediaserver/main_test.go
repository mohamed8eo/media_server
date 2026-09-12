package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePort(t *testing.T) {
	for _, port := range []int{1, 8080, 65535} {
		if err := validatePort(port); err != nil {
			t.Fatalf("port %d: %v", port, err)
		}
	}
	for _, port := range []int{0, -1, 65536} {
		if err := validatePort(port); err == nil {
			t.Fatalf("port %d unexpectedly accepted", port)
		}
	}
}

func TestConfigRoundTrip(t *testing.T) {
	t.Setenv("MEDIASERVER_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	want := Config{ProjectDir: "/mnt/media/project", DataDir: "/mnt/media/data", Port: 9090}
	if err := saveConfig(want); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %#v want %#v", got, want)
	}
	info, err := os.Stat(os.Getenv("MEDIASERVER_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("config mode = %o", info.Mode().Perm())
	}
}

func TestComposeEnvironment(t *testing.T) {
	env := strings.Join(composeEnv(Config{Port: 8123, DataDir: "/mnt/ssd/media-data"}), " ")
	if !strings.Contains(env, "PORT=8123") || !strings.Contains(env, "DATA_DIR=/mnt/ssd/media-data") {
		t.Fatalf("unexpected environment %q", env)
	}
}

func TestSystemdUnitIncludesBothMounts(t *testing.T) {
	unit := systemdUnit(Config{ProjectDir: "/mnt/ssd/project", DataDir: "/mnt/ssd/data", Port: 8080}, "/home/user/go/bin/mediaserver")
	for _, part := range []string{"RequiresMountsFor=/mnt/ssd/project /mnt/ssd/data", "ExecStart=/home/user/go/bin/mediaserver start", "ExecStop=/home/user/go/bin/mediaserver stop"} {
		if !strings.Contains(unit, part) {
			t.Fatalf("unit missing %q:\n%s", part, unit)
		}
	}
}

func TestEnsureEnvCreatesSecretWithoutReplacingExistingFile(t *testing.T) {
	project := t.TempDir()
	if err := ensureEnv(project); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, ".env")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "APP_ENV=local") || !strings.Contains(string(contents), "JWT_SECRET=") {
		t.Fatalf("unexpected env file: %q", contents)
	}
	if err := os.WriteFile(path, []byte("JWT_SECRET=keep-me\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ensureEnv(project); err != nil {
		t.Fatal(err)
	}
	contents, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "JWT_SECRET=keep-me\n" {
		t.Fatalf("existing env changed: %q", contents)
	}
}

func TestMoveDirMovesPersistedFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "media.txt"), []byte("media"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := moveDir(source, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source was not moved: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(destination, "nested", "media.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "media" {
		t.Fatalf("destination content = %q", contents)
	}
}
