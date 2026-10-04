// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli

// Import packages
import (
	"context" // context
	"errors"  // errors
	"fmt"     // fmt
	"os"      // os
	"os/exec" // exec
	"runtime" // runtime
	"strings" // strings

	"github.com/thorsphere/tserr" // tserr
)

// editorsWindows and editorsUnix are the lists of editors for
// Windows and Unix-like systems, respectively. They are fallbacks
// for the VISUAL and EDITOR environment variables, if both are unset.
var (
	editorsWindows = []string{"notepad", "notepad.exe"}     // Windows
	editorsUnix    = []string{"nano", "vim", "vi", "emacs"} // Unix-like systems
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
	// Get the editor command
	editorParts, err := getEditor()
	// Check if an error occurred
	if err != nil {
		// Return the error
		return "", err
	}
	// Create temporary file with initial text
	tmpFile, err := os.CreateTemp("", tmpFilePattern)
	// Check if an error occurred
	if err != nil {
		// Return the error
		return "", tserr.Op(&tserr.OpArgs{Op: "create temporary file", Fn: tmpFilePattern, Err: err})
	}
	// Get the path of the temporary file
	tmpPath := tmpFile.Name()
	// Defer the removal of the temporary file
	defer os.Remove(tmpPath)
	// Write the initial text to the temporary file
	if _, err := tmpFile.WriteString(initialText); err != nil {
		// Close the temporary file
		tmpFile.Close()
		// Return the error
		return "", tserr.Op(&tserr.OpArgs{Op: "write to temporary file", Fn: tmpPath, Err: err})
	}
	// Close the temporary file
	if err := tmpFile.Close(); err != nil {
		// Return the error, if any
		return "", tserr.Op(&tserr.OpArgs{Op: "close temporary file", Fn: tmpPath, Err: err})
	}
	// Prepare the editor command
	// Create the list of arguments
	args := make([]string, 0, len(editorParts))
	// Append the editor arguments
	args = append(args, editorParts[1:]...)
	// Append the path of the temporary file
	args = append(args, tmpPath)
	// Create a context with a cancel function
	ctx, cancel := context.WithCancel(ctx)
	// Defer the cancellation
	defer cancel()
	// Create the editor command
	cmd := exec.CommandContext(ctx, editorParts[0], args...)
	// Bind TTY. Editors need the process's real standard streams
	// (not p.in/p.out) so they can render their UI; the streams are
	// resolved through editorStreams so tests can substitute them.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = p.editorStreams()
	// Execute the editor command and wait for it to finish
	if err := cmd.Run(); err != nil {
		// Create a variable to hold the ExitError
		var exitErr *exec.ExitError
		// Check if the error is an ExitError
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
	// Check if an error occurred
	if err != nil {
		// Return the error
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
	// VISUAL takes precedence over EDITOR.
	for _, env := range []string{"VISUAL", "EDITOR"} {
		// Get the value of the environment variable.
		val := strings.TrimSpace(os.Getenv(env))
		// Skip if the value is empty.
		if val == "" {
			continue
		}
		// Split the value into words.
		parts, err := splitEditorCmd(val)
		// Skip if the value is not a valid editor command.
		if err != nil {
			return nil, err
		}
		// Skip if the value is empty.
		if len(parts) == 0 {
			continue
		}
		// Check if the first word is an executable.
		if _, err := exec.LookPath(parts[0]); err == nil {
			return parts, nil
		}
		// Fallback: the whole value may be an unquoted path containing spaces.
		if _, err := exec.LookPath(val); err == nil {
			return []string{val}, nil
		}
	}
	// 2. OS-specific fallbacks (unchanged, but returned as slices)
	// Create the list of candidates.
	var candidates []string
	// Windows: notepad, notepad.exe
	if runtime.GOOS == "windows" {
		candidates = editorsWindows
	} else { // Unix-like systems: nano, vim, vi, emacs
		candidates = editorsUnix
	}
	// Iterate over the candidates
	for _, candidate := range candidates {
		// Check if the candidate is an executable.
		if _, err := exec.LookPath(candidate); err == nil {
			// Return the candidate, if it is.
			return []string{candidate}, nil
		}
	}
	// If no candidate is found, return a NotFound error.
	return nil, tserr.NotFound("editor (VISUAL, EDITOR, or a fallback editor)")
}

// splitEditorCmd splits an editor command into words, honoring single
// quotes, double quotes, and (on non-Windows systems) backslash escapes.
// It is not a full shell: no variable expansion, globs, or command
// substitution — only the word-splitting needed for editor commands.
func splitEditorCmd(s string) ([]string, error) {
	// Check if system is Windows
	isWindows := runtime.GOOS == "windows"
	var (
		words []string        // The resulting slice of words
		cur   strings.Builder // The current word being built
	)
	// Define a function to flush the current word to the slice of words
	flush := func() {
		// If the current word is not empty, append it to the slice
		if cur.Len() > 0 {
			// Append the current word to the slice
			words = append(words, cur.String())
			// Reset the current word
			cur.Reset()
		}
	}
	// Define a slice of runes from the string
	runes := []rune(s)
	// Loop over the runes
	for i := 0; i < len(runes); i++ {
		// Switch on the current rune
		switch r := runes[i]; r {
		case ' ', '\t', '\n': // Space, tab, or newline
			flush() // Flush the current word
		case '\'': // literal until the closing quote
			// Find the closing quote
			j := i + 1
			// Loop over the runes until the closing quote
			for j < len(runes) && runes[j] != '\'' {
				// Write the current rune to the current word
				cur.WriteRune(runes[j])
				// Increment the index
				j++
			}
			// Check if the index is out of bounds
			if j >= len(runes) {
				// Return an error
				return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "editor command",
					Detail: fmt.Sprintf("unterminated single quote in %q", s),
				})
			}
			// Set the index to the closing quote
			i = j
		case '"': // backslash may escape " and \ inside
			// Find the closing quote
			j := i + 1
			// Loop over the runes until the closing quote
			for j < len(runes) && runes[j] != '"' {
				// If not on Windows, check if the backslash is followed by a backslash or a double quote
				if !isWindows && runes[j] == '\\' && j+1 < len(runes) &&
					(runes[j+1] == '"' || runes[j+1] == '\\') {
					// Write the backslash or double quote to the current word
					cur.WriteRune(runes[j+1])
					// Increment the index
					j += 2
					// Continue the loop
					continue
				}
				// Write the current rune to the current word, if not a backslash
				cur.WriteRune(runes[j])
				// Increment the index
				j++
			}
			// Check if the index is out of bounds
			if j >= len(runes) {
				// Return an error
				return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "editor command",
					Detail: fmt.Sprintf("unterminated double quote in %q", s),
				})
			}
			// Set the index to the closing quote
			i = j
		case '\\': // On Windows, backslash is a path separator, not an escape.
			// Check if not on Windows and the next rune is not outside the slice
			if !isWindows && i+1 < len(runes) {
				// Write the next rune to the current word
				cur.WriteRune(runes[i+1])
				// Increment the index
				i++
			} else { // If on Windows write the backslash
				cur.WriteRune(r)
			}
		default: // In all other cases, write the current rune to the current word
			cur.WriteRune(r)
		}
	}
	// Flush the current word
	flush()
	// Return the resulting slice of words
	return words, nil
}
