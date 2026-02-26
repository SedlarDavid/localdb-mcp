package db

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// brewPrefixes maps CLI tool names to the Homebrew formula prefixes where
// versioned copies may be installed (e.g. postgresql@18/bin/pg_dump).
var brewPrefixes = map[string]string{
	"pg_dump": "postgresql@",
	"psql":    "postgresql@",
}

// findCLITool returns the absolute path to the best available version of a CLI
// tool. On macOS it inspects Homebrew versioned formula directories so that the
// newest installed version is used regardless of PATH ordering. Falls back to
// exec.LookPath.
func findCLITool(name string) (string, error) {
	if runtime.GOOS == "darwin" {
		if prefix, ok := brewPrefixes[name]; ok {
			if p := findNewestBrewBinary(prefix, name); p != "" {
				return p, nil
			}
		}
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s is not installed or not in PATH; install it to use export/import", name)
	}
	return p, nil
}

// findNewestBrewBinary scans /opt/homebrew/opt and /usr/local/opt for
// versioned formula directories matching the given prefix (e.g. "postgresql@")
// and returns the path to the binary with the highest version number.
func findNewestBrewBinary(formulaPrefix, binary string) string {
	optDirs := []string{"/opt/homebrew/opt", "/usr/local/opt"}

	type candidate struct {
		version int
		path    string
	}
	var candidates []candidate

	for _, optDir := range optDirs {
		entries, err := os.ReadDir(optDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), formulaPrefix) {
				continue
			}
			verStr := strings.TrimPrefix(e.Name(), formulaPrefix)
			ver := 0
			fmt.Sscanf(verStr, "%d", &ver)
			if ver == 0 {
				continue
			}
			binPath := filepath.Join(optDir, e.Name(), "bin", binary)
			if _, err := os.Stat(binPath); err == nil {
				candidates = append(candidates, candidate{version: ver, path: binPath})
			}
		}
	}

	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].version > candidates[j].version
	})
	return candidates[0].path
}

// validateExportPath validates and normalizes the output file path for export.
// It ensures the parent directory exists.
func validateExportPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	dir := filepath.Dir(abs)
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("parent directory does not exist: %s", dir)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("parent path is not a directory: %s", dir)
	}
	return abs, nil
}

// validateImportPath validates the import file path exists and is readable.
func validateImportPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("file does not exist: %s", abs)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", abs)
	}
	return abs, nil
}

// truncateMsg truncates a string to maxLen characters for safe error reporting.
func truncateMsg(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "... (truncated)"
	}
	return s
}

// runCLI runs an external command with context and captures combined output.
// It does NOT log the args (which may contain credentials).
func runCLI(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %s", name, truncateMsg(string(out), 500))
	}
	return nil
}

// runCLIWithEnv runs an external command with extra environment variables.
func runCLIWithEnv(ctx context.Context, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %s", name, truncateMsg(string(out), 500))
	}
	return nil
}

// runCLIWithStdin runs an external command piping stdin from the given reader.
func runCLIWithStdin(ctx context.Context, env []string, stdin io.Reader, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdin = stdin
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %s", name, truncateMsg(string(out), 500))
	}
	return nil
}

// runCLICaptureStdout runs an external command and captures stdout to a file.
// Stderr is captured for error reporting.
func runCLICaptureStdout(ctx context.Context, outputPath string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()
	cmd.Stdout = f

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %s", name, truncateMsg(stderr.String(), 500))
	}
	return nil
}
