# lpcli

[![PkgGoDev](https://pkg.go.dev/badge/mod/github.com/thorsphere/lpcli)](https://pkg.go.dev/mod/github.com/thorsphere/lpcli)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/thorsphere/lpcli)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/thorsphere/lpcli)
![GitHub Top Language](https://img.shields.io/github/languages/top/thorsphere/lpcli)
[![CodeFactor](https://www.codefactor.io/repository/github/thorsphere/lpcli/badge)](https://www.codefactor.io/repository/github/thorsphere/lpcli)
![OSS Lifecycle](https://img.shields.io/osslifecycle/thorsphere/lpcli)

---

A lightweight Go package for interactive CLI tools, providing simple confirmation prompts, choice selection, and inline editing via the system's default editor ($VISUAL / $EDITOR).

Windows is supported on a best-effort basis; it is compile-checked but not regularly tested.

---

## Features

- **Yes/no confirmations**: `Confirm` prompts for a quick yes/no answer, with Enter defaulting to "yes" for low-stakes operations.
- **Choice selection**: `Prompt` presents a set of choices with keys, aliases, labels, and an optional default; unknown input re-prompts with a helpful hint.
- **Inline editing**: `Edit` opens text in the user's editor (`$VISUAL` / `$EDITOR`, with sensible fallbacks) and returns the edited result.
- **Context-aware**: every prompt takes a `context.Context` and returns an abort error once it is cancelled; `Edit` additionally kills the editor subprocess and removes its temporary file.
- **Testable I/O**: input and output streams can be swapped via `SetIn` / `SetOut` for testing or custom wiring.
- **Lightweight**: no terminal UI frameworks; a single small dependency (`tserr`) for error handling.

---

## Getting Started

Requires Go 1.27 or later.

```sh
go get github.com/thorsphere/lpcli
```

Optionally set `VISUAL` or `EDITOR` to control which editor `Edit` launches (`VISUAL` takes precedence, per POSIX convention). Values may include arguments, e.g. `EDITOR="code --wait"`. If neither variable is set, lpcli falls back to common editors (`nano`, `vim`, `vi`, `emacs` on Unix-like systems; `notepad` on Windows).

---

## Usage

Create a prompter once and reuse it. Prompts read from `os.Stdin` and write to `os.Stderr` by default, so they don't pollute piped output. Input matching is case-insensitive, and pressing Ctrl+D (EOF) on an empty line aborts cleanly.

```go
p := lpcli.NewPrompter("mytool")
```

**Confirm**: a yes/no prompt. Accepts `y`/`yes`/Enter (yes) and `n`/`no` (abort), case-insensitively. Returns an abort error on "no" or when `ctx` is cancelled. Since Enter means yes, the conventional rendering is `[Y/n]`: the uppercase letter marks the default.

```go
if err := p.Confirm(ctx, "Stage the changes? [Y/n]: "); err != nil {
    // user said no, or the context was cancelled
}
```

**Prompt**: select one of several choices. Each `Choice` has a `Value` (returned to you), a `Key` (what the user types), optional `Aliases`, an optional `Label` for the legend, and an optional `IsDefault` (Enter selects it). Returns the selected choice's `Value`. The rendered prompt lists the keys in brackets with a legend, uppercasing the default key, e.g. `What would you like to do? [A/p/q] (A=apply changes, p=show a plan, q=quit):`.

```go
action, err := p.Prompt(ctx, lpcli.SelectOptions{
    Message: "What would you like to do?",
    Choices: []lpcli.Choice{
        {Value: "apply", Key: "a", Aliases: []string{"apply"}, Label: "apply changes", IsDefault: true},
        {Value: "plan", Key: "p", Label: "show a plan"},
        {Value: "quit", Key: "q", Label: "quit"},
    },
})
```

**Edit**: open text in the user's editor and get the result back. The temporary file is removed automatically.

```go
content, err := p.Edit(ctx, "draft text")
```

## Example

A small program combining all three interactions:

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/thorsphere/lpcli"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    p := lpcli.NewPrompter("mytool")

    // 1. Confirm (Enter = yes)
    if err := p.Confirm(ctx, "Stage the changes before committing? [Y/n]: "); err != nil {
        fmt.Fprintln(os.Stderr, "aborted:", err)
        os.Exit(1)
    }

    // 2. Choose an action
    action, err := p.Prompt(ctx, lpcli.SelectOptions{
        Message: "How should the changes be committed?",
        Choices: []lpcli.Choice{
            {Value: "direct", Key: "d", Label: "commit directly", IsDefault: true},
            {Value: "edit", Key: "e", Label: "edit the message first"},
            {Value: "quit", Key: "q", Label: "quit without committing"},
        },
    })
    if err != nil {
        fmt.Fprintln(os.Stderr, "aborted:", err)
        os.Exit(1)
    }
    if action == "quit" {
        fmt.Println("nothing committed")
        return
    }

    message := "commit message"
    if action == "edit" {
        // 3. Optionally edit the message in $VISUAL / $EDITOR
        message, err = p.Edit(ctx, message)
        if err != nil {
            fmt.Fprintln(os.Stderr, "aborted:", err)
            os.Exit(1)
        }
    }

    fmt.Println("committing:", message)
}
```

---

## Known Limitations

- **Windows support is best-effort**: the code is compile-checked but not regularly tested on Windows.
- **Line-based input only**: prompts read whole lines from stdin; there is no raw-mode UI (no arrow keys or menus). Input should be line-terminated, though a non-empty final line at EOF is accepted.
- **Cancellation is only noticed between inputs**: `Confirm` and `Prompt` check `ctx` before each read; while waiting for the user to press Enter, cancellation is not observed until input arrives (or EOF). `Edit` is different: cancelling `ctx` kills the editor immediately.
- **The editor needs a real TTY**: `Edit` attaches the editor subprocess to the process's `os.Stdin`/`os.Stdout`/`os.Stderr`, not to the prompter's `SetIn`/`SetOut` streams, because editors require a terminal to render their UI.
- **Cancellation discards editor changes**: when `ctx` is cancelled, the editor process is killed and its unsaved changes are lost; the temporary file is still cleaned up.
- **Confirm defaults to "yes"**: pressing Enter confirms. Callers guarding destructive actions should use `Prompt` and require an explicit key instead.
- **An editor must be available**: `Edit` returns an error if no usable editor is found via `VISUAL`, `EDITOR`, or the built-in fallbacks.
- **An empty edit is not an error**: `Edit` returns the file's contents verbatim, so a user who deletes everything and saves gets `""` and a `nil` error. Callers that require content must check for it — `strings.TrimSpace(content) == ""` is usually what you want, since editors leave a trailing newline. Compare against the initial text as well if you need to detect "closed without editing".

---

## Documentation & Resources

- [Go Package Documentation](https://pkg.go.dev/github.com/thorsphere/lpcli): Complete API reference
- [Open Source Insights](https://deps.dev/go/github.com%2Fthorsphere%2Flpcli): Dependency analysis

---

## ⚖️ License & Commercial Usage

Copyright (c) 2026 thorsphere. All rights reserved.

This project is licensed under the **Functional Source License v1.1 (FSL-1.1-ALv2)**. 

* The use, modification, and redistribution of this Go package is completely free for private, educational, non-commercial, and internal purposes. 
* If you are a company or institution looking to use this package in a commercial product, service, or business environment, you must secure a commercial license.
* Each version of this software automatically converts to the fully open-source Apache License, Version 2.0 on the second anniversary of its release.

For full details, please see the [LICENSE](LICENSE.md) file.

### 💼 Commercial Licensing & Inquiries

To purchase a commercial license or discuss support options, please reach out directly:

* 📩 **Contact:** business at thorsphere dot com
* 💬 **Response Time:** Usually within a couple of business days.

*Please include your company name and a brief overview of your use case so I can provide the right licensing details.*
