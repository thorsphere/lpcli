// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli

// Import packages
import (
	"context" // context
	"fmt"     // fmt
	"strings" // strings

	"github.com/thorsphere/tserr" // tserr
)

// Choice represents an option the user can select. Value is the
// returned value/key (e.g. choiceAccept, "apply", etc.). Key is the
// primary input key displayed in the prompt, e.g. "y". Aliases are
// alternative inputs, e.g. ["yes"]. Label is a description for help/legend,
// e.g. "commit". IsDefault is whether pressing Enter selects this choice.
type Choice struct {
	Value     string   // The returned value/key (e.g. choiceAccept, "apply", etc.)
	Key       string   // Primary input key displayed in the prompt, e.g. "y"
	Aliases   []string // Alternative inputs, e.g. ["yes"]
	Label     string   // Description for help/legend, e.g. "commit"
	IsDefault bool     // Whether pressing Enter selects this choice
}

// SelectOptions configures the prompt. Message is the prompt message
// shown to the user, e.g. "Continue? [y/N/e] (y=yes, N=no, e=edit):".
// Choices are the options the user can select, e.g. [{Key: "y", Label: "yes"},
// {Key: "n", Label: "no"}]. The first choice with IsDefault set to true
// is the default choice.
type SelectOptions struct {
	Message string   // Prompt message
	Choices []Choice // Choices
}

// matches returns true if the input matches the key or any alias,
// case-insensitively. The input and the configured key and aliases
// are trimmed before comparison. Empty input never matches.
func (c Choice) matches(input string) bool {
	// Trim the input, so the method is robust even when called
	// with raw, untrimmed input. Prompt already passes pre-trimmed
	// input, but matches must not rely on that precondition.
	input = strings.TrimSpace(input)
	// Empty input must never select a choice. Without this guard,
	// a choice with an empty key (rejected by validate, but possible
	// when matches is called standalone) would match empty input.
	if input == "" {
		// If the input is empty, return false
		return false
	}
	// Check if the input matches the key
	if strings.EqualFold(strings.TrimSpace(c.Key), input) {
		// If the input matches the key, return true
		return true
	}
	// Iterate over the aliases
	for _, alias := range c.Aliases {
		// Check if the input matches the alias
		if strings.EqualFold(strings.TrimSpace(alias), input) {
			// If the input matches the alias, return true
			return true
		}
	}
	// If the input does not match any alias, return false
	return false
}

// Validate ensures options are well-formed and returns the default choice if one is configured.
// It returns an error otherwise. It checks for empty keys, multiple default choices, whether keys
// have surrounding whitespace, for duplicate keys after normalization, for empty aliases,
// for aliases with surrounding whitespace, and for duplicate aliases after normalization.
func (opts SelectOptions) validate() (*Choice, error) {
	// Check if there are any choices
	if len(opts.Choices) == 0 {
		// If there are no choices, return an error
		return nil, tserr.Empty("choices")
	}
	// Create a default choice
	var defaultChoice *Choice
	// Create a map of normalized input -> original choice key
	seen := make(map[string]string) // normalized input -> original choice key
	// Iterate over the choices
	for i := range opts.Choices {
		// Get the choice
		c := &opts.Choices[i]
		// Trim the choice key
		key := strings.TrimSpace(c.Key)
		// Check if the choice key is empty
		if key == "" {
			// If the choice key is empty, return an error
			return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "choice key",
				Detail: "cannot be empty",
			})
		}
		// Check if the trimmed choice key is the same as the original.
		// This ensures that the choice key does not have surrounding whitespace.
		if key != c.Key {
			// If the choice key is not the same as the original, return an error
			return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "choice key",
				Detail: fmt.Sprintf("%q has surrounding whitespace", c.Key),
			})
		}
		// Check multiple defaults
		if c.IsDefault {
			// Check if there is already a default choice
			if defaultChoice != nil {
				// If there is, return an error
				return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "choices",
					Detail: fmt.Sprintf("multiple default choices defined (%q and %q)", defaultChoice.Key, c.Key),
				})
			}
			// Set the default choice
			defaultChoice = c
		}
		// Check for duplicate key
		// First normalize the key
		normKey := strings.ToLower(key)
		// Check if the normalized key is already seen
		if existing, exists := seen[normKey]; exists {
			// If the normalized key is already seen, return an error
			return nil, tserr.DuplicateKey(&tserr.DuplicateKeyArgs{
				Key:      key,
				Existing: existing,
			})
		}
		// Add the normalized key to the seen map
		seen[normKey] = key
		// Check for duplicate aliases or alias colliding with another key
		// Iterate over the aliases
		for _, alias := range c.Aliases {
			// Trim the alias
			aliasTrimmed := strings.TrimSpace(alias)
			// Check if the alias is empty
			if aliasTrimmed == "" {
				// If the alias is empty, return an error
				return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "alias",
					Detail: fmt.Sprintf("choice %q has an empty alias", key),
				})
			}
			// Check if the alias is not the same as the original after trimming
			if aliasTrimmed != alias {
				// If the alias is not the same as the original, return an error
				// meaning that the alias has surrounding whitespace
				return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "alias",
					Detail: fmt.Sprintf("%q of choice %q has surrounding whitespace", alias, key),
				})
			}
			// Normalize the alias
			normAlias := strings.ToLower(aliasTrimmed)
			// Check if the normalized alias is the same as the normalized key
			if normAlias == normKey {
				// An alias identical to its own choice's key is redundant
				// but harmless; the key is already registered in seen.
				continue
			}
			// Check if the normalized alias is already seen
			if existing, exists := seen[normAlias]; exists {
				// Return an error if the normalized alias is already seen
				return nil, tserr.DuplicateKey(&tserr.DuplicateKeyArgs{
					Key:      alias,
					Existing: existing,
				})
			}
			// Add the normalized alias to the seen map
			seen[normAlias] = key
		}
	}
	// Return the default choice if one is configured and nil to indicate success
	return defaultChoice, nil
}

// Prompt prompts the user to select one of the provided choices
// interactively. Returns the selected choice's Value. Returns an error
// if ctx is cancelled.
func (p *Prompter) Prompt(ctx context.Context, opts SelectOptions) (string, error) {
	// Check if the prompter is nil
	if p == nil {
		// If the prompter is nil, return an error
		return "", tserr.NilPtr()
	}
	// Validate the options and get the default choice
	defaultChoice, err := opts.validate()
	// Check if validation failed
	if err != nil {
		// If validation failed, return the error
		return "", err
	}
	// Render the prompt message and get the display keys
	promptMsg, keyParts := opts.promptParts()
	// Retry until the user provides a valid answer
	for {
		// Check if context was cancelled
		if err := ctx.Err(); err != nil {
			// Prompt was cancelled by context, return an error
			return "", tserr.Aborted(p.Name)
		}
		// Print the prompt message
		fmt.Fprint(p.Out(), promptMsg)
		// Read the user input
		trimmed, err := p.readLine()
		// Check if reading the user input failed
		if err != nil {
			// If reading the user input failed, return the error
			return "", tserr.Op(&tserr.OpArgs{Op: "readLine", Fn: "prompter", Err: err})
		}
		// Check if the user input is empty and the default choice is configured
		if trimmed == "" && defaultChoice != nil {
			// If the user input is empty and the default choice is configured,
			// return the default choice's value
			return defaultChoice.Value, nil
		}
		// Iterate over the choices
		for _, c := range opts.Choices {
			// Check if the user input matches the choice
			if c.matches(trimmed) {
				// If the user input matches the choice, return the choice's value
				return c.Value, nil
			}
		}
		// Print the retry message
		fmt.Fprintf(p.Out(), "Unknown option %q. Please choose [%s].\n", trimmed, strings.Join(keyParts, "/"))
	}
}

// promptParts renders the prompt message shown to the user, e.g.:
//
//	Continue? [y/N/e] (y=yes, N=no, e=edit):
//
// The default choice's key is uppercased in full (so "abort" renders
// as "ABORT"); all other keys are lowercased. The legend reuses the
// same display keys as the bracket list, so the default appears
// uppercased in both. Choices with a Label appear in the parenthesized
// legend; choices without one are omitted from it. Keys are trimmed
// and empty keys are skipped, so rendering stays well-formed even
// with options that did not pass validate; if no displayable key
// remains, the message is rendered without brackets.
// promptParts returns the rendered prompt message and the list of
// display keys (used for the "Unknown option" retry message).
func (opts SelectOptions) promptParts() (msg string, keys []string) {
	// Create the key and legend parts
	var (
		keyParts    []string
		legendParts []string
	)
	// Iterate over the choices
	for _, c := range opts.Choices {
		// Trim the key for display, mirroring the leniency of
		// matches: the user types the trimmed key, so the trimmed
		// key is what should be shown. validate rejects padded
		// keys, so through Prompt the trim is a no-op.
		key := strings.TrimSpace(c.Key)
		// Check if the key is empty
		if key == "" {
			// Skip choices with empty keys: they are unselectable
			// (matches never matches empty input) and rendering
			// them would produce broken lists like "[/n]".
			// validate rejects them; this guard keeps the renderer
			// safe when called standalone.
			continue
		}
		// Check if the choice is the default
		if c.IsDefault {
			// Uppercase the whole key to signal that pressing
			// Enter selects this choice
			key = strings.ToUpper(key)
		} else {
			// Lowercase all other keys
			key = strings.ToLower(key)
		}
		// Append the display key to the key parts
		keyParts = append(keyParts, key)
		// Check if the choice has a label
		if c.Label != "" {
			// Append the legend entry, reusing the same display
			// key as the bracket list
			legendParts = append(legendParts, fmt.Sprintf("%s=%s", key, c.Label))
		}
	}
	// Check if any key is displayable
	if len(keyParts) == 0 {
		// Render without brackets instead of a broken "[]" list;
		// unreachable through Prompt, which requires choices
		return fmt.Sprintf("%s: ", opts.Message), keyParts
	}
	// Check if any legend entry exists
	if len(legendParts) > 0 {
		// Render the message with brackets and legend
		msg = fmt.Sprintf("%s [%s] (%s): ",
			opts.Message, strings.Join(keyParts, "/"), strings.Join(legendParts, ", "))
	} else {
		// Render the message with brackets only
		msg = fmt.Sprintf("%s [%s]: ", opts.Message, strings.Join(keyParts, "/"))
	}
	// Return the rendered message and the display keys
	return msg, keyParts
}
