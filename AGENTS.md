# AGENTS.md

## Development

### Git

#### Emoji Commit

- Use emoji + (category) + description format for commit messages.
  - Emoji should match the context
- Keep messages concise and descriptive.
- Examples:
  - 💚 (golangci-lint) upgrade to v2.11
  - 🎉 (kmw) new ratemw middleware
  - ✨ (bunrepo) new `ExcludeColumns()` option

## Language

### Golang

Use `go doc` command to find the library API.

```sh
go doc github.com/maypok86/otter/v2
```

- Use `maps`, `slices` library when possible
- Run `go fix` after coding
  - See https://go.dev/blog/gofix for details
