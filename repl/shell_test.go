package repl

import (
	"os"
	"os/exec"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "simple command",
			input: "echo hello",
			want:  []string{"echo", "hello"},
		},
		{
			name:  "command with multiple args",
			input: "ls -la /tmp",
			want:  []string{"ls", "-la", "/tmp"},
		},
		{
			name:  "double quoted string",
			input: `echo "hello world"`,
			want:  []string{"echo", "hello world"},
		},
		{
			name:  "single quoted string",
			input: `echo 'hello world'`,
			want:  []string{"echo", "hello world"},
		},
		{
			name:  "escaped space outside quotes",
			input: `echo hello\ world`,
			want:  []string{"echo", "hello world"},
		},
		{
			name:  "escaped quote in double quotes",
			input: `echo "hello \"world\""`,
			want:  []string{"echo", `hello "world"`},
		},
		{
			name:  "backslash literal in single quotes",
			input: `echo 'hello\world'`,
			want:  []string{"echo", `hello\world`},
		},
		{
			name:  "multiple spaces",
			input: "echo   hello    world",
			want:  []string{"echo", "hello", "world"},
		},
		{
			name:  "tabs and newlines",
			input: "echo\thello\nworld",
			want:  []string{"echo", "hello", "world"},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{},
		},
		{
			name:  "only whitespace",
			input: "   \t\n  ",
			want:  []string{},
		},
		{
			name:  "mixed quotes",
			input: `echo "hello" 'world' foo`,
			want:  []string{"echo", "hello", "world", "foo"},
		},
		{
			name:  "escaped backslash in double quotes",
			input: `echo "hello\\world"`,
			want:  []string{"echo", `hello\world`},
		},
		{
			name:  "escaped newline in double quotes",
			input: "echo \"hello\\nworld\"",
			want:  []string{"echo", "hellonworld"},
		},
		{
			name:    "unclosed double quote",
			input:   `echo "hello world`,
			wantErr: true,
		},
		{
			name:    "unclosed single quote",
			input:   `echo 'hello world`,
			wantErr: true,
		},
		{
			name:    "unterminated escape at end",
			input:   `echo hello\`,
			wantErr: true,
		},
		{
			name:  "complex command",
			input: `grep -r "search term" '/path/with spaces' --exclude='*.log'`,
			want:  []string{"grep", "-r", "search term", "/path/with spaces", "--exclude=*.log"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitCommand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SplitCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("SplitCommand() got %d parts, want %d parts\ngot:  %#v\nwant: %#v", len(got), len(tt.want), got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SplitCommand() part[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestShellExec_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		wantErr bool
	}{
		{
			name:    "empty command",
			cmd:     "",
			wantErr: true,
		},
		{
			name:    "only whitespace",
			cmd:     "   ",
			wantErr: true,
		},
		{
			name:    "unclosed quote",
			cmd:     `echo "hello`,
			wantErr: true,
		},
		{
			name:    "nonexistent command",
			cmd:     "this_command_definitely_does_not_exist_12345",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We expect these to return non-zero without calling exec
			ret := ShellExec(tt.cmd)
			if tt.wantErr && ret == 0 {
				t.Errorf("ShellExec() returned 0, expected non-zero error code")
			}
		})
	}
}

// TestShellExec_Success tests successful execution via subprocess
func TestShellExec_Success(t *testing.T) {
	// Check if we're being run as a subprocess
	if os.Getenv("GO_TEST_SUBPROCESS") == "1" {
		// This will replace the process, so the test never returns here on success
		ShellExec(os.Getenv("GO_TEST_SUBPROCESS_CMD"))
		return
	}

	tests := []struct {
		name     string
		cmd      string
		wantCode int
	}{
		{
			name:     "true command",
			cmd:      "/usr/bin/true",
			wantCode: 0,
		},
		{
			name:     "false command",
			cmd:      "/usr/bin/false",
			wantCode: 1,
		},
		{
			name:     "echo with args",
			cmd:      "/usr/bin/echo hello world",
			wantCode: 0,
		},
		{
			name:     "command with quotes",
			cmd:      `/usr/bin/echo "hello world"`,
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run the test in a subprocess
			cmd := exec.Command(os.Args[0], "-test.run=TestShellExec_Success/"+tt.name)
			cmd.Env = append(os.Environ(),
				"GO_TEST_SUBPROCESS=1",
				"GO_TEST_SUBPROCESS_CMD="+tt.cmd,
			)
			err := cmd.Run()

			// Check the exit code
			var gotCode int
			if err == nil {
				gotCode = 0
			} else if exitErr, ok := err.(*exec.ExitError); ok {
				gotCode = exitErr.ExitCode()
			} else {
				t.Fatalf("ShellExec(%q) unexpected error type: %v", tt.cmd, err)
			}

			if gotCode != tt.wantCode {
				t.Errorf("ShellExec(%q) exit code = %d, want %d", tt.cmd, gotCode, tt.wantCode)
			}
		})
	}
}
