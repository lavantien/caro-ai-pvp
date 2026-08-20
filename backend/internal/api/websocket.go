package api

import (
	"caro-ai-pvp/internal/uci"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || isLocalOrigin(origin)
	},
}

func HandleWebSocket(logger *slog.Logger, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("websocket upgrade failed", "err", err)
		return
	}
	defer conn.Close()
	// Bound incoming frame size; commands are short text lines.
	conn.SetReadLimit(4096)

	var mu sync.Mutex
	writer := &wsWriter{conn: conn, mu: &mu}
	handler := uci.NewUCIHandler(logger, writer)
	// Release the handler's engine (TT memory) when the peer disconnects.
	defer handler.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		handler.HandleCommand(string(msg))
	}
}

type wsWriter struct {
	conn *websocket.Conn
	mu   *sync.Mutex
}

func (w *wsWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	err := w.conn.WriteMessage(websocket.TextMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
