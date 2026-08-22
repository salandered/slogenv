# slogenv

Slog setup I use across projects.

Text output goes through [tint](https://github.com/lmittmann/tint), the only non-stdlib dependency.

## Usage

```go
cfg, err := logging.ConfigFromEnv()
if err != nil {
	return err
}
closer, err := logging.Setup(cfg, requestid.LogAttrs)
if err != nil {
	return err
}
defer func() { _ = closer.Close() }()
```

`Setup` installs the default logger with `slog.SetDefault` and echoes the resolved config at Info.
`NewHandler` is the same construction without the global write, for a caller that wants its own
`*slog.Logger`:

```go
h, closer, err := logging.NewHandler(cfg, nil)
```

The returned `io.Closer` closes the log file. A no-op in case of stdout.

## Environment

| Var          | Values                                                           | Default              | Notes                                                                                                     |
| ------------ | ---------------------------------------------------------------- | -------------------- | --------------------------------------------------------------------------------------------------------- |
| `LOG_LEVEL`  | `debug` `info` `warn` `error`                                    | `info`               | Minimum log level being printed.                                                                          |
| `LOG_FORMAT` | `text` `json`                                                    | `text`               | `text` is a human readable format (colorized if using stdout); `json` is for machines.                    |
| `LOG_FILE`   | file path                                                        | *(empty -> stdout)*  | If set, logs go to this file only.                                                                        |
| `LOG_TIME`   | `sec` `milli` `nano` `dt-sec` `dt-milli` `rfc3339` `rfc3339nano` | `dt-milli`           | timestamp layout; `dt-` means the date is printed. Ignored if `LOG_FORMAT` is `json` (always RFC3339Nano). |

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

Attrs injected right after the message, ahead of the call site's own attrs.
A nil `AttrsFunc` leaves the record untouched.

May be used without the rest of the package:

```go
slog.New(logging.NewContextHandler(myHandler, requestid.LogAttrs))
```

## Tests

```sh
go test ./...
go test -race ./...         
```
