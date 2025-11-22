package game

import (
	"encoding/json"
	"fmt"
	"os"
)

const MaxDeckSize = 39

type Deck struct {
	ID     int    `json:"ID"`
	Name   string `json:"Name"`
	Leader int    `json:"Leader"` // Card ID of the leader
	Cards  []int  `json:"Cards"`  // Array of card IDs (max 39)
}

var DeckDB = map[int]Deck{}

func LoadDecks(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var decks []Deck
	if err := json.Unmarshal(data, &decks); err != nil {
		return err
	}

	for _, deck := range decks {
		if len(deck.Cards) > MaxDeckSize {
			return fmt.Errorf("deck %q has %d cards, max is %d", deck.Name, len(deck.Cards), MaxDeckSize)
		}
		DeckDB[deck.ID] = deck
	}

	return nil
}
