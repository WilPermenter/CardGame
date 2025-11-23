package game

import (
	"encoding/json"
	"fmt"
	"os"
)

const MaxMainDeckSize = 30
const MaxVaultSize = 15

type Deck struct {
	ID       int    `json:"ID"`
	Name     string `json:"Name"`
	Leader   int    `json:"Leader"`   // Card ID of the leader
	MainDeck []int  `json:"MainDeck"` // Creature cards
	Vault    []int  `json:"Vault"`    // Land cards
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
		if len(deck.MainDeck) > MaxMainDeckSize {
			return fmt.Errorf("deck %q has %d main deck cards, max is %d", deck.Name, len(deck.MainDeck), MaxMainDeckSize)
		}
		if len(deck.Vault) > MaxVaultSize {
			return fmt.Errorf("deck %q has %d vault cards, max is %d", deck.Name, len(deck.Vault), MaxVaultSize)
		}
		DeckDB[deck.ID] = deck
	}

	return nil
}
