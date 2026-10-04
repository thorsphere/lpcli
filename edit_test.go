// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli_test

// Import packages for testing.
import (
	"bytes"         // bytes
	"context"       // context
	"fmt"           // fmt
	"os"            // os
	"path/filepath" // filepath
	"runtime"       // runtime
	"slices"        // slices
	"strings"       // strings
	"testing"       // testing
	"time"          // time

	"github.com/thorsphere/lpcli" // lpcli
	"github.com/thorsphere/tserr" // tserr
)

// TestSplitEditorCmd verifies splitEditorCmd's word splitting: plain
// words, whitespace separators, single and double quotes, empty
// quoted strings, per-platform backslash handling, and unterminated
// quote errors.
func TestSplitEditorCmd(t *testing.T) {
	// Create the tests
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr error
	}{
		{ // empty input yields no words
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{ // whitespace-only input yields no words
			name:  "whitespace only",
			input: " \t\n ",
			want:  nil,
		},
		{ // single word
			name:  "single word",
			input: "vim",
			want:  []string{"vim"},
		},
		{ // words separated by spaces
			name:  "words separated by spaces",
			input: "code --wait",
			want:  []string{"code", "--wait"},
		},
		{ // tabs and newlines separate words too
			name:  "tabs and newlines separate words",
			input: "code\t--wait\n--new-window",
			want:  []string{"code", "--wait", "--new-window"},
		},
		{ // repeated and surrounding whitespace collapses
			name:  "repeated and surrounding whitespace collapses",
			input: "  code \t --wait  ",
			want:  []string{"code", "--wait"},
		},
		{ // single quotes preserve spaces
			name:  "single quotes preserve spaces",
			input: "code --wait 'my file.txt'",
			want:  []string{"code", "--wait", "my file.txt"},
		},
		{ // double quotes preserve spaces
			name:  "double quotes preserve spaces",
			input: `code --wait "my file.txt"`,
			want:  []string{"code", "--wait", "my file.txt"},
		},
		{ // quotes join adjacent text into one word
			name:  "quotes join adjacent text",
			input: "vim -c'set nu'",
			want:  []string{"vim", "-cset nu"},
		},
		{ // double quotes are literal inside single quotes
			name:  "double quotes literal inside single quotes",
			input: `'say "hi"'`,
			want:  []string{`say "hi"`},
		},
		{ // single quotes are literal inside double quotes
			name:  "single quotes literal inside double quotes",
			input: `"don't"`,
			want:  []string{"don't"},
		},
		{ // empty quoted strings produce no word
			name:  "empty quoted strings produce no word",
			input: `a '' b ""`,
			want:  []string{"a", "b"},
		},
		{ // only empty quotes yields no words
			name:  "only empty quotes",
			input: `'' ""`,
			want:  nil,
		},
		{ // single quotes keep windows paths intact on every platform
			name:  "single-quoted windows path",
			input: `'C:\my editor.exe'`,
			want:  []string{`C:\my editor.exe`},
		},
		{ // double-quoted windows path keeps backslashes on every platform
			name:  "double-quoted windows path",
			input: `"C:\Program Files\editor.exe"`,
			want:  []string{`C:\Program Files\editor.exe`},
		},
		{ // backslash escapes the next character outside quotes on
			// non-Windows systems; on Windows it is a literal path
			// separator
			name:  "backslash outside quotes",
			input: `code\ --wait`,
			want: wantFor(
				[]string{"code --wait"},
				[]string{`code\`, "--wait"},
			),
		},
		{ // trailing backslash is literal on every platform
			name:  "trailing backslash",
			input: `vim\`,
			want:  []string{`vim\`},
		},
		{ // backslash before an ordinary character inside double
			// quotes is literal on every platform
			name:  "backslash before ordinary character in double quotes",
			input: `"a\nb"`,
			want:  []string{`a\nb`},
		},
		{ // escaped backslash inside double quotes collapses on
			// non-Windows systems and stays doubled on Windows
			name:  "escaped backslash inside double quotes",
			input: `"a\\b"`,
			want: wantFor(
				[]string{`a\b`},
				[]string{`a\\b`},
			),
		},
		{ // escaped double quote inside double quotes on non-Windows
			// systems; on Windows the backslash is literal, the quote
			// closes early, and the trailing quote is unterminated
			name:  "escaped double quote inside double quotes",
			input: `"a\"b"`,
			want: wantFor(
				[]string{`a"b`},
				nil,
			),
			wantErr: errFor(nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "editor command",
				Detail: fmt.Sprintf("unterminated double quote in %q", `"a\"b"`),
			})),
		},
		{ // unterminated single quote
			name:  "unterminated single quote",
			input: "vim -c 'foo",
			wantErr: tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "editor command",
				Detail: fmt.Sprintf("unterminated single quote in %q", "vim -c 'foo"),
			}),
		},
		{ // unterminated double quote
			name:  "unterminated double quote",
			input: `code --wait "my file.txt`,
			wantErr: tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "editor command",
				Detail: fmt.Sprintf("unterminated double quote in %q", `code --wait "my file.txt`),
			}),
		},
		{ // realistic command with flags and a quoted file name
			name:  "realistic command",
			input: `code --wait --new-window "release notes.txt"`,
			want:  []string{"code", "--wait", "--new-window", "release notes.txt"},
		},
		{ // words are split on runes, not bytes
			name:  "unicode word",
			input: "éditeur --wait",
			want:  []string{"éditeur", "--wait"},
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Split the editor command
			got, err := lpcli.SplitEditorCmd(tt.input)
			// Check if an error is expected
			if tt.wantErr != nil {
				// Check if the error is nil
				if err == nil {
					// If the error is nil, fail
					t.Fatal(tserr.NilFailed("splitEditorCmd()"))
				}
				// Check that the error is as expected
				if err.Error() != tt.wantErr.Error() {
					// If the error is not as expected, fail
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "splitEditorCmd() error", Want: tt.wantErr.Error(), Actual: err.Error()}))
				}
				// Return if the error is as expected
				return
			}
			// Check that the split is an error
			if err != nil {
				// If the split is an error, fail
				t.Fatal(tserr.Op(&tserr.OpArgs{Op: "splitEditorCmd()", Fn: "editor command", Err: err}))
			}
			// Check that the words are as expected
			if !slices.Equal(got, tt.want) {
				// If the words are not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "splitEditorCmd()", Want: fmt.Sprintf("%v", tt.want), Actual: fmt.Sprintf("%v", got)}))
			}
		})
	}
}

// TestGetEditor verifies getEditor's resolution order: VISUAL takes
// precedence over EDITOR, values are trimmed and split with quoting
// rules, unresolvable values fall through to the next source, and
// nothing found yields a NotFound error. PATH is redirected to a
// temporary directory of fake executables so the tests do not
// depend on the machine's installed editors.
func TestGetEditor(t *testing.T) {
	// The first fallback candidate for this platform
	first := fallbackCandidates()[0]
	// Create the tests
	tests := []struct {
		name    string
		visual  string   // VISUAL value; "" means unset
		editor  string   // EDITOR value; "" means unset
		fakes   []string // fake executables to place on PATH
		want    []string
		wantErr error
	}{
		{ // visual wins over editor
			name:   "visual takes precedence over editor",
			visual: "visual-editor",
			editor: "editor-editor",
			fakes:  []string{"visual-editor", "editor-editor"},
			want:   []string{"visual-editor"},
		},
		{ // editor used when visual unset
			name:   "editor used when visual unset",
			visual: "",
			editor: "my-editor",
			fakes:  []string{"my-editor"},
			want:   []string{"my-editor"},
		},
		{ // whitespace-only visual is skipped
			name:   "whitespace-only visual is skipped",
			visual: "   ",
			editor: "my-editor",
			fakes:  []string{"my-editor"},
			want:   []string{"my-editor"},
		},
		{ // visual with arguments
			name:   "visual with arguments",
			visual: "code --wait",
			fakes:  []string{"code"},
			want:   []string{"code", "--wait"},
		},
		{ // visual with quoted arguments
			name:   "visual with quoted arguments",
			visual: `code --msg 'open the file'`,
			fakes:  []string{"code"},
			want:   []string{"code", "--msg", "open the file"},
		},
		{ // visual is trimmed before splitting
			name:   "visual is trimmed",
			visual: "  code  --wait  ",
			fakes:  []string{"code"},
			want:   []string{"code", "--wait"},
		},
		{ // editor with arguments
			name:   "editor with arguments",
			visual: "",
			editor: "code --wait",
			fakes:  []string{"code"},
			want:   []string{"code", "--wait"},
		},
		{ // unresolvable visual falls through to editor
			name:   "unresolvable visual falls through to editor",
			visual: "missing-editor",
			editor: "my-editor",
			fakes:  []string{"my-editor"},
			want:   []string{"my-editor"},
		},
		{ // visual that splits to no words is skipped
			name:   "visual that splits to no words is skipped",
			visual: "''",
			editor: "my-editor",
			fakes:  []string{"my-editor"},
			want:   []string{"my-editor"},
		},
		{ // malformed visual errors even with a valid editor
			name:   "malformed visual errors even with valid editor",
			visual: "bad 'quote",
			editor: "my-editor",
			fakes:  []string{"my-editor"},
			wantErr: tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "editor command",
				Detail: fmt.Sprintf("unterminated single quote in %q", "bad 'quote"),
			}),
		},
		{ // malformed editor errors
			name:   "malformed editor errors",
			visual: "",
			editor: `bad "quote`,
			wantErr: tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "editor command",
				Detail: fmt.Sprintf("unterminated double quote in %q", `bad "quote`),
			}),
		},
		{ // unresolvable env vars fall back to the os candidate
			name:   "unresolvable env vars fall back to os candidate",
			visual: "missing-visual",
			editor: "missing-editor",
			fakes:  []string{first},
			want:   []string{first},
		},
		{ // nothing resolves
			name:    "no editor found",
			visual:  "missing-visual",
			editor:  "missing-editor",
			wantErr: tserr.NotFound("editor (VISUAL, EDITOR, or a fallback editor)"),
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Create a directory to use as PATH
			dir := t.TempDir()
			// Create the fake executables
			for _, name := range tt.fakes {
				// Create a fake editor
				fakeEditor(t, dir, name)
			}
			// Restrict PATH to the fake directory
			t.Setenv("PATH", dir)
			// Set VISUAL; "" means unset
			t.Setenv("VISUAL", tt.visual)
			// Set EDITOR; "" means unset
			t.Setenv("EDITOR", tt.editor)
			// Resolve the editor
			got, err := lpcli.GetEditor()
			// Check if an error is expected
			if tt.wantErr != nil {
				// Check if the error is nil
				if err == nil {
					// If the error is nil, fail
					t.Fatal(tserr.NilFailed("getEditor()"))
				}
				// Check that the error is as expected
				if err.Error() != tt.wantErr.Error() {
					// If the error is not as expected, fail
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "getEditor() error", Want: tt.wantErr.Error(), Actual: err.Error()}))
				}
				// Return if the error is as expected
				return
			}
			// Check that the resolution is an error
			if err != nil {
				// If the resolution is an error, fail
				t.Fatal(tserr.Op(&tserr.OpArgs{Op: "getEditor()", Fn: "editor", Err: err}))
			}
			// Check that the parts are as expected
			if !slices.Equal(got, tt.want) {
				// If the parts are not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "getEditor()", Want: fmt.Sprintf("%v", tt.want), Actual: fmt.Sprintf("%v", got)}))
			}
		})
	}
}

// TestGetEditorFallbackOrder verifies that getEditor tries the OS
// fallback candidates in list order: for every position in the
// candidate list, when only the candidates from that position
// onward exist on PATH, the first present one is returned. The
// candidate list comes from fallbackCandidates, so the test stays
// in sync with the production lists.
func TestGetEditorFallbackOrder(t *testing.T) {
	// The Windows candidate list (notepad, notepad.exe) cannot
	// exercise skipping: LookPath("notepad") already matches a
	// notepad.exe file, so the second candidate is never
	// independently reachable.
	if runtime.GOOS == "windows" {
		// Skip the test on Windows
		t.Skip("windows fallback candidates are not independently distinguishable")
	}
	// Get the candidate list for this platform
	candidates := fallbackCandidates()
	// For each position, only the candidates from that position
	// onward exist on PATH; the first present one must win.
	for i, want := range candidates {
		// Run a subtest per position
		t.Run(fmt.Sprintf("first present candidate is %s", want), func(t *testing.T) {
			// Create a directory to use as PATH
			dir := t.TempDir()
			// Create fakes only for the candidates from position i onward
			for _, name := range candidates[i:] {
				// Create a fake editor
				fakeEditor(t, dir, name)
			}
			// Restrict PATH to the fake directory
			t.Setenv("PATH", dir)
			// Unset the environment variables
			t.Setenv("VISUAL", "")
			// Unset the editor environment variable
			t.Setenv("EDITOR", "")
			// Resolve the editor
			got, err := lpcli.GetEditor()
			// Check that the resolution is an error
			if err != nil {
				// If the resolution is an error, fail
				t.Fatal(tserr.Op(&tserr.OpArgs{Op: "getEditor()", Fn: "editor", Err: err}))
			}
			// Check that the first present candidate is returned
			if !slices.Equal(got, []string{want}) {
				// If the first present candidate is not returned, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "getEditor()", Want: fmt.Sprintf("%v", []string{want}), Actual: fmt.Sprintf("%v", got)}))
			}
		})
	}
}

// TestGetEditorUnquotedPathWithSpaces verifies the whole-value
// fallback: when the first split word is not an executable but the
// entire value is, getEditor returns the value as a single unquoted
// path. VISUAL's whole-value fallback takes precedence over EDITOR.
func TestGetEditorUnquotedPathWithSpaces(t *testing.T) {
	// Create a directory to use as PATH
	dir := t.TempDir()
	// Create an editor whose file name contains a space
	path := fakeEditor(t, dir, "my editor")
	// Create a valid EDITOR to prove VISUAL wins
	fakeEditor(t, dir, "my-editor")
	// Restrict PATH to the fake directory
	t.Setenv("PATH", dir)
	// Set VISUAL to the unquoted path with spaces
	t.Setenv("VISUAL", path)
	// Set a valid EDITOR
	t.Setenv("EDITOR", "my-editor")
	// Resolve the editor
	got, err := lpcli.GetEditor()
	// Check that the resolution is an error
	if err != nil {
		// If the resolution is an error, fail
		t.Fatal(tserr.Op(&tserr.OpArgs{Op: "getEditor()", Fn: "editor", Err: err}))
	}
	// Check that the whole path is returned as a single word
	if !slices.Equal(got, []string{path}) {
		// If the whole path is not returned, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "getEditor()", Want: fmt.Sprintf("%v", []string{path}), Actual: fmt.Sprintf("%v", got)}))
	}
}

// TestEdit verifies Edit's round trip: the initial text is written
// to the temporary file, the editor receives the file path as its
// last argument, and the edited content is read back.
func TestEdit(t *testing.T) {
	// Create the tests
	tests := []struct {
		name    string
		mode    string // helper behavior
		text    string // text the helper appends
		initial string // initial text passed to Edit
		want    string // expected edited result
	}{
		{ // editor appends to the file
			name:    "editor appends to the file",
			mode:    "append",
			text:    "!",
			initial: "hello",
			want:    "hello!",
		},
		{ // editor exits without touching the file
			name:    "editor leaves text unchanged",
			mode:    "none",
			initial: "hello",
			want:    "hello",
		},
		{ // empty initial text
			name:    "empty initial text",
			mode:    "append",
			text:    "x",
			initial: "",
			want:    "x",
		},
		{ // multi-line text survives the round trip
			name:    "multi-line text",
			mode:    "none",
			initial: "line one\nline two\n",
			want:    "line one\nline two\n",
		},
		{ // editor empties the file: empty result is not an error
			name:    "editor empties the file",
			mode:    "truncate",
			initial: "hello",
			want:    "",
		},
		{ // editor leaves only whitespace
			name:    "editor leaves only whitespace",
			mode:    "write",
			text:    "\n\n",
			initial: "hello",
			want:    "\n\n",
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Point VISUAL at the fake editor
			t.Setenv("VISUAL", helperEditorCmd())
			// Spawn the helper behavior
			t.Setenv(helperGate, "1")
			// Select the helper mode
			t.Setenv(helperMode, tt.mode)
			// Set the text to append
			t.Setenv(helperText, tt.text)
			// Create a prompter
			p := lpcli.NewPrompter("lpcli")
			// Edit the text
			got, err := p.Edit(context.Background(), tt.initial)
			// Check that the edit is an error
			if err != nil {
				// If the edit is an error, fail
				t.Fatal(tserr.Op(&tserr.OpArgs{Op: "Edit()", Fn: "prompter", Err: err}))
			}
			// Check that the edited text is as expected
			if got != tt.want {
				// If the edited text is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Edit()", Want: tt.want, Actual: got}))
			}
		})
	}
}

// TestEditNilPrompter verifies that Edit rejects a nil prompter.
func TestEditNilPrompter(t *testing.T) {
	// Create a nil prompter
	var p *lpcli.Prompter
	// Edit with the nil prompter
	_, err := p.Edit(context.Background(), "text")
	// Check that the error is nil
	if err == nil {
		// If the error is nil, fail
		t.Fatal(tserr.NilFailed("Edit()"))
	}
	// Check that the error is a nil-pointer error
	if err.Error() != tserr.NilPtr().Error() {
		// If the error is not a nil-pointer error, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Edit() error", Want: tserr.NilPtr().Error(), Actual: err.Error()}))
	}
}

// TestEditNoEditor verifies that Edit propagates getEditor's
// NotFound error when no editor can be resolved.
func TestEditNoEditor(t *testing.T) {
	// Restrict PATH to an empty directory
	t.Setenv("PATH", t.TempDir())
	// Unset the environment variables
	t.Setenv("VISUAL", "")
	// Unset the editor environment variable
	t.Setenv("EDITOR", "")
	// Create a prompter
	p := lpcli.NewPrompter("lpcli")
	// Edit, which must fail
	_, err := p.Edit(context.Background(), "text")
	// Check that the error is nil
	if err == nil {
		// If the error is nil, fail
		t.Fatal(tserr.NilFailed("Edit()"))
	}
	// Check that the error is the NotFound error
	want := tserr.NotFound("editor (VISUAL, EDITOR, or a fallback editor)").Error()
	// Compare with the expected error
	if err.Error() != want {
		// If the error is not the NotFound error, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Edit() error", Want: want, Actual: err.Error()}))
	}
}

// TestEditEditorExitsNonZero verifies that an editor exiting by
// itself with a non-zero status (e.g. vim's :cq) is reported as a
// run error, not as an abort.
func TestEditEditorExitsNonZero(t *testing.T) {
	// Point VISUAL at the fake editor
	t.Setenv("VISUAL", helperEditorCmd())
	// Spawn the helper behavior
	t.Setenv(helperGate, "1")
	// Make the helper exit with status 3
	t.Setenv(helperMode, "fail")
	// Create a prompter
	p := lpcli.NewPrompter("lpcli")
	// Edit, which must fail
	_, err := p.Edit(context.Background(), "text")
	// Check that the error is nil
	if err == nil {
		// If the error is nil, fail
		t.Fatal(tserr.NilFailed("Edit()"))
	}
	// Check that the editor's exit status is reported
	if !strings.Contains(err.Error(), "exit status 3") {
		// If the exit status is not reported, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Edit() error", Want: "exit status 3", Actual: err.Error()}))
	}
}

// TestEditCancelled verifies that Edit returns promptly with an
// Aborted error when ctx is cancelled while the editor runs.
func TestEditCancelled(t *testing.T) {
	// Point VISUAL at the fake editor
	t.Setenv("VISUAL", helperEditorCmd())
	// Spawn the helper behavior
	t.Setenv(helperGate, "1")
	// Make the helper block
	t.Setenv(helperMode, "sleep")
	// Create a prompter
	p := lpcli.NewPrompter("lpcli")
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	// Ensure the context is cancelled
	defer cancel()
	// Cancel the context shortly after Edit starts
	go func() {
		// Wait for the editor to start
		time.Sleep(100 * time.Millisecond)
		// Cancel the context
		cancel()
	}()
	// Record the start time
	start := time.Now()
	// Edit, which must abort
	_, err := p.Edit(ctx, "text")
	// Check that the error is an Aborted error
	if err == nil || err.Error() != tserr.Aborted("lpcli").Error() {
		// If the error is not an Aborted error, fail
		t.Fatal(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Edit() error", Want: tserr.Aborted("lpcli").Error(), Actual: fmt.Sprintf("%v", err)}))
	}
	// Check that Edit returned promptly
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		// If Edit did not return promptly, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Edit() elapsed", Want: "< 5s", Actual: elapsed.String()}))
	}
}

// TestEditEditorStreams verifies that the editor subprocess is
// attached to the streams from editorStreams: output the editor
// writes lands in the substituted streams.
func TestEditEditorStreams(t *testing.T) {
	// Point VISUAL at the fake editor
	t.Setenv("VISUAL", helperEditorCmd())
	// Spawn the helper behavior
	t.Setenv(helperGate, "1")
	// Make the helper write to both streams
	t.Setenv(helperMode, "echo")
	// Create buffers for the editor's output
	var out, errOut bytes.Buffer
	// Create a prompter
	p := lpcli.NewPrompter("lpcli")
	// Substitute the editor streams
	lpcli.SetEditorStreams(p, bytes.NewReader(nil), &out, &errOut)
	// Edit the text
	if _, err := p.Edit(context.Background(), "text"); err != nil {
		// If the edit is an error, fail
		t.Fatal(tserr.Op(&tserr.OpArgs{Op: "Edit()", Fn: "prompter", Err: err}))
	}
	// Check that the editor's stdout was captured
	if out.String() != writeStdout {
		// If the stdout was not captured, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editor stdout", Want: writeStdout, Actual: out.String()}))
	}
	// Check that the editor's stderr was captured
	if errOut.String() != writeStderr {
		// If the stderr was not captured, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editor stderr", Want: writeStderr, Actual: errOut.String()}))
	}
}

// TestEditCleansTempFile verifies that Edit removes its temporary
// file after the editor exits. The temp directory is redirected to
// an empty directory (TMPDIR on Unix, TMP on Windows) so the check
// is deterministic.
func TestEditCleansTempFile(t *testing.T) {
	// Create a directory to use as the temp directory
	dir := t.TempDir()
	// Redirect the temp directory on Unix
	t.Setenv("TMPDIR", dir)
	// Redirect the temp directory on Windows
	t.Setenv("TMP", dir)
	// Point VISUAL at the fake editor
	t.Setenv("VISUAL", helperEditorCmd())
	// Spawn the helper behavior
	t.Setenv(helperGate, "1")
	// Make the helper append to the file
	t.Setenv(helperMode, "append")
	// Set the text to append
	t.Setenv(helperText, "!")
	// Create a prompter
	p := lpcli.NewPrompter("lpcli")
	// Edit the text
	if _, err := p.Edit(context.Background(), "text"); err != nil {
		// If the edit is an error, fail
		t.Fatal(tserr.Op(&tserr.OpArgs{Op: "Edit()", Fn: "prompter", Err: err}))
	}
	// List the temp directory
	entries, err := os.ReadDir(dir)
	// Check that listing is an error
	if err != nil {
		// If listing is an error, fail
		t.Fatal(tserr.Op(&tserr.OpArgs{Op: "read temp dir", Fn: dir, Err: err}))
	}
	// Check that the temp file was removed
	if len(entries) != 0 {
		// If the temp file was not removed, fail
		t.Error(tserr.EqualInt(&tserr.EqualIntArgs{Var: "temp dir entries", Want: 0, Actual: int64(len(entries))}))
	}
}

// TestEditEditorFailsToStart verifies the default error branch: an
// editor that LookPath resolves but the OS cannot execute. On Unix
// that is a script with a nonexistent interpreter; on Windows an
// .exe with invalid content. The error must be a run error, not an
// abort and not an exit status.
func TestEditEditorFailsToStart(t *testing.T) {
	// Create a directory for the broken editor
	dir := t.TempDir()
	// Create the broken editor per platform
	var path string
	// Check if the system is Windows
	if runtime.GOOS == "windows" {
		// A .exe with invalid content: LookPath resolves it by
		// extension, but CreateProcess rejects it
		path = filepath.Join(dir, "broken-editor.exe")
		// Write invalid content
		if err := os.WriteFile(path, []byte("not a pe file"), 0o755); err != nil {
			// If the file cannot be written, fail
			t.Fatal(tserr.Op(&tserr.OpArgs{Op: "create broken editor", Fn: path, Err: err}))
		}
	} else {
		// A script whose interpreter does not exist: LookPath
		// resolves it by the executable bit, but exec fails
		path = filepath.Join(dir, "broken-editor")
		// Write the broken shebang
		if err := os.WriteFile(path, []byte("#!/nonexistent-interpreter\n"), 0o755); err != nil {
			// If the file cannot be written, fail
			t.Fatal(tserr.Op(&tserr.OpArgs{Op: "create broken editor", Fn: path, Err: err}))
		}
	}
	// Point VISUAL at the broken editor
	t.Setenv("VISUAL", path)
	// Create a prompter
	p := lpcli.NewPrompter("lpcli")
	// Edit, which must fail
	_, err := p.Edit(context.Background(), "text")
	// Check that the error is nil
	if err == nil {
		// If the error is nil, fail
		t.Fatal(tserr.NilFailed("Edit()"))
	}
	// Check that the error is not an abort
	if err.Error() == tserr.Aborted("lpcli").Error() {
		// If the error is an abort, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Edit() error", Want: "run error", Actual: err.Error()}))
	}
	// Check that the error is not an exit status
	if strings.Contains(err.Error(), "exit status") {
		// If the error is an exit status, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Edit() error", Want: "start failure", Actual: err.Error()}))
	}
}

// TestEditEmptyResultIsNotAnError pins the documented contract: a
// user who deletes the whole buffer and saves gets an empty string
// and a nil error. Callers that require content must check for it
// themselves, so this test fails loudly if Edit ever starts
// rejecting empty results.
func TestEditEmptyResultIsNotAnError(t *testing.T) {
	// Point VISUAL at the fake editor
	t.Setenv("VISUAL", helperEditorCmd())
	// Spawn the helper behavior
	t.Setenv(helperGate, "1")
	// Make the helper empty the file
	t.Setenv(helperMode, "truncate")
	// Create a prompter
	p := lpcli.NewPrompter("lpcli")
	// Edit the text
	got, err := p.Edit(context.Background(), "hello")
	// Check that the edit is an error
	if err != nil {
		// If the edit is an error, fail
		t.Fatal(tserr.Op(&tserr.OpArgs{Op: "Edit()", Fn: "prompter", Err: err}))
	}
	// Check that the result is empty
	if got != "" {
		// If the result is not empty, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Edit()", Want: "", Actual: got}))
	}
}
