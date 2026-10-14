package runtime

import (
	"sync"
)

var jsonBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 512)
		return &buf
	},
}

// GetBuffer returns a buffer from the pool.
// The buffer may hold leftover data — call buf[:0] before use.
func GetBuffer() *[]byte {
	return jsonBufferPool.Get().(*[]byte)
}

// PutBuffer returns a buffer to the pool for reuse.
// The buffer is reset automatically.
func PutBuffer(buf *[]byte) {
	if buf == nil {
		return
	}
	*buf = (*buf)[:0]
	jsonBufferPool.Put(buf)
}

// MarshalWithPool serializes v as JSON using a pooled buffer.
// The returned []byte is a safe COPY — it may be used after the
// buffer is returned to the pool.
func MarshalWithPool(v interface{ AppendJSON([]byte) []byte }) []byte {
	buf := GetBuffer()
	defer PutBuffer(buf)

	*buf = (*buf)[:0]
	*buf = v.AppendJSON(*buf)

	// Copy before returning to the pool
	result := make([]byte, len(*buf))
	copy(result, *buf)
	return result
}
