# dog 🐶

A smarter `cat` for files, URLs, and GitHub — with content sniffing, JWT
decoding, JSON pretty-printing, bark alerts, and `tail -f` follow mode.

## Install

```sh
go install github.com/dog_util@latest
```

Or build locally:

```sh
go build -o dog .
```

## Usage

```
dog [OPTIONS] <TARGET>...
```

### Targets

| Pattern | Description |
|---------|-------------|
| `<file>` | Local file path |
| `-` | Standard input (default when no target given) |
| `http[s]://...` | HTTP/HTTPS URL |
| `gh:owner/repo[@ref]/path` | GitHub raw file (`ref` defaults to `main`) |

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `-f`, `--follow` | false | Follow file changes (like `tail -f`) |
| `-s`, `--sniff` | true | Auto-detect content (JSON, JWT, binary) |
| `--raw` | false | Stream raw bytes, no formatting |
| `-b`, `--bark <regex>` | — | Alert on regex match |
| `--poll-ms <n>` | 100 | Follow mode poll interval (ms) |
| `-v` | false | Verbose / debug logging |
| `--version` | — | Print version and exit |

### Environment

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | Bearer token for authenticated GitHub API requests |

## Examples

```sh
# Pretty-print a local JSON file
dog config.json

# Decode a JWT token from stdin
pbpaste | dog

# Follow a log file and alert on ERROR lines
dog -f --bark ERROR app.log

# Fetch and pretty-print a remote JSON API
dog https://api.github.com/repos/golang/go

# Fetch a file from GitHub (uses GITHUB_TOKEN if set)
dog gh:spf13/cobra@main/README.md

# Stream raw bytes from a URL
dog --raw https://example.com/archive.tar.gz | tar xz
```

## Content sniffing

When `--sniff` is enabled (default), `dog` inspects the first 4 KB of any
stream and applies formatting automatically:

1. **Binary** — non-UTF-8 or null-byte content is displayed as a hex dump.
2. **JWT** — a three-part dot-separated `eyJ…` string is decoded and
   pretty-printed as header + payload + signature preview.
3. **JSON** — valid JSON is pretty-printed with 2-space indentation.
4. **Text** — everything else is streamed line-by-line with optional bark
   alerting.

Pass `--raw` to bypass all sniffing and write bytes directly to stdout.

## Follow mode

`-f` / `--follow` works like `tail -f` but also detects file truncation and
rotation: when the file shrinks (e.g. log rotation), `dog` reopens it from the
beginning automatically. Use `--poll-ms` to tune the polling interval.

## Security

- All HTTPS requests enforce **TLS 1.2+**.
- `GITHUB_TOKEN` is passed as a `Bearer` header and is **never** logged.
- Secrets are not written to any log output.
