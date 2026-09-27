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
	"slices"  // slices
	"strconv" // strconv
	"testing" // testing

	"github.com/thorsphere/lpcli" // lpcli
	"github.com/thorsphere/tserr" // tserr
)

// TestValidate verifies validate's checks: empty choices, empty or
// padded keys, duplicate keys, duplicate or padded aliases, and
// multiple defaults, as well as the returned default choice.
func TestValidate(t *testing.T) {
	// Create the tests
	tests := []struct {
		name    string
		choices []lpcli.Choice
		wantKey string // expected default choice key, "" for none
		wantErr error
	}{
		{ // no choices
			name:    "no choices",
			choices: nil,
			wantErr: tserr.Empty("choices"),
		},
		{ // single choice without default
			name:    "single choice without default",
			choices: []lpcli.Choice{{Value: "a", Key: "a"}},
		},
		{ // single choice with default
			name:    "single choice with default",
			choices: []lpcli.Choice{{Value: "a", Key: "a", IsDefault: true}},
			wantKey: "a",
		},
		{ // first default wins
			name: "first default is returned",
			choices: []lpcli.Choice{
				{Value: "a", Key: "a", IsDefault: true},
				{Value: "b", Key: "b"},
			},
			wantKey: "a",
		},
		{ // empty key
			name:    "empty key",
			choices: []lpcli.Choice{{Value: "a", Key: ""}},
			wantErr: tserr.InvalidFormat(&tserr.InvalidFormatArgs{F: "choice key", Detail: "cannot be empty"}),
		},
		{ // key with surrounding whitespace
			name:    "key with surrounding whitespace",
			choices: []lpcli.Choice{{Value: "a", Key: " a"}},
			wantErr: tserr.InvalidFormat(&tserr.InvalidFormatArgs{F: "choice key", Detail: fmt.Sprintf("%q has surrounding whitespace", " a")}),
		},
		{ // key with only whitespace
			name:    "key with only whitespace",
			choices: []lpcli.Choice{{Value: "a", Key: "   "}},
			wantErr: tserr.InvalidFormat(&tserr.InvalidFormatArgs{F: "choice key", Detail: "cannot be empty"}),
		},
		{ // duplicate keys
			name:    "duplicate keys",
			choices: []lpcli.Choice{{Value: "a", Key: "a"}, {Value: "b", Key: "a"}},
			wantErr: tserr.DuplicateKey(&tserr.DuplicateKeyArgs{Key: "a", Existing: "a"}),
		},
		{ // duplicate keys after normalization
			name:    "duplicate keys after normalization",
			choices: []lpcli.Choice{{Value: "a", Key: "a"}, {Value: "b", Key: "A"}},
			wantErr: tserr.DuplicateKey(&tserr.DuplicateKeyArgs{Key: "A", Existing: "a"}),
		},
		{ // multiple defaults
			name: "multiple defaults",
			choices: []lpcli.Choice{
				{Value: "a", Key: "a", IsDefault: true},
				{Value: "b", Key: "b", IsDefault: true},
			},
			wantErr: tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "choices",
				Detail: fmt.Sprintf("multiple default choices defined (%q and %q)", "a", "b"),
			}),
		},
		{ // empty alias
			name:    "empty alias",
			choices: []lpcli.Choice{{Value: "a", Key: "a", Aliases: []string{""}}},
			wantErr: tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "alias",
				Detail: fmt.Sprintf("choice %q has an empty alias", "a"),
			}),
		},
		{ // whitespace-only alias
			name:    "whitespace-only alias",
			choices: []lpcli.Choice{{Value: "a", Key: "a", Aliases: []string{"  "}}},
			wantErr: tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "alias",
				Detail: fmt.Sprintf("choice %q has an empty alias", "a"),
			}),
		},
		{ // alias with surrounding whitespace
			name:    "alias with surrounding whitespace",
			choices: []lpcli.Choice{{Value: "a", Key: "a", Aliases: []string{" all "}}},
			wantErr: tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "alias",
				Detail: fmt.Sprintf("%q of choice %q has surrounding whitespace", " all ", "a"),
			}),
		},
		{ // duplicate aliases within one choice
			name:    "duplicate aliases within one choice",
			choices: []lpcli.Choice{{Value: "a", Key: "a", Aliases: []string{"all", "ALL"}}},
			wantErr: tserr.DuplicateKey(&tserr.DuplicateKeyArgs{Key: "ALL", Existing: "a"}),
		},
		{ // alias collides with another choice's key
			name: "alias collides with another key",
			choices: []lpcli.Choice{
				{Value: "a", Key: "a"},
				{Value: "b", Key: "b", Aliases: []string{"a"}},
			},
			wantErr: tserr.DuplicateKey(&tserr.DuplicateKeyArgs{Key: "a", Existing: "a"}),
		},
		{ // alias identical to own key is allowed
			name:    "alias identical to own key is allowed",
			choices: []lpcli.Choice{{Value: "a", Key: "a", Aliases: []string{"A", "a"}}},
		},
		{ // alias collides with another choice's alias
			name: "alias collides with another alias",
			choices: []lpcli.Choice{
				{Value: "a", Key: "a", Aliases: []string{"alpha"}},
				{Value: "b", Key: "b", Aliases: []string{"Alpha"}},
			},
			wantErr: tserr.DuplicateKey(&tserr.DuplicateKeyArgs{Key: "Alpha", Existing: "a"}),
		},
		{ // key collides with an earlier alias
			name: "key collides with earlier alias",
			choices: []lpcli.Choice{
				{Value: "a", Key: "a", Aliases: []string{"alpha"}},
				{Value: "b", Key: "alpha"},
			},
			wantErr: tserr.DuplicateKey(&tserr.DuplicateKeyArgs{Key: "alpha", Existing: "a"}),
		},
		{ // valid choices with defaults and aliases
			name: "valid choices with default and aliases",
			choices: []lpcli.Choice{
				{Value: "apply", Key: "y", Aliases: []string{"yes"}, Label: "apply", IsDefault: true},
				{Value: "no", Key: "n", Aliases: []string{"no"}, Label: "skip"},
				{Value: "edit", Key: "e", Label: "edit"},
			},
			wantKey: "y",
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Create the options
			opts := lpcli.SelectOptions{Message: "Continue?", Choices: tt.choices}
			// Validate the options
			got, err := lpcli.Validate(opts)
			// Check if an error is expected
			if tt.wantErr != nil {
				// Check if the error is nil
				if err == nil {
					// If the error is nil, fail
					t.Fatal(tserr.NilFailed("validate()"))
				}
				// Check that the error is as expected
				if err.Error() != tt.wantErr.Error() {
					// If the error is not as expected, fail
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "validate() error", Want: tt.wantErr.Error(), Actual: err.Error()}))
				}
				// Return if the error is as expected
				return
			}
			// Check that the validation is an error
			if err != nil {
				// If the validation is an error, fail
				t.Fatal(tserr.Op(&tserr.OpArgs{Op: "validate()", Fn: "options", Err: err}))
			}
			// Check the returned default choice
			if tt.wantKey == "" {
				// No default expected; got must be nil
				if got != nil {
					// If a default choice was returned, fail
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "validate() default", Want: "nil", Actual: got.Key}))
				}
				// Return if no default is expected
				return
			}
			// A default is expected; got must not be nil
			if got == nil {
				// If no default choice was returned, fail
				t.Fatal(tserr.NilFailed("validate() default choice"))
			}
			// Check that the default choice key is as expected
			if got.Key != tt.wantKey {
				// If the default choice key is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "validate() default key", Want: tt.wantKey, Actual: got.Key}))
			}
		})
	}
}

// TestMatches verifies that matches compares the input against the
// key and aliases case-insensitively, tolerating surrounding
// whitespace on the configured key and aliases.
func TestMatches(t *testing.T) {
	// Create the tests
	tests := []struct {
		name   string
		choice lpcli.Choice
		input  string
		want   bool
	}{
		{ // exact key match
			name:   "exact key match",
			choice: lpcli.Choice{Key: "y"},
			input:  "y",
			want:   true,
		},
		{ // key match ignores case of input
			name:   "key match ignores input case",
			choice: lpcli.Choice{Key: "y"},
			input:  "Y",
			want:   true,
		},
		{ // key match ignores case of configured key
			name:   "key match ignores configured key case",
			choice: lpcli.Choice{Key: "Y"},
			input:  "y",
			want:   true,
		},
		{ // key match tolerates padded configured key
			name:   "key match tolerates padded configured key",
			choice: lpcli.Choice{Key: " y "},
			input:  "y",
			want:   true,
		},
		{ // key match trims input
			name:   "key match trims input",
			choice: lpcli.Choice{Key: "y"},
			input:  " y ",
			want:   true,
		},
		{ // empty key never matches empty input
			name:   "empty key never matches empty input",
			choice: lpcli.Choice{Key: ""},
			input:  "",
			want:   false,
		},
		{ // non-matching input
			name:   "non-matching input",
			choice: lpcli.Choice{Key: "y"},
			input:  "n",
			want:   false,
		},
		{ // alias match
			name:   "alias match",
			choice: lpcli.Choice{Key: "y", Aliases: []string{"yes"}},
			input:  "yes",
			want:   true,
		},
		{ // alias match ignores input case
			name:   "alias match ignores input case",
			choice: lpcli.Choice{Key: "y", Aliases: []string{"yes"}},
			input:  "YES",
			want:   true,
		},
		{ // alias match ignores configured alias case
			name:   "alias match ignores configured alias case",
			choice: lpcli.Choice{Key: "y", Aliases: []string{"Yes"}},
			input:  "yes",
			want:   true,
		},
		{ // alias match tolerates padded configured alias
			name:   "alias match tolerates padded configured alias",
			choice: lpcli.Choice{Key: "y", Aliases: []string{" yes "}},
			input:  "yes",
			want:   true,
		},
		{ // second alias match
			name:   "second alias match",
			choice: lpcli.Choice{Key: "y", Aliases: []string{"yeah", "yes"}},
			input:  "yes",
			want:   true,
		},
		{ // no aliases to match
			name:   "no aliases to match",
			choice: lpcli.Choice{Key: "y", Aliases: nil},
			input:  "yes",
			want:   false,
		},
		{ // input matching neither key nor aliases
			name:   "input matching neither key nor aliases",
			choice: lpcli.Choice{Key: "y", Aliases: []string{"yes"}},
			input:  "maybe",
			want:   false,
		},
		{ // empty input matches nothing
			name:   "empty input matches nothing",
			choice: lpcli.Choice{Key: "y", Aliases: []string{"yes"}},
			input:  "",
			want:   false,
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Match the input against the choice
			got := lpcli.Matches(tt.choice, tt.input)
			// Check that the match is as expected
			if got != tt.want {
				// If the match is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "matches(" + tt.input + ")", Want: strconv.FormatBool(tt.want), Actual: strconv.FormatBool(got)}))
			}
		})
	}
}

// TestPromptParts verifies that promptParts renders the prompt message
// with the default choice's key uppercased, all other keys lowercased,
// and a legend for labeled choices, and returns the display keys in
// choice order for the retry message. Keys are trimmed and empty keys
// are skipped, so rendering stays well-formed even with options that
// did not pass validate.
func TestPromptParts(t *testing.T) {
	// Create the tests
	tests := []struct {
		name     string
		opts     lpcli.SelectOptions
		wantMsg  string
		wantKeys []string
	}{
		{ // choices without labels render the plain format
			name: "choices without labels render plain format",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{
					{Value: "yes", Key: "y"},
					{Value: "no", Key: "n"},
				},
			},
			wantMsg:  "Continue? [y/n]: ",
			wantKeys: []string{"y", "n"},
		},
		{ // choices with labels render the legend format
			name: "choices with labels render legend format",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{
					{Value: "yes", Key: "y", Label: "yes"},
					{Value: "no", Key: "n", Label: "no"},
					{Value: "edit", Key: "e", Label: "edit"},
				},
			},
			wantMsg:  "Continue? [y/n/e] (y=yes, n=no, e=edit): ",
			wantKeys: []string{"y", "n", "e"},
		},
		{ // default key is uppercased in the brackets
			name: "default key is uppercased in brackets",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{
					{Value: "yes", Key: "y"},
					{Value: "no", Key: "n", IsDefault: true},
				},
			},
			wantMsg:  "Continue? [y/N]: ",
			wantKeys: []string{"y", "N"},
		},
		{ // default key is uppercased in the legend too
			name: "default key is uppercased in legend too",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{
					{Value: "yes", Key: "y", Label: "yes"},
					{Value: "no", Key: "n", Label: "no", IsDefault: true},
				},
			},
			wantMsg:  "Continue? [y/N] (y=yes, N=no): ",
			wantKeys: []string{"y", "N"},
		},
		{ // unlabeled choices are omitted from the legend
			name: "unlabeled choices are omitted from legend",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{
					{Value: "yes", Key: "y", Label: "yes"},
					{Value: "no", Key: "n"},
					{Value: "edit", Key: "e", Label: "edit"},
				},
			},
			wantMsg:  "Continue? [y/n/e] (y=yes, e=edit): ",
			wantKeys: []string{"y", "n", "e"},
		},
		{ // uppercase key is lowercased when not default
			name: "uppercase key is lowercased when not default",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{
					{Value: "yes", Key: "Y"},
					{Value: "no", Key: "N", IsDefault: true},
				},
			},
			wantMsg:  "Continue? [y/N]: ",
			wantKeys: []string{"y", "N"},
		},
		{ // multi-character default key is fully uppercased
			name: "multi-character default key is fully uppercased",
			opts: lpcli.SelectOptions{
				Message: "Apply?",
				Choices: []lpcli.Choice{
					{Value: "apply", Key: "apply", Label: "apply changes"},
					{Value: "abort", Key: "abort", IsDefault: true},
				},
			},
			wantMsg:  "Apply? [apply/ABORT] (apply=apply changes): ",
			wantKeys: []string{"apply", "ABORT"},
		},
		{ // single choice renders one key
			name: "single choice renders one key",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{{Value: "yes", Key: "y", Label: "yes"}},
			},
			wantMsg:  "Continue? [y] (y=yes): ",
			wantKeys: []string{"y"},
		},
		{ // empty message renders a leading space
			name: "empty message renders leading space",
			opts: lpcli.SelectOptions{
				Message: "",
				Choices: []lpcli.Choice{{Value: "yes", Key: "y"}},
			},
			wantMsg:  " [y]: ",
			wantKeys: []string{"y"},
		},
		{ // no choices renders the message without brackets
			name: "no choices renders message without brackets",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: nil,
			},
			wantMsg:  "Continue?: ",
			wantKeys: nil,
		},
		{ // all keys empty renders the message without brackets
			name: "all keys empty renders message without brackets",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{
					{Value: "broken", Key: ""},
					{Value: "broken", Key: "  "},
				},
			},
			wantMsg:  "Continue?: ",
			wantKeys: nil,
		},
		{ // empty keys are skipped
			name: "empty keys are skipped",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{
					{Value: "broken", Key: ""},
					{Value: "no", Key: "n"},
				},
			},
			wantMsg:  "Continue? [n]: ",
			wantKeys: []string{"n"},
		},
		{ // whitespace-only keys are skipped
			name: "whitespace-only keys are skipped",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{
					{Value: "broken", Key: "  "},
					{Value: "no", Key: "n"},
				},
			},
			wantMsg:  "Continue? [n]: ",
			wantKeys: []string{"n"},
		},
		{ // padded keys are trimmed for display
			name: "padded keys are trimmed for display",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{{Value: "yes", Key: " y "}},
			},
			wantMsg:  "Continue? [y]: ",
			wantKeys: []string{"y"},
		},
		{ // padded default key is trimmed then uppercased
			name: "padded default key is trimmed then uppercased",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{
					{Value: "yes", Key: "y"},
					{Value: "no", Key: " n ", IsDefault: true},
				},
			},
			wantMsg:  "Continue? [y/N]: ",
			wantKeys: []string{"y", "N"},
		},
		{ // skipped choice omits its label
			name: "skipped choice omits its label",
			opts: lpcli.SelectOptions{
				Message: "Continue?",
				Choices: []lpcli.Choice{
					{Value: "broken", Key: "", Label: "broken"},
					{Value: "no", Key: "n", Label: "no"},
				},
			},
			wantMsg:  "Continue? [n] (n=no): ",
			wantKeys: []string{"n"},
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Render the prompt parts
			gotMsg, gotKeys := lpcli.PromptParts(tt.opts)
			// Check that the message is as expected
			if gotMsg != tt.wantMsg {
				// If the message is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "promptParts() msg", Want: tt.wantMsg, Actual: gotMsg}))
			}
			// Check that the keys are as expected
			if !slices.Equal(gotKeys, tt.wantKeys) {
				// If the keys are not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "promptParts() keys", Want: fmt.Sprintf("%v", tt.wantKeys), Actual: fmt.Sprintf("%v", gotKeys)}))
			}
		})
	}
}

// TestPrompt verifies Prompt's handling of key, alias and case-insensitive
// selection, the default choice on empty input, the retry loop for unknown
// options, and the nil prompter, validation, read and cancellation errors.
func TestPrompt(t *testing.T) {
	// Create a prompt name
	pn := "lpcli"
	// Create the choices shared by most tests
	sharedChoices := []lpcli.Choice{
		{Value: "apply", Key: "y", Aliases: []string{"yes"}, Label: "apply", IsDefault: true},
		{Value: "skip", Key: "n", Aliases: []string{"no"}, Label: "skip"},
		{Value: "edit", Key: "e", Label: "edit"},
	}
	// Create the tests
	tests := []struct {
		name    string
		input   io.Reader
		want    string
		wantErr error
	}{
		{ // key selects the choice
			name:  "key selects the choice",
			input: sr("y\n"),
			want:  "apply",
		},
		{ // uppercase key is accepted
			name:  "uppercase key is accepted",
			input: sr("Y\n"),
			want:  "apply",
		},
		{ // alias selects the choice
			name:  "alias selects the choice",
			input: sr("yes\n"),
			want:  "apply",
		},
		{ // mixed-case alias is accepted
			name:  "mixed-case alias is accepted",
			input: sr("No\n"),
			want:  "skip",
		},
		{ // second choice's key selects it
			name:  "second choice's key selects it",
			input: sr("n\n"),
			want:  "skip",
		},
		{ // unlabeled choice is selectable
			name:  "unlabeled choice is selectable",
			input: sr("e\n"),
			want:  "edit",
		},
		{ // empty input selects the default
			name:  "empty input selects the default",
			input: sr("\n"),
			want:  "apply",
		},
		{ // padded input is trimmed and matches
			name:  "padded input is trimmed and matches",
			input: sr("  n \n"),
			want:  "skip",
		},
		{ // unknown option then valid key
			name:  "unknown option then valid key",
			input: sr("maybe\nn\n"),
			want:  "skip",
		},
		{ // unknown option then default
			name:  "unknown option then default",
			input: sr("maybe\n\n"),
			want:  "apply",
		},
		{ // empty input without default retries until a match
			name:  "empty input without default retries until a match",
			input: sr("\n\ny\n"),
			want:  "apply",
		},
		{ // invalid options without default retry until a match
			name:  "invalid options without default retry until a match",
			input: sr("a\nb\ne\n"),
			want:  "edit",
		},
		{ // eof without input aborts, wrapped as readLine operation
			name:    "eof without input aborts",
			input:   sr(""),
			wantErr: tserr.Op(&tserr.OpArgs{Op: "readLine", Fn: "prompter", Err: tserr.Aborted(pn)}),
		},
		{ // eof after invalid option aborts, wrapped as readLine operation
			name:    "eof after invalid option aborts",
			input:   sr("maybe\n"),
			wantErr: tserr.Op(&tserr.OpArgs{Op: "readLine", Fn: "prompter", Err: tserr.Aborted(pn)}),
		},
		{ // eof with unterminated matching input is processed
			name:  "eof with unterminated matching input is processed",
			input: sr("n"),
			want:  "skip",
		},
		{ // read error is wrapped as readLine operation
			name:    "read error is wrapped as readLine operation",
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
			// Discard the prompt output
			p.SetOut(io.Discard)
			// Set the input to the test input
			p.SetIn(tt.input)
			// Create the options with the shared choices
			opts := lpcli.SelectOptions{Message: "Continue?", Choices: sharedChoices}
			// Run the prompt
			got, err := p.Prompt(context.Background(), opts)
			// Check if an error is expected
			if tt.wantErr != nil {
				// Check if the error is nil
				if err == nil {
					// If the error is nil, fail
					t.Fatal(tserr.NilFailed("Prompt()"))
				}
				// Check that the error is as expected
				if err.Error() != tt.wantErr.Error() {
					// If the error is not as expected, fail
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Prompt() error", Want: tt.wantErr.Error(), Actual: err.Error()}))
				}
				// Return if the error is as expected
				return
			}
			// Check that the prompt is an error
			if err != nil {
				// If the prompt is an error, fail
				t.Fatal(tserr.Op(&tserr.OpArgs{Op: "Prompt()", Fn: "prompter", Err: err}))
			}
			// Check that the value is as expected
			if got != tt.want {
				// If the value is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Prompt()", Want: tt.want, Actual: got}))
			}
		})
	}
}

// TestPromptValidateErrors verifies that Prompt rejects invalid options
// by returning the validate error unchanged, before reading input.
func TestPromptValidateErrors(t *testing.T) {
	// Create a prompt name
	pn := "lpcli"
	// Create the tests
	tests := []struct {
		name    string
		choices []lpcli.Choice
		wantErr error
	}{
		{ // no choices
			name:    "no choices",
			choices: nil,
			wantErr: tserr.Empty("choices"),
		},
		{ // duplicate keys after normalization
			name: "duplicate keys after normalization",
			choices: []lpcli.Choice{
				{Value: "a", Key: "y"},
				{Value: "b", Key: "Y"},
			},
			wantErr: tserr.DuplicateKey(&tserr.DuplicateKeyArgs{Key: "Y", Existing: "y"}),
		},
	}
	// Iterate over the tests
	for _, tt := range tests {
		// Run the test
		t.Run(tt.name, func(t *testing.T) {
			// Create a new prompter
			p := lpcli.NewPrompter(pn)
			// Set the input; validate fails before it is read
			p.SetIn(sr("y\n"))
			// Run the prompt
			_, err := p.Prompt(context.Background(), lpcli.SelectOptions{Choices: tt.choices})
			// Check that the validate error is returned
			if err == nil {
				// If the error is nil, fail
				t.Fatal(tserr.NilFailed("Prompt()"))
			}
			// Check that the error is as expected
			if err.Error() != tt.wantErr.Error() {
				// If the error is not as expected, fail
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Prompt() error", Want: tt.wantErr.Error(), Actual: err.Error()}))
			}
		})
	}
}

// TestPromptNilPrompter verifies that Prompt on a nil prompter returns
// an error instead of panicking. The nil check precedes validate and
// any read, so valid options are passed to make the result unambiguous
// and no input is configured.
func TestPromptNilPrompter(t *testing.T) {
	// Create a nil prompter
	var p *lpcli.Prompter
	// Create valid options, so the only possible error is the nil pointer
	opts := lpcli.SelectOptions{
		Message: "Continue?",
		Choices: []lpcli.Choice{{Value: "apply", Key: "y"}},
	}
	// Run the prompt
	got, err := p.Prompt(context.Background(), opts)
	// Check that the prompt is an error
	if err == nil {
		// If the prompt is not an error, fail
		t.Fatal(tserr.NilFailed("Prompt()"))
	}
	// Check that the error is the nil pointer error
	if err.Error() != tserr.NilPtr().Error() {
		// If the error is not as expected, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Prompt() error", Want: tserr.NilPtr().Error(), Actual: err.Error()}))
	}
	// Check that the returned value is empty
	if got != "" {
		// If the value is not empty, fail
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Prompt()", Want: "", Actual: got}))
	}
}
