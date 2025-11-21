package game

func (g *Game) HandleAction(a Action) []Event {
    // Wrong turn? Reject
    if a.PlayerID != g.Turn {
        return []Event{
            {
                Type: "NotYourTurn",
                Data: map[string]interface{}{
                    "expected": g.Turn,
                    "actual":   a.PlayerID,
                },
            },
        }
    }

    switch a.Type {
    case "end_turn":
        return g.endTurn(a)

    case "play_card":
        return g.playCard(a)

    default:
        return []Event{
            {
                Type: "UnknownAction",
                Data: map[string]interface{}{
                    "action": a.Type,
                },
            },
        }
    }
}

func (g *Game) playCard(a Action) []Event {
    // Very simple “deal 3 damage to opponent”
    opponent := 2
    if a.PlayerID == 2 {
        opponent = 1
    }

    g.Players[opponent-1].Life -= 3

    return []Event{
        {
            Type: "CardPlayed",
            Data: map[string]interface{}{
                "player": a.PlayerID,
                "card":   a.CardID,
            },
        },
        {
            Type: "Damage",
            Data: map[string]interface{}{
                "target": opponent,
                "amount": 3,
            },
        },
    }
}

func (g *Game) endTurn(a Action) []Event {
    // Switch turn
    if g.Turn == 1 {
        g.Turn = 2
    } else {
        g.Turn = 1
    }

    // Draw a card for the active player
    activePlayer := g.getPlayer(g.Turn)
    cardDrawn := g.drawCard(activePlayer)

    return []Event{
        {
            Type: "TurnChanged",
            Data: map[string]interface{}{
                "activePlayer": g.Turn,
            },
        },
        {
            Type: "CardDrawn",
            Data: map[string]interface{}{
                "player": g.Turn,
                "card":   cardDrawn,
            },
        },
    }
}

func (g *Game) getPlayer(id int) *Player {
    for _, p := range g.Players {
        if p.ID == id {
            return p
        }
    }
    return nil
}

func (g *Game) drawCard(p *Player) int {
    newCard := 100 + len(p.Hand)
    p.Hand = append(p.Hand, newCard)
    return newCard
}
