let ws = new WebSocket("ws://localhost:8080/ws");

ws.onmessage = (msg) => {
    const events = JSON.parse(msg.data);
    document.getElementById("log").textContent += JSON.stringify(events, null, 2) + "\n";
};

function playCard() {
    ws.send(JSON.stringify({
        playerId: 1,
        type: "play_card",
        cardId: 1,
    }));
}

function endTurn() {
    ws.send(JSON.stringify({
        playerId: 1, 
        type: "end_turn"
    }));
}
