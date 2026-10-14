package fasthttp

import (
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// defaultAccessLogger is the default logger used when none was set on Ctx.
var defaultAccessLogger domain.Logger = &byteLogger{}

// byteLogger writes access lines to stderr without per-call heap from log.Logger.
type byteLogger struct{}

func (byteLogger) Debug(msg string, _ ...any)  { writeLogLine(msg) }
func (byteLogger) Info(msg string, _ ...any)   { writeLogLine(msg) }
func (byteLogger) Warn(msg string, _ ...any)   { writeLogLine(msg) }
func (byteLogger) Error(msg string, _ ...any)  { writeLogLine(msg) }
func (byteLogger) With(_ ...any) domain.Logger { return byteLogger{} }

func writeLogLine(msg string) {
	bufp := logLinePool.Get().(*[]byte)
	buf := (*bufp)[:0]
	buf = append(buf, msg...)
	buf = append(buf, '\n')
	_, _ = os.Stderr.Write(buf)
	*bufp = buf[:0]
	logLinePool.Put(bufp)
}

var logLinePool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 160)
		return &b
	},
}

var accessLogBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 256)
		return &b
	},
}

// LoggerConfig configures the access-log middleware.
type LoggerConfig struct {
	// Format: "text" (Fiber-style, colored), "plain" (method path status ms), "json".
	Format string
	// SkipRoutes lists routes that must not be logged.
	// Defaults() uses ["/health","/ready","/live","/metrics"] when empty.
	SkipRoutes []string
	// Async buffers lines on a channel (non-blocking drop when full).
	Async bool
	// Colors: "auto" (TTY), "always", "never". Default "auto".
	Colors string
	// TimeFormat for Format=text. Default 2006/01/02 15:04:05 (rendered at one-second resolution).
	TimeFormat string
}

// DefaultSkipRoutes are skipped by access log when SkipRoutes is nil in Defaults.
var DefaultSkipRoutes = []string{"/health", "/ready", "/live", "/metrics"}

// ANSI
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiBlue    = "\033[34m"
	ansiMagenta = "\033[35m"
	ansiCyan    = "\033[36m"
	ansiWhite   = "\033[37m"
	ansiGray    = "\033[90m"
)

// LoggerMiddleware returns a pre+post middleware pair that logs
// each request with method, path, status, and duration.
func LoggerMiddleware(cfg ...LoggerConfig) (domain.Handler, domain.Handler) {
	return LoggerPre(cfg...), LoggerPost(cfg...)
}

// LoggerPre returns a pre-middleware that stores the start time.
func LoggerPre(cfg ...LoggerConfig) domain.Handler {
	return func(ctx *domain.Ctx) error {
		fc, ok := ctx.AdapterData.(*FastCtx)
		if ok && fc != nil {
			fc.SetStartTime(time.Now())
		}
		return nil
	}
}

// LoggerPost returns a post-middleware that logs the request.
func LoggerPost(cfg ...LoggerConfig) domain.Handler {
	var c LoggerConfig
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if c.Format == "" {
		c.Format = "text"
	}
	if c.TimeFormat == "" {
		c.TimeFormat = "2006/01/02 15:04:05"
	}
	if c.Colors == "" {
		c.Colors = "auto"
	}
	useColor := shouldColor(c.Colors)

	skip := make(map[string]struct{}, len(c.SkipRoutes))
	for _, r := range c.SkipRoutes {
		skip[r] = struct{}{}
	}

	var asyncCh chan []byte
	if c.Async {
		asyncCh = make(chan []byte, 4096)
		go asyncLogWriter(asyncCh)
	}

	return func(ctx *domain.Ctx) error {
		fc, ok := ctx.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}

		fctx := fc.FasthttpCtx()
		path := b2s(fctx.Path())
		if _, ok := skip[path]; ok {
			return nil
		}

		status := fctx.Response.StatusCode()
		method := b2s(fctx.Method())
		elapsed := time.Since(fc.StartTime())
		ip := fc.ClientIP()
		if ip == "" {
			ip = fc.IP()
		}
		rid := ctx.RequestID()
		if rid == "" {
			rid = fc.RequestID()
		}
		bytesOut := len(fctx.Response.Body())

		bufp := accessLogBufPool.Get().(*[]byte)
		buf := (*bufp)[:0]
		switch c.Format {
		case "json":
			buf = appendJSONAccess(buf, method, path, status, elapsed, ip, rid, bytesOut)
		case "plain":
			buf = appendPlainAccess(buf, method, path, status, elapsed)
		default: // text — Fiber-style: [DEBUG] - date - status - METHOD - lat - path
			buf = appendFiberAccess(buf, useColor, c.TimeFormat, method, path, status, elapsed)
		}
		buf = append(buf, '\n')

		if asyncCh != nil {
			line := append([]byte(nil), buf...)
			*bufp = buf[:0]
			accessLogBufPool.Put(bufp)
			select {
			case asyncCh <- line:
			default:
				asyncLogDropped.Add(1)
			}
			return nil
		}

		if l := fc.Ctx.LoggerOrNil(); l != nil && c.Format == "json" {
			msg := string(buf[:len(buf)-1])
			*bufp = buf[:0]
			accessLogBufPool.Put(bufp)
			l.Info(msg)
			return nil
		}

		_, _ = os.Stderr.Write(buf)
		*bufp = buf[:0]
		accessLogBufPool.Put(bufp)
		return nil
	}
}

// asyncLogDropped counts access-log lines discarded because the async queue was full.
var asyncLogDropped atomic.Uint64

// LoggerDropped returns how many access-log lines the async logger discarded
// under back-pressure since process start (always 0 with Async=false).
func LoggerDropped() uint64 { return asyncLogDropped.Load() }

// asyncLogWriter coalesces queued lines into one write syscall (up to 64 KiB),
// which is what makes async logging cheaper than one write per request.
func asyncLogWriter(ch <-chan []byte) {
	const maxBatch = 64 << 10
	batch := make([]byte, 0, maxBatch+1024)
	for line := range ch {
		batch = append(batch[:0], line...)
	drain:
		for len(batch) < maxBatch {
			select {
			case more, ok := <-ch:
				if !ok {
					break drain
				}
				batch = append(batch, more...)
			default:
				break drain
			}
		}
		_, _ = os.Stderr.Write(batch)
	}
}

func shouldColor(mode string) bool {
	switch mode {
	case "always", "true", "1", "on":
		return true
	case "never", "false", "0", "off":
		return false
	default: // auto
		fi, err := os.Stderr.Stat()
		if err != nil {
			return false
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
}

func appendPlainAccess(buf []byte, method, path string, status int, elapsed time.Duration) []byte {
	buf = append(buf, method...)
	buf = append(buf, ' ')
	buf = append(buf, path...)
	buf = append(buf, ' ')
	buf = strconv.AppendInt(buf, int64(status), 10)
	buf = append(buf, ' ')
	buf = strconv.AppendInt(buf, elapsed.Milliseconds(), 10)
	buf = append(buf, "ms"...)
	return buf
}

func appendJSONAccess(buf []byte, method, path string, status int, elapsed time.Duration, ip, rid string, bytesOut int) []byte {
	buf = append(buf, `{"time":"`...)
	buf = time.Now().AppendFormat(buf, time.RFC3339)
	buf = append(buf, `","method":"`...)
	buf = appendJSONString(buf, method)
	buf = append(buf, `","path":"`...)
	// path/ip come from the request: escape them so a crafted URL cannot
	// forge log entries (log injection).
	buf = appendJSONString(buf, path)
	buf = append(buf, `","status":`...)
	buf = strconv.AppendInt(buf, int64(status), 10)
	buf = append(buf, `,"latency":"`...)
	buf = append(buf, formatLatency(elapsed)...)
	buf = append(buf, `","latency_us":`...)
	buf = strconv.AppendInt(buf, elapsed.Microseconds(), 10)
	buf = append(buf, `,"ip":"`...)
	buf = appendJSONString(buf, ip)
	if rid != "" {
		buf = append(buf, `","request_id":"`...)
		buf = appendJSONString(buf, rid)
	}
	buf = append(buf, `","bytes":`...)
	buf = strconv.AppendInt(buf, int64(bytesOut), 10)
	buf = append(buf, '}')
	return buf
}

// appendJSONString appends s as a JSON string literal, escaping quotes,
// backslashes and control characters.
func appendJSONString(buf []byte, s string) []byte {
	const hex = "0123456789abcdef"
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			buf = append(buf, '\\', c)
		case c < 0x20:
			switch c {
			case '\n':
				buf = append(buf, '\\', 'n')
			case '\r':
				buf = append(buf, '\\', 'r')
			case '\t':
				buf = append(buf, '\\', 't')
			default:
				buf = append(buf, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xf])
			}
		default:
			buf = append(buf, c)
		}
	}
	return buf
}

// appendFiberAccess: [DEBUG] - 2020/12/04 23:29:24 - 200 - GET - 1.886us - /
func appendFiberAccess(buf []byte, color bool, timeFmt, method, path string, status int, elapsed time.Duration) []byte {
	now := cachedLogTime(timeFmt)
	lat := formatLatency(elapsed)
	statusColor := statusANSI(status)
	methodColor := methodANSI(method)

	// [DEBUG]
	if color {
		buf = append(buf, ansiWhite...)
	}
	buf = append(buf, "[DEBUG]"...)
	if color {
		buf = append(buf, ansiReset...)
	}
	buf = append(buf, " - "...)

	// date time
	if color {
		buf = append(buf, ansiGray...)
	}
	buf = append(buf, now...)
	if color {
		buf = append(buf, ansiReset...)
	}
	buf = append(buf, " - "...)

	// status
	if color {
		buf = append(buf, statusColor...)
		buf = append(buf, ansiBold...)
	}
	buf = strconv.AppendInt(buf, int64(status), 10)
	if color {
		buf = append(buf, ansiReset...)
	}
	buf = append(buf, " - "...)

	// method
	if color {
		buf = append(buf, methodColor...)
		buf = append(buf, ansiBold...)
	}
	buf = append(buf, method...)
	if color {
		buf = append(buf, ansiReset...)
	}
	buf = append(buf, " - "...)

	// latency
	if color {
		buf = append(buf, ansiGray...)
	}
	buf = append(buf, lat...)
	if color {
		buf = append(buf, ansiReset...)
	}
	buf = append(buf, " - "...)

	// path
	if color {
		buf = append(buf, ansiWhite...)
	}
	buf = append(buf, path...)
	if color {
		buf = append(buf, ansiReset...)
	}
	return buf
}

func statusANSI(status int) string {
	switch {
	case status >= 200 && status < 300:
		return ansiGreen
	case status >= 300 && status < 400:
		return ansiCyan
	case status >= 400 && status < 500:
		return ansiYellow
	default:
		return ansiRed
	}
}

// methodANSI is Fiber-style colors (GET blue, POST cyan, DELETE red…).
func methodANSI(method string) string {
	switch method {
	case "GET":
		return ansiBlue
	case "POST":
		return ansiCyan
	case "PUT":
		return ansiYellow
	case "PATCH":
		return ansiGreen
	case "DELETE":
		return ansiRed
	case "HEAD":
		return ansiMagenta
	case "OPTIONS":
		return ansiWhite
	default:
		return ansiWhite
	}
}

func formatLatency(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Microsecond:
		return strconv.FormatInt(d.Nanoseconds(), 10) + "ns"
	case d < time.Millisecond:
		us := float64(d.Nanoseconds()) / 1000.0
		return strconv.FormatFloat(us, 'f', 3, 64) + "us"
	case d < time.Second:
		ms := float64(d.Microseconds()) / 1000.0
		return strconv.FormatFloat(ms, 'f', 3, 64) + "ms"
	default:
		return strconv.FormatFloat(d.Seconds(), 'f', 3, 64) + "s"
	}
}

// Logger returns an access-log middleware pair (pre, post).
func Logger(cfg ...LoggerConfig) (domain.Handler, domain.Handler) {
	return LoggerMiddleware(cfg...)
}

type logTimeEntry struct {
	sec  int64
	fmt  string
	text string
}

var logTimeCache atomic.Pointer[logTimeEntry]

// cachedLogTime formats the wall clock at one-second granularity and reuses
// the string for every access-log line within the same second (the default
// layout has no sub-second part), avoiding a time.Format per request.
func cachedLogTime(layout string) string {
	now := time.Now()
	sec := now.Unix()
	if e := logTimeCache.Load(); e != nil && e.sec == sec && e.fmt == layout {
		return e.text
	}
	text := now.Format(layout)
	logTimeCache.Store(&logTimeEntry{sec: sec, fmt: layout, text: text})
	return text
}
