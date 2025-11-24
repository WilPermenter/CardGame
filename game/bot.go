// bot.go - Simple AI opponent
package game

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// BotDecide returns the next action the bot should take, or nil if nothing to do
func (g *Game) BotDecide(botUID string) *Action {
	player := g.Players[botUID]
	if player == nil {
		return nil
	}

	// Mulligan phase
	if g.MulliganPhase {
		return g.botMulliganDecision(botUID, player)
	}

	// Draw phase
	if g.DrawPhase && g.Turn == botUID {
		source := "main"

		// Count lands on field
		landsOnField := 0
		for _, fc := range player.Field {
			card := CardDB[fc.CardID]
			if card.CardType == "Land" {
				landsOnField++
			}
		}

		// Check for lands in hand
		hasLandInHand := false
		for _, cardID := range player.Hand {
			card := CardDB[cardID]
			if card.CardType == "Land" {
				hasLandInHand = true
				break
			}
		}

		// Draw from vault if <6 lands and no lands in hand
		if landsOnField < 6 && !hasLandInHand && len(player.VaultPile) > 0 {
			source = "vault"
		}

		return &Action{
			Type:      "draw_card",
			PlayerUID: botUID,
			Source:    source,
		}
	}

	// Combat response - if bot has priority
	if g.CombatPhase == "response_window" && g.PriorityPlayer == botUID {
		return g.botCombatResponse(botUID, player)
	}

	// Not our turn
	if g.Turn != botUID {
		return nil
	}

	// Main phase actions (in priority order)

	// 1. Play a land if we can
	if action := g.botPlayLand(botUID, player); action != nil {
		return action
	}

	// 2. Tap untapped lands for mana
	if action := g.botTapLands(botUID, player); action != nil {
		return action
	}

	// 3. Play leader if we can afford it
	if action := g.botPlayLeader(botUID, player); action != nil {
		return action
	}

	// 4. Play a creature/spell we can afford
	if action := g.botPlayCard(botUID, player); action != nil {
		return action
	}

	// 5. Attack with available creatures
	if action := g.botDeclareAttacks(botUID, player); action != nil {
		return action
	}

	// 6. Nothing else to do - end turn
	return &Action{
		Type:      "end_turn",
		PlayerUID: botUID,
	}
}

// botMulliganDecision decides whether to keep or mulligan
func (g *Game) botMulliganDecision(botUID string, player *Player) *Action {
	if g.MulliganDecisions[botUID] {
		return nil // Already decided
	}

	// Count lands and playable cards in hand
	lands := 0
	playables := 0
	for _, cardID := range player.Hand {
		card := CardDB[cardID]
		if card.CardType == "Land" {
			lands++
		} else if card.Cost.Total() <= 4 {
			playables++
		}
	}

	// Keep if we have 2-4 lands and at least 2 playable cards
	if lands >= 2 && lands <= 4 && playables >= 2 {
		return &Action{Type: "keep_hand", PlayerUID: botUID}
	}

	// Mulligan if this is first decision, otherwise keep
	// (Simple bot only mulligans once)
	if len(player.Hand) >= 7 {
		return &Action{Type: "mulligan", PlayerUID: botUID}
	}

	return &Action{Type: "keep_hand", PlayerUID: botUID}
}

// botPlayLand plays a land from hand if possible
func (g *Game) botPlayLand(botUID string, player *Player) *Action {
	if player.LandsPlayedThisTurn >= player.LandsPerTurn {
		return nil
	}

	for _, cardID := range player.Hand {
		card := CardDB[cardID]
		if card.CardType == "Land" {
			return &Action{
				Type:      "play_card",
				PlayerUID: botUID,
				CardID:    cardID,
			}
		}
	}
	return nil
}

// botTapLands taps an untapped land for mana
func (g *Game) botTapLands(botUID string, player *Player) *Action {
	for _, fc := range player.Field {
		card := CardDB[fc.CardID]
		if card.CardType == "Land" && !fc.IsTapped() {
			return &Action{
				Type:       "tap_card",
				PlayerUID:  botUID,
				InstanceID: fc.InstanceID,
			}
		}
	}
	return nil
}

// botPlayLeader plays the leader if affordable
func (g *Game) botPlayLeader(botUID string, player *Player) *Action {
	if player.Leader == 0 {
		return nil
	}

	card := CardDB[player.Leader]
	if player.ManaPool.CanAfford(card.Cost) {
		return &Action{
			Type:      "play_leader",
			PlayerUID: botUID,
		}
	}
	return nil
}

// botPlayCard plays the most expensive card we can afford
func (g *Game) botPlayCard(botUID string, player *Player) *Action {
	var bestCard int
	bestCost := -1

	for _, cardID := range player.Hand {
		card := CardDB[cardID]
		// Skip lands (handled separately) and instants (save for combat)
		if card.CardType == "Land" || card.CardType == "Instant" {
			continue
		}

		if player.ManaPool.CanAfford(card.Cost) {
			cost := card.Cost.Total()
			if cost > bestCost {
				bestCost = cost
				bestCard = cardID
			}
		}
	}

	if bestCard != 0 {
		return &Action{
			Type:      "play_card",
			PlayerUID: botUID,
			CardID:    bestCard,
		}
	}
	return nil
}

// botDeclareAttacks attacks with all available creatures
func (g *Game) botDeclareAttacks(botUID string, player *Player) *Action {
	// Find opponent
	var opponentUID string
	var opponent *Player
	for uid, p := range g.Players {
		if uid != botUID {
			opponentUID = uid
			opponent = p
			break
		}
	}

	if opponent == nil {
		return nil
	}

	// Check for untapped Taunt creatures on opponent's field
	tauntTargets := []*FieldCard{}
	for _, fc := range opponent.Field {
		card := CardDB[fc.CardID]
		if card.CardType == "Creature" && !fc.IsTapped() && card.HasAbility("Taunt") {
			tauntTargets = append(tauntTargets, fc)
		}
	}

	// Gather our attackers - all untapped creatures that can attack
	myAttackers := []*FieldCard{}
	for _, fc := range player.Field {
		card := CardDB[fc.CardID]
		if card.CardType == "Creature" && !fc.IsTapped() && fc.CanAttack {
			myAttackers = append(myAttackers, fc)
		}
	}

	if len(myAttackers) == 0 {
		return nil
	}

	attacks := []AttackDeclaration{}

	if len(tauntTargets) > 0 {
		// Must attack Taunt creatures - distribute attackers among taunts
		for i, attacker := range myAttackers {
			tauntIdx := i % len(tauntTargets)
			attacks = append(attacks, AttackDeclaration{
				AttackerInstanceID: attacker.InstanceID,
				TargetType:         "creature",
				TargetInstanceID:   tauntTargets[tauntIdx].InstanceID,
			})
		}
	} else {
		// No taunts - attack player directly
		for _, attacker := range myAttackers {
			attacks = append(attacks, AttackDeclaration{
				AttackerInstanceID: attacker.InstanceID,
				TargetType:         "player",
				TargetPlayerUID:    opponentUID,
			})
		}
	}

	if len(attacks) > 0 {
		return &Action{
			Type:      "declare_attacks",
			PlayerUID: botUID,
			Attacks:   attacks,
		}
	}
	return nil
}

// botCombatResponse handles combat instant response
func (g *Game) botCombatResponse(botUID string, player *Player) *Action {
	// Simple bot: try to play a buff instant on an attacker, otherwise pass

	// Check if we're the attacker and have buff instants
	if g.AttackingPlayer == botUID {
		// Find a buff instant we can afford
		for _, cardID := range player.Hand {
			card := CardDB[cardID]
			if card.CardType == "Instant" && player.ManaPool.CanAfford(card.Cost) {
				// Check if it's a buff (has Buff in script)
				if containsBuff(card.CustomScript) {
					// Find one of our attacking creatures to target
					for _, pa := range g.PendingAttacks {
						for _, fc := range player.Field {
							if fc.InstanceID == pa.AttackerInstanceID {
								return &Action{
									Type:       "play_instant",
									PlayerUID:  botUID,
									CardID:     cardID,
									InstanceID: fc.InstanceID,
								}
							}
						}
					}
				}
			}
		}
	} else {
		// We're defending - try to play damage/tap instants on attackers
		for _, cardID := range player.Hand {
			card := CardDB[cardID]
			if card.CardType == "Instant" && player.ManaPool.CanAfford(card.Cost) {
				// Check if it's a damage or tap instant
				if containsDamageOrTap(card.CustomScript) {
					// Target an attacking creature
					if len(g.PendingAttacks) > 0 {
						return &Action{
							Type:       "play_instant",
							PlayerUID:  botUID,
							CardID:     cardID,
							InstanceID: g.PendingAttacks[0].AttackerInstanceID,
						}
					}
				}
			}
		}
	}

	// Default: pass priority
	return &Action{
		Type:      "pass_priority",
		PlayerUID: botUID,
	}
}

// containsBuff checks if a script contains a Buff command
func containsBuff(script string) bool {
	return len(script) > 0 && (contains(script, "Buff(") || contains(script, "buff("))
}

// containsDamageOrTap checks if script has damage or tap
func containsDamageOrTap(script string) bool {
	return contains(script, "Damage") || contains(script, "damage") ||
		contains(script, "Tap") || contains(script, "tap") ||
		contains(script, "Destroy") || contains(script, "destroy") ||
		contains(script, "Bounce") || contains(script, "bounce")
}
