package fasthttp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFormatLatency(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Nanosecond, "500ns"},
		{1500 * time.Nanosecond, "1.500us"},
		{2 * time.Millisecond, "2.000ms"},
		{1500 * time.Millisecond, "1.500s"},
	}
	for _, tc := range cases {
		got := formatLatency(tc.d)
		if got != tc.want {
			t.Fatalf("%v: got %q want %q", tc.d, got, tc.want)
		}
	}
}

func TestAppendFiberAccess(t *testing.T) {
	buf := appendFiberAccess(nil, false, "2006/01/02 15:04:05", "GET", "/docs", 200, 1500*time.Microsecond)
	line := string(buf)
	for _, part := range []string{"[INFO]", " - ", "200", "GET", "/docs", "1.500ms"} {
		if !strings.Contains(line, part) {
			t.Fatalf("missing %q in %q", part, line)
		}
	}
	// Ordem Fiber: [LEVEL] - date - status - METHOD - lat - path
	if !strings.Contains(line, "200 - GET - ") || !strings.HasSuffix(line, " - /docs") {
		t.Fatalf("unexpected format: %q", line)
	}
	if strings.Contains(line, "\033[") {
		t.Fatalf("unexpected ansi without color: %q", line)
	}
	colored := appendFiberAccess(nil, true, "2006/01/02 15:04:05", "POST", "/auth/login", 401, time.Millisecond)
	if !strings.Contains(string(colored), ansiYellow) {
		t.Fatalf("expected yellow status for 401: %q", colored)
	}
	if !strings.Contains(string(colored), ansiCyan) {
		t.Fatalf("expected cyan method for POST: %q", colored)
	}
}

func TestAppendJSONAccess_LogInjection(t *testing.T) {
	// Um path malicioso não pode quebrar a estrutura do JSON.
	malicious := `/foo","role":"admin","injected":true,"x":"`
	buf := make([]byte, 0, 256)
	out := string(appendJSONAccess(buf, "GET", malicious, 200, time.Millisecond, "10.0.0.1", "", 0))

	dec := map[string]any{}
	if err := json.Unmarshal([]byte(out), &dec); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if got := dec["path"]; got != malicious {
		t.Fatalf("path roundtrip failed: %q", got)
	}
	if _, hasRole := dec["role"]; hasRole {
		t.Fatalf("log forging succeeded: %s", out)
	}

	// Control chars também escapados.
	ctrl := "/a\x00b\nc"
	buf2 := make([]byte, 0, 64)
	out2 := string(appendJSONString(buf2, ctrl))
	if strings.ContainsAny(out2[:len(out2)-1], "\x00\n") || !strings.Contains(out2, "\\u0000") {
		t.Fatalf("control chars not escaped: %q", out2)
	}
}

// The access-log level follows the status, so a 500 is not tagged [DEBUG].
func TestAppendFiberAccess_LevelByStatus(t *testing.T) {
	for status, tag := range map[int]string{200: "[INFO]", 302: "[INFO]", 404: "[WARN]", 503: "[ERROR]"} {
		line := string(appendFiberAccess(nil, false, "2006/01/02 15:04:05", "GET", "/x", status, time.Millisecond))
		if !strings.HasPrefix(line, tag+" - ") {
			t.Errorf("status %d: want prefix %q, got %q", status, tag, line)
		}
	}
}

func TestAppendJSONAccess_LevelAndMsg(t *testing.T) {
	out := appendJSONAccess(nil, "GET", "/x", 500, time.Millisecond, "10.0.0.1", "", 0)
	dec := map[string]any{}
	if err := json.Unmarshal(out, &dec); err != nil {
		t.Fatal(err)
	}
	if dec["level"] != "ERROR" || dec["msg"] != "http request" {
		t.Fatalf("level/msg = %v/%v in %s", dec["level"], dec["msg"], out)
	}
}

func TestContentDispositionUntrustedName(t *testing.T) {
	cases := map[string]string{
		"report.pdf":           `attachment; filename="report.pdf"`,
		"a\r\nSet-Cookie: x=1": `attachment; filename="aSet-Cookie: x=1"`,
		`evil\" ; x="y`:        `attachment; filename="evil ; x=y"`,
		"../../etc/passwd":     `attachment; filename=".._.._etc_passwd"`,
		"":                     `attachment; filename="download"`,
		"relatório.pdf":        `attachment; filename="relat_rio.pdf"; filename*=UTF-8''relat%C3%B3rio.pdf`,
	}
	for in, want := range cases {
		if got := contentDisposition("attachment", in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}
