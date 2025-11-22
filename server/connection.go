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

type Connection struct {
    ws        *websocket.Conn
    PlayerUID string
    GameID    string
}

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

func (c *Connection) handleGetCards(action game.Action) {
    // Send all cards to the client for local lookup
    cards := make(map[int]game.Card)
    for id, card := range game.CardDB {
        cards[id] = card
    }

    events := []game.Event{
        {
            Type: "CardList",
            Data: map[string]interface{}{
                "cards": cards,
            },
        },
    }

    resp, _ := json.Marshal(events)
    c.ws.WriteMessage(websocket.TextMessage, resp)
}

func (c *Connection) handleGetDecks(action game.Action) {
    // Build deck list with leader card info
    decks := make([]map[string]interface{}, 0)
    for _, deck := range game.DeckDB {
        leaderCard := game.CardDB[deck.Leader]
        decks = append(decks, map[string]interface{}{
            "id":         deck.ID,
            "name":       deck.Name,
            "leaderName": leaderCard.Name,
            "leaderId":   deck.Leader,
        })
    }

    events := []game.Event{
        {
            Type: "DeckList",
            Data: map[string]interface{}{
                "decks": decks,
            },
        },
    }

    resp, _ := json.Marshal(events)
    c.ws.WriteMessage(websocket.TextMessage, resp)
}

func (c *Connection) handleStartGame(action game.Action) {
    c.PlayerUID = action.PlayerUID

    g, _, err := game.Manager.CreateGame(action.PlayerUID, action.DeckID)
    if err != nil {
        events := []game.Event{
            {Type: "Error", Data: map[string]interface{}{"message": err.Error()}},
        }
        resp, _ := json.Marshal(events)
        c.ws.WriteMessage(websocket.TextMessage, resp)
        return
    }

    c.GameID = g.ID
    GameHub.JoinGame(c, g.ID)

    events := []game.Event{
        {
            Type: "GameCreated",
            Data: map[string]interface{}{
                "gameId":    g.ID,
                "playerUid": action.PlayerUID,
                "message":   "Waiting for opponent...",
            },
        },
    }

    resp, _ := json.Marshal(events)
    c.ws.WriteMessage(websocket.TextMessage, resp)
}

func (c *Connection) handleJoinGame(action game.Action) {
    c.PlayerUID = action.PlayerUID

    g, _, err := game.Manager.JoinGame(action.PlayerUID, action.DeckID)
    if err != nil {
        events := []game.Event{
            {
                Type: "Error",
                Data: map[string]interface{}{
                    "message": err.Error(),
                },
            },
        }
        resp, _ := json.Marshal(events)
        c.ws.WriteMessage(websocket.TextMessage, resp)
        return
    }

    c.GameID = g.ID
    GameHub.JoinGame(c, g.ID)

    // Build player info including hands
    playersInfo := make(map[string]interface{})
    for uid, player := range g.Players {
        playersInfo[uid] = map[string]interface{}{
            "hand":        player.Hand,
            "leader":      player.Leader,
            "deckSize":    len(player.DrawPile),
            "discardSize": len(player.Discard),
        }
    }

    // Broadcast mulligan phase to both players
    events := []game.Event{
        {
            Type: "MulliganPhase",
            Data: map[string]interface{}{
                "gameId":  g.ID,
                "players": playersInfo,
            },
        },
    }

    GameHub.Broadcast(g.ID, events)
}

func (c *Connection) handleListGames(action game.Action) {
    games := game.Manager.ListGames()

    events := []game.Event{
        {
            Type: "GameList",
            Data: map[string]interface{}{
                "games": games,
            },
        },
    }

    resp, _ := json.Marshal(events)
    c.ws.WriteMessage(websocket.TextMessage, resp)
}

func (c *Connection) handleJoinSpecificGame(action game.Action) {
    c.PlayerUID = action.PlayerUID

    g, _, err := game.Manager.JoinSpecificGame(action.GameID, action.PlayerUID, action.DeckID)
    if err != nil {
        events := []game.Event{
            {Type: "Error", Data: map[string]interface{}{"message": err.Error()}},
        }
        resp, _ := json.Marshal(events)
        c.ws.WriteMessage(websocket.TextMessage, resp)
        return
    }

    c.GameID = g.ID
    GameHub.JoinGame(c, g.ID)

    // Build player info
    playersInfo := make(map[string]interface{})
    for uid, player := range g.Players {
        playersInfo[uid] = map[string]interface{}{
            "hand":        player.Hand,
            "leader":      player.Leader,
            "deckSize":    len(player.DrawPile),
            "discardSize": len(player.Discard),
        }
    }

    // Broadcast mulligan phase to both players
    events := []game.Event{
        {
            Type: "MulliganPhase",
            Data: map[string]interface{}{
                "gameId":  g.ID,
                "players": playersInfo,
            },
        },
    }

    GameHub.Broadcast(g.ID, events)
}

func (c *Connection) handleGameAction(action game.Action) {
    if c.GameID == "" {
        events := []game.Event{
            {Type: "Error", Data: map[string]interface{}{"message": "Not in a game"}},
        }
        resp, _ := json.Marshal(events)
        c.ws.WriteMessage(websocket.TextMessage, resp)
        return
    }

    action.PlayerUID = c.PlayerUID // Use connection's UID
    g := game.Manager.GetGame(c.GameID)
    if g == nil {
        return
    }

    events := g.HandleAction(action)

    // Broadcast events to all players in the game
    GameHub.Broadcast(c.GameID, events)
}

func (c *Connection) handleLeaveGame(action game.Action) {
    if c.GameID == "" {
        return
    }

    // Only notify if game isn't already over
    g := game.Manager.GetGame(c.GameID)
    if g != nil && g.Winner == "" {
        events := []game.Event{
            {
                Type: "OpponentLeft",
                Data: map[string]interface{}{
                    "player": c.PlayerUID,
                },
            },
        }
        GameHub.BroadcastExcept(c.GameID, c, events)
    }

    // Remove from game
    GameHub.LeaveGame(c)
    c.GameID = ""
}

func getPlayerUIDs(g *game.Game) []string {
    uids := make([]string, 0, len(g.Players))
    for uid := range g.Players {
        uids = append(uids, uid)
    }
    return uids
}

func (c *Connection) handleChat(action game.Action) {
    if c.GameID == "" {
        return
    }

    // Sanitize message (basic length limit)
    msg := action.Message
    if len(msg) > 500 {
        msg = msg[:500]
    }
    if msg == "" {
        return
    }

    events := []game.Event{
        {
            Type: "ChatMessage",
            Data: map[string]interface{}{
                "player":  c.PlayerUID,
                "message": msg,
            },
        },
    }

    GameHub.Broadcast(c.GameID, events)
}

func (c *Connection) handleReconnectGame(action game.Action) {
    g := game.Manager.GetGame(action.GameID)
    if g == nil {
        events := []game.Event{
            {Type: "Error", Data: map[string]interface{}{"message": "Game not found"}},
        }
        resp, _ := json.Marshal(events)
        c.ws.WriteMessage(websocket.TextMessage, resp)
        return
    }

    // Check if player is in this game
    player, exists := g.Players[action.PlayerUID]
    if !exists {
        events := []game.Event{
            {Type: "Error", Data: map[string]interface{}{"message": "You are not in this game"}},
        }
        resp, _ := json.Marshal(events)
        c.ws.WriteMessage(websocket.TextMessage, resp)
        return
    }

    // Check if game is already over
    if g.Winner != "" {
        events := []game.Event{
            {Type: "Error", Data: map[string]interface{}{"message": "Game is already over", "winner": g.Winner}},
        }
        resp, _ := json.Marshal(events)
        c.ws.WriteMessage(websocket.TextMessage, resp)
        return
    }

    // Set connection state
    c.PlayerUID = action.PlayerUID
    c.GameID = g.ID
    GameHub.JoinGame(c, g.ID)

    // Find opponent
    var opponentUID string
    var opponent *game.Player
    for uid, p := range g.Players {
        if uid != action.PlayerUID {
            opponentUID = uid
            opponent = p
            break
        }
    }

    // Build reconnect state
    // Separate player's field into creatures and lands
    myCreatures := []*game.FieldCard{}
    myLands := []*game.FieldCard{}
    for _, fc := range player.Field {
        card := game.CardDB[fc.CardID]
        if card.CardType == "Land" {
            myLands = append(myLands, fc)
        } else {
            myCreatures = append(myCreatures, fc)
        }
    }

    // Separate opponent's field into creatures and lands
    opponentCreatures := []*game.FieldCard{}
    opponentLands := []*game.FieldCard{}
    if opponent != nil {
        for _, fc := range opponent.Field {
            card := game.CardDB[fc.CardID]
            if card.CardType == "Land" {
                opponentLands = append(opponentLands, fc)
            } else {
                opponentCreatures = append(opponentCreatures, fc)
            }
        }
    }

    opponentLife := 30
    if opponent != nil {
        opponentLife = opponent.Life
    }

    events := []game.Event{
        {
            Type: "GameReconnected",
            Data: map[string]interface{}{
                "gameId":           g.ID,
                "playerUid":        action.PlayerUID,
                "opponentUid":      opponentUID,
                "currentTurn":      g.Turn,
                "started":          g.Started,
                "myHand":           player.Hand,
                "myLife":           player.Life,
                "myField":          myCreatures,
                "myLands":          myLands,
                "myManaPool":       player.ManaPool,
                "myDeckSize":       len(player.DrawPile),
                "myDiscardSize":    len(player.Discard),
                "opponentLife":     opponentLife,
                "opponentField":    opponentCreatures,
                "opponentLands":    opponentLands,
            },
        },
    }

    resp, _ := json.Marshal(events)
    c.ws.WriteMessage(websocket.TextMessage, resp)

    // Notify opponent that player reconnected
    if opponent != nil {
        reconnectNotify := []game.Event{
            {
                Type: "PlayerReconnected",
                Data: map[string]interface{}{
                    "player": action.PlayerUID,
                },
            },
        }
        GameHub.BroadcastExcept(g.ID, c, reconnectNotify)
    }
}
