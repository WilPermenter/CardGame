// Auto-detect local vs production server
const isLocal = window.location.hostname === "localhost"
    || window.location.hostname === "127.0.0.1"
    || window.location.protocol === "file:";
const wsUrl = isLocal ? "ws://localhost:8080/ws" : "ws://134.199.204.89/ws";
let ws = new WebSocket(wsUrl);
let myUID = "";
let gameId = "";
let currentTurn = "";
let myHealth = 30;
let opponentHealth = 30;
let selectedDeckId = 0;
let myHand = [];
let myField = []; // Creature FieldCard objects
let myLands = []; // Land FieldCard objects
let myDeckSize = 0;
let myDiscardSize = 0;
let opponentField = []; // Opponent's creatures
let opponentLands = []; // Opponent's lands
let myManaPool = { White: 0, Blue: 0, Black: 0, Red: 0, Green: 0, Colorless: 0 };
let cardDB = {}; // Local card database

// Combat state
let combatMode = false;
let pendingAttacks = []; // Array of {attackerInstanceId, targetType, targetInstanceId, targetPlayerUid}
let selectedAttacker = null; // instanceId of creature being assigned a target

// Blocking state
let blockingMode = false;
let incomingAttacks = []; // Attacks we need to block
let availableBlockers = []; // Our creatures that can block
let pendingBlocks = []; // Array of {blockerInstanceId, attackerInstanceId}
let selectedBlocker = null; // instanceId of creature we're assigning to block

// Cookie helpers
function setCookie(name, value, days = 1) {
    const expires = new Date(Date.now() + days * 24 * 60 * 60 * 1000).toUTCString();
    document.cookie = `${name}=${encodeURIComponent(value)}; expires=${expires}; path=/`;
}

function getCookie(name) {
    const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'));
    return match ? decodeURIComponent(match[2]) : null;
}

function deleteCookie(name) {
    document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/`;
}

function saveGameState() {
    if (gameId && myUID) {
        setCookie('tcg_gameId', gameId);
        setCookie('tcg_playerUid', myUID);
    }
}

function clearGameState() {
    deleteCookie('tcg_gameId');
    deleteCookie('tcg_playerUid');
}

// Dark mode
function toggleDarkMode() {
    const isDark = document.body.classList.toggle('dark-mode');
    setCookie('tcg_darkMode', isDark ? '1' : '0', 365);
    updateDarkModeButton();
}

function updateDarkModeButton() {
    const btn = document.querySelector('.dark-toggle');
    if (btn) {
        btn.textContent = document.body.classList.contains('dark-mode') ? 'Light Mode' : 'Dark Mode';
    }
}

function loadDarkMode() {
    if (getCookie('tcg_darkMode') === '1') {
        document.body.classList.add('dark-mode');
    }
    updateDarkModeButton();
}

// Initialize dark mode on load
loadDarkMode();

ws.onopen = () => {
    log("Connected to server");
    // Request card and deck lists
    ws.send(JSON.stringify({ type: "get_cards" }));
    ws.send(JSON.stringify({ type: "get_decks" }));

    // Check for saved game state and auto-reconnect
    const savedGameId = getCookie('tcg_gameId');
    const savedPlayerUid = getCookie('tcg_playerUid');
    if (savedPlayerUid) {
        document.getElementById("uid").value = savedPlayerUid;
    }
    if (savedGameId && savedPlayerUid) {
        setStatus(`Attempting to reconnect to ${savedGameId}...`);
        // Auto-attempt reconnect
        reconnectGame(savedGameId, savedPlayerUid);
    }
};

function showReconnectButton(savedGameId, savedPlayerUid) {
    const reconnectBtn = document.getElementById("reconnect-btn");
    if (reconnectBtn) {
        reconnectBtn.style.display = "inline-block";
        reconnectBtn.onclick = () => reconnectGame(savedGameId, savedPlayerUid);
    }
}

function hideReconnectButton() {
    const reconnectBtn = document.getElementById("reconnect-btn");
    if (reconnectBtn) {
        reconnectBtn.style.display = "none";
    }
}

function reconnectGame(savedGameId, savedPlayerUid) {
    myUID = savedPlayerUid;
    ws.send(JSON.stringify({
        playerUid: savedPlayerUid,
        type: "reconnect_game",
        gameId: savedGameId
    }));
    setStatus("Reconnecting...");
}

ws.onmessage = (msg) => {
    const events = JSON.parse(msg.data);
    log(JSON.stringify(events, null, 2));

    // Handle specific events
    for (const event of events) {
        switch (event.type) {
            case "GameCreated":
                gameId = event.data.gameId;
                saveGameState();
                setStatus("Waiting for opponent to join... (Game ID: " + gameId + ")");
                break;

            case "CardList":
                cardDB = event.data.cards;
                log("Loaded " + Object.keys(cardDB).length + " cards");
                break;

            case "MulliganPhase":
                gameId = event.data.gameId;
                // Get our hand from the players info
                if (event.data.players[myUID]) {
                    myHand = event.data.players[myUID].hand || [];
                    myDeckSize = event.data.players[myUID].deckSize || 0;
                    myDiscardSize = event.data.players[myUID].discardSize || 0;
                }
                // Find opponent UID
                for (const uid of Object.keys(event.data.players)) {
                    if (uid !== myUID) {
                        window.opponentUID = uid;
                        break;
                    }
                }
                saveGameState();
                log("Mulligan phase - choose to keep or mulligan your hand");
                showMulliganUI();
                renderHand();
                break;

            case "PlayerKeptHand":
                log(`${event.data.player === myUID ? "You" : "Opponent"} kept their hand`);
                if (event.data.player === myUID) {
                    hideMulliganUI();
                    setStatus("Waiting for opponent's mulligan decision...");
                }
                break;

            case "PlayerMulliganed":
                log(`${event.data.player === myUID ? "You" : "Opponent"} mulliganed`);
                if (event.data.player === myUID) {
                    myHand = event.data.newHand || [];
                    hideMulliganUI();
                    setStatus("Waiting for opponent's mulligan decision...");
                    renderHand();
                }
                break;

            case "GameStarted":
                gameId = event.data.gameId;
                currentTurn = event.data.currentTurn;
                // Get our hand from the players info (may have changed from mulligan)
                if (event.data.players[myUID]) {
                    myHand = event.data.players[myUID].hand || [];
                    myDeckSize = event.data.players[myUID].deckSize || 0;
                    myDiscardSize = event.data.players[myUID].discardSize || 0;
                }
                // Find opponent UID
                for (const uid of Object.keys(event.data.players)) {
                    if (uid !== myUID) {
                        window.opponentUID = uid;
                        break;
                    }
                }
                saveGameState();
                hideMulliganUI();
                document.getElementById("game-controls").style.display = "block";
                document.getElementById("chat-section").style.display = "block";
                log("Game started!");
                renderHand();
                updateTurnStatus();
                break;

            case "TurnChanged":
                currentTurn = event.data.activePlayer;
                // If it's our turn, untap cards, clear summoning sickness, and reset mana pool
                if (currentTurn === myUID) {
                    myField.forEach(fc => {
                        fc.canAttack = true;
                        if (fc.status) {
                            fc.status.Tapped = 0;
                            fc.status.Summoned = 0;
                        }
                    });
                    myLands.forEach(fc => {
                        if (fc.status) fc.status.Tapped = 0;
                    });
                    myManaPool = { White: 0, Blue: 0, Black: 0, Red: 0, Green: 0, Colorless: 0 };
                    renderField();
                    renderLands();
                    updateManaPoolDisplay();
                }
                updateTurnStatus();
                break;

            case "CardDrawn":
                // If we drew a card, add it to our hand
                if (event.data.player === myUID) {
                    myHand.push(event.data.cardId);
                    myDeckSize--;
                    renderHand();
                }
                break;

            case "CreaturePlayed":
                // Creature moved from hand to field
                if (event.data.player === myUID) {
                    const idx = myHand.indexOf(event.data.cardId);
                    if (idx > -1) myHand.splice(idx, 1);
                    if (event.data.fieldCard) {
                        myField.push(event.data.fieldCard);
                    }
                    renderHand();
                    renderField();
                } else {
                    // Opponent played a creature
                    if (event.data.fieldCard) {
                        opponentField.push(event.data.fieldCard);
                    }
                    renderOpponentField();
                }
                break;

            case "LandPlayed":
                // Land moved from hand to lands area
                if (event.data.player === myUID) {
                    const idx = myHand.indexOf(event.data.cardId);
                    if (idx > -1) myHand.splice(idx, 1);
                    if (event.data.fieldCard) {
                        myLands.push(event.data.fieldCard);
                    }
                    renderHand();
                    renderLands();
                } else {
                    // Opponent played a land
                    if (event.data.fieldCard) {
                        opponentLands.push(event.data.fieldCard);
                    }
                    renderOpponentLands();
                }
                break;

            case "CardPlayed":
                // Spells - go to discard
                if (event.data.player === myUID) {
                    const idx = myHand.indexOf(event.data.cardId);
                    if (idx > -1) myHand.splice(idx, 1);
                    myDiscardSize++;
                    renderHand();
                }
                break;

            case "CardTapped":
                // Update tapped status on the card
                if (event.data.player === myUID) {
                    const fc = findFieldCard(myField, event.data.instanceId) ||
                               findFieldCard(myLands, event.data.instanceId);
                    if (fc) {
                        if (!fc.status) fc.status = {};
                        fc.status.Tapped = event.data.tapped ? 1 : 0;
                        renderField();
                        renderLands();
                    }
                } else {
                    const fc = findFieldCard(opponentField, event.data.instanceId) ||
                               findFieldCard(opponentLands, event.data.instanceId);
                    if (fc) {
                        if (!fc.status) fc.status = {};
                        fc.status.Tapped = event.data.tapped ? 1 : 0;
                        renderOpponentField();
                        renderOpponentLands();
                    }
                }
                break;

            case "CardUntapped":
                // Update untapped status on the card (e.g., Vigilance at end step)
                if (event.data.player === myUID) {
                    const fc = findFieldCard(myField, event.data.instanceId) ||
                               findFieldCard(myLands, event.data.instanceId);
                    if (fc) {
                        if (!fc.status) fc.status = {};
                        fc.status.Tapped = 0;
                        renderField();
                        renderLands();
                    }
                } else {
                    const fc = findFieldCard(opponentField, event.data.instanceId) ||
                               findFieldCard(opponentLands, event.data.instanceId);
                    if (fc) {
                        if (!fc.status) fc.status = {};
                        fc.status.Tapped = 0;
                        renderOpponentField();
                        renderOpponentLands();
                    }
                }
                if (event.data.reason) {
                    log(`Card untapped (${event.data.reason})`);
                }
                break;

            case "ManaAdded":
                // Update mana pool
                if (event.data.player === myUID && event.data.manaPool) {
                    myManaPool = event.data.manaPool;
                    updateManaPoolDisplay();
                }
                break;

            case "CardBurned":
                // Land burned from hand for mana
                if (event.data.player === myUID) {
                    const idx = myHand.indexOf(event.data.cardId);
                    if (idx > -1) myHand.splice(idx, 1);
                    myDiscardSize++;
                    renderHand();
                }
                break;

            case "Damage":
                const target = event.data.target;
                const amount = event.data.amount;
                if (target === myUID) {
                    myHealth -= amount;
                } else {
                    opponentHealth -= amount;
                }
                updateHealthDisplay();
                break;

            case "AttacksDeclared":
                log("Attacks declared by " + event.data.player);
                break;

            case "CombatDamage":
                // Update creature health from combat
                const targetInstId = event.data.targetInstanceId;
                if (event.data.targetType === "creature") {
                    // Find and update the creature's health
                    let fc = findFieldCard(myField, targetInstId);
                    if (fc) {
                        fc.currentHealth -= event.data.damage;
                        renderField();
                    } else {
                        fc = findFieldCard(opponentField, targetInstId);
                        if (fc) {
                            fc.currentHealth -= event.data.damage;
                            renderOpponentField();
                        }
                    }
                }
                break;

            case "CreatureDied":
                // Remove creature from field
                if (event.data.player === myUID) {
                    myField = myField.filter(fc => fc.instanceId !== event.data.instanceId);
                    myDiscardSize++;
                    renderField();
                } else {
                    opponentField = opponentField.filter(fc => fc.instanceId !== event.data.instanceId);
                    renderOpponentField();
                }
                break;

            case "GameOver":
                const winner = event.data.winner;
                if (winner === myUID) {
                    setTurnStatus("You win!");
                } else {
                    setTurnStatus("You lose!");
                }
                clearGameState();
                disableGameControls();
                break;

            case "OpponentLeft":
                setTurnStatus("Opponent left - You win!");
                clearGameState();
                disableGameControls();
                break;

            case "DeckList":
                populateDeckSelect(event.data.decks);
                break;

            case "GameList":
                displayGameList(event.data.games || []);
                break;

            case "GameReconnected":
                // Restore full game state
                gameId = event.data.gameId;
                myUID = event.data.playerUid;
                window.opponentUID = event.data.opponentUid;
                currentTurn = event.data.currentTurn;
                myHand = event.data.myHand || [];
                myHealth = event.data.myLife;
                myField = event.data.myField || [];
                myLands = event.data.myLands || [];
                myManaPool = event.data.myManaPool || { White: 0, Blue: 0, Black: 0, Red: 0, Green: 0, Colorless: 0 };
                myDeckSize = event.data.myDeckSize || 0;
                myDiscardSize = event.data.myDiscardSize || 0;
                opponentHealth = event.data.opponentLife;
                opponentField = event.data.opponentField || [];
                opponentLands = event.data.opponentLands || [];

                // Update UI
                hideReconnectButton();
                showInGameLobby();

                // Check if still in mulligan phase
                if (event.data.mulliganPhase && !event.data.mulliganDecided) {
                    // Show mulligan UI - player hasn't decided yet
                    showMulliganUI();
                    renderHand();
                    setStatus("Reconnected - Waiting for mulligan decision");
                } else if (event.data.mulliganPhase && event.data.mulliganDecided) {
                    // Player decided but waiting for opponent
                    document.getElementById("game-controls").style.display = "block";
                    renderHand();
                    setStatus("Reconnected - Waiting for opponent's mulligan decision");
                } else {
                    // Game started - show full game UI
                    document.getElementById("game-controls").style.display = "block";
                    document.getElementById("chat-section").style.display = "block";
                    renderHand();
                    renderField();
                    renderLands();
                    renderOpponentField();
                    renderOpponentLands();
                    updateHealthDisplay();
                    updateManaPoolDisplay();
                    updateTurnStatus();
                    setStatus("Reconnected to game " + gameId);
                }
                break;

            case "PlayerReconnected":
                log("Player " + event.data.player + " reconnected");
                addChatMessage("System", event.data.player + " reconnected");
                break;

            case "ChatMessage":
                addChatMessage(event.data.player, event.data.message);
                break;

            case "BlockersNeeded":
                // We need to declare blockers
                if (event.data.defender === myUID) {
                    blockingMode = true;
                    incomingAttacks = event.data.attacks || [];
                    availableBlockers = event.data.availableBlockers || [];
                    pendingBlocks = [];
                    selectedBlocker = null;
                    console.log("BlockersNeeded received:");
                    console.log("  myField:", myField.map(f => ({id: f.instanceId, cardId: f.cardId})));
                    console.log("  availableBlockers:", availableBlockers);
                    console.log("  incomingAttacks:", incomingAttacks);
                    renderField();
                    renderOpponentField();
                    updateBlockingUI();
                    addChatMessage("System", "You are being attacked! Assign blockers or confirm.");
                }
                break;

            case "BlockersDeclared":
                blockingMode = false;
                incomingAttacks = [];
                availableBlockers = [];
                pendingBlocks = [];
                selectedBlocker = null;
                renderField();
                renderOpponentField();
                updateBlockingUI();
                log("Blockers declared: " + JSON.stringify(event.data.blockers));
                break;

            case "Error":
                // If reconnect failed, clear the cookies and reset status
                if (event.data.message === "Game not found" || event.data.message === "You are not in this game") {
                    clearGameState();
                    hideReconnectButton();
                    setStatus("Enter a UID and select a deck");
                    log("Reconnect failed: " + event.data.message);
                } else {
                    setStatus("Error: " + event.data.message);
                }
                break;
        }
    }
};

ws.onclose = () => {
    log("Disconnected from server");
};

function getUID() {
    return document.getElementById("uid").value.trim();
}

function getSelectedDeck() {
    const select = document.getElementById("deck-select");
    return parseInt(select.value) || 0;
}

function populateDeckSelect(decks) {
    const select = document.getElementById("deck-select");
    select.innerHTML = '<option value="">-- Select a deck --</option>';
    for (const deck of decks) {
        const option = document.createElement("option");
        option.value = deck.id;
        option.textContent = `${deck.name} (Leader: ${deck.leaderName})`;
        select.appendChild(option);
    }
}

function startGame() {
    myUID = getUID();
    if (!myUID) {
        alert("Please enter a UID");
        return;
    }

    selectedDeckId = getSelectedDeck();
    if (!selectedDeckId) {
        alert("Please select a deck");
        return;
    }

    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "start_game",
        deckId: selectedDeckId
    }));
    setStatus("Creating game...");
    showInGameLobby();
}

function joinGame() {
    myUID = getUID();
    if (!myUID) {
        alert("Please enter a UID");
        return;
    }

    selectedDeckId = getSelectedDeck();
    if (!selectedDeckId) {
        alert("Please select a deck");
        return;
    }

    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "join_game",
        deckId: selectedDeckId
    }));
    setStatus("Joining game...");
    showInGameLobby();
}

function showInGameLobby() {
    document.getElementById("start-btn").style.display = "none";
    document.getElementById("browse-btn").style.display = "none";
    document.getElementById("deck-select-area").style.display = "none";
    document.getElementById("leave-btn").style.display = "inline-block";
    document.getElementById("game-list-container").style.display = "none";
    hideReconnectButton();
}

function showOutOfGameLobby() {
    document.getElementById("start-btn").style.display = "inline-block";
    document.getElementById("browse-btn").style.display = "inline-block";
    document.getElementById("deck-select-area").style.display = "block";
    document.getElementById("leave-btn").style.display = "none";
    document.getElementById("game-list-container").style.display = "block";
}

function playCard(cardId) {
    if (currentTurn !== myUID) {
        alert("Not your turn!");
        return;
    }
    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "play_card",
        cardId: cardId
    }));
}

function renderHand() {
    const handEl = document.getElementById("hand");
    handEl.innerHTML = "";

    for (const cardId of myHand) {
        const card = cardDB[cardId];
        const cardEl = document.createElement("div");
        cardEl.className = "card";
        cardEl.onclick = () => playCard(cardId);

        if (card) {
            const costStr = formatCost(card.Cost);
            let burnBtn = '';
            if (card.CardType === "Land") {
                burnBtn = `<button class="burn-btn" onclick="event.stopPropagation(); burnCard(${cardId});">Burn</button>`;
            }
            const abilitiesStr = formatAbilities(card.Abilities);
            cardEl.innerHTML = `
                <div class="card-name">${card.Name}</div>
                <div class="card-cost">${costStr}</div>
                <div class="card-stats">${card.Attack}/${card.Defense}</div>
                <div class="card-type">${card.CardType}</div>
                ${abilitiesStr}
                ${burnBtn}
            `;
        } else {
            cardEl.innerHTML = `<div class="card-name">Card #${cardId}</div>`;
        }

        handEl.appendChild(cardEl);
    }

    document.getElementById("hand-count").textContent = myHand.length;
    document.getElementById("deck-count").textContent = myDeckSize;
    document.getElementById("discard-count").textContent = myDiscardSize;
}

function burnCard(cardId) {
    if (currentTurn !== myUID) {
        alert("Not your turn!");
        return;
    }
    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "burn_card",
        cardId: cardId
    }));
}

// Combat functions
function toggleCombatMode() {
    if (currentTurn !== myUID) {
        alert("Not your turn!");
        return;
    }
    combatMode = !combatMode;
    selectedAttacker = null;
    if (!combatMode) {
        pendingAttacks = [];
    }
    renderField();
    renderOpponentField();
    updateCombatUI();
}

function selectAttacker(instanceId) {
    if (!combatMode || currentTurn !== myUID) return;

    const fc = findFieldCard(myField, instanceId);
    if (!fc) return;

    // Check if can attack
    const isSummoned = fc.status && fc.status.Summoned > 0;
    const isTapped = fc.status && fc.status.Tapped > 0;
    if (isSummoned || isTapped) {
        alert("This creature cannot attack!");
        return;
    }

    // Check if already assigned
    const alreadyAttacking = pendingAttacks.find(a => a.attackerInstanceId === instanceId);
    if (alreadyAttacking) {
        // Remove from pending
        pendingAttacks = pendingAttacks.filter(a => a.attackerInstanceId !== instanceId);
        selectedAttacker = null;
    } else {
        selectedAttacker = instanceId;
    }
    renderField();
    renderOpponentField();
    updateCombatUI();
}

function selectTarget(targetType, targetInstanceId, targetPlayerUid) {
    if (!combatMode || selectedAttacker === null) return;

    pendingAttacks.push({
        attackerInstanceId: selectedAttacker,
        targetType: targetType,
        targetInstanceId: targetInstanceId || 0,
        targetPlayerUid: targetPlayerUid || ""
    });
    selectedAttacker = null;
    renderField();
    renderOpponentField();
    updateCombatUI();
}

function confirmAttacks() {
    if (pendingAttacks.length === 0) {
        alert("No attacks selected!");
        return;
    }
    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "declare_attacks",
        attacks: pendingAttacks
    }));
    combatMode = false;
    pendingAttacks = [];
    selectedAttacker = null;
    renderField();
    renderOpponentField();
    updateCombatUI();
}

function cancelCombat() {
    combatMode = false;
    pendingAttacks = [];
    selectedAttacker = null;
    renderField();
    renderOpponentField();
    updateCombatUI();
}

function updateCombatUI() {
    const combatBtn = document.getElementById("combat-btn");
    const confirmBtn = document.getElementById("confirm-attacks-btn");
    const cancelBtn = document.getElementById("cancel-combat-btn");
    const combatStatus = document.getElementById("combat-status");

    if (combatBtn) combatBtn.textContent = combatMode ? "Exit Combat" : "Combat";
    if (confirmBtn) confirmBtn.style.display = combatMode ? "inline-block" : "none";
    if (cancelBtn) cancelBtn.style.display = combatMode ? "inline-block" : "none";

    if (combatStatus) {
        if (!combatMode) {
            combatStatus.textContent = "";
        } else if (selectedAttacker) {
            combatStatus.textContent = "Select a target (opponent creature or opponent's health)";
        } else {
            combatStatus.textContent = `Combat Mode: ${pendingAttacks.length} attacks pending. Click your creatures to select attackers.`;
        }
    }

    updateOpponentHealthTargeting();
}

// Blocking functions
function selectBlocker(instanceId) {
    if (!blockingMode) return;

    // Check if this creature can block
    const canBlock = availableBlockers.some(b => b.instanceId === instanceId);
    if (!canBlock) {
        alert("This creature cannot block!");
        return;
    }

    // Check if already assigned
    const alreadyBlocking = pendingBlocks.find(b => b.blockerInstanceId === instanceId);
    if (alreadyBlocking) {
        // Remove from pending
        pendingBlocks = pendingBlocks.filter(b => b.blockerInstanceId !== instanceId);
        selectedBlocker = null;
    } else {
        selectedBlocker = instanceId;
    }
    renderField();
    renderOpponentField();
    updateBlockingUI();
}

function canBlockAttacker(blockerInstanceId, attackerInstanceId) {
    // Find blocker abilities
    const blocker = availableBlockers.find(b => b.instanceId === blockerInstanceId);
    if (!blocker) return false;

    // Find attacker abilities
    const attack = incomingAttacks.find(a => a.attackerInstanceId === attackerInstanceId);
    if (!attack) return false;

    const blockerAbilities = blocker.abilities || [];
    const attackerAbilities = attack.attackerAbilities || [];

    // Flying creatures can only be blocked by Flying or Reach
    if (attackerAbilities.includes("Flying")) {
        if (!blockerAbilities.includes("Flying") && !blockerAbilities.includes("Reach")) {
            return false;
        }
    }

    return true;
}

function selectAttackerToBlock(attackerInstanceId) {
    if (!blockingMode || selectedBlocker === null) return;

    // Find the attack
    const attack = incomingAttacks.find(a => a.attackerInstanceId === attackerInstanceId);
    if (!attack) return;

    // Check if this blocker can block this attacker (Flying/Reach check)
    if (!canBlockAttacker(selectedBlocker, attackerInstanceId)) {
        alert("This creature cannot block a Flying creature! Need Flying or Reach.");
        return;
    }

    // Can block attacks targeting player OR your creatures
    pendingBlocks.push({
        blockerInstanceId: selectedBlocker,
        attackerInstanceId: attackerInstanceId
    });
    selectedBlocker = null;
    renderField();
    renderOpponentField();
    updateBlockingUI();
}

function confirmBlockers() {
    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "declare_blockers",
        blockers: pendingBlocks
    }));
}

function skipBlocking() {
    // Confirm with no blockers
    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "declare_blockers",
        blockers: []
    }));
}

function updateBlockingUI() {
    const blockingStatus = document.getElementById("blocking-status");
    const confirmBlockBtn = document.getElementById("confirm-block-btn");
    const skipBlockBtn = document.getElementById("skip-block-btn");

    if (blockingStatus) {
        if (!blockingMode) {
            blockingStatus.textContent = "";
            blockingStatus.style.display = "none";
        } else if (selectedBlocker) {
            blockingStatus.textContent = "Click an attacking creature to block it";
            blockingStatus.style.display = "block";
        } else {
            blockingStatus.textContent = `Blocking Mode: ${pendingBlocks.length} blockers assigned. Click your creatures to select blockers.`;
            blockingStatus.style.display = "block";
        }
    }

    if (confirmBlockBtn) confirmBlockBtn.style.display = blockingMode ? "inline-block" : "none";
    if (skipBlockBtn) skipBlockBtn.style.display = blockingMode ? "inline-block" : "none";
}

function formatCost(cost) {
    if (!cost) return "Free";
    const parts = [];
    if (cost.White) parts.push(cost.White + "W");
    if (cost.Blue) parts.push(cost.Blue + "U");
    if (cost.Black) parts.push(cost.Black + "B");
    if (cost.Red) parts.push(cost.Red + "R");
    if (cost.Green) parts.push(cost.Green + "G");
    if (cost.Colorless) parts.push(cost.Colorless);
    return parts.join(" ") || "Free";
}

function formatAbilities(abilities) {
    if (!abilities || abilities.length === 0) return '';
    return `<div class="card-abilities">${abilities.join(', ')}</div>`;
}

function renderField() {
    const fieldEl = document.getElementById("field");
    fieldEl.innerHTML = "";

    for (const fc of myField) {
        const card = cardDB[fc.cardId];
        const cardEl = document.createElement("div");
        const isSummoned = fc.status && fc.status.Summoned > 0;
        const isTapped = fc.status && fc.status.Tapped > 0;
        const isSelected = selectedAttacker === fc.instanceId;
        const isAttacking = pendingAttacks.some(a => a.attackerInstanceId === fc.instanceId);

        let classes = "field-card";
        if (!isSummoned && !isTapped) classes += " can-attack";
        if (isTapped) classes += " tapped";
        if (isSelected) classes += " selected-attacker";
        if (isAttacking) classes += " attacking";
        cardEl.className = classes;

        // Blocking click handler (check first, takes priority)
        if (blockingMode) {
            const canBlock = availableBlockers.some(b => b.instanceId === fc.instanceId);
            const isSelectedBlocker = selectedBlocker === fc.instanceId;
            const isBlocking = pendingBlocks.some(b => b.blockerInstanceId === fc.instanceId);

            console.log("Blocking mode - creature:", fc.instanceId, "canBlock:", canBlock, "availableBlockers:", availableBlockers);

            if (canBlock) {
                cardEl.classList.add("can-block");
                cardEl.style.cursor = "pointer";
            }
            if (isSelectedBlocker) {
                cardEl.classList.add("selected-blocker");
            }
            if (isBlocking) {
                cardEl.classList.add("blocking");
            }

            // Attach click handler for any creature that can block
            if (canBlock || isBlocking) {
                const instanceId = fc.instanceId;
                cardEl.addEventListener("click", function() {
                    selectBlocker(instanceId);
                });
            }
        }
        // Combat click handler (only if not in blocking mode)
        else if (combatMode && !isSummoned && !isTapped) {
            cardEl.onclick = () => selectAttacker(fc.instanceId);
            cardEl.style.cursor = "pointer";
        }

        const effectiveAttack = (card?.Attack || 0) + (fc.damageModifier || 0);
        const effectiveHealth = fc.currentHealth;
        const maxHealth = (card?.Defense || 0) + (fc.healthModifier || 0);

        if (card) {
            let statusText = '';
            if (isTapped) {
                statusText = '<div style="color:#999;font-size:10px;">Tapped</div>';
            } else if (isSummoned) {
                statusText = '<div style="color:#999;font-size:10px;">Summoned</div>';
            } else if (isAttacking) {
                statusText = '<div style="color:#f44336;font-size:10px;">Attacking</div>';
            } else if (isSelected) {
                statusText = '<div style="color:#2196f3;font-size:10px;">Select Target</div>';
            } else {
                statusText = '<div style="color:#ff9800;font-size:10px;">Ready</div>';
            }
            const abilitiesStr = formatAbilities(card.Abilities);
            cardEl.innerHTML = `
                <div class="card-name">${card.Name}</div>
                <div class="card-attack">ATK: ${effectiveAttack}</div>
                <div class="card-health">HP: ${effectiveHealth}/${maxHealth}</div>
                ${abilitiesStr}
                ${statusText}
            `;
        } else {
            cardEl.innerHTML = `<div class="card-name">Card #${fc.cardId}</div>`;
        }

        fieldEl.appendChild(cardEl);
    }

    document.getElementById("field-count").textContent = myField.length;
}

function renderLands() {
    const landsEl = document.getElementById("lands");
    landsEl.innerHTML = "";

    for (const fc of myLands) {
        const card = cardDB[fc.cardId];
        const cardEl = document.createElement("div");
        const isTapped = fc.status && fc.status.Tapped > 0;
        cardEl.className = "land-card" + (isTapped ? " tapped" : "");
        cardEl.onclick = () => tapLand(fc.instanceId);

        if (card) {
            cardEl.innerHTML = `<div class="card-name">${card.Name}</div>` +
                (isTapped ? '<div class="tapped-label">Tapped</div>' : '');
        } else {
            cardEl.innerHTML = `<div class="card-name">Land #${fc.cardId}</div>`;
        }

        landsEl.appendChild(cardEl);
    }

    document.getElementById("lands-count").textContent = myLands.length;
    updateManaPoolDisplay();
}

function tapLand(instanceId) {
    if (currentTurn !== myUID) {
        alert("Not your turn!");
        return;
    }
    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "tap_card",
        instanceId: instanceId
    }));
}

function findFieldCard(arr, instanceId) {
    return arr.find(fc => fc.instanceId === instanceId);
}

function updateManaPoolDisplay() {
    const manaEl = document.getElementById("mana-display");
    if (manaEl) {
        manaEl.textContent = formatMana(myManaPool);
    }
}

function getAvailableMana() {
    const mana = { White: 0, Blue: 0, Black: 0, Red: 0, Green: 0, Colorless: 0 };
    for (const fc of myLands) {
        const card = cardDB[fc.cardId];
        if (card && card.CardType === "Land") {
            // Derive mana from land name
            if (card.Name.includes("White")) mana.White++;
            else if (card.Name.includes("Blue")) mana.Blue++;
            else if (card.Name.includes("Black")) mana.Black++;
            else if (card.Name.includes("Red")) mana.Red++;
            else if (card.Name.includes("Green")) mana.Green++;
        }
    }
    return mana;
}

function formatMana(mana) {
    const parts = [];
    if (mana.White) parts.push(mana.White + "W");
    if (mana.Blue) parts.push(mana.Blue + "U");
    if (mana.Black) parts.push(mana.Black + "B");
    if (mana.Red) parts.push(mana.Red + "R");
    if (mana.Green) parts.push(mana.Green + "G");
    if (mana.Colorless) parts.push(mana.Colorless);
    return parts.join(" ") || "0";
}

function updateManaDisplay() {
    const mana = getAvailableMana();
    const manaEl = document.getElementById("mana-display");
    if (manaEl) {
        manaEl.textContent = formatMana(mana);
    }
}

function renderOpponentField() {
    const fieldEl = document.getElementById("opponent-field");
    fieldEl.innerHTML = "";

    for (const fc of opponentField) {
        const card = cardDB[fc.cardId];
        const cardEl = document.createElement("div");
        const isTapped = fc.status && fc.status.Tapped > 0;
        const isTargeted = pendingAttacks.some(a => a.targetType === "creature" && a.targetInstanceId === fc.instanceId);

        let classes = "field-card opponent";
        if (isTapped) classes += " tapped";
        if (isTargeted) classes += " targeted";
        if (combatMode && selectedAttacker !== null) classes += " targetable";
        cardEl.className = classes;

        // Combat target click handler
        if (combatMode && selectedAttacker !== null) {
            cardEl.onclick = () => selectTarget("creature", fc.instanceId, "");
            cardEl.style.cursor = "crosshair";
        }

        // Blocking mode - show attacking creatures (can block attacks targeting player OR your creatures)
        if (blockingMode) {
            const isAttacking = incomingAttacks.some(a => a.attackerInstanceId === fc.instanceId);
            const isBeingBlocked = pendingBlocks.some(b => b.attackerInstanceId === fc.instanceId);

            if (isAttacking) {
                cardEl.classList.add("attacking");
                if (selectedBlocker !== null && !isBeingBlocked) {
                    cardEl.classList.add("blockable");
                    const instanceId = fc.instanceId;
                    cardEl.addEventListener("click", function() {
                        selectAttackerToBlock(instanceId);
                    });
                    cardEl.style.cursor = "crosshair";
                }
            }
            if (isBeingBlocked) {
                cardEl.classList.add("being-blocked");
            }
        }

        const effectiveAttack = (card?.Attack || 0) + (fc.damageModifier || 0);
        const effectiveHealth = fc.currentHealth;
        const maxHealth = (card?.Defense || 0) + (fc.healthModifier || 0);

        if (card) {
            let targetText = isTargeted ? '<div style="color:#ff5722;font-size:10px;">Targeted</div>' : '';
            const abilitiesStr = formatAbilities(card.Abilities);
            cardEl.innerHTML = `
                <div class="card-name">${card.Name}</div>
                <div class="card-attack">ATK: ${effectiveAttack}</div>
                <div class="card-health">HP: ${effectiveHealth}/${maxHealth}</div>
                ${abilitiesStr}
                ${targetText}
            `;
        } else {
            cardEl.innerHTML = `<div class="card-name">Card #${fc.cardId}</div>`;
        }

        fieldEl.appendChild(cardEl);
    }

    document.getElementById("opponent-field-count").textContent = opponentField.length;
}

function renderOpponentLands() {
    const landsEl = document.getElementById("opponent-lands");
    landsEl.innerHTML = "";

    for (const fc of opponentLands) {
        const card = cardDB[fc.cardId];
        const cardEl = document.createElement("div");
        cardEl.className = "land-card opponent";

        if (card) {
            cardEl.innerHTML = `<div class="card-name">${card.Name}</div>`;
        } else {
            cardEl.innerHTML = `<div class="card-name">Land #${fc.cardId}</div>`;
        }

        landsEl.appendChild(cardEl);
    }

    document.getElementById("opponent-lands-count").textContent = opponentLands.length;
}

function endTurn() {
    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "end_turn"
    }));
}

function setStatus(text) {
    document.getElementById("status").textContent = text;
}

function updateTurnStatus() {
    const isMyTurn = currentTurn === myUID;
    document.getElementById("turn-status").textContent =
        isMyTurn ? "Your turn!" : `Waiting for ${currentTurn}...`;
}

function log(text) {
    const logEl = document.getElementById("log");
    logEl.textContent += text + "\n";
    logEl.scrollTop = logEl.scrollHeight;
}

function toggleSection(btn) {
    const content = btn.closest('.section').querySelector('.section-content');
    content.classList.toggle('hidden');
    btn.textContent = content.classList.contains('hidden') ? 'show' : 'hide';
}

function updateHealthDisplay() {
    document.getElementById("my-health").textContent = myHealth;
    document.getElementById("opponent-health").textContent = opponentHealth;
    updateOpponentHealthTargeting();
}

function updateOpponentHealthTargeting() {
    const opponentBox = document.getElementById("opponent-health-box");
    if (!opponentBox) return;

    const isTargeted = pendingAttacks.some(a => a.targetType === "player");

    if (combatMode && selectedAttacker !== null) {
        opponentBox.classList.add("targetable");
        opponentBox.style.cursor = "crosshair";
        opponentBox.onclick = () => selectOpponentAsTarget();
    } else {
        opponentBox.classList.remove("targetable");
        opponentBox.style.cursor = "default";
        opponentBox.onclick = null;
    }

    if (isTargeted) {
        opponentBox.classList.add("targeted");
    } else {
        opponentBox.classList.remove("targeted");
    }
}

function selectOpponentAsTarget() {
    if (!combatMode || selectedAttacker === null) return;
    // Find opponent UID
    const opponentUID = getOpponentUID();
    selectTarget("player", 0, opponentUID);
}

function getOpponentUID() {
    // We need to track opponent UID - for now derive from event or default
    return window.opponentUID || "opponent";
}

function setTurnStatus(text) {
    document.getElementById("turn-status").textContent = text;
}

function disableGameControls() {
    const buttons = document.querySelectorAll("#game-controls .section-content button:not(.hide-btn):not(#leave-btn)");
    buttons.forEach(btn => btn.disabled = true);
}

function leaveGame() {
    // Tell server we're leaving
    if (gameId) {
        ws.send(JSON.stringify({
            playerUid: myUID,
            type: "leave_game"
        }));
    }

    // Clear game cookies
    clearGameState();

    // Reset state
    gameId = "";
    currentTurn = "";
    myHealth = 30;
    opponentHealth = 30;
    myHand = [];
    myField = [];
    myLands = [];
    myDeckSize = 0;
    myDiscardSize = 0;
    opponentField = [];
    opponentLands = [];
    myManaPool = { White: 0, Blue: 0, Black: 0, Red: 0, Green: 0, Colorless: 0 };

    // Reset UI
    hideMulliganUI();
    document.getElementById("game-controls").style.display = "none";
    document.getElementById("chat-section").style.display = "none";
    document.getElementById("chat-messages").innerHTML = "";
    document.getElementById("my-health").textContent = "30";
    document.getElementById("opponent-health").textContent = "30";
    document.getElementById("turn-status").textContent = "";
    document.getElementById("hand").innerHTML = "";
    document.getElementById("field").innerHTML = "";
    document.getElementById("lands").innerHTML = "";
    document.getElementById("hand-count").textContent = "0";
    document.getElementById("field-count").textContent = "0";
    document.getElementById("lands-count").textContent = "0";
    document.getElementById("deck-count").textContent = "0";
    document.getElementById("discard-count").textContent = "0";
    document.getElementById("mana-display").textContent = "0";
    document.getElementById("opponent-field").innerHTML = "";
    document.getElementById("opponent-lands").innerHTML = "";
    document.getElementById("opponent-field-count").textContent = "0";
    document.getElementById("opponent-lands-count").textContent = "0";

    // Re-enable buttons for next game
    const buttons = document.querySelectorAll("#game-controls .section-content button:not(.hide-btn)");
    buttons.forEach(btn => btn.disabled = false);

    showOutOfGameLobby();
    setStatus("Enter a UID and select a deck");
}

// Chat functions
function sendChat() {
    const input = document.getElementById("chat-input");
    const message = input.value.trim();
    if (!message || !gameId) return;

    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "chat",
        message: message
    }));

    input.value = "";
}

function addChatMessage(player, message) {
    const chatEl = document.getElementById("chat-messages");
    const isMe = player === myUID;
    const isSystem = player === "System";

    const msgDiv = document.createElement("div");
    msgDiv.style.marginBottom = "5px";

    if (isSystem) {
        msgDiv.style.color = "#999";
        msgDiv.style.fontStyle = "italic";
        msgDiv.innerHTML = message;
    } else {
        const nameColor = isMe ? "#2196f3" : "#f44336";
        msgDiv.innerHTML = `<span style="color:${nameColor};font-weight:bold;">${player}:</span> ${escapeHtml(message)}`;
    }

    chatEl.appendChild(msgDiv);
    chatEl.scrollTop = chatEl.scrollHeight;
}

function escapeHtml(text) {
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
}

// Lobby functions
function refreshGameList() {
    ws.send(JSON.stringify({ type: "list_games" }));
}

function displayGameList(games) {
    const listEl = document.getElementById("game-list");
    if (!listEl) return;

    if (games.length === 0) {
        listEl.innerHTML = '<p style="color:#999;">No games available. Create one!</p>';
        return;
    }

    listEl.innerHTML = "";
    for (const game of games) {
        const gameEl = document.createElement("div");
        gameEl.className = "game-item";

        const playerList = game.players.join(", ") || "Empty";
        const status = game.started ? "In Progress" : `Waiting (${game.playerCount}/2)`;
        const statusClass = game.started ? "game-status in-progress" : "game-status";
        const canJoin = !game.started && game.playerCount < 2;

        gameEl.innerHTML = `
            <div class="game-info">
                <strong>${game.gameId}</strong>
                <span class="${statusClass}">${status}</span>
            </div>
            <div class="game-players">Players: ${playerList}</div>
            ${canJoin ? `<button onclick="joinSpecificGame('${game.gameId}')">Join</button>` : ''}
        `;
        listEl.appendChild(gameEl);
    }
}

function joinSpecificGame(gameIdToJoin) {
    myUID = getUID();
    if (!myUID) {
        alert("Please enter a UID");
        return;
    }

    selectedDeckId = getSelectedDeck();
    if (!selectedDeckId) {
        alert("Please select a deck");
        return;
    }

    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "join_specific_game",
        gameId: gameIdToJoin,
        deckId: selectedDeckId
    }));
    setStatus("Joining game...");
    showInGameLobby();
}

// Mulligan functions
function showMulliganUI() {
    const mulliganDiv = document.getElementById("mulligan-controls");
    if (mulliganDiv) {
        mulliganDiv.style.display = "block";
    }
    // Show game controls so hand is visible
    document.getElementById("game-controls").style.display = "block";
    setStatus("Mulligan phase - Keep your hand or draw a new one");
}

function hideMulliganUI() {
    const mulliganDiv = document.getElementById("mulligan-controls");
    if (mulliganDiv) {
        mulliganDiv.style.display = "none";
    }
}

function keepHand() {
    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "keep_hand"
    }));
}

function takeMulligan() {
    ws.send(JSON.stringify({
        playerUid: myUID,
        type: "mulligan"
    }));
}
