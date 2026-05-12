# pretty-zerolog

A small CLI that pretty-prints [zerolog](https://github.com/rs/zerolog) JSON log
output using zerolog's built-in `ConsoleWriter`. Pipe logs in from a running
program or pass one or more JSONL files as arguments.

## Installation

### Homebrew

```sh
brew install cybo42/tap/pretty-zerolog
```

### Go install

```sh
go install github.com/cybo42/pretty-zerolog@latest
```

### From source

```sh
git clone https://github.com/cybo42/pretty-zerolog.git
cd pretty-zerolog
make build
```

This produces a `pretty-zerolog` binary in the project root.

### Releases

Prebuilt binaries for macOS, Linux, and Windows are available on the
[GitHub Releases](https://github.com/cybo42/pretty-zerolog/releases) page.

## Usage

```
pretty-zerolog [--time-format default|full] [file ...]
```

### Flags

- `--time-format` — `default` (kitchen time, e.g. `3:04PM`) or `full`
  (RFC1123). Defaults to `default`.

### Pipe from a running program

Most zerolog setups write JSON to stderr, so redirect stderr into the pipe:

```sh
my-app 2>&1 | pretty-zerolog
```

Or with process substitution to only pretty-print stderr:

```sh
my-app 2> >(pretty-zerolog)
```

### Read from files

```sh
pretty-zerolog app.jsonl
pretty-zerolog --time-format full app.jsonl other.jsonl
```

## Example

Given JSON input like:

```json
{"level":"info","time":"2026-05-12T10:15:30Z","message":"server started","port":8080}
{"level":"error","time":"2026-05-12T10:15:31Z","message":"db connect failed","err":"timeout"}
```

`pretty-zerolog` renders it with colored levels, a compact timestamp, and
key=value fields — the same output you would get from
`zerolog.ConsoleWriter` in development.

## License

MIT — see [LICENSE](LICENSE).
