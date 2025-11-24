// connection.go - WebSocket connection management
package server

import (
	"encoding/json"
	"log"
	"net/http"

	"card-game/game"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Connection represents a WebSocket connection to a client
type Connection struct {
	ws        *websocket.Conn
	PlayerUID string
	GameID    string
}

// ServeWs handles WebSocket upgrade requests
func ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	c := &Connection{ws: conn}
	GameHub.Register(c)
	go c.readLoop()
}

// readLoop reads messages from the WebSocket and routes them to handlers
func (c *Connection) readLoop() {
	defer func() {
		GameHub.Unregister(c)
		c.ws.Close()
	}()

	for {
		_, msgBytes, err := c.ws.ReadMessage()
		if err != nil {
			return
		}

		var action game.Action
		if err := json.Unmarshal(msgBytes, &action); err != nil {
			log.Println("bad action:", err)
			continue
		}

		switch action.Type {
		case "get_cards":
			c.handleGetCards(action)
		case "get_decks":
			c.handleGetDecks(action)
		case "start_game":
			c.handleStartGame(action)
		case "start_ai_game":
			c.handleStartAIGame(action)
		case "join_game":
			c.handleJoinGame(action)
		case "list_games":
			c.handleListGames(action)
		case "join_specific_game":
			c.handleJoinSpecificGame(action)
		case "leave_game":
			c.handleLeaveGame(action)
		case "reconnect_game":
			c.handleReconnectGame(action)
		case "chat":
			c.handleChat(action)
		default:
			c.handleGameAction(action)
		}
	}
}
