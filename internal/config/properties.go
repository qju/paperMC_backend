package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const fileName string = "server.properties"

// LoadProperties reads the server.properties file from the given path
// and returns a map of property keys to values.
func LoadProperties(path string) (map[string]string, error) {
	fullPath := filepath.Join(path, fileName)
	fi, err := os.Open(fullPath)

	if err != nil {
		return nil, err
	}
	defer fi.Close()

	props := make(map[string]string)
	scanner := bufio.NewScanner(fi)
	// scanner.Split("=")
	for scanner.Scan() {
		// Save the read line and clean it
		line := scanner.Text()
		cleanLine := strings.TrimSpace(line)

		// check if comment or empty string, if so continue
		if cleanLine == "" || strings.HasPrefix(cleanLine, "#") {
			continue
		}

		// split line in to property and value
		parts := strings.SplitN(cleanLine, "=", 2)
		if len(parts) != 2 {
			continue
		}
		props[parts[0]] = parts[1]
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return props, nil
}

// SaveProperties overwrites the server.properties file with the given properties.
// It does not preserve comments or formatting of the original file.
func SaveProperties(path string, props map[string]string) error {

	fullPath := filepath.Join(path, fileName)
	// 1. Create file (truncate it if exists)
	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 2. Extract keys to a slice
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}

	// 3. Sort keys (deterministic output)
	sort.Strings(keys)

	// 4. Write loop
	for _, k := range keys {
		line := fmt.Sprintf("%s=%s\n", k, props[k])
		if _, err := file.WriteString(line); err != nil {
			return err
		}
	}
	return nil
}

// SaveProperties2 updates the server.properties file with the given changes.
// It preserves existing comments and structure, appending new properties at the end.
func SaveProperties2(path string, changes map[string]string) error {
	fullPath := filepath.Join(path, fileName)

	// 1. Read the WHOLE original file
	// We need the original content to preserve comments
	input, err := os.ReadFile(fullPath)
	if err != nil {
		// if file doesn't exist, just create i fresh (fallback to simple write)
		if os.IsNotExist(err) {
			return saveNewFile(fullPath, changes)
		}
		return err
	}

	var output bytes.Buffer

	// Track which keys we have processed so we can add NEW once at the end
	processedKeys := make(map[string]bool)

	scanner := bufio.NewScanner(bytes.NewReader(input))
	for scanner.Scan() {
		line := scanner.Text()
		cleanLine := strings.TrimSpace(line)
		if cleanLine == "" || strings.HasPrefix(cleanLine, "#") {
			output.WriteString(line + "\n")
			continue
		}

		// split line in to property and value
		parts := strings.SplitN(cleanLine, "=", 2)

		if len(parts) == 2 {
			key := parts[0]
			if newValue, exists := changes[key]; exists {
				output.WriteString(fmt.Sprintf("%s=%s\n", key, newValue))
				processedKeys[key] = true
			} else {
				output.WriteString(line + "\n")
			}
		} else {
			output.WriteString(line + "\n")
		}
	}

	// Append new keys that were not in the file
	for k, v := range changes {
		if !processedKeys[k] {
			output.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}

	return os.WriteFile(fullPath, output.Bytes(), 0644)
}

func saveNewFile(fullPath string, props map[string]string) error {
	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		line := fmt.Sprintf("%s=%s\n", k, props[k])
		if _, err := file.WriteString(line); err != nil {
			return err
		}
	}
	return nil
}
