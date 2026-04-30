package main

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// LoadDotEnv searches for a .env file in multiple locations and loads it.
// Search order:
//  1. Directory of the executable
//  2. Directory of this source file (useful for "go run")
func LoadDotEnv(filename string) error {
	if filename == "" {
		filename = ".env"
	}

	var paths []string

	// 1. directory of the executable
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), filename))
	}

	// 2. directory of this source file (for "go run" development)
	if _, srcFile, _, ok := runtime.Caller(0); ok {
		paths = append(paths, filepath.Join(filepath.Dir(srcFile), filename))
	}

	for _, p := range paths {
		if err := loadDotEnvFile(p); err == nil {
			return nil // loaded successfully
		} else if !os.IsNotExist(err) {
			return err // file exists but failed to read
		}
	}

	return nil // no .env file found anywhere, that's okay
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`) // remove surrounding quotes

		// Only set if not already defined in the environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}
