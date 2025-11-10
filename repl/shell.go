package repl

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"fortio.org/log"
)

// SplitCommand splits a command string into parts, handling quoted strings.
// Supports single quotes (no escaping), double quotes (backslash escaping), and basic escaping with backslash.
// Returns an error if quotes are unclosed or escape sequences are unterminated.
func SplitCommand(cmd string) ([]string, error) {
	var parts []string
	var current strings.Builder
	inQuote := rune(0)
	escaped := false

	for _, r := range cmd {
		// Handle escaping - but only outside single quotes or inside double quotes
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		// Backslash handling depends on quote context
		if r == '\\' {
			if inQuote == '\'' {
				// Inside single quotes, backslash is literal
				current.WriteRune(r)
			} else {
				// Outside quotes or inside double quotes, backslash escapes next char
				escaped = true
			}
			continue
		}
		// Inside a quoted string
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		// Start of a quoted string
		if r == '"' || r == '\'' {
			inQuote = r
			continue
		}
		// Whitespace outside quotes separates arguments
		if r == ' ' || r == '\t' || r == '\n' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	// Check for unterminated escape sequence
	if escaped {
		return nil, errors.New("unterminated escape sequence: command ends with backslash")
	}
	// Check for unclosed quotes
	if inQuote != 0 {
		return nil, fmt.Errorf("unclosed quote: missing closing %c", inQuote)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts, nil
}

// ShellExec executes a shell command by replacing the current process using syscall.Exec.
// This function does not return if the exec is successful.
func ShellExec(cmd string) int {
	parts, err := SplitCommand(cmd)
	if err != nil {
		return log.FErrf("Error parsing command: %v", err)
	}
	if len(parts) == 0 {
		return log.FErrf("No command provided for exec")
	}
	execCmd, err := exec.LookPath(parts[0])
	if err != nil {
		return log.FErrf("Error finding command %q: %v", parts[0], err)
	}
	execArgs := parts[1:]
	log.Infof("Executing command: %s %d args (%v)", execCmd, len(execArgs), execArgs)
	env := syscall.Environ() // inherit current environment
	err = syscall.Exec(execCmd, append([]string{execCmd}, execArgs...), env)
	if err != nil {
		return log.FErrf("Error exec'ing process %q: %v", execCmd, err)
	}
	// not reached if Exec is successful
	panic("unreachable")
}
