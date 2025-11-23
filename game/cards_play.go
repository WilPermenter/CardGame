// cards_play.go - Card playing, tapping, burning
package game

// playCard handles playing a card from hand
func (g *Game) playCard(a Action) []Event {
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

	// Check mana cost (lands are free)
	if card.CardType != "Land" && card.Cost.Total() > 0 {
		if !player.ManaPool.CanAfford(card.Cost) {
			return []Event{{Type: "Error", Data: map[string]interface{}{
				"message":   "Not enough mana in pool",
				"required":  card.Cost,
				"available": player.ManaPool,
			}}}
		}
		player.ManaPool.Spend(card.Cost)
	}

	// Remove from hand
	player.Hand = append(player.Hand[:cardIdx], player.Hand[cardIdx+1:]...)

	events := []Event{}

	switch card.CardType {
	case "Creature":
		fieldCard := g.NewFieldCard(a.CardID, a.PlayerUID, a.PlayerUID)
		player.Field = append(player.Field, fieldCard)

		events = append(events, Event{
			Type: "CreaturePlayed",
			Data: map[string]interface{}{
				"player":     a.PlayerUID,
				"cardId":     a.CardID,
				"instanceId": fieldCard.InstanceID,
				"fieldCard":  fieldCard,
				"manaPool":   player.ManaPool,
			},
		})

		// Execute ETB script
		if card.CustomScript != "" {
			ctx := &ScriptContext{
				Game:      g,
				Card:      fieldCard,
				Caster:    player,
				CasterUID: a.PlayerUID,
			}
			scriptEvents := ExecuteScript(card.CustomScript, ctx)
			events = append(events, scriptEvents...)

			for uid, p := range g.Players {
				events = append(events, g.handleDeaths(p, uid)...)
			}
		}

	case "Land":
		if player.LandsPlayedThisTurn >= player.LandsPerTurn {
			player.Hand = append(player.Hand, a.CardID)
			return []Event{{Type: "Error", Data: map[string]interface{}{
				"message":      "Already played max lands this turn",
				"landsPlayed":  player.LandsPlayedThisTurn,
				"landsPerTurn": player.LandsPerTurn,
			}}}
		}

		fieldCard := g.NewFieldCard(a.CardID, a.PlayerUID, a.PlayerUID)
		player.Field = append(player.Field, fieldCard)
		player.LandsPlayedThisTurn++

		events = append(events, Event{
			Type: "LandPlayed",
			Data: map[string]interface{}{
				"player":              a.PlayerUID,
				"cardId":              a.CardID,
				"instanceId":          fieldCard.InstanceID,
				"fieldCard":           fieldCard,
				"landsPlayedThisTurn": player.LandsPlayedThisTurn,
				"landsPerTurn":        player.LandsPerTurn,
			},
		})

	default:
		// Spells go to discard
		player.Discard = append(player.Discard, a.CardID)

		events = append(events, Event{
			Type: "CardPlayed",
			Data: map[string]interface{}{
				"player":   a.PlayerUID,
				"cardId":   a.CardID,
				"manaPool": player.ManaPool,
			},
		})

		// Execute spell script
		if card.CustomScript != "" {
			ctx := &ScriptContext{
				Game:      g,
				Card:      nil,
				Caster:    player,
				CasterUID: a.PlayerUID,
			}
			scriptEvents := ExecuteScript(card.CustomScript, ctx)
			events = append(events, scriptEvents...)

			for uid, p := range g.Players {
				events = append(events, g.handleDeaths(p, uid)...)
			}

			// Check for game over
			for uid, p := range g.Players {
				if p.Life <= 0 && g.Winner == "" {
					for otherUID := range g.Players {
						if otherUID != uid {
							g.Winner = otherUID
							events = append(events, Event{
								Type: "GameOver",
								Data: map[string]interface{}{"winner": otherUID},
							})
							break
						}
					}
				}
			}
		}
	}

	return events
}

// playLeader handles playing the leader card
func (g *Game) playLeader(a Action) []Event {
	player := g.Players[a.PlayerUID]

	if player.Leader == 0 {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "No leader to play or already played"}}}
	}

	card := CardDB[player.Leader]

	if card.Cost.Total() > 0 {
		if !player.ManaPool.CanAfford(card.Cost) {
			return []Event{{Type: "Error", Data: map[string]interface{}{
				"message":   "Not enough mana in pool",
				"required":  card.Cost,
				"available": player.ManaPool,
			}}}
		}
		player.ManaPool.Spend(card.Cost)
	}

	fieldCard := g.NewFieldCard(player.Leader, a.PlayerUID, a.PlayerUID)
	player.Field = append(player.Field, fieldCard)

	leaderID := player.Leader
	player.Leader = 0

	events := []Event{
		{
			Type: "LeaderPlayed",
			Data: map[string]interface{}{
				"player":     a.PlayerUID,
				"cardId":     leaderID,
				"instanceId": fieldCard.InstanceID,
				"fieldCard":  fieldCard,
				"manaPool":   player.ManaPool,
			},
		},
	}

	if card.CustomScript != "" {
		ctx := &ScriptContext{
			Game:      g,
			Card:      fieldCard,
			Caster:    player,
			CasterUID: a.PlayerUID,
		}
		scriptEvents := ExecuteScript(card.CustomScript, ctx)
		events = append(events, scriptEvents...)

		for uid, p := range g.Players {
			events = append(events, g.handleDeaths(p, uid)...)
		}
	}

	return events
}

// tapCard handles tapping a card for mana
func (g *Game) tapCard(a Action) []Event {
	player := g.Players[a.PlayerUID]

	var targetCard *FieldCard
	for _, fc := range player.Field {
		if fc.InstanceID == a.InstanceID {
			targetCard = fc
			break
		}
	}

	if targetCard == nil {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Card not found on field"}}}
	}

	if targetCard.IsTapped() {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Card is already tapped"}}}
	}

	targetCard.SetTapped(true)

	events := []Event{
		{
			Type: "CardTapped",
			Data: map[string]interface{}{
				"player":     a.PlayerUID,
				"instanceId": a.InstanceID,
				"tapped":     true,
			},
		},
	}

	// Add mana if it's a land
	card := CardDB[targetCard.CardID]
	if card.CardType == "Land" {
		provided := card.GetProvidedMana()
		player.ManaPool.White += provided.White
		player.ManaPool.Blue += provided.Blue
		player.ManaPool.Black += provided.Black
		player.ManaPool.Red += provided.Red
		player.ManaPool.Green += provided.Green
		player.ManaPool.Colorless += provided.Colorless

		events = append(events, Event{
			Type: "ManaAdded",
			Data: map[string]interface{}{
				"player":   a.PlayerUID,
				"added":    provided,
				"manaPool": player.ManaPool,
			},
		})
	}

	return events
}

// burnCard handles burning a land from hand for mana
func (g *Game) burnCard(a Action) []Event {
	player := g.Players[a.PlayerUID]

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

	if card.CardType != "Land" {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Only lands can be burned for mana"}}}
	}

	player.Hand = append(player.Hand[:cardIdx], player.Hand[cardIdx+1:]...)
	player.Discard = append(player.Discard, a.CardID)

	provided := card.GetProvidedMana()
	player.ManaPool.White += provided.White
	player.ManaPool.Blue += provided.Blue
	player.ManaPool.Black += provided.Black
	player.ManaPool.Red += provided.Red
	player.ManaPool.Green += provided.Green
	player.ManaPool.Colorless += provided.Colorless

	return []Event{
		{
			Type: "CardBurned",
			Data: map[string]interface{}{
				"player": a.PlayerUID,
				"cardId": a.CardID,
			},
		},
		{
			Type: "ManaAdded",
			Data: map[string]interface{}{
				"player":   a.PlayerUID,
				"added":    provided,
				"manaPool": player.ManaPool,
			},
		},
	}
}
