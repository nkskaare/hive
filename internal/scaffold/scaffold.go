package scaffold

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/*
var templates embed.FS

// Init writes the default hive config and templates into the target directory
func Init(targetDir string) error {
	entries, err := templates.ReadDir("templates")
	if err != nil {
		return fmt.Errorf("reading embedded templates: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		dest := filepath.Join(targetDir, name)

		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("  ⏭  %s already exists, skipping\n", name)
			continue
		}

		data, err := templates.ReadFile("templates/" + name)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", name, err)
		}

		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", dest, err)
		}
		fmt.Printf("  ✅ Created %s\n", name)
	}

	return nil
}
