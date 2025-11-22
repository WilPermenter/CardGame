package game

import (
    "fmt"
    "math/rand"
    "sync"
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

    // Combat state
    CombatPhase    string              // "", "attackers_declared", "blockers_declared"
    PendingAttacks []PendingAttack     // Attacks waiting for blockers/resolution
    AttackingPlayer string             // UID of player who declared attacks
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
    DrawPile            []int        // Shuffled cards to draw from
    Discard             []int        // Played/discarded cards
    Field               []*FieldCard // Cards on the battlefield
    Life                int
    DeckID              int          // Deck ID
    Leader              int          // Leader card ID
    ManaPool            ManaCost     // Current mana available to spend
    LandsPerTurn        int          // Max lands that can be played per turn (default 1)
    LandsPlayedThisTurn int          // Lands played this turn
    MinHandLimit        int          // Minimum hand size to draw up to (default 5)
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

const InitialHandSize = 5

// ShuffleDeck creates a shuffled draw pile from a deck
func ShuffleDeck(cards []int) []int {
    pile := make([]int, len(cards))
    copy(pile, cards)
    rand.Shuffle(len(pile), func(i, j int) {
        pile[i], pile[j] = pile[j], pile[i]
    })
    return pile
}

// DrawCards draws n cards from a player's draw pile
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

    player := &Player{
        UID:          playerUID,
        Hand:         []int{},
        DrawPile:     ShuffleDeck(deck.Cards),
        Discard:      []int{},
        Field:        []*FieldCard{},
        Life:         30,
        DeckID:       deckID,
        Leader:       deck.Leader,
        LandsPerTurn: 1,
        MinHandLimit: 5,
    }

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

    player := &Player{
        UID:          playerUID,
        Hand:         []int{},
        DrawPile:     ShuffleDeck(deck.Cards),
        Discard:      []int{},
        Field:        []*FieldCard{},
        Life:         30,
        DeckID:       deckID,
        Leader:       deck.Leader,
        LandsPerTurn: 1,
        MinHandLimit: 5,
    }

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

    player := &Player{
        UID:          playerUID,
        Hand:         []int{},
        DrawPile:     ShuffleDeck(deck.Cards),
        Discard:      []int{},
        Field:        []*FieldCard{},
        Life:         30,
        DeckID:       deckID,
        Leader:       deck.Leader,
        LandsPerTurn: 1,
        MinHandLimit: 5,
    }

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

// DrawInitialHands draws starting hands for all players
func (g *Game) DrawInitialHands() map[string][]int {
    hands := make(map[string][]int)
    for uid, player := range g.Players {
        drawn := player.DrawCards(InitialHandSize)
        hands[uid] = drawn
    }
    return hands
}
