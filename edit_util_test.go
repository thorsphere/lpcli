// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli_test

// Import packages for testing.
import (
	"fmt"           // fmt
	"os"            // os
	"path/filepath" // filepath
	"runtime"       // runtime
	"testing"       // testing
	"time"          // time

	"github.com/thorsphere/lpcli" // lpcli
	"github.com/thorsphere/tserr" // tserr
)

// The environment variables that control TestHelperEditor when it
// is re-executed as the fake editor subprocess.
const (
	helperGate  = "LPCLI_HELPER_GATE" // set when re-executed as editor
	helperMode  = "LPCLI_HELPER_MODE" // selects the editor behavior
	helperText  = "LPCLI_HELPER_TEXT" // text the editor appends
	writeStdout = "stdout "           // editor writes to stdout
	writeStderr = "stderr"            // editor writes to stderr
)

// wantFor returns the expected words for the platform the tests run
// on: nonWindows on Unix-like systems, windows on Windows. Backslash
// handling in splitEditorCmd is platform-dependent, so the table
// cases that exercise it need per-platform expectations.
func wantFor(nonWindows, windows []string) []string {
	// Check if the system is Windows
	if runtime.GOOS == "windows" {
		// Return the Windows expectation
		return windows
	}
	// Return the non-Windows expectation
	return nonWindows
}

// errFor returns the expected error for the platform the tests run
// on: nonWindows on Unix-like systems, windows on Windows.
func errFor(nonWindows, windows error) error {
	// Check if the system is Windows
	if runtime.GOOS == "windows" {
		// Return the Windows expectation
		return windows
	}
	// Return the non-Windows expectation
	return nonWindows
}

// fakeEditor creates an executable file named name inside dir and
// returns its path, so exec.LookPath can resolve it. On Windows the
// name is given an .exe extension because LookPath there only
// resolves files with a PATHEXT extension; on Unix the file is made
// executable. The file is never executed, only resolved.
func fakeEditor(t *testing.T, dir, name string) string {
	// Mark the function as a test helper
	t.Helper()
	// Check if the system is Windows
	if runtime.GOOS == "windows" {
		// Append the executable extension to the name
		name += ".exe"
	}
	// Join the directory and the name
	path := filepath.Join(dir, name)
	// Write a minimal script to the file
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		// Fail if the file cannot be written
		t.Fatal(tserr.Op(&tserr.OpArgs{Op: "WriteFile", Fn: path, Err: err}))
	}
	// Return the path of the fake editor
	return path
}

// fallbackCandidates returns the OS-specific fallback candidate
// list getEditor uses, mirroring the selection in edit.go:
// EditorsWindows on Windows, EditorsUnix elsewhere. Tests derive
// their expectations from it so they stay in sync with the
// production lists.
func fallbackCandidates() []string {
	// Check if the system is Windows
	if runtime.GOOS == "windows" {
		// Return the Windows candidates
		return lpcli.EditorsWindows
	}
	// Return the Unix candidates
	return lpcli.EditorsUnix
}

// helperEditorCmd returns a VISUAL value that re-executes the test
// binary as the fake editor. The binary path is wrapped in raw
// double quotes — not %q, which would escape backslashes and break
// Windows paths — so splitEditorCmd keeps it as one word.
func helperEditorCmd() string {
	// Quote the binary path and append the test filter
	return "\"" + os.Args[0] + "\" -test.run=TestHelperEditor --"
}

// TestHelperEditor is not a real test: it is the entry point of the
// fake editor subprocess spawned by the Edit tests. The test binary
// is re-executed with -test.run=TestHelperEditor and behaves like
// an editor according to helperMode; Edit passes the temporary file
// as the last argument. When the suite runs normally the gate is
// unset and the function returns immediately. This is the
// helper-process pattern used by os/exec's own tests.
func TestHelperEditor(t *testing.T) {
	// Return when not spawned as a helper
	if os.Getenv(helperGate) == "" {
		return
	}
	// The temporary file is the last argument
	path := os.Args[len(os.Args)-1]
	// Behave according to the mode
	switch os.Getenv(helperMode) {
	case "append": // append text to the file
		// Open the file for appending
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
		// Exit if the file cannot be opened
		if err != nil {
			fmt.Fprintln(os.Stderr, tserr.Op(&tserr.OpArgs{Op: "open temp file", Fn: path, Err: err}))
			os.Exit(2)
		}
		// Append the text
		if _, err := f.WriteString(os.Getenv(helperText)); err != nil {
			// Exit if the file cannot be written
			fmt.Fprintln(os.Stderr, tserr.Op(&tserr.OpArgs{Op: "write temp file", Fn: path, Err: err}))
			os.Exit(2)
		}
		// Close the file
		f.Close()
	case "echo": // write to the attached streams
		fmt.Fprint(os.Stdout, writeStdout)
		fmt.Fprint(os.Stderr, writeStderr)
	case "sleep": // block long enough to be cancelled
		time.Sleep(10 * time.Second)
	case "fail": // exit non-zero by itself
		os.Exit(3)
	}
	// Exit successfully
	os.Exit(0)
}
