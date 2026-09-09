# slogenv

[![CI](https://github.com/salandered/slogenv/actions/workflows/ci.yml/badge.svg)](https://github.com/salandered/slogenv/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/salandered/slogenv.svg)](https://pkg.go.dev/github.com/salandered/slogenv)
[![codecov](https://codecov.io/gh/salandered/slogenv/branch/main/graph/badge.svg)](https://codecov.io/gh/salandered/slogenv)
[![Go Version](https://img.shields.io/github/go-mod/go-version/salandered/slogenv)](go.mod)
[![Latest tag](https://img.shields.io/github/v/tag/salandered/slogenv?sort=semver&label=release)](https://github.com/salandered/slogenv/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Slog setup.

Text output goes through [tint](https://github.com/lmittmann/tint).

JSON output is the stdlib [slog.JSONHandler](https://pkg.go.dev/log/slog#JSONHandler). Verbose and simple.

## Compatibility

No backward compatibility between versions.

## Usage

`Setup` installs the default logger with `slog.SetDefault` and echoes the resolved config at Info.

```go
cfg, err := slogenv.ConfigFromEnv()
if err != nil {
	return err
}
closer, err := slogenv.Setup(cfg, requestid.LogAttrs)
if err != nil {
	return err
}
defer func() { _ = closer.Close() }()
```

`NewHandler` is the same construction without the global write, for a caller that wants its own
`*slog.Logger`:

```go
h, closer, err := slogenv.NewHandler(cfg, nil)
```

The returned `io.Closer` closes the log file. A no-op in case of stdout.

## Environment

See `func ConfigFromEnv() (Config, error)`.

Values are trimmed and lowercased. An unknown one is an error.
`Config`'s zero value resolves to the same defaults.

## Context attrs

Passed using function

```go
type AttrsFunc func(ctx context.Context) []slog.Attr
```

Example

```go
func LogAttrs(ctx context.Context) []slog.Attr {
	if id := FromContext(ctx); id != "" {
		return []slog.Attr{slog.String("request_id", id)}
	}
	return nil
}
```

Attrs would be injected right after the message, before the call site's own attrs.
A nil `AttrsFunc` leaves the record untouched.

May be used without the rest of the package:

```go
slog.New(slogenv.NewContextHandler(myHandler, requestid.LogAttrs))
```

## Dev

### Test

```sh
make audit
go test -race ./...
```

### Release

Check CI is ok.

```sh
git tag --list
git tag -a v0.x.0 -m "v0.x.0"
git push origin v0.x.0
```
