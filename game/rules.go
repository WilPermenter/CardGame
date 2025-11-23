// rules.go - Core game rules: action handling, turn management, priority
package game

import "time"

// checkPriority determines if a player can take an action
func (g *Game) checkPriority(playerUID string, actionType string) bool {
	// During response window, only priority player can act
	if g.CombatPhase == "response_window" {
		if actionType == "play_instant" || actionType == "pass_priority" || actionType == "tap_card" || actionType == "burn_card" {
			return playerUID == g.PriorityPlayer
		}
		return false
	}

	// During combat, special rules apply
	if g.CombatPhase == "attackers_declared" {
		if actionType == "declare_blockers" {
			for uid := range g.Players {
				if uid != g.AttackingPlayer {
					return playerUID == uid
				}
			}
		}
		return false
	}

	// Normal priority - active player's turn
	return playerUID == g.Turn
}

// HandleAction is the main entry point for all game actions
func (g *Game) HandleAction(a Action) []Event {
	g.LastActivity = time.Now()

	// Game already over?
	if g.Winner != "" {
		return []Event{{Type: "GameOver", Data: map[string]interface{}{"winner": g.Winner}}}
	}

	// Handle mulligan phase
	if g.MulliganPhase {
		switch a.Type {
		case "keep_hand":
			return g.keepHand(a)
		case "mulligan":
			return g.takeMulligan(a)
		default:
			return []Event{{Type: "MulliganPhaseActive", Data: map[string]interface{}{"message": "Must keep hand or mulligan first"}}}
		}
	}

	// Game not started?
	if !g.Started {
		return []Event{{Type: "GameNotStarted", Data: map[string]interface{}{}}}
	}

	// Handle draw phase
	if g.DrawPhase && a.PlayerUID == g.Turn {
		if a.Type == "draw_card" {
			return g.drawCardAction(a)
		}
		return []Event{{Type: "MustDraw", Data: map[string]interface{}{"message": "Must draw a card first"}}}
	}

	// Priority check
	if !g.checkPriority(a.PlayerUID, a.Type) {
		return []Event{
			{
				Type: "NotYourPriority",
				Data: map[string]interface{}{
					"turn":        g.Turn,
					"combatPhase": g.CombatPhase,
					"player":      a.PlayerUID,
				},
			},
		}
	}

	// Route to appropriate handler
	switch a.Type {
	case "end_turn":
		return g.endTurn(a)
	case "play_card":
		return g.playCard(a)
	case "tap_card":
		return g.tapCard(a)
	case "burn_card":
		return g.burnCard(a)
	case "declare_attacks":
		return g.declareAttacks(a)
	case "play_leader":
		return g.playLeader(a)
	case "play_instant":
		return g.playInstant(a)
	case "pass_priority":
		return g.passPriority(a)
	default:
		return []Event{
			{
				Type: "UnknownAction",
				Data: map[string]interface{}{"action": a.Type},
			},
		}
	}
}

// endTurn handles ending the current turn
func (g *Game) endTurn(a Action) []Event {
	events := []Event{}

	// Untap Vigilance creatures
	endingPlayer := g.Players[g.Turn]
	for _, fc := range endingPlayer.Field {
		if fc.IsTapped() {
			card := CardDB[fc.CardID]
			if card.HasAbility("Vigilance") {
				fc.SetTapped(false)
				events = append(events, Event{
					Type: "CardUntapped",
					Data: map[string]interface{}{
						"player":     g.Turn,
						"instanceId": fc.InstanceID,
						"reason":     "Vigilance",
					},
				})
			}
		}
	}

	// Switch turn
	for uid := range g.Players {
		if uid != g.Turn {
			g.Turn = uid
			break
		}
	}

	activePlayer := g.Players[g.Turn]

	// Start of turn: untap, clear sickness, clear mana
	for _, fc := range activePlayer.Field {
		fc.SetTapped(false)
		fc.CanAttack = true
		fc.Status["Summoned"] = 0
	}
	activePlayer.ManaPool.Clear()
	activePlayer.LandsPlayedThisTurn = 0

	events = append(events, Event{
		Type: "TurnChanged",
		Data: map[string]interface{}{
			"activePlayer": g.Turn,
			"manaPool":     activePlayer.ManaPool,
		},
	})

	// Enter draw phase
	g.DrawPhase = true
	events = append(events, Event{
		Type: "DrawPhase",
		Data: map[string]interface{}{
			"player":       g.Turn,
			"mainDeckSize": len(activePlayer.DrawPile),
			"vaultSize":    len(activePlayer.VaultPile),
		},
	})

	return events
}

// drawCard draws a card from main deck
func (g *Game) drawCard(p *Player) (int, bool) {
	if len(p.DrawPile) == 0 {
		return 0, false
	}
	drawn := p.DrawCards(1)
	return drawn[0], true
}

// drawCardAction handles the draw_card action
func (g *Game) drawCardAction(a Action) []Event {
	player := g.Players[a.PlayerUID]

	var cardDrawn int
	var drawn []int

	switch a.Source {
	case "main":
		if len(player.DrawPile) == 0 {
			return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Main deck is empty"}}}
		}
		drawn = player.DrawCards(1)
		cardDrawn = drawn[0]
	case "vault":
		if len(player.VaultPile) == 0 {
			return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Vault is empty"}}}
		}
		drawn = player.DrawFromVault(1)
		cardDrawn = drawn[0]
	default:
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Invalid source, must be 'main' or 'vault'"}}}
	}

	g.DrawPhase = false

	return []Event{
		{
			Type: "CardDrawn",
			Data: map[string]interface{}{
				"player":       a.PlayerUID,
				"cardId":       cardDrawn,
				"source":       a.Source,
				"mainDeckSize": len(player.DrawPile),
				"vaultSize":    len(player.VaultPile),
			},
		},
	}
}
