// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli_test

// Import packages for testing.
import (
	"errors"  // errors
	"io"      // io
	"strconv" // strconv
	"strings" // strings
	"testing" // testing

	"github.com/thorsphere/lpcli" // lpcli
	"github.com/thorsphere/tserr" // tserr
)

// errorReader fails every Read with a non-EOF error, exercising the
// non-EOF error path of readLine.
type errorReader struct{ err error }

// Read always fails with the error.
func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

// TestReadLine verifies readLine's handling of terminated lines,
// unterminated EOF input, empty input, and propagated read errors.
func TestReadLine(t *testing.T) {
	// Create the tests
	tests := []struct {
		name    string
		input   io.Reader
		want    string
		wantErr error
	}{
		{ // simple line
			name:  "simple line",
			input: strings.NewReader("hello\n"),
			want:  "hello",
		},
		{ // trims surrounding whitespace
			name:  "trims surrounding whitespace",
			input: strings.NewReader("  hello world \n"),
			want:  "hello world",
		},
		{ // empty line
			name:  "empty line",
			input: strings.NewReader("\n"),
			want:  "",
		},
		{ // windows line ending
			name:  "windows line ending",
			input: strings.NewReader("hello\r\n"),
			want:  "hello",
		},
		{ // eof with unterminated content
			name:  "eof with unterminated content",
			input: strings.NewReader("partial"),
			want:  "partial",
		},
		{ // eof with unterminated whitespace only
			name:    "eof with unterminated whitespace only",
			input:   strings.NewReader("   "),
			wantErr: tserr.Aborted("lpcli"),
		},
		{ // eof without input aborts
			name:    "eof without input aborts",
			input:   strings.NewReader(""),
			wantErr: tserr.Aborted("lpcli"),
		},
		{ // non-eof error is propagated
			name:    "non-eof error is propagated",
			input:   errorReader{err: errors.New("read failure")},
			wantErr: errors.New("read failure"),
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Create a new prompter
			p := lpcli.NewPrompter("lpcli")
			// Set the input to the test input
			p.SetIn(tt.input)
			// Read a line from the input
			got, err := p.ReadLine()
			// Check if an error is expected
			if tt.wantErr != nil {
				// Check if the error is nil
				if err == nil {
					// If the error is nil, fail
					t.Fatal(tserr.NilFailed("readLine()"))
				}
				// Check that the error is as expected
				if err.Error() != tt.wantErr.Error() {
					// If the error is not as expected, fail
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "readLine() error", Want: tt.wantErr.Error(), Actual: err.Error()}))
				}
				// Return if the error is as expected
				return
			}
			// Check that the read is an error
			if err != nil {
				// If the read is an error, fail
				t.Fatal(tserr.Op(&tserr.OpArgs{Op: "readLine()", Fn: "prompter", Err: err}))
			}
			// Check that the read is as expected
			if got != tt.want {
				// If the read is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "readLine()", Want: tt.want, Actual: got}))
			}
		})
	}
}

// TestReadLineSequential verifies that consecutive calls consume one line
// each from a single buffered reader, including an unterminated final line.
// The test also verifies that the reader is reset after each call.
func TestReadLineSequential(t *testing.T) {
	// Create the tests
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{ // multiple lines
			name:  "multiple lines",
			input: "first\nsecond\nthird\n",
			want:  []string{"first", "second", "third"},
		},
		{ // final line unterminated
			name:  "final line unterminated",
			input: "first\nsecond",
			want:  []string{"first", "second"},
		},
		{ // blank lines preserved as empty
			name:  "blank lines preserved as empty",
			input: "a\n\nb\n",
			want:  []string{"a", "", "b"},
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Create a new prompter
			p := lpcli.NewPrompter("lpcli")
			// Set the input to the test input
			p.SetIn(strings.NewReader(tt.input))
			// Read lines from the input
			for i, want := range tt.want {
				// Read a line from the input
				got, err := p.ReadLine()
				// Check that the read is an error
				if err != nil {
					// If the read is an error, fail
					t.Fatal(tserr.Op(&tserr.OpArgs{Op: "readLine() call " + strconv.Itoa(i+1), Fn: "prompter", Err: err}))
				}
				// Check that the read is as expected
				if got != want {
					// If the read is not as expected, fail
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "readLine() call " + strconv.Itoa(i+1), Want: want, Actual: got}))
				}
			}
		})
	}
}

// TestReadLineSetInResetsReader verifies that SetIn discards the cached
// buffered reader, so subsequent reads come from the new source.
func TestReadLineSetInResetsReader(t *testing.T) {
	// Create a new prompter
	p := lpcli.NewPrompter("lpcli")
	// Set the input to an original source
	p.SetIn(strings.NewReader("from-first\nsecond-from-first\n"))
	// Read a line from the original source
	first, err := p.ReadLine()
	// Check that the first read is an error
	if err != nil {
		// If the first read is an error, fail
		t.Fatal(tserr.Op(&tserr.OpArgs{Op: "readLine()", Fn: "prompter", Err: err}))
	}
	// Check that the first read is from the original source
	if first != "from-first" {
		// If the first read is not from the original source, fail
		t.Fatal(tserr.EqualStr(&tserr.EqualStrArgs{Var: "readLine()", Want: "from-first", Actual: first}))
	}
	// Set the input to a new source
	p.SetIn(strings.NewReader("from-second\n"))
	// Read a line from the new source
	got, err := p.ReadLine()
	// Check that the second read is an error
	if err != nil {
		// If the second read is an error, fail
		t.Fatal(tserr.Op(&tserr.OpArgs{Op: "readLine() after SetIn", Fn: "prompter", Err: err}))
	}
	// Check that the second read is from the new source
	if got != "from-second" {
		// If the second read is not from the new source, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "readLine() after SetIn", Want: "from-second", Actual: got}))
	}
}
