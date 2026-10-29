package fasthttp

import (
	"bufio"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// The first event must reach the client while the handler is still running. Before
// the stream writer, the response was buffered and every event arrived together
// when the handler returned, which made SSE useless for progress or token streams.
func TestSSEHandler_StreamsBeforeHandlerReturns(t *testing.T) {
	release := make(chan struct{})
	app := New()
	app.Get("/events", SSEHandler(func(sse domain.SSE) error {
		if err := sse.Send("tick", 1); err != nil {
			return err
		}
		<-release // hold the handler open until the client has seen the first event
		return sse.SendRaw("tick", "two\nlines")
	}))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	srv := &fasthttp.Server{Handler: app.Handler()}
	go func() { _ = srv.Serve(ln) }()
	var released bool
	unblock := func() {
		if !released {
			released = true
			close(release)
		}
	}
	// Runs before Shutdown (defers are LIFO): a failing assertion must still free
	// the handler, or Shutdown would wait on it forever.
	defer func() { _ = srv.Shutdown() }()
	defer unblock()

	// A buffered implementation never even sends the headers while the handler is
	// blocked, so the client needs its own deadline to fail instead of hanging.
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + ln.Addr().String() + "/events")
	if err != nil {
		t.Fatalf("no response while the handler is running (events buffered, not streamed): %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}

	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()

	read := func(want string) {
		t.Helper()
		select {
		case got := <-lines:
			if got != want {
				t.Fatalf("line = %q, want %q", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %q: the event was buffered, not streamed", want)
		}
	}

	read("event: tick")
	read("data: 1")
	read("")
	unblock()
	read("event: tick")
	read("data: two")
	read("data: lines")
}

func TestFastSSE_SendRawMultiline(t *testing.T) {
	var buf strings.Builder
	sse := &FastSSE{w: &buf}
	if err := sse.SendRaw("msg", "a\r\nb\nc"); err != nil {
		t.Fatal(err)
	}
	if want := "event: msg\ndata: a\ndata: b\ndata: c\n\n"; buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}
