// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli

import (
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
	if strings.EqualFold(c.Key, input) {
		return true
	}
	for _, alias := range c.Aliases {
		if strings.EqualFold(alias, input) {
			return true
		}
	}
	return false
}

// Validate ensures options are well-formed and returns the default choice if one is configured.
func (opts SelectOptions) Validate() (*Choice, error) {
	if len(opts.Choices) == 0 {
		return nil, tserr.Empty("choices")
	}

	var defaultChoice *Choice
	seen := make(map[string]string) // normalized input -> original choice key

	for i := range opts.Choices {
		c := &opts.Choices[i]
		key := strings.TrimSpace(c.Key)
		if key == "" {
			return nil, tserr.InvalidFormat("choice key cannot be empty")
		}

		// Check multiple defaults
		if c.IsDefault {
			if defaultChoice != nil {
				return nil, tserr.InvalidFormat(fmt.Sprintf("multiple default choices defined (%q and %q)", defaultChoice.Key, c.Key))
			}
			defaultChoice = c
		}

		// Check for duplicate key
		normKey := strings.ToLower(key)
		if existing, exists := seen[normKey]; exists {
			return nil, tserr.InvalidFormat(fmt.Sprintf("duplicate choice key %q (already registered by choice %q)", key, existing))
		}
		seen[normKey] = key

		// Check for duplicate aliases or alias colliding with another key
		for _, alias := range c.Aliases {
			aliasTrimmed := strings.TrimSpace(alias)
			if aliasTrimmed == "" {
				continue
			}
			normAlias := strings.ToLower(aliasTrimmed)
			if existing, exists := seen[normAlias]; exists {
				return nil, tserr.InvalidFormat(fmt.Sprintf("duplicate alias %q in choice %q (already registered by choice %q)", alias, key, existing))
			}
			seen[normAlias] = key
		}
	}

	return defaultChoice, nil
}

// Prompt prompts the user to select one of the provided choices interactively.
func (p *Prompter) Prompt(opts SelectOptions) (string, error) {
	if p == nil {
		return "", tserr.NilPtr()
	}

	defaultChoice, err := opts.Validate()
	if err != nil {
		return "", err
	}

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

	var promptMsg string
	if len(legendParts) > 0 {
		promptMsg = fmt.Sprintf("%s [%s] (%s): ",
			opts.Message,
			strings.Join(keyParts, "/"),
			strings.Join(legendParts, ", "),
		)
	} else {
		promptMsg = fmt.Sprintf("%s [%s]: ",
			opts.Message,
			strings.Join(keyParts, "/"),
		)
	}

	for {
		fmt.Fprint(p.Out, promptMsg)
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

		fmt.Fprintf(p.Out, "Unknown option %q. Please choose [%s].\n", trimmed, strings.Join(keyParts, "/"))
	}
}
