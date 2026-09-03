// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/thorsphere/tserr"
)

const (
	tmpFilePattern = "lpcli-edit-*.txt"
)

type Prompter struct {
	Name string // Name of the cli tool
	In   io.Reader
	Out  io.Writer // usually os.Stderr for interactive prompts
}

func NewPrompter(name string) *Prompter {
	return &Prompter{
		Name: name,
		In:   os.Stdin,
		Out:  os.Stderr,
	}
}

func (p *Prompter) Confirm(reader *bufio.Reader, message string) error {
	fmt.Fprint(p.Out, message)
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	choice := strings.ToLower(strings.TrimSpace(input))
	switch choice {
	case "y", "yes", "":
		return nil
	case "n", "no":
		return tserr.Aborted(p.Name)
	default:
		fmt.Fprintf(p.Out, "Unknown option %q. Please choose [y/n].\n", choice)
		return p.Confirm(reader, message)
	}
}

// Edit opens the initial text in the user's default editor and returns the result.
func (p *Prompter) Edit(initialText string) (string, error) {
	editor, err := getEditor()
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
	parts := strings.Fields(editor)
	args := append(parts[1:], tmpPath)
	cmd := exec.Command(parts[0], args...)

	// Bind TTY
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the editor command and wait for it to finish
	if err := cmd.Run(); err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "run editor", Fn: editor, Err: err})
	}

	// Read the edited content from the temporary file
	editedBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "read edited file", Fn: tmpPath, Err: err})
	}

	return string(editedBytes), nil
}

func getEditor() (string, error) {
	// 1. Check user-configured environment variables
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if val := strings.TrimSpace(os.Getenv(env)); val != "" {
			parts := strings.Fields(val)
			if len(parts) > 0 {
				if _, err := exec.LookPath(parts[0]); err == nil {
					return val, nil
				}
			}
		}
	}

	// 2. Fall back to OS-specific candidate editors
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{"notepad", "notepad.exe"}
	} else {
		candidates = []string{"nano", "vim", "vi", "emacs"}
	}

	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", tserr.NotFound("editor (VISUAL or EDITOR)")
}
