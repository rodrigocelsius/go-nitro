package fasthttp

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// TestFileHeader_SafeFilename covers V-19: a client-supplied Filename is
// untrusted and must never be used verbatim to build a destination path —
// SafeFilename() strips directory components/traversal sequences down to
// a bare file name.
func TestFileHeader_SafeFilename(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain name", "photo.jpg", "photo.jpg"},
		{"path traversal", "../../etc/passwd", "passwd"},
		{"absolute path", "/etc/passwd", "passwd"},
		{"windows-style traversal", `..\..\evil.exe`, `\..\evil.exe`}, // backslash is not a path separator on unix; treated as one filename with leading dots stripped
		{"leading dots", "...secret", "secret"},
		{"empty", "", "upload"},
		{"only slashes", "///", "upload"},
	}
	for _, tt := range tests {
		fh := &FileHeader{Filename: tt.in}
		if got := fh.SafeFilename(); got != tt.want {
			t.Errorf("SafeFilename(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestDetectFileType covers V-18: content type must be sniffed from magic
// bytes, not trusted from the client-supplied filename/Content-Type.
func TestDetectFileType(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	if got := DetectFileType(png); got != "image/png" {
		t.Errorf("DetectFileType(png bytes) = %q, want image/png", got)
	}

	// A file named "photo.jpg" whose actual bytes are an HTML payload
	// must be detected as HTML, not trusted as an image just because of
	// its extension/Content-Type.
	html := []byte("<!DOCTYPE html><script>alert(1)</script>")
	got := DetectFileType(html)
	if got != "text/html; charset=utf-8" {
		t.Errorf("DetectFileType(html bytes) = %q, want text/html", got)
	}
}

func buildMultipartRequest(t *testing.T, field, filename string, content []byte) *fasthttp.RequestCtx {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/upload")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentType(w.FormDataContentType())
	ctx.Request.SetBody(b.Bytes())
	return ctx
}

// TestFormFile_MaxSize covers V-35: FormFile/FormFiles had no way to cap
// an individual file's size — only the overall request body limit
// applied — so io.ReadAll would buffer an entire large field in memory
// per upload. An explicit maxSize must reject oversized files with
// ErrFileTooLarge instead of reading them fully.
func TestFormFile_MaxSize(t *testing.T) {
	var gotErr error
	app := New()
	app.Post("/upload", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		_, gotErr = fc.FormFile("file", 5)
		return nil
	})

	ctx := buildMultipartRequest(t, "file", "big.txt", []byte("this content is way over 5 bytes"))
	app.Handler()(ctx)

	if !errors.Is(gotErr, ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", gotErr)
	}
}

// TestFormFile_MaxSize_WithinLimit ensures the maxSize cap does not
// reject files at or under the limit.
func TestFormFile_MaxSize_WithinLimit(t *testing.T) {
	var gotFh *FileHeader
	var gotErr error
	app := New()
	app.Post("/upload", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		gotFh, gotErr = fc.FormFile("file", 32)
		return nil
	})

	ctx := buildMultipartRequest(t, "file", "small.txt", []byte("small content"))
	app.Handler()(ctx)

	if gotErr != nil {
		t.Fatalf("unexpected error: %v", gotErr)
	}
	if gotFh == nil {
		t.Fatal("expected a file header")
	}
	data, _ := gotFh.ReadAll()
	if string(data) != "small content" {
		t.Fatalf("content = %q", data)
	}
}

// TestFormFiles_MaxSize applies the same per-file cap to FormFiles
// (multiple files under the same field key).
func TestFormFiles_MaxSize(t *testing.T) {
	var gotErr error
	app := New()
	app.Post("/upload", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		_, gotErr = fc.FormFiles("files", 3)
		return nil
	})

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw1, _ := w.CreateFormFile("files", "a.txt")
	io.WriteString(fw1, "ok")
	fw2, _ := w.CreateFormFile("files", "b.txt")
	io.WriteString(fw2, "too long")
	w.Close()

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/upload")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentType(w.FormDataContentType())
	ctx.Request.SetBody(b.Bytes())
	app.Handler()(ctx)

	if !errors.Is(gotErr, ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", gotErr)
	}
}

// TestFormFile_NoMaxSize preserves backward compatibility: omitting
// maxSize keeps reading the whole file regardless of size.
func TestFormFile_NoMaxSize(t *testing.T) {
	var gotErr error
	app := New()
	app.Post("/upload", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		_, gotErr = fc.FormFile("file")
		return nil
	})

	ctx := buildMultipartRequest(t, "file", "big.txt", []byte("this content is way over 5 bytes"))
	app.Handler()(ctx)

	if gotErr != nil {
		t.Fatalf("unexpected error without maxSize: %v", gotErr)
	}
}
