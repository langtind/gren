package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/langtind/gren/internal/logging"
)

// handleLogs implements `gren logs`.
func (c *CLI) handleLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	pathOnly := fs.Bool("path", false, "Print only the log file path")
	follow := fs.Bool("f", false, "Follow the log (like tail -f)")
	fs.BoolVar(follow, "follow", false, "Follow the log (like tail -f)")
	last := fs.Bool("last", false, "Print the last error block and its hook-output pointer")
	fs.BoolVar(last, "errors", false, "Alias for --last")
	hooks := fs.Bool("hooks", false, "List recent per-run hook output logs")
	n := fs.Int("n", 50, "Number of trailing lines to print")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := logging.GetLogPath()
	if path == "" {
		return fmt.Errorf("logging not initialised; no log path")
	}
	if *pathOnly {
		fmt.Println(path)
		return nil
	}
	if *hooks {
		return listHookLogs()
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read log: %w", err)
	}
	if *last {
		// Prefer the full captured output of the last failed hook — that's the
		// actual cause, not just a pointer to a file. Fall back to the last error
		// block for non-hook errors (e.g. a bad CLI invocation).
		if p := lastFailedHookLogPath(string(content)); p != "" {
			if data, readErr := os.ReadFile(p); readErr == nil {
				fmt.Printf("Last hook failure — captured output (%s):\n\n%s", p, string(data))
				return nil
			}
		}
		fmt.Println(lastErrorBlock(string(content)))
		return nil
	}
	for _, line := range tailLines(string(content), *n) {
		fmt.Println(line)
	}
	if *follow {
		return followFile(path)
	}
	return nil
}

// tailLines returns the last n lines of s, in order.
func tailLines(s string, n int) []string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// lastFailedHookLogPath returns the per-run hook log path from the most recent
// "hook full output → <path>" line (logged only when a hook fails), or "" if
// there is none.
func lastFailedHookLogPath(s string) string {
	const marker = "full output → "
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if idx := strings.Index(lines[i], marker); idx != -1 {
			return strings.TrimSpace(lines[i][idx+len(marker):])
		}
	}
	return ""
}

// lastErrorBlock returns the last "[ERROR]" line plus any non-timestamped
// continuation lines (captured stdout/stderr) that follow it, or a friendly
// message if there are none. Timestamped lines start with "[20".
func lastErrorBlock(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	idx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "[ERROR]") {
			idx = i
			break
		}
	}
	if idx == -1 {
		return "no [ERROR] entries in log"
	}
	end := idx + 1
	for end < len(lines) && !strings.HasPrefix(lines[end], "[20") {
		end++
	}
	return strings.Join(lines[idx:end], "\n")
}

// followFile prints new lines appended to path until interrupted.
func followFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			fmt.Print(line)
			continue
		}
		if err != nil {
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// listHookLogs prints the paths of recent per-run hook output logs.
func listHookLogs() error {
	dir := logging.HookLogDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("no hook logs yet")
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			fmt.Println(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}
