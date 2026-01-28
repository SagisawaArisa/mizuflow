package service

import (
	"mizuflow/internal/metrics"
	v1 "mizuflow/pkg/api/v1"
	"mizuflow/pkg/logger"
	"time"
)

type Client struct {
	Send       chan v1.Message
	Namespaces map[string]bool
	Env        string
}

type Hub struct {
	clients    map[*Client]bool
	Broadcast  chan v1.Message
	Register   chan *Client
	Unregister chan *Client

	observer metrics.HubObserver
	heartbeatInterval time.Duration
}

func NewHub(obs metrics.HubObserver, heartbeatInterval time.Duration) *Hub {
	return &Hub{
		clients:           make(map[*Client]bool),
		Broadcast:         make(chan v1.Message),
		Register:          make(chan *Client, 512),
		Unregister:        make(chan *Client, 512),
		observer:          obs,
		heartbeatInterval: heartbeatInterval,
	}
}

func (h *Hub) Run() {
	heartbeatTicker := time.NewTicker(h.heartbeatInterval)
	defer heartbeatTicker.Stop()
	shards := make(map[string]map[string]map[*Client]struct{})
	wildcards := make(map[*Client]struct{})
	addToShards := func(client *Client) {
		if client.Namespaces["*"] {
			wildcards[client] = struct{}{}
			return
		}
		envShards, ok := shards[client.Env]
		if !ok {
			envShards = make(map[string]map[*Client]struct{})
			shards[client.Env] = envShards
		}
		for namespace := range client.Namespaces {
			clients, ok := envShards[namespace]
			if !ok {
				clients = make(map[*Client]struct{})
				envShards[namespace] = clients
			}
			clients[client] = struct{}{}
		}
	}
	removeFromShards := func(client *Client) {
		if _, ok := wildcards[client]; ok {
			delete(wildcards, client)
			return
		}
		envShards, ok := shards[client.Env]
		if !ok {
			return
		}
		for namespace := range client.Namespaces {
			clients, ok := envShards[namespace]
			if !ok {
				continue
			}
			delete(clients, client)
			if len(clients) == 0 {
				delete(envShards, namespace)
			}
		}
		if len(envShards) == 0 {
			delete(shards, client.Env)
		}
	}
	removeClient := func(client *Client) {
		if _, ok := h.clients[client]; !ok {
			return
		}
		delete(h.clients, client)
		removeFromShards(client)
		close(client.Send)
		h.observer.DecOnline()
	}
	sendMessage := func(client *Client, message v1.Message) {
		select {
		case client.Send <- message:
		default:
			logger.Warn("client disconnected")
			removeClient(client)
		}
	}
	for {
		select {
		case client := <-h.Register:
			h.clients[client] = true
			addToShards(client)
			h.observer.IncOnline()
		case client := <-h.Unregister:
			if _, ok := h.clients[client]; ok {
				removeClient(client)
			}
		case message := <-h.Broadcast:
			for client := range wildcards {
				sendMessage(client, message)
			}
			if envShards, ok := shards[message.Env]; ok {
				if clients, ok := envShards[message.Namespace]; ok {
					for client := range clients {
						sendMessage(client, message)
					}
				}
			}
		case <-heartbeatTicker.C:
			heartbeat := v1.Message{
				Type: "ping",
			}
			for client := range h.clients {
				sendMessage(client, heartbeat)
			}
		}
	}
}
