package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHandlerWritesJsonToLogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	logger := newLoggerForTest(t, Config{Level: slog.LevelWarn, Format: FormatJSON, File: path}, nil)

	// when
	logger.Info("filtered out by the warn level")
	logger.Warn("written", "board_id", "main")

	// then
	entry := decodeLogLine(t, readFile(t, path)) // single line: the info one is below the level
	require.Equal(t, "WARN", entry["level"])
	require.Equal(t, "written", entry["msg"])
	require.Equal(t, "main", entry["board_id"])
}

func TestNewHandlerAppendsToExistingLogFileNotTruncate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	require.NoError(t, os.WriteFile(path, []byte("previous run\n"), 0o644))
	logger := newLoggerForTest(t, Config{Format: FormatJSON, File: path}, nil)

	// when
	logger.Info("second run")

	// then
	lines := splitNonEmptyLines(readFile(t, path))
	require.Len(t, lines, 2) // the previous run, then ours
	require.Equal(t, "previous run", lines[0])
	require.Equal(t, "second run", decodeLogLine(t, lines[1])["msg"])
}

func TestNewHandlerRejectsBadConfigBeforeCreatingLogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")

	// when
	h, closer, err := NewHandler(Config{TimeFormat: "hh:mm:ss", File: path}, nil)

	// then
	require.Error(t, err)
	require.Nil(t, h)
	require.Nil(t, closer)
	require.NoFileExists(t, path)
}

func TestNewHandlerWritesToStdoutWhenNoFileIsSet(t *testing.T) {
	_, closer, err := NewHandler(Config{}, nil)
	require.NoError(t, err)
	require.NoError(t, closer.Close()) // a no-op, must not close stdout
	require.NoError(t, closer.Close()) // and must stay safe to repeat
}

func TestNewHandlerAddsCtxAttrsFromAttrsFunc(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	logger := newLoggerForTest(t, Config{Format: FormatJSON, File: path}, testAttrs)

	// when
	logger.InfoContext(withTestID(context.Background(), "req-42"), "written")

	// then
	require.Equal(t, "req-42", decodeLogLine(t, readFile(t, path))[testIDKey])
}

func TestSetupInstallsTheDefaultLoggerAndEchoesTheConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	setupForTest(t, Config{Format: FormatJSON, File: path}, testAttrs)

	// when
	slog.InfoContext(withTestID(context.Background(), "req-42"), "written")

	// then
	lines := splitNonEmptyLines(readFile(t, path))
	require.Len(t, lines, 2) // the Setup config line, then ours

	configured := decodeLogLine(t, lines[0])
	require.Equal(t, "logging configured", configured["msg"])
	require.Equal(t, path, configured["cfg_output"])
	require.Equal(t, string(TimeDateMilli), configured["cfg_time_format"]) // resolved, not the caller's blank

	written := decodeLogLine(t, lines[1])
	require.Equal(t, "written", written["msg"])
	require.Equal(t, "req-42", written[testIDKey])
}

func TestContextHandlerAddsAttrsFromContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewContextHandler(slog.NewJSONHandler(&buf, nil), testAttrs))

	ctx := withTestID(context.Background(), "req-42")

	// when
	logger.InfoContext(ctx, "written", "board_id", "main")

	// then
	entry := decodeLogLine(t, buf.String())
	require.Equal(t, "req-42", entry[testIDKey])
	require.Equal(t, "main", entry["board_id"])
}

func TestContextHandlerOmitsAttrsWhenContextHasNone(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewContextHandler(slog.NewJSONHandler(&buf, nil), testAttrs))

	logger.InfoContext(context.Background(), "written")

	require.NotContains(t, decodeLogLine(t, buf.String()), testIDKey)
}

// slog.With uses WithAttrs which returns an inner handler if not wrapped correctly
func TestContextHandlerKeepsCtxAttrsAfterWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewContextHandler(slog.NewJSONHandler(&buf, nil), testAttrs)).With("component", "storage")

	ctx := withTestID(context.Background(), "req-42")

	// when
	logger.InfoContext(ctx, "written")

	// then
	entry := decodeLogLine(t, buf.String())
	require.Equal(t, "req-42", entry[testIDKey])
	require.Equal(t, "storage", entry["component"])
}

func TestContextHandlerKeepsCtxAttrsAfterWithGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewContextHandler(slog.NewJSONHandler(&buf, nil), testAttrs)).WithGroup("http")

	logger.InfoContext(withTestID(context.Background(), "req-42"), "written", "route", "/scores")

	// the ctx attr joins the record, so an open group swallows it along with the rest
	entry := decodeLogLine(t, buf.String())
	group, ok := entry["http"].(map[string]any)
	require.True(t, ok, "entry: %v", entry)
	require.Equal(t, "req-42", group[testIDKey])
	require.Equal(t, "/scores", group["route"])
}

func TestNewContextHandlerWithNilFuncReturnsTheHandlerUnwrapped(t *testing.T) {
	next := slog.NewJSONHandler(&bytes.Buffer{}, nil)
	require.Same(t, next, NewContextHandler(next, nil))
}

func TestNewContextHandlerSkipsRebuildWhenAttrsFuncReturnsNothing(t *testing.T) {
	var buf bytes.Buffer
	empty := func(context.Context) []slog.Attr { return nil }
	logger := slog.New(NewContextHandler(slog.NewJSONHandler(&buf, nil), empty))

	logger.InfoContext(context.Background(), "written", "board_id", "main")

	entry := decodeLogLine(t, buf.String())
	require.Equal(t, "written", entry["msg"])
	require.Equal(t, "main", entry["board_id"])
}

// The extractor the module does not ship: the ctx key belongs to the caller.
// Mirrors the apex's use case.

const testIDKey = "request_id"

type testCtxKey int

const testIDCtxKey testCtxKey = iota

func withTestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, testIDCtxKey, id)
}

func testAttrs(ctx context.Context) []slog.Attr {
	id, _ := ctx.Value(testIDCtxKey).(string)
	if id == "" {
		return nil
	}
	return []slog.Attr{slog.String(testIDKey, id)}
}

// Builds a logger from cfg without touching the default one, and closes the log file at cleanup.
func newLoggerForTest(t *testing.T, cfg Config, fn AttrsFunc) *slog.Logger {
	t.Helper()

	h, closer, err := NewHandler(cfg, fn)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := closer.Close(); err != nil {
			t.Errorf("error while closing logger %v", err)
		}
	})
	return slog.New(h)
}

// 'Setup' must not be called directly: it replaces the default logger process wide.
func setupForTest(t *testing.T, cfg Config, fn AttrsFunc) {
	t.Helper()

	previous := slog.Default()
	closer, err := Setup(cfg, fn)
	require.NoError(t, err)

	t.Cleanup(func() {
		slog.SetDefault(previous)
		// The log file is closed here and not in the test body
		if err := closer.Close(); err != nil {
			t.Errorf("error while closing logger %v", err)
		}
	})
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

// unmarshals a single json log line, failing if the input holds anything else
func decodeLogLine(t *testing.T, raw string) map[string]any {
	// example (happy):
	// 		raw is `{"level":"info","msg":"User logged in","user_id":123}`
	// 		splitNonEmptyLines returns ["{\"level\":\"info\",...}"]
	// 		json.Unmarshal -> map[string]any{"level":"info", "msg":"User logged in", "user_id":123}

	t.Helper()

	lines := splitNonEmptyLines(raw)
	require.Len(t, lines, 1)

	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &entry))
	return entry
}

func splitNonEmptyLines(s string) []string {
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}
