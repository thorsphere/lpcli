// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package lpcli

import (
	"bufio"
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

// Prompt prompts the user to select one of the provided choices interactively.
func (p *Prompter) Prompt(reader *bufio.Reader, opts SelectOptions) (string, error) {
	if len(opts.Choices) == 0 {
		return "", tserr.Empty("choices")
	}

	var (
		keyParts      []string
		legendParts   []string
		defaultChoice *Choice
	)

	for _, c := range opts.Choices {
		key := c.Key
		if c.IsDefault {
			key = strings.ToUpper(key)
			copyChoice := c
			defaultChoice = &copyChoice
		} else {
			key = strings.ToLower(key)
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
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		trimmed := strings.TrimSpace(input)

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
