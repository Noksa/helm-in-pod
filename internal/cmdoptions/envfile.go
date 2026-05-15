package cmdoptions

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseEnvFiles reads environment variables from the specified files and merges
// them into opts.Env. File format: KEY=VALUE per line. Lines starting with #
// and empty lines are skipped. Values may be optionally quoted (single or double).
// Later files override earlier files. Explicit --env flags always take precedence.
func (o *ExecOptions) ParseEnvFiles() error {
	if len(o.EnvFiles) == 0 {
		return nil
	}

	// Save explicit --env flags so they always win
	explicit := o.Env

	merged := make(map[string]string)
	for _, path := range o.EnvFiles {
		vars, err := parseEnvFile(path)
		if err != nil {
			return err
		}
		for k, v := range vars {
			merged[k] = v
		}
	}

	// Overlay explicit flags on top
	for k, v := range explicit {
		merged[k] = v
	}
	o.Env = merged
	return nil
}

// parseEnvFile reads a single env file and returns key-value pairs.
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open env file %q: %w", path, err)
	}
	defer f.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || line[0] == '#' {
			continue
		}

		// Optional "export " prefix
		line = strings.TrimPrefix(line, "export ")

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("env file %q line %d: invalid format (expected KEY=VALUE)", path, lineNum)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("env file %q line %d: empty key", path, lineNum)
		}

		value = strings.TrimSpace(value)
		value = unquote(value)
		result[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read env file %q: %w", path, err)
	}
	return result, nil
}

// unquote removes matching surrounding quotes (single or double).
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
