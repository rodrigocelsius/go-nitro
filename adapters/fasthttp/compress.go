package fasthttp

import (
	"compress/gzip"
	"sync"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

var (
	acceptEncHeader = []byte("Accept-Encoding")
	gzipValue       = []byte("gzip")
	varyHeader      = []byte("Vary")
)

type gzipWriter struct {
	writer *gzip.Writer
	buf    []byte
}

var gzipPool = &sync.Pool{
	New: func() any {
		return &gzipWriter{}
	},
}

// CompressConfig configures the compression middleware behavior.
type CompressConfig struct {
	// Level gzip compression level. Default: gzip.DefaultCompression.
	Level int
}

// CompressMiddleware returns middleware that compresses HTTP responses with gzip
// when the client supports it. Must be registered via UsePost to run after the handler.
func CompressMiddleware(cfg ...CompressConfig) domain.Handler {
	level := gzip.DefaultCompression
	if len(cfg) > 0 {
		level = cfg[0].Level
	}
	if level < gzip.DefaultCompression || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}

	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		fctx := fc.FasthttpCtx()

		// skip if client does not accept gzip
		if !hasGzip(fctx.Request.Header.PeekBytes(acceptEncHeader)) {
			return nil
		}

		// skip if already compressed
		if len(fctx.Response.Header.ContentEncoding()) > 0 {
			return nil
		}

		body := fctx.Response.Body()
		if len(body) < 1024 {
			return nil
		}

		fctx.Response.Header.SetCanonical(varyHeader, acceptEncHeader)

		gw := gzipPool.Get().(*gzipWriter)
		var buf []byte
		if cap(gw.buf) < len(body) {
			buf = make([]byte, 0, len(body))
		} else {
			buf = gw.buf[:0]
		}

		w, _ := gzip.NewWriterLevel(&writeBuf{buf: &buf}, level)
		w.Write(body)
		w.Close()

		gw.buf = buf
		gzipPool.Put(gw)

		fctx.Response.Header.SetContentEncoding("gzip")
		fctx.Response.SetBody(buf)
		return nil
	}
}

type writeBuf struct {
	buf *[]byte
}

func (w *writeBuf) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

func hasGzip(hdr []byte) bool {
	for i := 0; i+3 < len(hdr); i++ {
		if hdr[i] == 'g' && hdr[i+1] == 'z' && hdr[i+2] == 'i' && hdr[i+3] == 'p' {
			if i+4 >= len(hdr) || hdr[i+4] == ',' || hdr[i+4] == ' ' || hdr[i+4] == ';' {
				return true
			}
		}
	}
	return false
}
