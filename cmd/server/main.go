package main

import (
    "log"
    "net/http"

    "card-game/game"
    "card-game/server"
)

func main() {
    // Load cards and decks
    if err := game.LoadCards("data/cards.json"); err != nil {
        log.Fatal("Failed to load cards:", err)
    }
    log.Printf("Loaded %d cards", len(game.CardDB))

    if err := game.LoadDecks("data/decks.json"); err != nil {
        log.Fatal("Failed to load decks:", err)
    }
    log.Printf("Loaded %d decks", len(game.DeckDB))

    router := server.NewRouter()

    log.Println("Server running on :8080")
    err := http.ListenAndServe(":8080", router)
    if err != nil {
        log.Fatal(err)
    }
}
