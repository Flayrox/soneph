package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WSMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

type WSHub struct {
	mu         sync.Mutex
	clients    map[*websocket.Conn]bool
	upgrader   websocket.Upgrader
	broadcast  chan WSMessage
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

func NewWSHub() *WSHub {
	h := &WSHub{
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan WSMessage, 100),
		register:  make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for dev / containerized use
			},
		},
	}
	go h.run()
	return h
}

func (h *WSHub) Broadcast(event string, data interface{}) {
	h.broadcast <- WSMessage{Event: event, Data: data}
}

func (h *WSHub) run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			payload, err := json.Marshal(msg)
			if err != nil {
				continue
			}

			h.mu.Lock()
			for conn := range h.clients {
				err := conn.WriteMessage(websocket.TextMessage, payload)
				if err != nil {
					log.Printf("WS error, closing client: %v", err)
					conn.Close()
					delete(h.clients, conn)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *WSHub) HandleWS(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	h.register <- conn

	defer func() {
		h.unregister <- conn
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}
