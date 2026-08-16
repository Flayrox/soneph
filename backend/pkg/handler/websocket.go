package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	// writeWait bounds how long a single socket write may take. A client
	// that can't keep up (slow network, dead tab) is dropped instead of
	// blocking the whole hub — and therefore the download pipeline.
	writeWait = 10 * time.Second

	// pongWait / pingPeriod keep the connection alive and detect dead peers.
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second

	// sendBuffer is the per-client outbound queue. A slow client fills it
	// and gets disconnected — it never blocks other clients.
	sendBuffer = 32
)

type WSMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

type WSClient struct {
	conn *websocket.Conn
	send chan []byte
}

type WSHub struct {
	mu         sync.Mutex
	clients    map[*WSClient]bool
	upgrader   websocket.Upgrader
	broadcast  chan WSMessage
	register   chan *WSClient
	unregister chan *WSClient
}

func NewWSHub() *WSHub {
	h := &WSHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan WSMessage, 100),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for dev / containerized use
			},
		},
	}
	go h.run()
	return h
}

// Broadcast publishes an event to every connected client. It never blocks:
// if the hub is saturated, the message is dropped rather than stalling the
// downloader that called us.
func (h *WSHub) Broadcast(event string, data interface{}) {
	select {
	case h.broadcast <- WSMessage{Event: event, Data: data}:
	default:
		slog.Warn("ws: broadcast queue full, dropping event", "event", event)
	}
}

func (h *WSHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			go h.writePump(client)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			payload, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- payload:
				default:
					// Client's queue is full → it's too slow. Drop it so a
					// single laggard never blocks the rest of the hub.
					slog.Warn("ws: disconnecting slow client", "remote", client.conn.RemoteAddr())
					delete(h.clients, client)
					close(client.send)
				}
			}
			h.mu.Unlock()
		}
	}
}

// writePump drains a client's outbound queue with a write deadline. On any
// error it unregisters the client (which closes the socket and unblocks the
// read pump in HandleWS).
func (h *WSHub) writePump(client *WSClient) {
	defer func() {
		h.unregister <- client
		client.conn.Close()
	}()

	for payload := range client.send {
		_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := client.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
}

func (h *WSHub) HandleWS(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("ws: upgrade failed", "err", err)
		return
	}

	client := &WSClient{conn: conn, send: make(chan []byte, sendBuffer)}
	h.register <- client

	// Read pump: keeps the connection alive and detects disconnects so the
	// client is unregistered (and its queue drained).
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	defer func() {
		h.unregister <- client
		conn.Close()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
