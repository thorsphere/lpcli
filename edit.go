// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/thorsphere/tserr"
)

// Edit opens the initial text in the user's default editor and
// returns the result. It returns promptly if ctx is cancelled,
// cleaning up the temporary file.
// The editor subprocess is attached to the process's standard
// streams (os.Stdin/Stdout/Stderr), not the prompter's In/Out,
// because editors require a real terminal to render their UI.
func (p *Prompter) Edit(ctx context.Context, initialText string) (string, error) {
	// If the prompter is nil, return an error
	if p == nil {
		return "", tserr.NilPtr()
	}

	editorParts, err := getEditor()
	if err != nil {
		return "", err
	}

	// Create temporary file with initial text
	tmpFile, err := os.CreateTemp("", tmpFilePattern)
	if err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "create temporary file", Fn: tmpFilePattern, Err: err})
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(initialText); err != nil {
		tmpFile.Close()
		return "", tserr.Op(&tserr.OpArgs{Op: "write to temporary file", Fn: tmpPath, Err: err})
	}

	if err := tmpFile.Close(); err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "close temporary file", Fn: tmpPath, Err: err})
	}

	// Prepare the editor command
	args := make([]string, 0, len(editorParts))
	args = append(args, editorParts[1:]...)
	args = append(args, tmpPath)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, editorParts[0], args...)

	// Bind TTY. Editors need the process's real standard streams
	// (not p.in/p.out) so they can render their UI; the streams are
	// resolved through editorStreams so tests can substitute them.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = p.editorStreams()

	// Execute the editor command and wait for it to finish
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		switch {
		case errors.As(err, &exitErr) && exitErr.ExitCode() >= 0:
			// The editor ran and exited by itself with a non-zero
			// status (e.g. vim's :cq). Report the real error even if
			// ctx is done — the exit is the cause, not cancellation.
			return "", tserr.Op(&tserr.OpArgs{Op: "run editor", Fn: editorParts[0], Err: err})

		case ctx.Err() != nil:
			// The editor was killed by context cancellation (SIGKILL
			// from exec.CommandContext surfaces as a signal death,
			// i.e. ExitCode() == -1, or as a non-ExitError).
			return "", tserr.Aborted(p.Name)

		default:
			// Editor failed to start, or died from a signal unrelated
			// to our cancellation.
			return "", tserr.Op(&tserr.OpArgs{Op: "run editor", Fn: editorParts[0], Err: err})
		}
	}

	// Read the edited content from the temporary file
	editedBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "read edited file", Fn: tmpPath, Err: err})
	}
	// Return the edited content and nil to indicate success
	return string(editedBytes), nil
}

// getEditor resolves the editor command to use, following the POSIX
// convention: the VISUAL environment variable takes precedence over
// EDITOR (see POSIX 8.3, "Environment Variables": VISUAL is preferred
// for full-screen editors, EDITOR as a fallback for line editors).
// The value may contain arguments (e.g. "code --wait") and is split
// with shell-like quoting rules; if the first word is not an executable,
// the whole value is retried as a single unquoted path with spaces.
// If neither variable is set or usable, it falls back to a list of
// common editors (notepad on Windows; nano, vim, vi, emacs elsewhere).
// It returns a NotFound error if no editor can be found.
func getEditor() ([]string, error) {
	// 1. User-configured environment variables
	for _, env := range []string{"VISUAL", "EDITOR"} {
		val := strings.TrimSpace(os.Getenv(env))
		if val == "" {
			continue
		}
		parts, err := splitEditorCmd(val)
		if err != nil {
			return nil, err
		}
		if len(parts) == 0 {
			continue
		}
		if _, err := exec.LookPath(parts[0]); err == nil {
			return parts, nil
		}
		// Fallback: the whole value may be an unquoted path containing spaces.
		if _, err := exec.LookPath(val); err == nil {
			return []string{val}, nil
		}
	}

	// 2. OS-specific fallbacks (unchanged, but returned as slices)
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{"notepad", "notepad.exe"}
	} else {
		candidates = []string{"nano", "vim", "vi", "emacs"}
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate); err == nil {
			return []string{candidate}, nil
		}
	}

	return nil, tserr.NotFound("editor (VISUAL, EDITOR, or a fallback editor)")
}

// splitEditorCmd splits an editor command into words, honoring single
// quotes, double quotes, and (on non-Windows systems) backslash escapes.
// It is not a full shell: no variable expansion, globs, or command
// substitution — only the word-splitting needed for editor commands.
func splitEditorCmd(s string) ([]string, error) {
	isWindows := runtime.GOOS == "windows"

	var (
		words []string
		cur   strings.Builder
	)
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; r {
		case ' ', '\t', '\n':
			flush()

		case '\'': // literal until the closing quote
			j := i + 1
			for j < len(runes) && runes[j] != '\'' {
				cur.WriteRune(runes[j])
				j++
			}
			if j >= len(runes) {
				return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "editor command",
					Detail: fmt.Sprintf("unterminated single quote in %q", s),
				})
			}
			i = j

		case '"': // backslash may escape " and \ inside
			j := i + 1
			for j < len(runes) && runes[j] != '"' {
				if !isWindows && runes[j] == '\\' && j+1 < len(runes) &&
					(runes[j+1] == '"' || runes[j+1] == '\\') {
					cur.WriteRune(runes[j+1])
					j += 2
					continue
				}
				cur.WriteRune(runes[j])
				j++
			}
			if j >= len(runes) {
				return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "editor command",
					Detail: fmt.Sprintf("unterminated double quote in %q", s),
				})
			}
			i = j

		case '\\':
			// On Windows, backslash is a path separator, not an escape.
			if !isWindows && i+1 < len(runes) {
				cur.WriteRune(runes[i+1])
				i++
			} else {
				cur.WriteRune(r)
			}

		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return words, nil
}
