package game

type Action struct {
    PlayerID int    `json:"playerId"`
    Type     string `json:"type"`      // "play_card", "attack", etc.
    CardID   int    `json:"cardId"`
    TargetID int    `json:"targetId"`
}
