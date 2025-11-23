package game

import (
    "fmt"
    "log"
    "math/rand"
    "sync"
    "time"
)

type Game struct {
    ID             string
    Players        map[string]*Player // keyed by player UID
    Turn           string             // UID of whose turn it is
    Started        bool
    Winner         string             // UID of winner, empty if game ongoing
    NextInstanceID int                // Counter for unique field card IDs

    // Mulligan state
    MulliganPhase     bool            // true while waiting for mulligan decisions
    MulliganDecisions map[string]bool // tracks each player's decision (true = decided)

    // Draw phase state
    DrawPhase bool // true when active player must draw before taking other actions

    // Combat state
    CombatPhase    string              // "", "attackers_declared", "response_window"
    PendingAttacks []PendingAttack     // Attacks waiting for blockers/resolution
    AttackingPlayer string             // UID of player who declared attacks

    // Response window state (for instants during combat)
    PriorityPlayer string          // UID of player who has priority to play instants
    PassedPlayers  map[string]bool // Tracks which players passed priority consecutively

    // Cleanup tracking
    LastActivity time.Time           // Updated on every action
    Disconnects  map[string]time.Time // playerUID -> disconnect time
}

// PendingAttack represents an attack waiting for blocker assignment
type PendingAttack struct {
    AttackerInstanceID int    `json:"attackerInstanceId"`
    TargetType         string `json:"targetType"`         // "creature" or "player"
    TargetInstanceID   int    `json:"targetInstanceId"`   // for creature targets
    TargetPlayerUID    string `json:"targetPlayerUid"`    // for player targets
    BlockerInstanceID  int    `json:"blockerInstanceId"`  // 0 if not blocked
}

// NewFieldCard creates a new card on the battlefield
func (g *Game) NewFieldCard(cardID int, owner, castedBy string) *FieldCard {
    card := CardDB[cardID]
    fc := &FieldCard{
        InstanceID:     g.NextInstanceID,
        CardID:         cardID,
        Owner:          owner,
        CastedBy:       castedBy,
        DamageModifier: 0,
        HealthModifier: 0,
        CurrentHealth:  card.Defense,
        CanAttack:      false, // Summoning sickness by default
        Status:         make(map[string]int),
    }
    g.NextInstanceID++

    // Set summoned status (summoning sickness)
    fc.Status["Summoned"] = 1

    // Check for Haste ability - bypasses summoning sickness
    for _, ability := range card.Abilities {
        if ability == "Haste" {
            fc.CanAttack = true
            fc.Status["Summoned"] = 0
            break
        }
    }

    return fc
}

type Player struct {
    UID                 string
    Hand                []int
    DrawPile            []int        // Shuffled main deck (creatures) to draw from
    VaultPile           []int        // Shuffled vault (lands) to draw from
    Discard             []int        // Played/discarded cards
    Field               []*FieldCard // Cards on the battlefield
    Life                int
    DeckID              int          // Deck ID
    Leader              int          // Leader card ID
    ManaPool            ManaCost     // Current mana available to spend
    LandsPerTurn        int          // Max lands that can be played per turn (default 1)
    LandsPlayedThisTurn int          // Lands played this turn
    MinHandLimit        int          // Minimum hand size to draw up to (default 1)
}

// GetAvailableMana counts the total mana available from lands on the field
func (p *Player) GetAvailableMana() ManaCost {
    available := ManaCost{}
    for _, fc := range p.Field {
        card := CardDB[fc.CardID]
        if card.CardType == "Land" {
            provided := card.GetProvidedMana()
            available.White += provided.White
            available.Blue += provided.Blue
            available.Black += provided.Black
            available.Red += provided.Red
            available.Green += provided.Green
            available.Colorless += provided.Colorless
        }
    }
    return available
}

// FieldCard represents a card that's been played onto the battlefield
type FieldCard struct {
    InstanceID     int            `json:"instanceId"`     // Unique ID for this instance on field
    CardID         int            `json:"cardId"`         // Reference to the card in CardDB
    Owner          string         `json:"owner"`          // UID of player who owns the card
    CastedBy       string         `json:"castedBy"`       // UID of player who played the card
    DamageModifier int            `json:"damageModifier"` // +/- to attack
    HealthModifier int            `json:"healthModifier"` // +/- to defense/health
    CurrentHealth  int            `json:"currentHealth"`  // Current health (starts at card's Defense)
    CanAttack      bool           `json:"canAttack"`      // Whether it can attack this turn (summoning sickness)
    Status         map[string]int `json:"status"`         // Status values (Tapped=1, etc.)
}

// IsTapped returns whether the card is tapped
func (fc *FieldCard) IsTapped() bool {
    if fc.Status == nil {
        return false
    }
    return fc.Status["Tapped"] > 0
}

// SetTapped sets the tapped status
func (fc *FieldCard) SetTapped(tapped bool) {
    if fc.Status == nil {
        fc.Status = make(map[string]int)
    }
    if tapped {
        fc.Status["Tapped"] = 1
    } else {
        fc.Status["Tapped"] = 0
    }
}

// IsSummoned returns whether the card has summoning sickness
func (fc *FieldCard) IsSummoned() bool {
    if fc.Status == nil {
        return false
    }
    return fc.Status["Summoned"] > 0
}

// GetAttack returns the effective attack value
func (fc *FieldCard) GetAttack() int {
    card := CardDB[fc.CardID]
    return card.Attack + fc.DamageModifier
}

// GetMaxHealth returns the effective max health
func (fc *FieldCard) GetMaxHealth() int {
    card := CardDB[fc.CardID]
    return card.Defense + fc.HealthModifier
}

// IsDead returns true if the card should be removed from field
func (fc *FieldCard) IsDead() bool {
    return fc.CurrentHealth <= 0
}

const InitialMainDeckDraw = 5
const InitialVaultDraw = 2
const DefaultMinHandLimit = 1
const DefaultLandsPerTurn = 1
const DefaultLife = 30

// NewPlayer creates a new player with a deck
func NewPlayer(uid string, deck Deck) *Player {
    return &Player{
        UID:          uid,
        Hand:         []int{},
        DrawPile:     ShuffleDeck(deck.MainDeck),
        VaultPile:    ShuffleDeck(deck.Vault),
        Discard:      []int{},
        Field:        []*FieldCard{},
        Life:         DefaultLife,
        DeckID:       deck.ID,
        Leader:       deck.Leader,
        LandsPerTurn: DefaultLandsPerTurn,
        MinHandLimit: DefaultMinHandLimit,
    }
}

// ShuffleDeck creates a shuffled draw pile from a deck
func ShuffleDeck(cards []int) []int {
    pile := make([]int, len(cards))
    copy(pile, cards)
    rand.Shuffle(len(pile), func(i, j int) {
        pile[i], pile[j] = pile[j], pile[i]
    })
    return pile
}

// DrawCards draws n cards from the main deck (DrawPile)
func (p *Player) DrawCards(n int) []int {
    drawn := []int{}
    for i := 0; i < n && len(p.DrawPile) > 0; i++ {
        card := p.DrawPile[0]
        p.DrawPile = p.DrawPile[1:]
        p.Hand = append(p.Hand, card)
        drawn = append(drawn, card)
    }
    return drawn
}

// DrawFromVault draws n cards from the vault (land deck)
func (p *Player) DrawFromVault(n int) []int {
    drawn := []int{}
    for i := 0; i < n && len(p.VaultPile) > 0; i++ {
        card := p.VaultPile[0]
        p.VaultPile = p.VaultPile[1:]
        p.Hand = append(p.Hand, card)
        drawn = append(drawn, card)
    }
    return drawn
}

// GameManager handles multiple concurrent games
type GameManager struct {
    mu       sync.RWMutex
    games    map[string]*Game // gameID -> Game
    waiting  *Game            // game waiting for second player
    nextID   int
}

var Manager = &GameManager{
    games:  make(map[string]*Game),
    nextID: 1,
}

func (gm *GameManager) CreateGame(playerUID string, deckID int) (*Game, *Player, error) {
    gm.mu.Lock()
    defer gm.mu.Unlock()

    deck, ok := DeckDB[deckID]
    if !ok {
        return nil, nil, fmt.Errorf("deck not found: %d", deckID)
    }

    gameID := fmt.Sprintf("game_%d", gm.nextID)
    gm.nextID++

    player := NewPlayer(playerUID, deck)

    g := &Game{
        ID:             gameID,
        Players:        map[string]*Player{playerUID: player},
        Turn:           playerUID,
        Started:        false,
        NextInstanceID: 1,
    }

    gm.games[gameID] = g
    gm.waiting = g

    return g, player, nil
}

func (gm *GameManager) JoinGame(playerUID string, deckID int) (*Game, *Player, error) {
    gm.mu.Lock()
    defer gm.mu.Unlock()

    if gm.waiting == nil {
        return nil, nil, fmt.Errorf("no game available")
    }

    deck, ok := DeckDB[deckID]
    if !ok {
        return nil, nil, fmt.Errorf("deck not found: %d", deckID)
    }

    g := gm.waiting

    // Don't let same player join twice
    if _, exists := g.Players[playerUID]; exists {
        return nil, nil, fmt.Errorf("already in this game")
    }

    player := NewPlayer(playerUID, deck)

    g.Players[playerUID] = player
    gm.waiting = nil // game is full

    // Draw initial hands for both players
    g.DrawInitialHands()

    // Start mulligan phase (game starts after both players decide)
    g.MulliganPhase = true
    g.MulliganDecisions = make(map[string]bool)

    return g, player, nil
}

func (gm *GameManager) GetGame(gameID string) *Game {
    gm.mu.RLock()
    defer gm.mu.RUnlock()
    return gm.games[gameID]
}

// GameInfo for lobby display
type GameInfo struct {
    GameID      string   `json:"gameId"`
    PlayerCount int      `json:"playerCount"`
    Players     []string `json:"players"`
    Started     bool     `json:"started"`
}

// ListGames returns all games for the lobby
func (gm *GameManager) ListGames() []GameInfo {
    gm.mu.RLock()
    defer gm.mu.RUnlock()

    games := []GameInfo{}
    for _, g := range gm.games {
        players := []string{}
        for uid := range g.Players {
            players = append(players, uid)
        }
        games = append(games, GameInfo{
            GameID:      g.ID,
            PlayerCount: len(g.Players),
            Players:     players,
            Started:     g.Started,
        })
    }
    return games
}

// JoinSpecificGame joins a specific game by ID
func (gm *GameManager) JoinSpecificGame(gameID, playerUID string, deckID int) (*Game, *Player, error) {
    gm.mu.Lock()
    defer gm.mu.Unlock()

    g, exists := gm.games[gameID]
    if !exists {
        return nil, nil, fmt.Errorf("game not found")
    }
    if g.Started || g.MulliganPhase || len(g.Players) >= 2 {
        return nil, nil, fmt.Errorf("game is full")
    }
    if _, exists := g.Players[playerUID]; exists {
        return nil, nil, fmt.Errorf("already in this game")
    }

    deck, ok := DeckDB[deckID]
    if !ok {
        return nil, nil, fmt.Errorf("deck not found")
    }

    player := NewPlayer(playerUID, deck)

    g.Players[playerUID] = player
    if gm.waiting == g {
        gm.waiting = nil
    }
    g.DrawInitialHands()

    // Start mulligan phase (game starts after both players decide)
    g.MulliganPhase = true
    g.MulliganDecisions = make(map[string]bool)

    return g, player, nil
}

// DrawInitialHands draws starting hands for all players (5 from main deck, 2 from vault)
func (g *Game) DrawInitialHands() {
    for _, player := range g.Players {
        player.DrawCards(InitialMainDeckDraw)
        player.DrawFromVault(InitialVaultDraw)
    }
}

// MarkPlayerDisconnected records when a player disconnected
func (g *Game) MarkPlayerDisconnected(playerUID string) {
    if g.Disconnects == nil {
        g.Disconnects = make(map[string]time.Time)
    }
    g.Disconnects[playerUID] = time.Now()
}

// MarkPlayerReconnected clears disconnect status when player reconnects
func (g *Game) MarkPlayerReconnected(playerUID string) {
    if g.Disconnects != nil {
        delete(g.Disconnects, playerUID)
    }
}

// AllPlayersDisconnectedFor checks if all players have been disconnected for the given duration
func (g *Game) AllPlayersDisconnectedFor(duration time.Duration) bool {
    if g.Disconnects == nil || len(g.Disconnects) < len(g.Players) {
        return false
    }
    cutoff := time.Now().Add(-duration)
    for _, disconnectTime := range g.Disconnects {
        if disconnectTime.After(cutoff) {
            return false // Someone disconnected too recently
        }
    }
    return true
}

// RemoveGame removes a game from the manager
func (gm *GameManager) RemoveGame(gameID string) {
    gm.mu.Lock()
    defer gm.mu.Unlock()
    if gm.waiting != nil && gm.waiting.ID == gameID {
        gm.waiting = nil
    }
    delete(gm.games, gameID)
    log.Printf("Game %s removed", gameID)
}

// GetAllGameIDs returns all game IDs for cleanup iteration
func (gm *GameManager) GetAllGameIDs() []string {
    gm.mu.RLock()
    defer gm.mu.RUnlock()
    ids := make([]string, 0, len(gm.games))
    for id := range gm.games {
        ids = append(ids, id)
    }
    return ids
}

// Cleanup constants
const (
    DisconnectTimeout = 1 * time.Minute  // Kill if both disconnected 1+ min
    InactivityTimeout = 5 * time.Minute  // Kill if no activity 5+ min
    CleanupInterval   = 30 * time.Second // Check every 30 seconds
)

// StartCleanupRoutine starts the background cleanup goroutine
func (gm *GameManager) StartCleanupRoutine() {
    go func() {
        ticker := time.NewTicker(CleanupInterval)
        defer ticker.Stop()
        log.Println("Game cleanup routine started")
        for range ticker.C {
            gm.cleanupStaleGames()
        }
    }()
}

func (gm *GameManager) cleanupStaleGames() {
    gameIDs := gm.GetAllGameIDs()
    now := time.Now()

    for _, gameID := range gameIDs {
        g := gm.GetGame(gameID)
        if g == nil {
            continue
        }

        shouldRemove := false
        reason := ""

        // Check 1: Both players disconnected for 1+ minute
        if len(g.Players) > 0 && g.AllPlayersDisconnectedFor(DisconnectTimeout) {
            shouldRemove = true
            reason = "both players disconnected"
        }

        // Check 2: No activity for 5+ minutes (only for started games)
        if g.Started && !g.LastActivity.IsZero() && now.Sub(g.LastActivity) > InactivityTimeout {
            shouldRemove = true
            reason = "inactivity timeout"
        }

        if shouldRemove {
            log.Printf("Removing game %s: %s", gameID, reason)
            gm.RemoveGame(gameID)
        }
    }
}
