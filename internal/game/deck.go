package game

import (
	"math/rand"
	"time"
)

// Suit represents the card suit
type Suit string

// The four suits mapped for the Rang sequence
const (
	Spades   Suit = "Spades"   // Kali
	Diamonds Suit = "Diamonds" // Chokat
	Clubs    Suit = "Clubs"    // Phulli
	Hearts   Suit = "Hearts"   // Lal
)

// Rank represents the card value
type Rank int

// 2 is low, Ace is high
const (
	Two Rank = iota + 2
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
)

// Card represents a single playing card
type Card struct {
	Suit Suit `json:"suit"`
	Rank Rank `json:"rank"`
}

type Deck []Card

// NewDeck creates a standard 52-card deck (no jokers)
func NewDeck() Deck {
	suits := []Suit{Spades, Diamonds, Clubs, Hearts}
	deck := make(Deck, 0, 52)

	for _, suit := range suits {
		for rank := Two; rank <= Ace; rank++ {
			deck = append(deck, Card{Suit: suit, Rank: rank})
		}
	}
	return deck
}

// Shuffle randomizes the deck in place
func (d Deck) Shuffle() {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(d), func(i, j int) {
		d[i], d[j] = d[j], d[i]
	})
}

// Deal distributes cards for a specific round of Kachuful.
// Round 1 deals max cards, Round 2 deals max - 1, down to 1 card.
// Deal distributes 1 to 4 cards per player in a repeating 4-round cycle.
func Deal(playerCount int, roundNumber int) [][]Card {
	deck := NewDeck()
	deck.Shuffle()

	// Round 1 -> 1 card, Round 2 -> 2, Round 3 -> 3, Round 4 -> 4, Round 5 -> 1...
	cardsToDeal := ((roundNumber - 1) % 4) + 1

	hands := make([][]Card, playerCount)
	deckIndex := 0

	for i := 0; i < playerCount; i++ {
		hands[i] = make([]Card, cardsToDeal)
		for j := 0; j < cardsToDeal; j++ {
			hands[i][j] = deck[deckIndex]
			deckIndex++
		}
	}

	return hands
}
