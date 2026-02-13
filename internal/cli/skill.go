package cli

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/langtind/gren/internal/output"
)

var (
	skillFS      fs.FS
	skillDirName string
	skillName    string
)

// SetSkillFS sets the embedded filesystem for skill installation
func SetSkillFS(fsys fs.FS, dirName, name string) {
	skillFS = fsys
	skillDirName = dirName
	skillName = name
}

type fileEntry struct {
	relPath string
	embPath string
}

// walkSkillFiles walks the embedded skill directory and returns all files
func walkSkillFiles() ([]fileEntry, error) {
	if skillFS == nil {
		return nil, fmt.Errorf("no skill files embedded")
	}

	var files []fileEntry
	err := fs.WalkDir(skillFS, skillDirName, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(skillDirName, path)
		files = append(files, fileEntry{relPath: rel, embPath: path})
		return nil
	})
	return files, err
}

// detectExistingFiles checks which files already exist in destination
func detectExistingFiles(destDir string) []string {
	files, err := walkSkillFiles()
	if err != nil {
		return nil
	}

	var existing []string
	for _, f := range files {
		dest := filepath.Join(destDir, f.relPath)
		if _, err := os.Stat(dest); err == nil {
			existing = append(existing, f.relPath)
		}
	}
	return existing
}

// installSkillToPath installs skill files to the specified path
func installSkillToPath(destDir string, force bool) error {
	if skillFS == nil {
		return fmt.Errorf("no skill files embedded")
	}

	files, err := walkSkillFiles()
	if err != nil {
		return fmt.Errorf("reading embedded files: %w", err)
	}

	// Check for existing files
	existing := detectExistingFiles(destDir)

	// If files exist and not force, prompt for confirmation
	if len(existing) > 0 && !force {
		output.Infof("Installing to %s", destDir)
		for _, f := range files {
			marker := ""
			for _, e := range existing {
				if e == f.relPath {
					marker = output.Dim(" (exists)")
					break
				}
			}
			fmt.Printf("  %s%s\n", f.relPath, marker)
		}

		fmt.Printf("\n%s Overwrite %d existing file(s)? [y/N] ", output.Dim("?"), len(existing))
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(answer)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Install files
	for _, f := range files {
		dest := filepath.Join(destDir, f.relPath)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		data, err := fs.ReadFile(skillFS, f.embPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", f.relPath, err)
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", f.relPath, err)
		}
	}

	output.Successf("Installed %d file(s) to %s", len(files), destDir)
	return nil
}

// installSkillCmd handles the install-skill command
func (c *CLI) installSkillCmd(args []string) error {
	var pathFlag string
	var force bool

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p", "--path":
			if i+1 < len(args) {
				pathFlag = args[i+1]
				i++
			}
		case "-f", "--force":
			force = true
		case "-h", "--help":
			output.Info("Usage: gren install-skill [options]")
			fmt.Println()
			fmt.Println("Install the Claude Code skill for gren to ~/.claude/skills/gren/")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  -p, --path <dir>   Parent directory (default: ~/.claude/skills/)")
			fmt.Println("  -f, --force        Overwrite existing files without prompting")
			fmt.Println("  -h, --help         Show this help")
			return nil
		}
	}

	baseDir := pathFlag
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("finding home directory: %w", err)
		}
		baseDir = filepath.Join(home, ".claude", "skills")
	}

	destDir := filepath.Join(baseDir, skillName)
	return installSkillToPath(destDir, force)
}
