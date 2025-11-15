package util

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// CmdSpec describes a subprocess to run.
type CmdSpec struct {
	Path    string   // Binary path
	Args    []string // Arguments
	Env     []string // Optional environment variables (KEY=VALUE). If nil, inherit.
	Dir     string   // Working directory; empty = inherit.
	Verbose bool     // Stream stdout/stderr while capturing

	// New options for per-line streaming and memory control:
	StdoutLine    func(string) // Called for each stdout line (if non-nil)
	StderrLine    func(string) // Called for each stderr line (if non-nil)
	CaptureStdout bool         // When false, do not buffer stdout into CmdResult (still invoke StdoutLine)
}

// CmdResult contains captured output and exit status.
type CmdResult struct {
	Stdout []byte
	Stderr []byte
	Code   int
	Err    error
}

// Run executes the command, optionally streaming output if Verbose is true.
// It always captures stderr. Stdout capture can be disabled with CaptureStdout=false.
// On non-zero exit, returns an error describing the exit code, while also
// populating CmdResult.Code and captured buffers.
func Run(ctx context.Context, spec CmdSpec) (CmdResult, error) {
	var stdoutBuf, stderrBuf bytes.Buffer

	cmd := exec.CommandContext(ctx, spec.Path, spec.Args...)
	if spec.Dir != "" {
		cmd.Dir = spec.Dir
	}
	if spec.Env != nil {
		cmd.Env = append(os.Environ(), spec.Env...)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return CmdResult{Stdout: nil, Stderr: nil, Code: -1, Err: err}, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return CmdResult{Stdout: nil, Stderr: nil, Code: -1, Err: err}, err
	}

	if spec.Verbose {
		// Print the command line before execution
		fmt.Fprintf(os.Stderr, "+ %s\n", shellQuote(spec.Path, spec.Args))
	}

	if err := cmd.Start(); err != nil {
		return CmdResult{Stdout: nil, Stderr: nil, Code: -1, Err: err}, err
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// stdout reader goroutine
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stdoutPipe)
		// Increase buffer size to handle large JSON outputs (e.g., yt-dlp --dump-json)
		// Default is 64KB, but YouTube metadata can be 500KB+
		const maxCapacity = 1024 * 1024 // 1 MB
		buf := make([]byte, 0, 64*1024)  // initial buffer
		sc.Buffer(buf, maxCapacity)
		for sc.Scan() {
			line := sc.Text()
			// Invoke callback first so real-time consumers see it
			if spec.StdoutLine != nil {
				spec.StdoutLine(line)
			}
			// Verbose streaming to terminal
			if spec.Verbose {
				fmt.Fprintln(os.Stdout, line)
			}
			// Optional capture to buffer
			if spec.CaptureStdout || spec.StdoutLine == nil {
				stdoutBuf.WriteString(line)
				stdoutBuf.WriteByte('\n')
			}
		}
		// If the scanner errors, preserve it in buffers for debugging
		if err := sc.Err(); err != nil {
			// Do not fail outright; command exit will reflect errors
			if spec.Verbose {
				fmt.Fprintf(os.Stderr, "stdout scan error: %v\n", err)
			}
		}
	}()

	// stderr reader goroutine
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stderrPipe)
		// Increase buffer size for large stderr outputs
		const maxCapacity = 1024 * 1024 // 1 MB
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, maxCapacity)
		for sc.Scan() {
			line := sc.Text()
			if spec.StderrLine != nil {
				spec.StderrLine(line)
			}
			if spec.Verbose {
				fmt.Fprintln(os.Stderr, line)
			}
			// Always capture stderr
			stderrBuf.WriteString(line)
			stderrBuf.WriteByte('\n')
		}
		if err := sc.Err(); err != nil {
			if spec.Verbose {
				fmt.Fprintf(os.Stderr, "stderr scan error: %v\n", err)
			}
		}
	}()

	waitErr := cmd.Wait()
	// Ensure readers drain remaining data
	wg.Wait()

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
