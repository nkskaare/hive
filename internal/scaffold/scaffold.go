package scaffold

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/hive.toml
var configTemplate []byte

// Init writes the default hive.toml into the target directory
func Init(targetDir string) error {
	dest := filepath.Join(targetDir, "hive.toml")

	if _, err := os.Stat(dest); err == nil {
		fmt.Println("  ⏭  hive.toml already exists, skipping")
		return nil
	}

	if err := os.WriteFile(dest, configTemplate, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}
	fmt.Println("  ✅ Created hive.toml")
	return nil
}
