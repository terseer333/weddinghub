// Package config resolves WeddingHub's process configuration. Every setting is an environment
// variable, and an optional .env file supplies them for local runs so the same names work in a
// deployment and on a laptop.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// candidates are the environment files consulted in order when WEDDINGHUB_ENV_FILE is unset:
// the working directory (a run from backend/) and then the repository root.
var candidates = []string{".env", filepath.Join("..", ".env")}

// LoadEnvFile applies the environment file to the process environment and reports which file it
// used, or "" when there is none. A value already present in the environment always wins, so a
// deployment's real settings are never overwritten by a file in the working directory, and an
// explicitly configured WEDDINGHUB_ENV_FILE that cannot be read is an error rather than a
// silent fall back to defaults.
func LoadEnvFile() (string, error) {
	path := strings.TrimSpace(os.Getenv("WEDDINGHUB_ENV_FILE"))
	if path == "" {
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				path = candidate
				break
			}
		}
	}
	if path == "" {
		return "", nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseEnvLine(scanner.Text())
		if !ok {
			continue
		}
		// LookupEnv separates "unset" from "set to empty", so an empty deployment variable is
		// still an explicit choice.
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return path, err
		}
	}
	if err := scanner.Err(); err != nil {
		return path, err
	}
	return path, nil
}

// parseEnvLine reads one KEY=VALUE line. Blank lines and comments are skipped, an optional
// "export " prefix is ignored, and one layer of matching quotes is removed so a value may hold
// spaces. Inline comments are deliberately not treated as comments: a # belongs to the value,
// which keeps connection strings and secrets intact.
func parseEnvLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	key, value, found := strings.Cut(line, "=")
	key = strings.TrimSpace(key)
	if !found || key == "" || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if quote := value[0]; (quote == '"' || quote == '\'') && value[len(value)-1] == quote {
			value = value[1 : len(value)-1]
		}
	}
	return key, value, true
}

// ListenAddress returns the address the HTTP server listens on. WEDDINGHUB_ADDR wins; next a
// PORT injected by a host such as Render, which also requires binding every interface; and the
// local default comes last.
func ListenAddress() string {
	if address := strings.TrimSpace(os.Getenv("WEDDINGHUB_ADDR")); address != "" {
		return address
	}
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		return "0.0.0.0:" + port
	}
	return ":8080"
}
