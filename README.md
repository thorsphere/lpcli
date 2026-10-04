# lpcli
A lightweight Go package for interactive CLI tools, providing simple confirmation prompts, text inputs, and inline editing via the system's default editor ($EDITOR).

Windows is supported on a best-effort basis; it is compile-checked but not regularly tested.

Intended usage:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

content, err := p.Edit(ctx, "draft text")
```
