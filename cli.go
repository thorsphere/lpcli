// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli

import (
	"bufio"
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
	In     io.Reader
	Out    io.Writer // usually os.Stderr for interactive prompts
	reader *bufio.Reader
}

func NewPrompter(name string) *Prompter {
	return &Prompter{
		Name:   name,
		In:     os.Stdin,
		Out:    os.Stderr,
		reader: bufio.NewReader(os.Stdin),
	}
}

// Helper to ensure reader stays in sync even if p.In was changed after construction
func (p *Prompter) getReader() *bufio.Reader {
	if p.reader == nil {
		in := p.In
		if in == nil {
			in = os.Stdin
		}
		p.reader = bufio.NewReader(in)
	}
	return p.reader
}

func (p *Prompter) Confirm(message string) error {
    // If the prompter is nil, return an error
    if p == nil {
        return tserr.NilPtr()
    }

    for {
        fmt.Fprint(p.Out, message)
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
            fmt.Fprintf(p.Out, "Unknown option %q. Please choose [y/n].\n", choice)
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
