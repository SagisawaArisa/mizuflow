package service

import "delta-conf/pkg/logger"

type Message struct {
	Revision int64  `json:"revision"`
	Key      string `json:"key"`
	Value    string `json:"value"`
}
type Hub struct {
	clients    map[chan Message]bool
	Broadcast  chan Message
	Register   chan chan Message
	Unregister chan chan Message
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[chan Message]bool),
		Broadcast:  make(chan Message),
		Register:   make(chan chan Message),
		Unregister: make(chan chan Message),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.clients[client] = true
		case client := <-h.Unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client)
			}
		case message := <-h.Broadcast:
			for client := range h.clients {
				select {
				case client <- message:
				default:
					logger.Warn("client disconnected")
					close(client)
					delete(h.clients, client)
				}
			}
		}
	}
}
