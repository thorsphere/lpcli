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
	"reflect"
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
	src    io.Reader
}

func NewPrompter(name string) *Prompter {
	return &Prompter{
		Name: name,
		In:   os.Stdin,
		Out:  os.Stderr,
		src:  os.Stdin, // keep src in sync with In; reader is created lazily by getReader()
	}
}

func (p *Prompter) getReader() *bufio.Reader {
	in := p.In
	if in == nil {
		in = os.Stdin
	}

	if p.reader == nil || !sameReader(p.src, in) {
		p.reader = bufio.NewReader(in)
		p.src = in
	}

	return p.reader
}

// sameReader reports whether two io.Reader values refer to the same source.
// Direct interface comparison panics when the dynamic type is uncomparable
// (e.g. a struct containing a slice), so differing or uncomparable types are
// conservatively reported as different, which forces a rebuild.
func sameReader(a, b io.Reader) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	if ta != tb || !ta.Comparable() {
		return false
	}
	return a == b
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
