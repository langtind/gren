package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/langtind/gren/internal/output"
)

// humanOutOverride redirects this package's human-facing chatter — phase
// summaries, prompts, progress notes. It is nil normally and os.Stderr while a
// --format=json command is running.
//
// Machine output never goes through here: a JSON payload is written straight to
// os.Stdout by emitJSON, which is the whole point of the split.
var humanOutOverride io.Writer

// humanOut resolves the sink at call time. Binding os.Stdout once at package
// init would ignore any later reassignment of it, which is precisely what the
// capture helpers in this package's tests do.
func humanOut() io.Writer {
	if humanOutOverride != nil {
		return humanOutOverride
	}
	return os.Stdout
}

// addFormatFlag registers the shared --format flag on fs. Every machine-readable
// command spells it the same way, so `--format=json` means the same thing across
// create, list, delete, and hook-run.
func addFormatFlag(fs *flag.FlagSet) *string {
	return fs.String("format", "", "Output format: json for machine-readable output")
}

// parseFormat resolves --format into a bool, rejecting anything we don't emit
// rather than silently falling back to human output — a caller that asked for
// json and got prose would otherwise discover the mistake by parse failure.
func parseFormat(format string) (bool, error) {
	switch format {
	case "":
		return false, nil
	case "json":
		return true, nil
	default:
		return false, fmt.Errorf("unsupported format %q; supported formats: json", format)
	}
}

// enterJSONMode hands stdout to the JSON payload alone: this package's chatter
// and everything the output package prints are both redirected to stderr for
// the duration. The returned function restores both.
//
// The invariant it buys — in JSON mode, stdout carries the payload and nothing
// else — is what makes the output parseable without the caller having to strip
// banners, warnings, or hook phase lines. Progress still reaches stderr, so it
// stays visible in a terminal and in captured logs.
//
// Callers use it as:
//
//	if jsonMode {
//	    defer enterJSONMode()()
//	}
func enterJSONMode() func() {
	prevHuman := humanOutOverride
	humanOutOverride = os.Stderr
	restoreOutput := output.SetStdout(os.Stderr)
	return func() {
		humanOutOverride = prevHuman
		restoreOutput()
	}
}

// emitJSON writes v to real stdout as indented JSON with a trailing newline.
// It deliberately takes os.Stdout rather than humanOut: this is the payload,
// and it must land on stdout even while every human-facing writer points at
// stderr.
func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
