package util

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// CmdSpec describes a subprocess to run.
type CmdSpec struct {
	Path    string   // Binary path
	Args    []string // Arguments
	Env     []string // Optional environment variables (KEY=VALUE). If nil, inherit.
	Dir     string   // Working directory; empty = inherit.
	Verbose bool     // Stream stdout/stderr while capturing
}

// CmdResult contains captured output and exit status.
type CmdResult struct {
	Stdout []byte
	Stderr []byte
	Code   int
	Err    error
}

// Run executes the command, optionally streaming output if Verbose is true.
// It always captures stdout and stderr. On non-zero exit, returns an error
// explaining the exit code, while also populating CmdResult.Code and buffers.
func Run(ctx context.Context, spec CmdSpec) (CmdResult, error) {
	var stdoutBuf, stderrBuf bytes.Buffer

	cmd := exec.CommandContext(ctx, spec.Path, spec.Args...)
	if spec.Dir != "" {
		cmd.Dir = spec.Dir
	}
	if spec.Env != nil {
		cmd.Env = append(os.Environ(), spec.Env...)
	}

	// Prepare stdout/stderr writers
	var stdoutW io.Writer = &stdoutBuf
	var stderrW io.Writer = &stderrBuf
	if spec.Verbose {
		stdoutW = io.MultiWriter(&stdoutBuf, os.Stdout)
		stderrW = io.MultiWriter(&stderrBuf, os.Stderr)
		// Print the command line
		fmt.Fprintf(os.Stderr, "+ %s\n", shellQuote(spec.Path, spec.Args))
	}
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	err := cmd.Start()
	if err != nil {
		return CmdResult{Stdout: stdoutBuf.Bytes(), Stderr: stderrBuf.Bytes(), Code: -1, Err: err}, err
	}

	waitErr := cmd.Wait()
	code := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}

	res := CmdResult{
		Stdout: stdoutBuf.Bytes(),
		Stderr: stderrBuf.Bytes(),
		Code:   code,
		Err:    waitErr,
	}

	if waitErr != nil {
		// Return a descriptive error but keep captured outputs.
		return res, fmt.Errorf("command failed (exit %d): %w", code, waitErr)
	}
	return res, nil
}

// shellQuote returns a printable shell-like command string for logging.
func shellQuote(path string, args []string) string {
	b := &strings.Builder{}
	b.WriteString(quote(path))
	for _, a := range args {
		b.WriteByte(' ')
		b.WriteString(quote(a))
	}
	return b.String()
}

func quote(s string) string {
	if s == "" {
		return "''"
	}
	// Simple quoting: wrap in single quotes and escape existing single quotes.
	if strings.ContainsAny(s, " \t\n\"'\\$`(){}[]*&;|<>?!") {
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	}
	return s
}
