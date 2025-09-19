package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsGitRepo checks if current directory is a git repository
func IsGitRepo() bool {
	_, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	return err == nil
}

// GetRepoName returns the name of the current git repository
func GetRepoName() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	repoPath := strings.TrimSpace(string(output))
	return filepath.Base(repoPath)
}

// GetCurrentDirectory returns the current working directory name
func GetCurrentDirectory() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Base(wd)
}

// IsInitialized checks if gren has been initialized in this repo
func IsInitialized() bool {
	_, err := os.Stat(".wt")
	return err == nil
}