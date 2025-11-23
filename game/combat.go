// combat.go - Combat system: attacks, response window, damage resolution
package game

// getUntappedTaunts returns all untapped creatures with Taunt on a player's field
func getUntappedTaunts(player *Player) []*FieldCard {
	taunts := []*FieldCard{}
	for _, fc := range player.Field {
		if fc.IsTapped() {
			continue
		}
		card := CardDB[fc.CardID]
		if card.HasAbility("Taunt") {
			taunts = append(taunts, fc)
		}
	}
	return taunts
}

// declareAttacks handles the declare_attacks action
func (g *Game) declareAttacks(a Action) []Event {
	if len(a.Attacks) == 0 {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "No attacks declared"}}}
	}

	if g.CombatPhase != "" {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Combat already in progress"}}}
	}

	player := g.Players[a.PlayerUID]

	// Find opponent
	var opponentUID string
	for uid := range g.Players {
		if uid != a.PlayerUID {
			opponentUID = uid
			break
		}
	}
	opponent := g.Players[opponentUID]

	// Check for Taunt creatures
	tauntCreatures := getUntappedTaunts(opponent)
	hasTaunts := len(tauntCreatures) > 0
	tauntIDs := make(map[int]bool)
	for _, fc := range tauntCreatures {
		tauntIDs[fc.InstanceID] = true
	}

	// Validate attacks and build pending attacks
	pendingAttacks := []PendingAttack{}

	for _, atk := range a.Attacks {
		// Find attacker
		var attacker *FieldCard
		for _, fc := range player.Field {
			if fc.InstanceID == atk.AttackerInstanceID {
				attacker = fc
				break
			}
		}
		if attacker == nil {
			return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Attacker not found", "instanceId": atk.AttackerInstanceID}}}
		}

		// Validate attacker state
		if attacker.IsTapped() {
			return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Attacker is tapped", "instanceId": atk.AttackerInstanceID}}}
		}
		if attacker.IsSummoned() {
			return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Attacker has summoning sickness", "instanceId": atk.AttackerInstanceID}}}
		}

		card := CardDB[attacker.CardID]
		validTargets := card.ValidAttackTargets
		if validTargets == "" {
			validTargets = "Any"
		}

		// Validate target
		if atk.TargetType == "creature" {
			if validTargets == "Player" {
				return []Event{{Type: "Error", Data: map[string]interface{}{"message": "This creature can only attack players", "instanceId": atk.AttackerInstanceID}}}
			}
			var target *FieldCard
			for _, fc := range opponent.Field {
				if fc.InstanceID == atk.TargetInstanceID {
					target = fc
					break
				}
			}
			if target == nil {
				return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Target creature not found", "targetInstanceId": atk.TargetInstanceID}}}
			}
		} else if atk.TargetType == "player" {
			if validTargets == "Creatures" {
				return []Event{{Type: "Error", Data: map[string]interface{}{"message": "This creature can only attack creatures", "instanceId": atk.AttackerInstanceID}}}
			}
			if atk.TargetPlayerUID != opponentUID {
				return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Can only attack opponent"}}}
			}
			if hasTaunts {
				return []Event{{Type: "Error", Data: map[string]interface{}{
					"message":    "Cannot attack player while opponent has Taunt creatures",
					"instanceId": atk.AttackerInstanceID,
				}}}
			}
		} else {
			return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Invalid target type", "targetType": atk.TargetType}}}
		}

		// Taunt check for creature targets
		if hasTaunts && atk.TargetType == "creature" && !tauntIDs[atk.TargetInstanceID] {
			return []Event{{Type: "Error", Data: map[string]interface{}{
				"message":          "Must attack a creature with Taunt",
				"instanceId":       atk.AttackerInstanceID,
				"targetInstanceId": atk.TargetInstanceID,
			}}}
		}

		// Tap attacker
		attacker.SetTapped(true)
		attacker.CanAttack = false

		pendingAttacks = append(pendingAttacks, PendingAttack{
			AttackerInstanceID: atk.AttackerInstanceID,
			TargetType:         atk.TargetType,
			TargetInstanceID:   atk.TargetInstanceID,
			TargetPlayerUID:    atk.TargetPlayerUID,
			BlockerInstanceID:  0,
		})
	}

	// Store combat state
	g.CombatPhase = "attackers_declared"
	g.PendingAttacks = pendingAttacks
	g.AttackingPlayer = a.PlayerUID

	// Build attacks with abilities for client
	attacksWithAbilities := []map[string]interface{}{}
	for _, pa := range pendingAttacks {
		var attackerCard Card
		for _, fc := range player.Field {
			if fc.InstanceID == pa.AttackerInstanceID {
				attackerCard = CardDB[fc.CardID]
				break
			}
		}
		attacksWithAbilities = append(attacksWithAbilities, map[string]interface{}{
			"attackerInstanceId": pa.AttackerInstanceID,
			"targetType":         pa.TargetType,
			"targetInstanceId":   pa.TargetInstanceID,
			"targetPlayerUid":    pa.TargetPlayerUID,
			"attackerAbilities":  attackerCard.Abilities,
		})
	}

	events := []Event{}

	// Emit CardTapped events
	for _, pa := range pendingAttacks {
		events = append(events, Event{
			Type: "CardTapped",
			Data: map[string]interface{}{
				"player":     a.PlayerUID,
				"instanceId": pa.AttackerInstanceID,
				"tapped":     true,
			},
		})
	}

	events = append(events, Event{
		Type: "AttacksDeclared",
		Data: map[string]interface{}{
			"player":  a.PlayerUID,
			"attacks": pendingAttacks,
		},
	})

	// Enter response window
	g.CombatPhase = "response_window"
	g.PriorityPlayer = opponentUID
	g.PassedPlayers = make(map[string]bool)

	defenderInstants := g.getInstantsInHand(opponentUID)
	attackerInstants := g.getInstantsInHand(a.PlayerUID)

	events = append(events, Event{
		Type: "ResponseWindow",
		Data: map[string]interface{}{
			"attacker":         a.PlayerUID,
			"defender":         opponentUID,
			"priorityPlayer":   opponentUID,
			"attacks":          attacksWithAbilities,
			"defenderInstants": defenderInstants,
			"attackerInstants": attackerInstants,
		},
	})

	return events
}

// getInstantsInHand returns instant cards in a player's hand
func (g *Game) getInstantsInHand(playerUID string) []map[string]interface{} {
	player := g.Players[playerUID]
	instants := []map[string]interface{}{}
	for _, cardID := range player.Hand {
		card := CardDB[cardID]
		if card.CardType == "Instant" {
			canAfford := player.ManaPool.CanAfford(card.Cost)
			instants = append(instants, map[string]interface{}{
				"cardId":    cardID,
				"name":      card.Name,
				"cost":      card.Cost,
				"canAfford": canAfford,
			})
		}
	}
	return instants
}

// playInstant handles playing an instant during combat response
func (g *Game) playInstant(a Action) []Event {
	if g.CombatPhase != "response_window" {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Not in response window"}}}
	}

	player := g.Players[a.PlayerUID]

	// Find card in hand
	cardIdx := -1
	for i, c := range player.Hand {
		if c == a.CardID {
			cardIdx = i
			break
		}
	}
	if cardIdx == -1 {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Card not in hand"}}}
	}

	card := CardDB[a.CardID]

	if card.CardType != "Instant" {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Only instants can be played during combat"}}}
	}

	if !player.ManaPool.CanAfford(card.Cost) {
		return []Event{{Type: "Error", Data: map[string]interface{}{
			"message":   "Not enough mana",
			"required":  card.Cost,
			"available": player.ManaPool,
		}}}
	}

	// Find target creature
	var targetCreature *FieldCard
	if a.InstanceID != 0 {
		for _, p := range g.Players {
			for _, fc := range p.Field {
				if fc.InstanceID == a.InstanceID {
					targetCreature = fc
					break
				}
			}
		}
		if targetCreature == nil {
			return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Target creature not found"}}}
		}
	}

	// Spend mana and remove from hand
	player.ManaPool.Spend(card.Cost)
	player.Hand = append(player.Hand[:cardIdx], player.Hand[cardIdx+1:]...)
	player.Discard = append(player.Discard, a.CardID)

	events := []Event{
		{
			Type: "InstantPlayed",
			Data: map[string]interface{}{
				"player":           a.PlayerUID,
				"cardId":           a.CardID,
				"targetInstanceId": a.InstanceID,
				"manaPool":         player.ManaPool,
			},
		},
	}

	// Execute script
	if card.CustomScript != "" {
		ctx := &ScriptContext{
			Game:      g,
			Card:      nil,
			Caster:    player,
			CasterUID: a.PlayerUID,
			Target:    targetCreature,
		}
		scriptEvents := ExecuteScript(card.CustomScript, ctx)
		events = append(events, scriptEvents...)

		for uid, p := range g.Players {
			events = append(events, g.handleDeaths(p, uid)...)
		}
	}

	// Reset passes and switch priority
	g.PassedPlayers = make(map[string]bool)
	for uid := range g.Players {
		if uid != a.PlayerUID {
			g.PriorityPlayer = uid
			break
		}
	}

	events = append(events, Event{
		Type: "PriorityChanged",
		Data: map[string]interface{}{
			"priorityPlayer": g.PriorityPlayer,
		},
	})

	return events
}

// passPriority handles passing during response window
func (g *Game) passPriority(a Action) []Event {
	if g.CombatPhase != "response_window" {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Not in response window"}}}
	}

	g.PassedPlayers[a.PlayerUID] = true

	// Check if both passed
	allPassed := true
	for uid := range g.Players {
		if !g.PassedPlayers[uid] {
			allPassed = false
			break
		}
	}

	if allPassed {
		return g.resolveCombat()
	}

	// Switch priority
	for uid := range g.Players {
		if uid != a.PlayerUID {
			g.PriorityPlayer = uid
			break
		}
	}

	return []Event{
		{
			Type: "PlayerPassed",
			Data: map[string]interface{}{"player": a.PlayerUID},
		},
		{
			Type: "PriorityChanged",
			Data: map[string]interface{}{"priorityPlayer": g.PriorityPlayer},
		},
	}
}

// resolveCombat resolves all pending attacks
func (g *Game) resolveCombat() []Event {
	events := []Event{{
		Type: "CombatResolving",
		Data: map[string]interface{}{},
	}}

	attackerPlayer := g.Players[g.AttackingPlayer]

	var defenderUID string
	for uid := range g.Players {
		if uid != g.AttackingPlayer {
			defenderUID = uid
			break
		}
	}
	defender := g.Players[defenderUID]

	// Resolve each attack
	for _, pa := range g.PendingAttacks {
		var attackerCreature *FieldCard
		for _, fc := range attackerPlayer.Field {
			if fc.InstanceID == pa.AttackerInstanceID {
				attackerCreature = fc
				break
			}
		}
		if attackerCreature == nil || attackerCreature.IsDead() {
			continue
		}

		attackerCard := CardDB[attackerCreature.CardID]
		attackerDamage := attackerCreature.GetAttack()
		attackerHasFirstStrike := attackerCard.HasAbility("FirstStrike") || attackerCard.HasAbility("DoubleStrike")
		attackerHasDoubleStrike := attackerCard.HasAbility("DoubleStrike")

		if pa.TargetType == "player" {
			defender.Life -= attackerDamage
			events = append(events, Event{
				Type: "Damage",
				Data: map[string]interface{}{
					"target": pa.TargetPlayerUID,
					"amount": attackerDamage,
					"source": attackerCreature.InstanceID,
				},
			})
			if attackerHasDoubleStrike {
				defender.Life -= attackerDamage
				events = append(events, Event{
					Type: "Damage",
					Data: map[string]interface{}{
						"target":       pa.TargetPlayerUID,
						"amount":       attackerDamage,
						"source":       attackerCreature.InstanceID,
						"doubleStrike": true,
					},
				})
			}
		} else if pa.TargetType == "creature" {
			var targetCreature *FieldCard
			for _, fc := range defender.Field {
				if fc.InstanceID == pa.TargetInstanceID {
					targetCreature = fc
					break
				}
			}
			if targetCreature == nil || targetCreature.IsDead() {
				continue
			}

			targetCard := CardDB[targetCreature.CardID]
			targetDamage := targetCreature.GetAttack()
			targetHasFirstStrike := targetCard.HasAbility("FirstStrike") || targetCard.HasAbility("DoubleStrike")
			targetHasDoubleStrike := targetCard.HasAbility("DoubleStrike")

			events = append(events, resolveCombatDamage(
				attackerCreature, attackerDamage, attackerHasFirstStrike, attackerHasDoubleStrike,
				targetCreature, targetDamage, targetHasFirstStrike, targetHasDoubleStrike,
			)...)
		}
	}

	// Handle deaths and game over
	events = append(events, g.handleDeaths(attackerPlayer, g.AttackingPlayer)...)
	events = append(events, g.handleDeaths(defender, defenderUID)...)

	if defender.Life <= 0 {
		g.Winner = g.AttackingPlayer
		events = append(events, Event{
			Type: "GameOver",
			Data: map[string]interface{}{"winner": g.AttackingPlayer},
		})
	}
	if attackerPlayer.Life <= 0 {
		g.Winner = defenderUID
		events = append(events, Event{
			Type: "GameOver",
			Data: map[string]interface{}{"winner": defenderUID},
		})
	}

	// Clear combat state
	g.CombatPhase = ""
	g.PendingAttacks = nil
	g.AttackingPlayer = ""
	g.PriorityPlayer = ""
	g.PassedPlayers = nil

	events = append(events, Event{
		Type: "CombatEnded",
		Data: map[string]interface{}{},
	})

	return events
}

// resolveCombatDamage handles damage between two creatures with FirstStrike/DoubleStrike
func resolveCombatDamage(
	creature1 *FieldCard, damage1 int, hasFirstStrike1 bool, hasDoubleStrike1 bool,
	creature2 *FieldCard, damage2 int, hasFirstStrike2 bool, hasDoubleStrike2 bool,
) []Event {
	events := []Event{}

	// First Strike phase
	if hasFirstStrike1 && hasFirstStrike2 {
		creature1.CurrentHealth -= damage2
		creature2.CurrentHealth -= damage1
		events = append(events, Event{
			Type: "CombatDamage",
			Data: map[string]interface{}{
				"attackerInstanceId": creature1.InstanceID,
				"targetType":         "creature",
				"targetInstanceId":   creature2.InstanceID,
				"damage":             damage1,
				"firstStrike":        true,
			},
		})
		events = append(events, Event{
			Type: "CombatDamage",
			Data: map[string]interface{}{
				"attackerInstanceId": creature2.InstanceID,
				"targetType":         "creature",
				"targetInstanceId":   creature1.InstanceID,
				"damage":             damage2,
				"firstStrike":        true,
			},
		})
	} else if hasFirstStrike1 && !hasFirstStrike2 {
		creature2.CurrentHealth -= damage1
		events = append(events, Event{
			Type: "CombatDamage",
			Data: map[string]interface{}{
				"attackerInstanceId": creature1.InstanceID,
				"targetType":         "creature",
				"targetInstanceId":   creature2.InstanceID,
				"damage":             damage1,
				"firstStrike":        true,
			},
		})
	} else if hasFirstStrike2 && !hasFirstStrike1 {
		creature1.CurrentHealth -= damage2
		events = append(events, Event{
			Type: "CombatDamage",
			Data: map[string]interface{}{
				"attackerInstanceId": creature2.InstanceID,
				"targetType":         "creature",
				"targetInstanceId":   creature1.InstanceID,
				"damage":             damage2,
				"firstStrike":        true,
			},
		})
	}

	// Normal damage phase
	creature1DealsNormal := (!hasFirstStrike1 || hasDoubleStrike1) && !creature1.IsDead()
	creature2DealsNormal := (!hasFirstStrike2 || hasDoubleStrike2) && !creature2.IsDead()

	if creature1DealsNormal && creature2DealsNormal {
		creature1.CurrentHealth -= damage2
		creature2.CurrentHealth -= damage1
		events = append(events, Event{
			Type: "CombatDamage",
			Data: map[string]interface{}{
				"attackerInstanceId": creature1.InstanceID,
				"targetType":         "creature",
				"targetInstanceId":   creature2.InstanceID,
				"damage":             damage1,
				"doubleStrike":       hasDoubleStrike1,
			},
		})
		events = append(events, Event{
			Type: "CombatDamage",
			Data: map[string]interface{}{
				"attackerInstanceId": creature2.InstanceID,
				"targetType":         "creature",
				"targetInstanceId":   creature1.InstanceID,
				"damage":             damage2,
				"doubleStrike":       hasDoubleStrike2,
			},
		})
	} else if creature1DealsNormal {
		creature2.CurrentHealth -= damage1
		events = append(events, Event{
			Type: "CombatDamage",
			Data: map[string]interface{}{
				"attackerInstanceId": creature1.InstanceID,
				"targetType":         "creature",
				"targetInstanceId":   creature2.InstanceID,
				"damage":             damage1,
				"doubleStrike":       hasDoubleStrike1,
			},
		})
	} else if creature2DealsNormal {
		creature1.CurrentHealth -= damage2
		events = append(events, Event{
			Type: "CombatDamage",
			Data: map[string]interface{}{
				"attackerInstanceId": creature2.InstanceID,
				"targetType":         "creature",
				"targetInstanceId":   creature1.InstanceID,
				"damage":             damage2,
				"doubleStrike":       hasDoubleStrike2,
			},
		})
	}

	return events
}

// handleDeaths removes dead creatures from field
func (g *Game) handleDeaths(p *Player, playerUID string) []Event {
	events := []Event{}
	alive := []*FieldCard{}
	for _, fc := range p.Field {
		card := CardDB[fc.CardID]
		if card.CardType == "Creature" && fc.IsDead() {
			p.Discard = append(p.Discard, fc.CardID)
			events = append(events, Event{
				Type: "CreatureDied",
				Data: map[string]interface{}{
					"player":     playerUID,
					"instanceId": fc.InstanceID,
					"cardId":     fc.CardID,
				},
			})
		} else {
			alive = append(alive, fc)
		}
	}
	p.Field = alive
	return events
}
