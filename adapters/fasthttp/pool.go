package fasthttp

import (
	"bytes"
	"sync"
)

// bufferPool is a reusable pool of bytes.Buffer to avoid allocations
// on the HTTP response hot path. Each buffer is reset before returning
// to the pool.
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// GetBuffer gets a bytes.Buffer from the pool. The returned buffer may contain
// data from prior use; always call buf.Reset() before use or use
// the helpers below.
func GetBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

// PutBuffer returns a bytes.Buffer to the pool after reset.
func PutBuffer(buf *bytes.Buffer) {
	buf.Reset()
	bufferPool.Put(buf)
}

// BufferWrite writes data into a pooled buffer and returns the bytes.
// The buffer is automatically returned to the pool afterwards.
func BufferWrite(data []byte) []byte {
	buf := GetBuffer()
	defer PutBuffer(buf)
	buf.Write(data)
	// Copy data before returning to the pool
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result
}

// BufferWriteString writes a string into a pooled buffer and returns the bytes.
func BufferWriteString(s string) []byte {
	buf := GetBuffer()
	defer PutBuffer(buf)
	buf.WriteString(s)
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result
}

// BufferConcat concatenates multiple byte slices in a pooled buffer.
func BufferConcat(parts ...[]byte) []byte {
	buf := GetBuffer()
	defer PutBuffer(buf)
	for _, p := range parts {
		buf.Write(p)
	}
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result
}
