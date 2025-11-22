package game

import (
	"encoding/json"
	"os"
)

type ManaCost struct {
	White     int `json:"White,omitempty"`
	Blue      int `json:"Blue,omitempty"`
	Black     int `json:"Black,omitempty"`
	Red       int `json:"Red,omitempty"`
	Green     int `json:"Green,omitempty"`
	Colorless int `json:"Colorless,omitempty"`
}

// Total returns the total mana (colored + colorless)
func (m ManaCost) Total() int {
	return m.White + m.Blue + m.Black + m.Red + m.Green + m.Colorless
}

// ColoredTotal returns just the colored mana total (excluding colorless)
func (m ManaCost) ColoredTotal() int {
	return m.White + m.Blue + m.Black + m.Red + m.Green
}

// CanAfford checks if available mana can pay the cost
// Colorless cost can be paid with any color
func (available ManaCost) CanAfford(cost ManaCost) bool {
	// First check colored requirements
	if available.White < cost.White ||
		available.Blue < cost.Blue ||
		available.Black < cost.Black ||
		available.Red < cost.Red ||
		available.Green < cost.Green {
		return false
	}

	// Calculate remaining mana after paying colored costs
	remaining := available.White - cost.White +
		available.Blue - cost.Blue +
		available.Black - cost.Black +
		available.Red - cost.Red +
		available.Green - cost.Green +
		available.Colorless

	// Check if we can pay colorless cost with remaining
	return remaining >= cost.Colorless
}

// Spend deducts the cost from the mana pool (modifies in place via pointer)
// Returns the updated pool after spending
func (pool *ManaCost) Spend(cost ManaCost) {
	// Deduct colored costs first
	pool.White -= cost.White
	pool.Blue -= cost.Blue
	pool.Black -= cost.Black
	pool.Red -= cost.Red
	pool.Green -= cost.Green

	// Deduct colorless from any remaining (prioritize colorless mana first)
	remaining := cost.Colorless
	if pool.Colorless >= remaining {
		pool.Colorless -= remaining
	} else {
		remaining -= pool.Colorless
		pool.Colorless = 0
		// Spend from colors (in order: white, blue, black, red, green)
		colors := []*int{&pool.White, &pool.Blue, &pool.Black, &pool.Red, &pool.Green}
		for _, c := range colors {
			if remaining <= 0 {
				break
			}
			if *c >= remaining {
				*c -= remaining
				remaining = 0
			} else {
				remaining -= *c
				*c = 0
			}
		}
	}
}

// Clear resets all mana to zero
func (pool *ManaCost) Clear() {
	pool.White = 0
	pool.Blue = 0
	pool.Black = 0
	pool.Red = 0
	pool.Green = 0
	pool.Colorless = 0
}

type Card struct {
	ID                 int      `json:"ID"`
	Name               string   `json:"Name"`
	Cost               ManaCost `json:"Cost"`
	Provides           ManaCost `json:"Provides"` // What mana this land produces
	Attack             int      `json:"Attack"`
	Defense            int      `json:"Defense"`
	CardType           string   `json:"CardType"`
	CardText           string   `json:"CardText"`
	Abilities          []string `json:"Abilities"`
	ValidAttackTargets string   `json:"ValidAttackTargets"`
	CustomScript       string   `json:"CustomScript"`
}

// HasAbility checks if the card has a specific ability
func (c Card) HasAbility(ability string) bool {
	for _, a := range c.Abilities {
		if a == ability {
			return true
		}
	}
	return false
}

// GetProvidedMana returns the mana this card provides (for lands)
func (c Card) GetProvidedMana() ManaCost {
	// If Provides is explicitly set, use it
	if c.Provides.Total() > 0 {
		return c.Provides
	}
	// Otherwise, derive from card name for basic lands
	if c.CardType == "Land" {
		switch {
		case contains(c.Name, "White"):
			return ManaCost{White: 1}
		case contains(c.Name, "Blue"):
			return ManaCost{Blue: 1}
		case contains(c.Name, "Black"):
			return ManaCost{Black: 1}
		case contains(c.Name, "Red"):
			return ManaCost{Red: 1}
		case contains(c.Name, "Green"):
			return ManaCost{Green: 1}
		}
	}
	return ManaCost{}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

var CardDB = map[int]Card{}

func LoadCards(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cards []Card
	if err := json.Unmarshal(data, &cards); err != nil {
		return err
	}

	for _, card := range cards {
		CardDB[card.ID] = card
	}

	return nil
}
