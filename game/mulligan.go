// mulligan.go - Mulligan phase logic
package game

// keepHand - player keeps their current hand during mulligan phase
func (g *Game) keepHand(a Action) []Event {
	if g.MulliganDecisions[a.PlayerUID] {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Already made mulligan decision"}}}
	}

	g.MulliganDecisions[a.PlayerUID] = true

	events := []Event{
		{
			Type: "PlayerKeptHand",
			Data: map[string]interface{}{"player": a.PlayerUID},
		},
	}

	events = append(events, g.checkMulliganComplete()...)
	return events
}

// takeMulligan - player shuffles hand back and draws a new one
func (g *Game) takeMulligan(a Action) []Event {
	if g.MulliganDecisions[a.PlayerUID] {
		return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Already made mulligan decision"}}}
	}

	player := g.Players[a.PlayerUID]

	// Separate hand into lands (vault) and non-lands (main deck)
	for _, cardID := range player.Hand {
		card := CardDB[cardID]
		if card.CardType == "Land" {
			player.VaultPile = append(player.VaultPile, cardID)
		} else {
			player.DrawPile = append(player.DrawPile, cardID)
		}
	}
	player.Hand = []int{}

	// Shuffle both piles
	player.DrawPile = ShuffleDeck(player.DrawPile)
	player.VaultPile = ShuffleDeck(player.VaultPile)

	// Draw new hand
	player.DrawCards(InitialMainDeckDraw)
	player.DrawFromVault(InitialVaultDraw)

	g.MulliganDecisions[a.PlayerUID] = true

	events := []Event{
		{
			Type: "PlayerMulliganed",
			Data: map[string]interface{}{
				"player":    a.PlayerUID,
				"newHand":   player.Hand,
				"deckSize":  len(player.DrawPile),
				"vaultSize": len(player.VaultPile),
			},
		},
	}

	events = append(events, g.checkMulliganComplete()...)
	return events
}

// checkMulliganComplete checks if both players decided and starts the game
func (g *Game) checkMulliganComplete() []Event {
	for uid := range g.Players {
		if !g.MulliganDecisions[uid] {
			return []Event{}
		}
	}

	// Both decided - start the game
	g.MulliganPhase = false
	g.Started = true
	g.DrawPhase = true

	playersInfo := make(map[string]interface{})
	for uid, player := range g.Players {
		playersInfo[uid] = map[string]interface{}{
			"hand":        player.Hand,
			"leader":      player.Leader,
			"deckSize":    len(player.DrawPile),
			"vaultSize":   len(player.VaultPile),
			"discardSize": len(player.Discard),
		}
	}

	activePlayer := g.Players[g.Turn]

	return []Event{
		{
			Type: "GameStarted",
			Data: map[string]interface{}{
				"gameId":      g.ID,
				"players":     playersInfo,
				"currentTurn": g.Turn,
			},
		},
		{
			Type: "DrawPhase",
			Data: map[string]interface{}{
				"player":       g.Turn,
				"mainDeckSize": len(activePlayer.DrawPile),
				"vaultSize":    len(activePlayer.VaultPile),
			},
		},
	}
}
