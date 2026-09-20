// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/thorsphere/tserr"
)

const (
	tmpFilePattern = "lpcli-edit-*.txt"
)

type Prompter struct {
	Name   string // Name of the cli tool
	in     io.Reader
	out    io.Writer // usually os.Stderr for interactive prompts
	reader *bufio.Reader

	// Streams attached to the editor subprocess spawned by Edit.
	// Editors need a real TTY, so these default to the process's
	// standard streams rather than p.in/p.out. They are unexported
	// hooks: tests may substitute them via setEditorStreams.
	editorStdin  io.Reader
	editorStdout io.Writer
	editorStderr io.Writer
}

func NewPrompter(name string) *Prompter {
	return &Prompter{
		Name: name,
		in:   os.Stdin,
		out:  os.Stderr,
	}
}

func (p *Prompter) getReader() *bufio.Reader {
	in := p.in
	if in == nil {
		in = os.Stdin
	}
	if p.reader == nil {
		p.reader = bufio.NewReader(in)
	}
	return p.reader
}

func (p *Prompter) In() io.Reader { return p.in }

// SetIn changes the input source and discards any buffered reader, so
// subsequent reads come from r. Prefer this over assigning In directly once
// the prompter has been used, since a cached reader would otherwise keep
// reading from the previous source. Passing nil restores os.Stdin.
func (p *Prompter) SetIn(r io.Reader) {
	p.in = r
	p.reader = nil
}

func (p *Prompter) Out() io.Writer {
	if p.out == nil {
		return os.Stderr
	}
	return p.out
}

// SetOut changes the writer used for prompt output. Passing nil restores
// os.Stderr.
func (p *Prompter) SetOut(w io.Writer) { p.out = w }

// Confirm prompts the user for a yes/no confirmation. Pressing Enter
// (empty input) is treated as "yes", matching the low-stakes nature of
// the operations this package is designed for; callers guarding
// destructive actions should require an explicit "y". Returns
// tserr.Aborted on "n"/"no" or when ctx is cancelled.
func (p *Prompter) Confirm(ctx context.Context, message string) error {
	// If the prompter is nil, return an error
	if p == nil {
		return tserr.NilPtr()
	}

	for {
		// Check if context was cancelled
		if err := ctx.Err(); err != nil {
			// Prompt was cancelled by context, return an error
			return tserr.Aborted(p.Name)
		}

		fmt.Fprint(p.Out(), message)
		choice, err := p.readLine()
		if err != nil {
			return err
		}

		switch strings.ToLower(choice) {
		case "y", "yes", "":
			return nil
		case "n", "no":
			return tserr.Aborted(p.Name)
		default:
			fmt.Fprintf(p.Out(), "Unknown option %q. Please choose [y/n].\n", choice)
		}
	}
}

// readLine reads a line of input, handling non-newline-terminated EOF cleanly.
func (p *Prompter) readLine() (string, error) {
	reader := p.getReader()
	if reader == nil {
		return "", tserr.NilParam("reader")
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			trimmed := strings.TrimSpace(input)
			// If user provided input before EOF, process it
			if trimmed != "" {
				return trimmed, nil
			}
			// If EOF was reached without input, abort cleanly
			return "", tserr.Aborted(p.Name)
		}
		return "", err
	}

	return strings.TrimSpace(input), nil
}

// setEditorStreams overrides the streams attached to the editor
// subprocess spawned by Edit. Intended for tests; passing nil for
// any argument restores the corresponding os.Std* stream.
func (p *Prompter) setEditorStreams(stdin io.Reader, stdout, stderr io.Writer) {
	p.editorStdin = stdin
	p.editorStdout = stdout
	p.editorStderr = stderr
}

// editorStreams returns the streams to attach to the editor subprocess,
// falling back to the process's standard streams when unset.
func (p *Prompter) editorStreams() (io.Reader, io.Writer, io.Writer) {
	stdin := io.Reader(os.Stdin)
	if p.editorStdin != nil {
		stdin = p.editorStdin
	}
	stdout := io.Writer(os.Stdout)
	if p.editorStdout != nil {
		stdout = p.editorStdout
	}
	stderr := io.Writer(os.Stderr)
	if p.editorStderr != nil {
		stderr = p.editorStderr
	}
	return stdin, stdout, stderr
}
