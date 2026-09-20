// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli

import (
	"context"
	"fmt"
	"strings"

	"github.com/thorsphere/tserr"
)

// Choice represents an option the user can select.
type Choice struct {
	Value     string   // The returned value/key (e.g. choiceAccept, "apply", etc.)
	Key       string   // Primary input key displayed in the prompt, e.g. "y"
	Aliases   []string // Alternative inputs, e.g. ["yes"]
	Label     string   // Description for help/legend, e.g. "commit"
	IsDefault bool     // Whether pressing Enter selects this choice
}

type SelectOptions struct {
	Message string
	Choices []Choice
}

// Matches returns true if the normalized input matches the key or any alias.
func (c Choice) matches(input string) bool {
	if strings.EqualFold(strings.TrimSpace(c.Key), input) {
		return true
	}
	for _, alias := range c.Aliases {
		if strings.EqualFold(strings.TrimSpace(alias), input) {
			return true
		}
	}
	return false
}

// Validate ensures options are well-formed and returns the default choice if one is configured.
func (opts SelectOptions) validate() (*Choice, error) {
	if len(opts.Choices) == 0 {
		return nil, tserr.Empty("choices")
	}

	var defaultChoice *Choice
	seen := make(map[string]string) // normalized input -> original choice key

	for i := range opts.Choices {
		c := &opts.Choices[i]
		key := strings.TrimSpace(c.Key)
		if key == "" {
			return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "choice key",
				Detail: "cannot be empty",
			})
		}
		if key != c.Key {
			return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "choice key",
				Detail: fmt.Sprintf("%q has surrounding whitespace", c.Key),
			})
		}

		// Check multiple defaults
		if c.IsDefault {
			if defaultChoice != nil {
				return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "choices",
					Detail: fmt.Sprintf("multiple default choices defined (%q and %q)", defaultChoice.Key, c.Key),
				})
			}
			defaultChoice = c
		}

		// Check for duplicate key
		normKey := strings.ToLower(key)
		if existing, exists := seen[normKey]; exists {
			return nil, tserr.DuplicateKey(&tserr.DuplicateKeyArgs{
				Key:      key,
				Existing: existing,
			})
		}
		seen[normKey] = key

		// Check for duplicate aliases or alias colliding with another key
		for _, alias := range c.Aliases {
			aliasTrimmed := strings.TrimSpace(alias)
			if aliasTrimmed == "" {
				return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "alias",
					Detail: fmt.Sprintf("choice %q has an empty alias", key),
				})
			}
			if aliasTrimmed != alias {
				return nil, tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "alias",
					Detail: fmt.Sprintf("%q of choice %q has surrounding whitespace", alias, key),
				})
			}
			normAlias := strings.ToLower(aliasTrimmed)
			if normAlias == normKey {
				// An alias identical to its own choice's key is redundant
				// but harmless; the key is already registered in seen.
				continue
			}
			if existing, exists := seen[normAlias]; exists {
				return nil, tserr.DuplicateKey(&tserr.DuplicateKeyArgs{
					Key:      alias,
					Existing: existing,
				})
			}
			seen[normAlias] = key
		}
	}

	return defaultChoice, nil
}

// Prompt prompts the user to select one of the provided choices
// interactively. Returns the selected choice's Value. Returns an error
// if ctx is cancelled.
func (p *Prompter) Prompt(ctx context.Context, opts SelectOptions) (string, error) {
	if p == nil {
		return "", tserr.NilPtr()
	}

	defaultChoice, err := opts.validate()
	if err != nil {
		return "", err
	}

	promptMsg, keyParts := opts.promptParts()

	for {
		// Check if context was cancelled
		if err := ctx.Err(); err != nil {
			// Prompt was cancelled by context, return an error
			return "", tserr.Aborted(p.Name)
		}

		fmt.Fprint(p.Out(), promptMsg)
		trimmed, err := p.readLine()
		if err != nil {
			return "", err
		}

		if trimmed == "" && defaultChoice != nil {
			return defaultChoice.Value, nil
		}

		for _, c := range opts.Choices {
			if c.matches(trimmed) {
				return c.Value, nil
			}
		}

		fmt.Fprintf(p.Out(), "Unknown option %q. Please choose [%s].\n", trimmed, strings.Join(keyParts, "/"))
	}
}

// promptParts renders the prompt message shown to the user, e.g.:
//
//	Continue? [y/N/e] (y=yes, n=no, e=edit):
//
// The default choice's key is uppercased; all others are lowercased.
// Choices with a Label appear in the parenthesized legend.
// promptParts returns the rendered prompt message and the list of
// display keys (used for the "Unknown option" retry message).
func (opts SelectOptions) promptParts() (msg string, keys []string) {
    var (
        keyParts    []string
        legendParts []string
    )

    for _, c := range opts.Choices {
        key := strings.ToLower(c.Key)
        if c.IsDefault {
            key = strings.ToUpper(c.Key)
        }
        keyParts = append(keyParts, key)

        if c.Label != "" {
            legendParts = append(legendParts, fmt.Sprintf("%s=%s", key, c.Label))
        }
    }

    if len(legendParts) > 0 {
        msg = fmt.Sprintf("%s [%s] (%s): ",
            opts.Message, strings.Join(keyParts, "/"), strings.Join(legendParts, ", "))
    } else {
        msg = fmt.Sprintf("%s [%s]: ", opts.Message, strings.Join(keyParts, "/"))
    }
    return msg, keyParts
}
