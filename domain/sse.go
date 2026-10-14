package domain

// SSE is the Server-Sent Events interface.
type SSE interface {
	// Send sends an SSE event with a name and JSON data.
	Send(event string, data any) error
	// SendRaw sends an SSE event with raw data.
	SendRaw(event, data string) error
	// Retry sets the client reconnection time in milliseconds.
	Retry(ms int) error
	// Close closes the SSE connection.
	Close() error
}

// SSEHandler is the handler function for SSE.
type SSEHandler func(sse SSE) error
