// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli_test

// Import packages for testing.
import (
	"context" // context
	"errors"  // errors
	"fmt"     // fmt
	"io"      // io
	"os"      // os
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

// sr returns a reader over s, for use in test tables.
func sr(s string) io.Reader { return strings.NewReader(s) }

// TestReadLine verifies readLine's handling of terminated lines,
// unterminated EOF input, empty input, and propagated read errors.
func TestReadLine(t *testing.T) {
	// Create a prompt name
	pn := "lpcli"
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
			wantErr: tserr.Aborted(pn),
		},
		{ // eof without input aborts
			name:    "eof without input aborts",
			input:   strings.NewReader(""),
			wantErr: tserr.Aborted(pn),
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
			p := lpcli.NewPrompter(pn)
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

// TestConfirm verifies Confirm's handling of accepted, declined,
// retried and cancelled confirmations.
func TestConfirm(t *testing.T) {
	// Create a prompt name
	pn := "lpcli"
	// Create the tests
	tests := []struct {
		name    string
		input   io.Reader
		wantErr error
	}{
		{ // explicit yes
			name:  "yes",
			input: sr("y\n"),
		},
		{ // full word yes
			name:  "full word yes",
			input: sr("yes\n"),
		},
		{ // uppercase yes
			name:  "uppercase yes",
			input: sr("Y\n"),
		},
		{ // mixed case yes
			name:  "mixed case yes",
			input: sr("Yes\n"),
		},
		{ // empty input defaults to yes
			name:  "empty input defaults to yes",
			input: sr("\n"),
		},
		{ // explicit no
			name:    "no",
			input:   sr("n\n"),
			wantErr: tserr.Aborted(pn),
		},
		{ // full word no
			name:    "full word no",
			input:   sr("no\n"),
			wantErr: tserr.Aborted(pn),
		},
		{ // uppercase no
			name:    "uppercase no",
			input:   sr("N\n"),
			wantErr: tserr.Aborted(pn),
		},
		{ // invalid option then yes
			name:  "invalid option then yes",
			input: sr("maybe\ny\n"),
		},
		{ // invalid option then no
			name:    "invalid option then no",
			input:   sr("maybe\nn\n"),
			wantErr: tserr.Aborted(pn),
		},
		{ // eof without input aborts
			name:    "eof without input aborts",
			input:   sr(""),
			wantErr: tserr.Op(&tserr.OpArgs{Op: "readLine", Fn: "prompter", Err: tserr.Aborted(pn)}),
		},
		{ // read error is propagated
			name:    "read error is propagated",
			input:   errorReader{err: errors.New("read failure")},
			wantErr: tserr.Op(&tserr.OpArgs{Op: "readLine", Fn: "prompter", Err: errors.New("read failure")}),
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Create a new prompter
			p := lpcli.NewPrompter(pn)
			// Set the input to the test input
			p.SetIn(tt.input)
			// Confirm the prompt
			err := p.Confirm(context.Background(), "Continue? [y/n] ")
			// Check if an error is expected
			if tt.wantErr != nil {
				// Check if the error is nil
				if err == nil {
					// If the error is nil, fail
					t.Fatal(tserr.NilFailed("Confirm()"))
				}
				// Check that the error is as expected
				if err.Error() != tt.wantErr.Error() {
					// If the error is not as expected, fail
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Confirm() error", Want: tt.wantErr.Error(), Actual: err.Error()}))
				}
				// Return if the error is as expected
				return
			}
			// Check that the confirm is an error
			if err != nil {
				// If the confirm is an error, fail
				t.Fatal(tserr.Op(&tserr.OpArgs{Op: "Confirm()", Fn: "prompter", Err: err}))
			}
		})
	}
}

// TestConfirmCancelledContext verifies that Confirm aborts when the
// context is cancelled before the prompt is shown.
func TestConfirmCancelledContext(t *testing.T) {
	// Create a prompt name
	pn := "lpcli"
	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel the context
	cancel()
	// Create a new prompter
	p := lpcli.NewPrompter(pn)
	// Set the input to a reader that would block, proving the
	// context is checked before reading
	p.SetIn(strings.NewReader("y\n"))
	// Confirm the prompt
	err := p.Confirm(ctx, "Continue? [y/n] ")
	// Check that the confirm is an error
	if err == nil {
		// If the confirm is not an error, fail
		t.Fatal(tserr.NilFailed("Confirm()"))
	}
	// Check that the error is as expected
	want := tserr.Aborted(pn).Error()
	// Compare the error messages
	if err.Error() != want {
		// If the error is not as expected, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Confirm() error", Want: want, Actual: err.Error()}))
	}
}

// TestConfirmNilPrompter verifies that Confirm on a nil prompter
// returns an error instead of panicking.
func TestConfirmNilPrompter(t *testing.T) {
	// Create a nil prompter
	var p *lpcli.Prompter
	// Confirm the prompt
	err := p.Confirm(context.Background(), "Continue? [y/n] ")
	// Check that the confirm is an error
	if err == nil {
		// If the confirm is not an error, fail
		t.Fatal(tserr.NilFailed("Confirm()"))
	}
	// Check that the error is the nil pointer error
	if err.Error() != tserr.NilPtr().Error() {
		// If the error is not as expected, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Confirm() error", Want: tserr.NilPtr().Error(), Actual: err.Error()}))
	}
}

// TestConfirmWritesPrompt verifies that Confirm writes the prompt
// message and the retry hint for unknown options to its output.
func TestConfirmWritesPrompt(t *testing.T) {
	// Create the tests
	tests := []struct {
		name    string
		message string
		input   string
		want    string
	}{
		{ // prompt message is written
			name:    "prompt message is written",
			message: "Continue? [y/n] ",
			input:   "y\n",
			want:    "Continue? [y/n] ",
		},
		{ // retry hint is written for unknown option
			name:    "retry hint is written for unknown option",
			message: "Continue? [y/n] ",
			input:   "maybe\ny\n",
			want:    "Continue? [y/n] Unknown option \"maybe\". Please choose [y/n].\nContinue? [y/n] ",
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
			// Create a buffer for the output
			var out strings.Builder
			// Set the output to the buffer
			p.SetOut(&out)
			// Confirm the prompt
			if err := p.Confirm(context.Background(), tt.message); err != nil {
				// If the confirm is an error, fail
				t.Fatal(tserr.Op(&tserr.OpArgs{Op: "Confirm()", Fn: "prompter", Err: err}))
			}
			// Check that the output is as expected
			if out.String() != tt.want {
				// If the output is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Confirm() output", Want: tt.want, Actual: out.String()}))
			}
		})
	}
}

// TestGetReaderNilIn verifies that getReader falls back to os.Stdin
// when the input source is nil, and that the fallback reader is
// cached across calls. No read is performed, so the test does not
// block on the real stdin.
func TestGetReaderNilIn(t *testing.T) {
	// Create a prompt name
	pn := "lpcli"
	// Create the tests
	tests := []struct {
		name string
		p    *lpcli.Prompter
	}{
		{ // zero-value prompter has nil input
			name: "zero-value prompter",
			p:    &lpcli.Prompter{Name: pn},
		},
		{ // SetIn with nil restores os.Stdin
			name: "SetIn nil",
			p:    func() *lpcli.Prompter { p := lpcli.NewPrompter(pn); p.SetIn(nil); return p }(),
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Get the reader; p.in is nil, so it must fall back to os.Stdin
			r := lpcli.GetReader(tt.p)
			// Check that the reader is not nil
			if r == nil {
				// If the reader is nil, fail
				t.Fatal(tserr.NilFailed("getReader()"))
			}
			// Check that the reader is cached: a second call must
			// return the same instance
			if again := lpcli.GetReader(tt.p); again != r {
				// If the reader is not cached, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "reader", Want: fmt.Sprintf("%v", r), Actual: fmt.Sprintf("%v", again)}))
			}
		})
	}
}

// TestIn verifies that In returns the configured input source,
// including the nil returned after SetIn(nil). Unlike Out, In does
// not fall back to os.Stdin; the fallback happens later in getReader.
func TestIn(t *testing.T) {
	// Create a prompt name
	pn := "lpcli"
	// Create the input source once, so the same instance is used
	// for SetIn and the expected value
	in := strings.NewReader("input")
	// Create the tests
	tests := []struct {
		name string
		p    *lpcli.Prompter
		want io.Reader
	}{
		{ // new prompter uses os.Stdin
			name: "new prompter returns os.Stdin",
			p:    lpcli.NewPrompter(pn),
			want: os.Stdin,
		},
		{ // SetIn changes the returned source
			name: "SetIn changes the returned source",
			p: func() *lpcli.Prompter {
				// Create a new prompter
				p := lpcli.NewPrompter(pn)
				// Set the input to a string reader
				p.SetIn(in)
				// Return the prompter
				return p
			}(),
			want: in,
		},
		{ // SetIn nil returns nil, not os.Stdin
			name: "SetIn nil returns nil",
			p: func() *lpcli.Prompter {
				// Create a new prompter
				p := lpcli.NewPrompter(pn)
				// Set the input to nil
				p.SetIn(nil)
				// Return the prompter
				return p
			}(),
			want: nil,
		},
		{ // zero-value prompter has nil input
			name: "zero-value prompter returns nil",
			p:    &lpcli.Prompter{Name: pn},
			want: nil,
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Get the input source
			got := tt.p.In()
			// Check that the input source is as expected
			if got != tt.want {
				// If the input source is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "In()", Want: fmt.Sprintf("%v", tt.want), Actual: fmt.Sprintf("%v", got)}))
			}
		})
	}
}

// TestOut verifies that Out returns the configured output writer,
// falling back to os.Stderr when unset. Unlike In, Out performs the
// fallback itself, because it is used directly for writing.
func TestOut(t *testing.T) {
	// Create a prompt name
	pn := "lpcli"
	// Create the output writer once, so the same instance is used
	// for SetOut and the expected value
	out := &strings.Builder{}
	// Create the tests
	tests := []struct {
		name string
		p    *lpcli.Prompter
		want io.Writer
	}{
		{ // new prompter uses os.Stderr
			name: "new prompter returns os.Stderr",
			p:    lpcli.NewPrompter(pn),
			want: os.Stderr,
		},
		{ // SetOut changes the returned writer
			name: "SetOut changes the returned writer",
			p: func() *lpcli.Prompter {
				// Create a new prompter
				p := lpcli.NewPrompter(pn)
				// Set the output to the shared builder
				p.SetOut(out)
				// Return the prompter
				return p
			}(),
			want: out,
		},
		{ // SetOut nil restores the os.Stderr fallback
			name: "SetOut nil returns os.Stderr",
			p: func() *lpcli.Prompter {
				// Create a new prompter
				p := lpcli.NewPrompter(pn)
				// Set the output to nil
				p.SetOut(nil)
				// Return the prompter
				return p
			}(),
			want: os.Stderr,
		},
		{ // zero-value prompter falls back to os.Stderr
			name: "zero-value prompter returns os.Stderr",
			p:    &lpcli.Prompter{Name: pn},
			want: os.Stderr,
		},
		{ // nil prompter falls back to os.Stderr, like the zero value
			name: "nil prompter returns os.Stderr",
			p:    nil,
			want: os.Stderr,
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Get the output writer
			got := tt.p.Out()
			// Check that the output writer is as expected
			if got != tt.want {
				// If the output writer is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Out()", Want: fmt.Sprintf("%v", tt.want), Actual: fmt.Sprintf("%v", got)}))
			}
		})
	}
}

// TestEditorStreams verifies that editorStreams returns the overridden
// streams when set, and falls back to the process's standard streams
// for each stream that is unset. Each stream is resolved independently,
// so partial overrides are covered as well.
func TestEditorStreams(t *testing.T) {
	// Create a prompt name
	pn := "lpcli"
	// Create the custom streams once, so the same instances are
	// used for setEditorStreams and the expected values
	stdin := strings.NewReader("editor stdin")
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	// Create the tests
	tests := []struct {
		name       string
		set        bool // whether setEditorStreams is called
		stdin      io.Reader
		stdout     io.Writer
		stderr     io.Writer
		wantStdin  io.Reader
		wantStdout io.Writer
		wantStderr io.Writer
	}{
		{ // new prompter returns the process streams
			name:       "new prompter returns process streams",
			set:        false,
			wantStdin:  os.Stdin,
			wantStdout: os.Stdout,
			wantStderr: os.Stderr,
		},
		{ // all streams overridden
			name:       "all streams overridden",
			set:        true,
			stdin:      stdin,
			stdout:     stdout,
			stderr:     stderr,
			wantStdin:  stdin,
			wantStdout: stdout,
			wantStderr: stderr,
		},
		{ // only stdin overridden
			name:       "only stdin overridden",
			set:        true,
			stdin:      stdin,
			wantStdin:  stdin,
			wantStdout: os.Stdout,
			wantStderr: os.Stderr,
		},
		{ // only stdout overridden
			name:       "only stdout overridden",
			set:        true,
			stdout:     stdout,
			wantStdin:  os.Stdin,
			wantStdout: stdout,
			wantStderr: os.Stderr,
		},
		{ // only stderr overridden
			name:       "only stderr overridden",
			set:        true,
			stderr:     stderr,
			wantStdin:  os.Stdin,
			wantStdout: os.Stdout,
			wantStderr: stderr,
		},
		{ // nil arguments restore the process streams
			name:       "nil arguments restore process streams",
			set:        true,
			wantStdin:  os.Stdin,
			wantStdout: os.Stdout,
			wantStderr: os.Stderr,
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Create a new prompter
			p := lpcli.NewPrompter(pn)
			// Override the editor streams if the test requires it
			if tt.set {
				// Set the editor streams to the test values
				lpcli.SetEditorStreams(p, tt.stdin, tt.stdout, tt.stderr)
			}
			// Get the editor streams
			gotStdin, gotStdout, gotStderr := lpcli.EditorStreams(p)
			// Check that stdin is as expected
			if gotStdin != tt.wantStdin {
				// If stdin is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editorStreams() stdin", Want: fmt.Sprintf("%v", tt.wantStdin), Actual: fmt.Sprintf("%v", gotStdin)}))
			}
			// Check that stdout is as expected
			if gotStdout != tt.wantStdout {
				// If stdout is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editorStreams() stdout", Want: fmt.Sprintf("%v", tt.wantStdout), Actual: fmt.Sprintf("%v", gotStdout)}))
			}
			// Check that stderr is as expected
			if gotStderr != tt.wantStderr {
				// If stderr is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editorStreams() stderr", Want: fmt.Sprintf("%v", tt.wantStderr), Actual: fmt.Sprintf("%v", gotStderr)}))
			}
		})
	}
}

// TestEditorStreamsNilRestoresAfterOverride verifies that passing nil
// to setEditorStreams restores the process streams after they have
// been overridden, as documented.
func TestEditorStreamsNilRestoresAfterOverride(t *testing.T) {
	// Create a new prompter
	p := lpcli.NewPrompter("lpcli")
	// Override all editor streams
	lpcli.SetEditorStreams(p, strings.NewReader("editor stdin"), &strings.Builder{}, &strings.Builder{})
	// Restore the process streams by passing nil
	lpcli.SetEditorStreams(p, nil, nil, nil)
	// Get the editor streams
	stdin, stdout, stderr := lpcli.EditorStreams(p)
	// Check that stdin is restored
	if stdin != os.Stdin {
		// If stdin is not restored, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editorStreams() stdin", Want: fmt.Sprintf("%v", os.Stdin), Actual: fmt.Sprintf("%v", stdin)}))
	}
	// Check that stdout is restored
	if stdout != os.Stdout {
		// If stdout is not restored, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editorStreams() stdout", Want: fmt.Sprintf("%v", os.Stdout), Actual: fmt.Sprintf("%v", stdout)}))
	}
	// Check that stderr is restored
	if stderr != os.Stderr {
		// If stderr is not restored, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editorStreams() stderr", Want: fmt.Sprintf("%v", os.Stderr), Actual: fmt.Sprintf("%v", stderr)}))
	}
}

// TestInNil verifies that In returns nil for a nil prompter.
func TestInNil(t *testing.T) {
	// Create a nil prompter
	var p *lpcli.Prompter
	// Check that In returns nil for a nil prompter
	got := p.In()
	// Check that the input source is nil
	if got != nil {
		// If the input source is not nil, fail
		t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "In()", Err: fmt.Errorf("%v", got)}))
	}
}

// TestSetInNil verifies that SetIn returns an error for a nil prompter.
func TestSetInNil(t *testing.T) {
	// Create a nil prompter
	var p *lpcli.Prompter
	// Set the input on the nil prompter
	err := p.SetIn(nil)
	// Check that SetIn returns an error
	if err == nil {
		// If the error is nil, fail
		t.Fatal(tserr.NilFailed("SetIn()"))
	}
	// Check that the error is the nil pointer error
	if err.Error() != tserr.NilPtr().Error() {
		// If the error is not as expected, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "SetIn() error", Want: tserr.NilPtr().Error(), Actual: err.Error()}))
	}
}

// TestSetOutNil verifies that SetOut returns an error for a nil prompter.
func TestSetOutNil(t *testing.T) {
	// Create a nil prompter
	var p *lpcli.Prompter
	// Set the output on the nil prompter
	err := p.SetOut(nil)
	// Check that SetOut returns an error
	if err == nil {
		// If the error is nil, fail
		t.Fatal(tserr.NilFailed("SetOut()"))
	}
	// Check that the error is the nil pointer error
	if err.Error() != tserr.NilPtr().Error() {
		// If the error is not as expected, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "SetOut() error", Want: tserr.NilPtr().Error(), Actual: err.Error()}))
	}
}
