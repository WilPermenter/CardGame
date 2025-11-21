package server

import (
    "encoding/json"
    "log"
    "net/http"

    "github.com/gorilla/websocket"
    "card-game/game"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

func ServeWs(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println("upgrade error:", err)
        return
    }

    c := &Connection{ws: conn}
    go c.readLoop()
}

type Connection struct {
    ws *websocket.Conn
}

func (c *Connection) readLoop() {
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

        events := game.CurrentGame.HandleAction(action)

        // send events back
        resp, _ := json.Marshal(events)
        c.ws.WriteMessage(websocket.TextMessage, resp)
    }
}
