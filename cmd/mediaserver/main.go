// Command mediaserver manages a local Docker deployment of MediaServer.
package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	serviceName       = "mediaserver.service"
	defaultProjectDir = "/mnt/mediaserver-ssd/mediaserver"
)

type Config struct {
	ProjectDir string `json:"projectDir"`
	DataDir    string `json:"dataDir"`
	Port       int    `json:"port"`
}

func defaultConfig() Config {
	return Config{ProjectDir: defaultProjectDir, DataDir: filepath.Join(defaultProjectDir, "data"), Port: 8080}
}

func configPath() (string, error) {
	if p := os.Getenv("MEDIASERVER_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mediaserver", "config.json"), nil
}

func loadConfig() (Config, error) {
	p, err := configPath()
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return defaultConfig(), nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}
	if err := validateConfig(c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func saveConfig(c Config) error {
	if err := validateConfig(c); err != nil {
		return err
	}
	p, err := configPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o600)
}

func validateConfig(c Config) error {
	if !filepath.IsAbs(c.ProjectDir) || !filepath.IsAbs(c.DataDir) {
		return errors.New("project and data paths must be absolute")
	}
	if c.ProjectDir == "/" || c.DataDir == "/" {
		return errors.New("refusing to use / as a project or data path")
	}
	return validatePort(c.Port)
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func requireAvailablePort(port int) error {
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("port %d is already in use", port)
	}
	return l.Close()
}

func run(dir string, env []string, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), env...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func output(dir string, env []string, name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), env...)
	b, err := c.CombinedOutput()
	return strings.TrimSpace(string(b)), err
}

func composeEnv(c Config) []string {
	return []string{"PORT=" + strconv.Itoa(c.Port), "DATA_DIR=" + c.DataDir}
}

func compose(c Config, args ...string) error {
	return run(c.ProjectDir, composeEnv(c), "docker", append([]string{"compose"}, args...)...)
}

func ensureProject(c Config) error {
	info, err := os.Stat(c.ProjectDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project directory is unavailable: %s", c.ProjectDir)
	}
	if _, err := os.Stat(filepath.Join(c.ProjectDir, "docker-compose.yml")); err != nil {
		return fmt.Errorf("docker-compose.yml not found in %s", c.ProjectDir)
	}
	return nil
}

// requireMountedWritable rejects paths backed by the root filesystem. This prevents
// a missing removable drive from silently creating a second local data store.
func requireMountedWritable(path string) error {
	if !filepath.IsAbs(path) || path == "/" {
		return errors.New("data path must be a non-root absolute path")
	}
	if err := os.MkdirAll(path, 0o750); err != nil {
		return err
	}
	target, err := output("", nil, "findmnt", "-no", "TARGET", "--target", path)
	if err != nil || target == "" || target == "/" {
		return fmt.Errorf("data path is not on a dedicated mounted filesystem: %s", path)
	}
	test, err := os.CreateTemp(path, ".mediaserver-write-test-")
	if err != nil {
		return fmt.Errorf("data path is not writable: %w", err)
	}
	name := test.Name()
	_ = test.Close()
	return os.Remove(name)
}

func ensureData(c Config) error {
	if err := requireMountedWritable(c.DataDir); err != nil {
		return err
	}
	for _, name := range []string{"db", "storage"} {
		if err := os.MkdirAll(filepath.Join(c.DataDir, name), 0o750); err != nil {
			return err
		}
	}
	return nil
}

func ensureEnv(project string) error {
	path := filepath.Join(project, ".env")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("APP_ENV=local\nJWT_SECRET=%x\n", secret)), 0o600)
}

func start(c Config, build bool) error {
	if err := ensureProject(c); err != nil {
		return err
	}
	if err := ensureData(c); err != nil {
		return err
	}
	args := []string{"up", "-d"}
	if build {
		args = append(args, "--build")
	}
	if err := compose(c, args...); err != nil {
		return err
	}
	fmt.Printf("MediaServer is running at http://localhost:%d\n", c.Port)
	return nil
}

func stop(c Config) error {
	if err := ensureProject(c); err != nil {
		return err
	}
	return compose(c, "stop")
}

func dirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return errors.Is(err, io.EOF), err
}

func copyDir(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(destination, rel)
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(dst, info.Mode())
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("cannot migrate non-regular file %s", path)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		info, err := entry.Info()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func moveDir(source, destination string) error {
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := os.Rename(source, destination); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if err := copyDir(source, destination); err != nil {
		return err
	}
	return os.RemoveAll(source)
}

func setDataPath(c Config, destination string) error {
	destination = filepath.Clean(destination)
	if destination == c.DataDir {
		return nil
	}
	if !filepath.IsAbs(destination) || destination == "/" {
		return errors.New("data path must be a non-root absolute path")
	}
	if err := requireMountedWritable(destination); err != nil {
		return err
	}
	empty, err := dirEmpty(destination)
	if err != nil {
		return err
	}
	if !empty {
		return fmt.Errorf("destination data path must be empty: %s", destination)
	}
	if err := stop(c); err != nil {
		return err
	}
	moved := []string{}
	for _, name := range []string{"db", "storage"} {
		from, to := filepath.Join(c.DataDir, name), filepath.Join(destination, name)
		if err := moveDir(from, to); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = moveDir(filepath.Join(destination, moved[i]), filepath.Join(c.DataDir, moved[i]))
			}
			return fmt.Errorf("move data: %w", err)
		}
		moved = append(moved, name)
	}
	next := c
	next.DataDir = destination
	if err := saveConfig(next); err != nil {
		return err
	}
	if err := refreshServiceUnit(next); err != nil {
		return err
	}
	return start(next, false)
}

func systemdUnit(c Config, executable string) string {
	return fmt.Sprintf("[Unit]\nDescription=MediaServer Docker service\nRequiresMountsFor=%s %s\nAfter=network-online.target\n\n[Service]\nType=oneshot\nRemainAfterExit=yes\nExecStart=%s start\nExecStop=%s stop\nRestart=on-failure\nRestartSec=30\n\n[Install]\nWantedBy=default.target\n", c.ProjectDir, c.DataDir, executable, executable)
}

func servicePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "systemd", "user", serviceName), nil
}

func installService(c Config, executable string) error {
	exe := executable
	var err error
	if exe == "" {
		exe, err = os.Executable()
	}
	if err != nil {
		return err
	}
	path, err := servicePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(systemdUnit(c, exe)), 0o644); err != nil {
		return err
	}
	if err := run("", nil, "systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	return run("", nil, "systemctl", "--user", "enable", "--now", serviceName)
}

// refreshServiceUnit updates an installed unit without enabling a unit the
// user deliberately disabled.
func refreshServiceUnit(c Config) error {
	path, err := servicePath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(systemdUnit(c, exe)), 0o644); err != nil {
		return err
	}
	return run("", nil, "systemctl", "--user", "daemon-reload")
}

func installedExecutable(project string) (string, error) {
	bin, err := output(project, nil, "go", "env", "GOBIN")
	if err != nil {
		return "", err
	}
	if bin == "" {
		gopath, err := output(project, nil, "go", "env", "GOPATH")
		if err != nil {
			return "", err
		}
		bin = filepath.Join(strings.Split(gopath, string(os.PathListSeparator))[0], "bin")
	}
	return filepath.Join(bin, "mediaserver"), nil
}

func setAutostart(c Config, enabled bool) error {
	if enabled {
		return installService(c, "")
	}
	return run("", nil, "systemctl", "--user", "disable", "--now", serviceName)
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: mediaserver <setup|start|stop|status|update|remove|autostart|config> [options]")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	c, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	command := os.Args[1]
	switch command {
	case "setup":
		fs := flag.NewFlagSet("setup", flag.ExitOnError)
		project := fs.String("project-dir", c.ProjectDir, "project directory")
		data := fs.String("data-path", c.DataDir, "data directory")
		port := fs.Int("port", c.Port, "port")
		_ = fs.Parse(os.Args[2:])
		dataWasSet := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "data-path" {
				dataWasSet = true
			}
		})
		if !dataWasSet {
			*data = filepath.Join(*project, "data")
		}
		c = Config{ProjectDir: filepath.Clean(*project), DataDir: filepath.Clean(*data), Port: *port}
		if err = ensureProject(c); err == nil {
			err = requireAvailablePort(c.Port)
		}
		if err == nil {
			err = ensureData(c)
		}
		if err == nil {
			err = ensureEnv(c.ProjectDir)
		}
		if err == nil {
			err = saveConfig(c)
		}
		if err == nil {
			err = run(c.ProjectDir, nil, "go", "install", "./cmd/mediaserver")
		}
		if err == nil {
			var exe string
			exe, err = installedExecutable(c.ProjectDir)
			if err == nil {
				err = installService(c, exe)
			}
		}
	case "start":
		err = start(c, true)
	case "stop":
		err = stop(c)
	case "status":
		fmt.Printf("Project: %s\nData: %s\nURL: http://localhost:%d\n", c.ProjectDir, c.DataDir, c.Port)
		if state, stateErr := output("", nil, "systemctl", "--user", "is-enabled", serviceName); stateErr == nil {
			fmt.Println("Autostart:", state)
		} else {
			fmt.Println("Autostart: disabled or unavailable")
		}
		err = compose(c, "ps")
	case "update":
		if err = ensureProject(c); err == nil {
			out, statusErr := output(c.ProjectDir, nil, "git", "status", "--porcelain")
			if statusErr != nil {
				err = statusErr
			} else if out != "" {
				err = errors.New("refusing update: Git working tree is not clean")
			}
		}
		if err == nil {
			err = run(c.ProjectDir, nil, "git", "pull", "--ff-only")
		}
		if err == nil {
			err = start(c, true)
		}
	case "remove":
		fs := flag.NewFlagSet("remove", flag.ExitOnError)
		purge := fs.Bool("purge-data", false, "delete persistent data")
		force := fs.Bool("force", false, "confirm data deletion")
		_ = fs.Parse(os.Args[2:])
		if *purge && !*force {
			err = errors.New("--purge-data requires --force")
		}
		if err == nil {
			err = setAutostart(c, false)
		}
		if err == nil {
			err = compose(c, "down", "--remove-orphans")
		}
		if err == nil && *purge {
			err = os.RemoveAll(c.DataDir)
		}
	case "autostart":
		if len(os.Args) != 3 || (os.Args[2] != "enable" && os.Args[2] != "disable") {
			usage()
			os.Exit(2)
		}
		err = setAutostart(c, os.Args[2] == "enable")
	case "config":
		if len(os.Args) == 3 && os.Args[2] == "get" {
			b, _ := json.MarshalIndent(c, "", "  ")
			fmt.Println(string(b))
			return
		}
		if len(os.Args) != 5 || os.Args[2] != "set" {
			usage()
			os.Exit(2)
		}
		switch os.Args[3] {
		case "port":
			var p int
			p, err = strconv.Atoi(os.Args[4])
			if err == nil {
				err = validatePort(p)
			}
			if err == nil && p != c.Port {
				err = requireAvailablePort(p)
			}
			if err == nil {
				c.Port = p
				err = saveConfig(c)
			}
			if err == nil {
				err = start(c, false)
			}
		case "data-path":
			err = setDataPath(c, os.Args[4])
		default:
			err = errors.New("config key must be port or data-path")
		}
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
