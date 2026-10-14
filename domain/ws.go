package domain

// WebSocket is the interface for WebSocket connections.
type WebSocket interface {
	ReadMessage() ([]byte, error)
	WriteMessage(data []byte) error
	Close() error
}

// WSHandler is the handler function for WebSocket.
type WSHandler func(ws WebSocket) error

// WSHub manages WebSocket connections with broadcasting and rooms.
type WSHub interface {
	// Broadcast sends a message to all connections.
	Broadcast(data []byte)
	// BroadcastTo sends a message to all connections in a room.
	BroadcastTo(room string, data []byte)
	// Join adds a connection to a room.
	Join(room string, ws WebSocket)
	// Leave removes a connection from a room.
	Leave(room string, ws WebSocket)
	// CloseAll closes all managed connections.
	CloseAll()
}
