package game

// AttackDeclaration represents a single attack in combat
type AttackDeclaration struct {
    AttackerInstanceID int    `json:"attackerInstanceId"`
    TargetType         string `json:"targetType"`      // "creature" or "player"
    TargetInstanceID   int    `json:"targetInstanceId"` // instanceId for creature targets
    TargetPlayerUID    string `json:"targetPlayerUid"`  // UID for player targets
}

// BlockerDeclaration represents a blocker assignment
type BlockerDeclaration struct {
    BlockerInstanceID  int `json:"blockerInstanceId"`  // The creature blocking
    AttackerInstanceID int `json:"attackerInstanceId"` // The attacker being blocked
}

type Action struct {
    PlayerUID  string               `json:"playerUid"`
    Type       string               `json:"type"` // "start_game", "join_game", "play_card", "end_turn", "tap_card", "declare_attacks", "draw_card", etc.
    GameID     string               `json:"gameId"`
    CardID     int                  `json:"cardId"`
    TargetID   int                  `json:"targetId"`
    DeckID     int                  `json:"deckId"`
    AIDeckID   int                  `json:"aiDeckId"`   // For AI game: deck for the AI opponent
    InstanceID int                  `json:"instanceId"` // For targeting specific field cards
    Attacks    []AttackDeclaration  `json:"attacks"`    // For batch combat declarations
    Blockers   []BlockerDeclaration `json:"blockers"`   // For blocker assignments
    Message    string               `json:"message"`    // For chat messages
    Source     string               `json:"source"`     // For draw_card: "main" or "vault"
}
