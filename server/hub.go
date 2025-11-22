package server

import (
    "card-game/game"
    "encoding/json"
    "sync"

    "github.com/gorilla/websocket"
)

type Hub struct {
    mu          sync.RWMutex
    connections map[*Connection]bool
    // gameID -> list of connections in that game
    gameConns map[string][]*Connection
}

var GameHub = &Hub{
    connections: make(map[*Connection]bool),
    gameConns:   make(map[string][]*Connection),
}

func (h *Hub) Register(c *Connection) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.connections[c] = true
}

func (h *Hub) Unregister(c *Connection) {
    h.mu.Lock()
    defer h.mu.Unlock()
    delete(h.connections, c)

    // Remove from game connections and track disconnect
    if c.GameID != "" {
        // Mark player as disconnected for cleanup tracking
        if g := game.Manager.GetGame(c.GameID); g != nil && c.PlayerUID != "" {
            g.MarkPlayerDisconnected(c.PlayerUID)
        }

        conns := h.gameConns[c.GameID]
        for i, conn := range conns {
            if conn == c {
                h.gameConns[c.GameID] = append(conns[:i], conns[i+1:]...)
                break
            }
        }
    }
}

func (h *Hub) JoinGame(c *Connection, gameID string) {
    h.mu.Lock()
    defer h.mu.Unlock()
    c.GameID = gameID
    h.gameConns[gameID] = append(h.gameConns[gameID], c)
}

// Broadcast sends a message to all players in a game
func (h *Hub) Broadcast(gameID string, msg interface{}) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    data, _ := json.Marshal(msg)
    for _, c := range h.gameConns[gameID] {
        c.ws.WriteMessage(websocket.TextMessage, data)
    }
}

// BroadcastExcept sends a message to all players except one
func (h *Hub) BroadcastExcept(gameID string, exclude *Connection, msg interface{}) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    data, _ := json.Marshal(msg)
    for _, c := range h.gameConns[gameID] {
        if c != exclude {
            c.ws.WriteMessage(websocket.TextMessage, data)
        }
    }
}

// LeaveGame removes a connection from its game
func (h *Hub) LeaveGame(c *Connection) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if c.GameID == "" {
        return
    }

    conns := h.gameConns[c.GameID]
    for i, conn := range conns {
        if conn == c {
            h.gameConns[c.GameID] = append(conns[:i], conns[i+1:]...)
            break
        }
    }
}
