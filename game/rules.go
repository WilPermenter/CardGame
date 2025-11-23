package game

import "time"

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

// checkPriority determines if a player can take an action right now
func (g *Game) checkPriority(playerUID string, actionType string) bool {
    // During response window, only the priority player can act
    if g.CombatPhase == "response_window" {
        // Only priority player can play instants or pass
        if actionType == "play_instant" || actionType == "pass_priority" {
            return playerUID == g.PriorityPlayer
        }
        // No other actions allowed during response window
        return false
    }

    // During combat (attackers declared but before response), special rules
    if g.CombatPhase == "attackers_declared" {
        // Only the defender can declare blockers (disabled)
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

func (g *Game) HandleAction(a Action) []Event {
    // Update activity timestamp for cleanup tracking
    g.LastActivity = time.Now()

    // Game already over?
    if g.Winner != "" {
        return []Event{{Type: "GameOver", Data: map[string]interface{}{"winner": g.Winner}}}
    }

    // Handle mulligan phase actions before game starts
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

    // Game not started yet?
    if !g.Started {
        return []Event{{Type: "GameNotStarted", Data: map[string]interface{}{}}}
    }

    // Handle draw phase - must draw before taking other actions
    if g.DrawPhase && a.PlayerUID == g.Turn {
        if a.Type == "draw_card" {
            return g.drawCardAction(a)
        }
        return []Event{{Type: "MustDraw", Data: map[string]interface{}{"message": "Must draw a card first"}}}
    }

    // Priority check - who can act right now?
    hasPriority := g.checkPriority(a.PlayerUID, a.Type)
    if !hasPriority {
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

    // DISABLED - blocking removed
    // case "declare_blockers":
    //     return g.declareBlockers(a)

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
                Data: map[string]interface{}{
                    "action": a.Type,
                },
            },
        }
    }
}

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

    // Check if player can afford the card (lands are free to play)
    if card.CardType != "Land" && card.Cost.Total() > 0 {
        if !player.ManaPool.CanAfford(card.Cost) {
            return []Event{{Type: "Error", Data: map[string]interface{}{
                "message":   "Not enough mana in pool",
                "required":  card.Cost,
                "available": player.ManaPool,
            }}}
        }
        // Spend the mana
        player.ManaPool.Spend(card.Cost)
    }

    // Remove from hand
    player.Hand = append(player.Hand[:cardIdx], player.Hand[cardIdx+1:]...)

    events := []Event{}

    // Handle based on card type
    switch card.CardType {
    case "Creature":
        // Add creature to field
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

        // Execute ETB (Enter The Battlefield) script if present
        if card.CustomScript != "" {
            ctx := &ScriptContext{
                Game:      g,
                Card:      fieldCard,
                Caster:    player,
                CasterUID: a.PlayerUID,
            }
            scriptEvents := ExecuteScript(card.CustomScript, ctx)
            events = append(events, scriptEvents...)

            // Handle any deaths caused by script effects
            for uid, p := range g.Players {
                events = append(events, g.handleDeaths(p, uid)...)
            }
        }

    case "Land":
        // Check if player can play more lands this turn
        if player.LandsPlayedThisTurn >= player.LandsPerTurn {
            // Put card back in hand since we already removed it
            player.Hand = append(player.Hand, a.CardID)
            return []Event{{Type: "Error", Data: map[string]interface{}{
                "message":      "Already played max lands this turn",
                "landsPlayed":  player.LandsPlayedThisTurn,
                "landsPerTurn": player.LandsPerTurn,
            }}}
        }

        // Lands go to field too (for mana, later)
        fieldCard := g.NewFieldCard(a.CardID, a.PlayerUID, a.PlayerUID)
        player.Field = append(player.Field, fieldCard)
        player.LandsPlayedThisTurn++

        events = append(events, Event{
            Type: "LandPlayed",
            Data: map[string]interface{}{
                "player":           a.PlayerUID,
                "cardId":           a.CardID,
                "instanceId":       fieldCard.InstanceID,
                "fieldCard":        fieldCard,
                "landsPlayedThisTurn": player.LandsPlayedThisTurn,
                "landsPerTurn":     player.LandsPerTurn,
            },
        })

    default:
        // Spells and other cards go to discard after use
        player.Discard = append(player.Discard, a.CardID)

        events = append(events, Event{
            Type: "CardPlayed",
            Data: map[string]interface{}{
                "player":   a.PlayerUID,
                "cardId":   a.CardID,
                "manaPool": player.ManaPool,
            },
        })

        // Execute spell script if present
        if card.CustomScript != "" {
            ctx := &ScriptContext{
                Game:      g,
                Card:      nil, // Spells don't stay on field
                Caster:    player,
                CasterUID: a.PlayerUID,
            }
            scriptEvents := ExecuteScript(card.CustomScript, ctx)
            events = append(events, scriptEvents...)

            // Handle any deaths caused by script effects
            for uid, p := range g.Players {
                events = append(events, g.handleDeaths(p, uid)...)
            }

            // Check for game over from script damage
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

func (g *Game) playLeader(a Action) []Event {
    player := g.Players[a.PlayerUID]

    // Check if player has a leader to play
    if player.Leader == 0 {
        return []Event{{Type: "Error", Data: map[string]interface{}{"message": "No leader to play or already played"}}}
    }

    card := CardDB[player.Leader]

    // Check if player can afford the card
    if card.Cost.Total() > 0 {
        if !player.ManaPool.CanAfford(card.Cost) {
            return []Event{{Type: "Error", Data: map[string]interface{}{
                "message":   "Not enough mana in pool",
                "required":  card.Cost,
                "available": player.ManaPool,
            }}}
        }
        // Spend the mana
        player.ManaPool.Spend(card.Cost)
    }

    // Add leader to field as a creature
    fieldCard := g.NewFieldCard(player.Leader, a.PlayerUID, a.PlayerUID)
    player.Field = append(player.Field, fieldCard)

    // Clear the leader (can only be played once)
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

    // Execute ETB (Enter The Battlefield) script if present
    if card.CustomScript != "" {
        ctx := &ScriptContext{
            Game:      g,
            Card:      fieldCard,
            Caster:    player,
            CasterUID: a.PlayerUID,
        }
        scriptEvents := ExecuteScript(card.CustomScript, ctx)
        events = append(events, scriptEvents...)

        // Handle any deaths caused by script effects
        for uid, p := range g.Players {
            events = append(events, g.handleDeaths(p, uid)...)
        }
    }

    return events
}

func (g *Game) endTurn(a Action) []Event {
    events := []Event{}

    // End step: Untap Vigilance creatures for the ending player
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

    // Switch turn to the other player
    for uid := range g.Players {
        if uid != g.Turn {
            g.Turn = uid
            break
        }
    }

    activePlayer := g.Players[g.Turn]

    // Untap all cards, clear summoning sickness, and clear mana pool at start of turn
    for _, fc := range activePlayer.Field {
        fc.SetTapped(false)
        fc.CanAttack = true
        fc.Status["Summoned"] = 0 // Clear summoning sickness
    }
    activePlayer.ManaPool.Clear()
    activePlayer.LandsPlayedThisTurn = 0 // Reset lands played counter

    events = append(events, Event{
        Type: "TurnChanged",
        Data: map[string]interface{}{
            "activePlayer": g.Turn,
            "manaPool":     activePlayer.ManaPool,
        },
    })

    // Enter draw phase - player must choose to draw from main deck or vault
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

func (g *Game) drawCard(p *Player) (int, bool) {
    if len(p.DrawPile) == 0 {
        return 0, false // No cards left to draw
    }
    drawn := p.DrawCards(1)
    return drawn[0], true
}

func (g *Game) drawCardAction(a Action) []Event {
    player := g.Players[a.PlayerUID]
    events := []Event{}

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

    events = append(events, Event{
        Type: "CardDrawn",
        Data: map[string]interface{}{
            "player":       a.PlayerUID,
            "cardId":       cardDrawn,
            "source":       a.Source,
            "mainDeckSize": len(player.DrawPile),
            "vaultSize":    len(player.VaultPile),
        },
    })

    return events
}

func (g *Game) tapCard(a Action) []Event {
    player := g.Players[a.PlayerUID]

    // Find the field card by instance ID
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

    // Can't tap already tapped cards
    if targetCard.IsTapped() {
        return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Card is already tapped"}}}
    }

    // Tap the card
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

    // If it's a land, add mana to pool
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

func (g *Game) burnCard(a Action) []Event {
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

    // Only lands can be burned for mana
    if card.CardType != "Land" {
        return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Only lands can be burned for mana"}}}
    }

    // Remove from hand and add to discard
    player.Hand = append(player.Hand[:cardIdx], player.Hand[cardIdx+1:]...)
    player.Discard = append(player.Discard, a.CardID)

    // Add mana to pool
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

func (g *Game) declareAttacks(a Action) []Event {
    if len(a.Attacks) == 0 {
        return []Event{{Type: "Error", Data: map[string]interface{}{"message": "No attacks declared"}}}
    }

    // Can't declare attacks if combat is already in progress
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

    // Check for Taunt creatures on opponent's field
    tauntCreatures := getUntappedTaunts(opponent)
    hasTaunts := len(tauntCreatures) > 0
    tauntIDs := make(map[int]bool)
    for _, fc := range tauntCreatures {
        tauntIDs[fc.InstanceID] = true
    }

    // Validate all attacks and build pending attacks
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

        // Check if attacker can attack
        if attacker.IsTapped() {
            return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Attacker is tapped", "instanceId": atk.AttackerInstanceID}}}
        }
        if attacker.IsSummoned() {
            return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Attacker has summoning sickness", "instanceId": atk.AttackerInstanceID}}}
        }

        card := CardDB[attacker.CardID]

        // Validate target based on ValidAttackTargets
        validTargets := card.ValidAttackTargets
        if validTargets == "" {
            validTargets = "Any"
        }

        if atk.TargetType == "creature" {
            if validTargets == "Player" {
                return []Event{{Type: "Error", Data: map[string]interface{}{"message": "This creature can only attack players", "instanceId": atk.AttackerInstanceID}}}
            }
            // Find target creature on opponent's field
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
            // Taunt check: If opponent has untapped Taunt creatures, cannot attack the player directly
            if hasTaunts {
                return []Event{{Type: "Error", Data: map[string]interface{}{
                    "message":    "Cannot attack player while opponent has Taunt creatures",
                    "instanceId": atk.AttackerInstanceID,
                }}}
            }
        } else {
            return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Invalid target type", "targetType": atk.TargetType}}}
        }

        // Taunt check: If opponent has untapped Taunt creatures, can only attack Taunt creatures
        if hasTaunts && atk.TargetType == "creature" && !tauntIDs[atk.TargetInstanceID] {
            return []Event{{Type: "Error", Data: map[string]interface{}{
                "message":          "Must attack a creature with Taunt",
                "instanceId":       atk.AttackerInstanceID,
                "targetInstanceId": atk.TargetInstanceID,
            }}}
        }

        // Tap attacker immediately (will emit event later)
        attacker.SetTapped(true)
        attacker.CanAttack = false

        pendingAttacks = append(pendingAttacks, PendingAttack{
            AttackerInstanceID: atk.AttackerInstanceID,
            TargetType:         atk.TargetType,
            TargetInstanceID:   atk.TargetInstanceID,
            TargetPlayerUID:    atk.TargetPlayerUID,
            BlockerInstanceID:  0, // No blocker yet
        })
    }

    // Store combat state
    g.CombatPhase = "attackers_declared"
    g.PendingAttacks = pendingAttacks
    g.AttackingPlayer = a.PlayerUID

    // Build list of creatures that can block (untapped, not being attacked)
    availableBlockers := []map[string]interface{}{}
    for _, fc := range opponent.Field {
        card := CardDB[fc.CardID]
        if card.CardType != "Creature" {
            continue
        }
        if fc.IsTapped() {
            continue
        }
        // Check if this creature is being attacked
        isBeingAttacked := false
        for _, pa := range pendingAttacks {
            if pa.TargetType == "creature" && pa.TargetInstanceID == fc.InstanceID {
                isBeingAttacked = true
                break
            }
        }
        if !isBeingAttacked {
            availableBlockers = append(availableBlockers, map[string]interface{}{
                "instanceId": fc.InstanceID,
                "cardId":     fc.CardID,
                "abilities":  card.Abilities,
            })
        }
    }

    // Add attacker abilities to pending attacks for client-side filtering
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

    // Emit CardTapped events for each attacker
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

    // Enter response window - defender gets priority first
    g.CombatPhase = "response_window"
    g.PriorityPlayer = opponentUID
    g.PassedPlayers = make(map[string]bool)

    // Build list of instant cards in each player's hand
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

// getInstantsInHand returns a list of instant cards in a player's hand
func (g *Game) getInstantsInHand(playerUID string) []map[string]interface{} {
    player := g.Players[playerUID]
    instants := []map[string]interface{}{}
    for _, cardID := range player.Hand {
        card := CardDB[cardID]
        if card.CardType == "Instant" {
            // Check if player can afford the card
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

// playInstant handles playing an instant during the response window
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

    // Must be an instant
    if card.CardType != "Instant" {
        return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Only instants can be played during combat"}}}
    }

    // Check if player can afford the card
    if !player.ManaPool.CanAfford(card.Cost) {
        return []Event{{Type: "Error", Data: map[string]interface{}{
            "message":   "Not enough mana",
            "required":  card.Cost,
            "available": player.ManaPool,
        }}}
    }

    // Find target creature if targeting is needed
    var targetCreature *FieldCard
    if a.InstanceID != 0 {
        // Find the target creature on either player's field
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

    // Spend the mana
    player.ManaPool.Spend(card.Cost)

    // Remove from hand
    player.Hand = append(player.Hand[:cardIdx], player.Hand[cardIdx+1:]...)

    // Add to discard
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

    // Execute the instant's script
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

        // Handle any deaths caused by the instant
        for uid, p := range g.Players {
            events = append(events, g.handleDeaths(p, uid)...)
        }
    }

    // Reset passed players since an action was taken
    g.PassedPlayers = make(map[string]bool)

    // Switch priority to other player
    for uid := range g.Players {
        if uid != a.PlayerUID {
            g.PriorityPlayer = uid
            break
        }
    }

    // Send updated response window state
    events = append(events, Event{
        Type: "PriorityChanged",
        Data: map[string]interface{}{
            "priorityPlayer": g.PriorityPlayer,
        },
    })

    return events
}

// passPriority handles a player passing priority during the response window
func (g *Game) passPriority(a Action) []Event {
    if g.CombatPhase != "response_window" {
        return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Not in response window"}}}
    }

    // Mark this player as passed
    g.PassedPlayers[a.PlayerUID] = true

    // Check if both players have passed consecutively
    allPassed := true
    for uid := range g.Players {
        if !g.PassedPlayers[uid] {
            allPassed = false
            break
        }
    }

    if allPassed {
        // Both players passed - resolve combat
        return g.resolveCombat()
    }

    // Switch priority to other player
    for uid := range g.Players {
        if uid != a.PlayerUID {
            g.PriorityPlayer = uid
            break
        }
    }

    return []Event{
        {
            Type: "PlayerPassed",
            Data: map[string]interface{}{
                "player": a.PlayerUID,
            },
        },
        {
            Type: "PriorityChanged",
            Data: map[string]interface{}{
                "priorityPlayer": g.PriorityPlayer,
            },
        },
    }
}

// resolveCombat resolves all pending attacks after the response window closes
func (g *Game) resolveCombat() []Event {
    events := []Event{{
        Type: "CombatResolving",
        Data: map[string]interface{}{},
    }}

    attackerPlayer := g.Players[g.AttackingPlayer]

    // Find defender
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
            continue // Attacker died during response window
        }

        attackerCard := CardDB[attackerCreature.CardID]
        attackerDamage := attackerCreature.GetAttack()
        attackerHasFirstStrike := attackerCard.HasAbility("FirstStrike") || attackerCard.HasAbility("DoubleStrike")
        attackerHasDoubleStrike := attackerCard.HasAbility("DoubleStrike")

        if pa.TargetType == "player" {
            // Damage to player
            defender.Life -= attackerDamage
            events = append(events, Event{
                Type: "Damage",
                Data: map[string]interface{}{
                    "target": pa.TargetPlayerUID,
                    "amount": attackerDamage,
                    "source": attackerCreature.InstanceID,
                },
            })
            // DoubleStrike deals damage twice
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
            // Find target creature
            var targetCreature *FieldCard
            for _, fc := range defender.Field {
                if fc.InstanceID == pa.TargetInstanceID {
                    targetCreature = fc
                    break
                }
            }
            if targetCreature == nil || targetCreature.IsDead() {
                // Target died during response window - attack hits player instead? Or fizzles?
                // For now, attack fizzles
                continue
            }

            targetCard := CardDB[targetCreature.CardID]
            targetDamage := targetCreature.GetAttack()
            targetHasFirstStrike := targetCard.HasAbility("FirstStrike") || targetCard.HasAbility("DoubleStrike")
            targetHasDoubleStrike := targetCard.HasAbility("DoubleStrike")

            // Resolve damage based on FirstStrike/DoubleStrike
            events = append(events, resolveCombatDamage(
                attackerCreature, attackerDamage, attackerHasFirstStrike, attackerHasDoubleStrike,
                targetCreature, targetDamage, targetHasFirstStrike, targetHasDoubleStrike,
            )...)
        }
    }

    // Handle deaths
    events = append(events, g.handleDeaths(attackerPlayer, g.AttackingPlayer)...)
    events = append(events, g.handleDeaths(defender, defenderUID)...)

    // Check for game over
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

// DISABLED - declareBlockers (blocking removed)
// This function is kept for reference but is no longer called.
// Combat is now resolved immediately in declareAttacks.
func (g *Game) declareBlockers(a Action) []Event {
    // BLOCKING DISABLED - this function is no longer used
    return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Blocking has been disabled"}}}

    /* Original blocking code kept for reference:
    // Must be in attackers_declared phase
    if g.CombatPhase != "attackers_declared" {
        return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Not in blocking phase"}}}
    }

    // Only the defender can declare blockers
    var defenderUID string
    for uid := range g.Players {
        if uid != g.AttackingPlayer {
            defenderUID = uid
            break
        }
    }
    if a.PlayerUID != defenderUID {
        return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Only the defender can declare blockers"}}}
    }

    defender := g.Players[defenderUID]

    // Validate and assign blockers
    for _, block := range a.Blockers {
        // Find the blocker creature
        var blocker *FieldCard
        for _, fc := range defender.Field {
            if fc.InstanceID == block.BlockerInstanceID {
                blocker = fc
                break
            }
        }
        if blocker == nil {
            return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Blocker not found", "instanceId": block.BlockerInstanceID}}}
        }

        // Check blocker is valid (untapped, is a creature)
        blockerCard := CardDB[blocker.CardID]
        if blockerCard.CardType != "Creature" {
            return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Only creatures can block", "instanceId": block.BlockerInstanceID}}}
        }
        if blocker.IsTapped() {
            return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Tapped creatures cannot block", "instanceId": block.BlockerInstanceID}}}
        }

        // Find the attacker to check for Flying
        attackerPlayer := g.Players[g.AttackingPlayer]
        var attacker *FieldCard
        for _, fc := range attackerPlayer.Field {
            if fc.InstanceID == block.AttackerInstanceID {
                attacker = fc
                break
            }
        }
        if attacker != nil {
            attackerCard := CardDB[attacker.CardID]
            // Flying creatures can only be blocked by Flying or Reach
            if attackerCard.HasAbility("Flying") {
                if !blockerCard.HasAbility("Flying") && !blockerCard.HasAbility("Reach") {
                    return []Event{{Type: "Error", Data: map[string]interface{}{
                        "message": "Flying creatures can only be blocked by creatures with Flying or Reach",
                        "attackerInstanceId": block.AttackerInstanceID,
                        "blockerInstanceId": block.BlockerInstanceID,
                    }}}
                }
            }
        }

        // Find the attack being blocked
        found := false
        for i := range g.PendingAttacks {
            if g.PendingAttacks[i].AttackerInstanceID == block.AttackerInstanceID {
                // Can block attacks targeting the player OR your creatures
                g.PendingAttacks[i].BlockerInstanceID = block.BlockerInstanceID
                found = true
                break
            }
        }
        if !found {
            return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Attack not found", "attackerInstanceId": block.AttackerInstanceID}}}
        }
    }

    // Update combat phase
    g.CombatPhase = "blockers_declared"

    attackerPlayer := g.Players[g.AttackingPlayer]

    events := []Event{
        {
            Type: "BlockersDeclared",
            Data: map[string]interface{}{
                "defender": defenderUID,
                "blockers": a.Blockers,
                "attacks":  g.PendingAttacks,
            },
        },
    }

    // Resolve combat damage with FirstStrike
    for _, pa := range g.PendingAttacks {
        var attackerCreature *FieldCard
        for _, fc := range attackerPlayer.Field {
            if fc.InstanceID == pa.AttackerInstanceID {
                attackerCreature = fc
                break
            }
        }
        if attackerCreature == nil {
            continue
        }

        attackerCard := CardDB[attackerCreature.CardID]
        attackerDamage := attackerCreature.GetAttack()
        attackerHasFirstStrike := attackerCard.HasAbility("FirstStrike") || attackerCard.HasAbility("DoubleStrike")
        attackerHasDoubleStrike := attackerCard.HasAbility("DoubleStrike")

        if pa.BlockerInstanceID != 0 {
            // Blocked - fight the blocker
            var blockerCreature *FieldCard
            for _, fc := range defender.Field {
                if fc.InstanceID == pa.BlockerInstanceID {
                    blockerCreature = fc
                    break
                }
            }
            if blockerCreature == nil {
                continue
            }

            blockerCard := CardDB[blockerCreature.CardID]
            blockerDamage := blockerCreature.GetAttack()
            blockerHasFirstStrike := blockerCard.HasAbility("FirstStrike") || blockerCard.HasAbility("DoubleStrike")
            blockerHasDoubleStrike := blockerCard.HasAbility("DoubleStrike")

            // Resolve damage based on FirstStrike/DoubleStrike
            events = append(events, resolveCombatDamage(
                attackerCreature, attackerDamage, attackerHasFirstStrike, attackerHasDoubleStrike,
                blockerCreature, blockerDamage, blockerHasFirstStrike, blockerHasDoubleStrike,
            )...)

            // TODO: Trample is commented out for now - needs redesign
            // if attackerCard.HasAbility("Trample") && blockerCreature.IsDead() {
            //     // Negative health = overkill damage that tramples through
            //     excessDamage := -blockerCreature.CurrentHealth
            //     if excessDamage > 0 {
            //         if pa.TargetType == "player" {
            //             defender.Life -= excessDamage
            //             events = append(events, Event{
            //                 Type: "Damage",
            //                 Data: map[string]interface{}{
            //                     "target":  pa.TargetPlayerUID,
            //                     "amount":  excessDamage,
            //                     "source":  attackerCreature.InstanceID,
            //                     "trample": true,
            //                 },
            //             })
            //         } else if pa.TargetType == "creature" {
            //             // Find the original target creature
            //             var targetCreature *FieldCard
            //             for _, fc := range defender.Field {
            //                 if fc.InstanceID == pa.TargetInstanceID {
            //                     targetCreature = fc
            //                     break
            //                 }
            //             }
            //             if targetCreature != nil {
            //                 targetCreature.CurrentHealth -= excessDamage
            //                 events = append(events, Event{
            //                     Type: "CombatDamage",
            //                     Data: map[string]interface{}{
            //                         "attackerInstanceId": attackerCreature.InstanceID,
            //                         "targetType":         "creature",
            //                         "targetInstanceId":   targetCreature.InstanceID,
            //                         "damage":             excessDamage,
            //                         "trample":            true,
            //                     },
            //                 })
            //             }
            //         }
            //     }
            // }
        } else {
            // Not blocked - damage to original target
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
            } else if pa.TargetType == "creature" {
                var targetCreature *FieldCard
                for _, fc := range defender.Field {
                    if fc.InstanceID == pa.TargetInstanceID {
                        targetCreature = fc
                        break
                    }
                }
                if targetCreature != nil {
                    targetCard := CardDB[targetCreature.CardID]
                    targetDamage := targetCreature.GetAttack()
                    targetHasFirstStrike := targetCard.HasAbility("FirstStrike") || targetCard.HasAbility("DoubleStrike")
                    targetHasDoubleStrike := targetCard.HasAbility("DoubleStrike")

                    // Resolve damage based on FirstStrike/DoubleStrike
                    events = append(events, resolveCombatDamage(
                        attackerCreature, attackerDamage, attackerHasFirstStrike, attackerHasDoubleStrike,
                        targetCreature, targetDamage, targetHasFirstStrike, targetHasDoubleStrike,
                    )...)
                }
            }
        }
    }

    // Handle deaths
    events = append(events, g.handleDeaths(attackerPlayer, g.AttackingPlayer)...)
    events = append(events, g.handleDeaths(defender, defenderUID)...)

    // Check for game over
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

    return events
    // End of original blocking code */
}

// resolveCombatDamage handles damage between two creatures with FirstStrike/DoubleStrike logic
func resolveCombatDamage(
    creature1 *FieldCard, damage1 int, hasFirstStrike1 bool, hasDoubleStrike1 bool,
    creature2 *FieldCard, damage2 int, hasFirstStrike2 bool, hasDoubleStrike2 bool,
) []Event {
    events := []Event{}

    // First Strike phase - creatures with FirstStrike or DoubleStrike deal damage
    if hasFirstStrike1 && hasFirstStrike2 {
        // Both have first strike - simultaneous in first strike phase
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
        // Only creature 1 has first strike
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
        // Only creature 2 has first strike
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
    // Creature 1 deals normal damage if: no first strike, OR has double strike (and survived)
    creature1DealsNormal := (!hasFirstStrike1 || hasDoubleStrike1) && !creature1.IsDead()
    // Creature 2 deals normal damage if: no first strike, OR has double strike (and survived)
    creature2DealsNormal := (!hasFirstStrike2 || hasDoubleStrike2) && !creature2.IsDead()

    if creature1DealsNormal && creature2DealsNormal {
        // Both deal damage in normal phase - simultaneous
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
        // Only creature 1 deals normal damage
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
        // Only creature 2 deals normal damage
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

// handleDeaths removes dead creatures from field and moves them to discard
func (g *Game) handleDeaths(p *Player, playerUID string) []Event {
    events := []Event{}
    alive := []*FieldCard{}
    for _, fc := range p.Field {
        card := CardDB[fc.CardID]
        // Only creatures can die - lands and other non-creature cards stay on field
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

// keepHand - player keeps their current hand during mulligan phase
func (g *Game) keepHand(a Action) []Event {
    // Check if player already decided
    if g.MulliganDecisions[a.PlayerUID] {
        return []Event{{Type: "Error", Data: map[string]interface{}{"message": "Already made mulligan decision"}}}
    }

    g.MulliganDecisions[a.PlayerUID] = true

    events := []Event{
        {
            Type: "PlayerKeptHand",
            Data: map[string]interface{}{
                "player": a.PlayerUID,
            },
        },
    }

    // Check if both players have decided
    events = append(events, g.checkMulliganComplete()...)

    return events
}

// takeMulligan - player shuffles hand back and draws a new one
func (g *Game) takeMulligan(a Action) []Event {
    // Check if player already decided
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

    // Draw new hand (5 from main, 2 from vault)
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

    // Check if both players have decided
    events = append(events, g.checkMulliganComplete()...)

    return events
}

// checkMulliganComplete checks if both players decided and starts the game
func (g *Game) checkMulliganComplete() []Event {
    // Check if all players have decided
    for uid := range g.Players {
        if !g.MulliganDecisions[uid] {
            return []Event{} // Still waiting
        }
    }

    // Both decided - start the game!
    g.MulliganPhase = false
    g.Started = true
    g.DrawPhase = true // First player must draw

    // Build player info for GameStarted event
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
