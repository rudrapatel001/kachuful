package game

import (
	"errors"
	"fmt"
	"sort"
)

type GamePhase string

const (
	PhaseBidding  GamePhase = "BIDDING"
	PhasePlaying  GamePhase = "PLAYING"
	PhaseGameOver GamePhase = "GAME_OVER"
)

type PlayerState struct {
	ID          string
	Hand        []Card
	Bid         int
	TricksWon   int
	Score       int
	RoundScores []int
}

type PlayedCard struct {
	PlayerIndex int
	Card        Card
}

type Game struct {
	Players             []*PlayerState
	RoundNumber         int
	TotalRounds         int
	CardsDealtThisRound int
	DealerIndex         int
	TurnIndex           int
	Phase               GamePhase
	TrumpSuit           string

	CurrentTrick []PlayedCard
	LedSuit      string
}

var suitOrder = map[Suit]int{
	Spades:   0,
	Diamonds: 1,
	Clubs:    2,
	Hearts:   3,
}

func sortHand(hand []Card) {
	sort.Slice(hand, func(i, j int) bool {
		if suitOrder[hand[i].Suit] != suitOrder[hand[j].Suit] {
			return suitOrder[hand[i].Suit] < suitOrder[hand[j].Suit]
		}
		return hand[i].Rank < hand[j].Rank
	})
}

// NewGame initializes a new game state for the room with a host-selected round limit.
func NewGame(playerIDs []string, totalRounds int) *Game {
	if totalRounds < 4 || totalRounds > 40 || totalRounds%4 != 0 {
		totalRounds = 4
	}

	players := make([]*PlayerState, len(playerIDs))
	for i, id := range playerIDs {
		players[i] = &PlayerState{
			ID:          id,
			Bid:         -1,
			RoundScores: make([]int, 0, totalRounds),
		}
	}

	g := &Game{
		Players:     players,
		RoundNumber: 1,
		TotalRounds: totalRounds,
		DealerIndex: 0,
	}

	g.StartRound()
	return g
}

// StartRound deals 1-4 sorted cards and sets the Trump suit based on Ka-Chu-Fu-L (4-round cycle).
func (g *Game) StartRound() {
	playerCount := len(g.Players)
	cycle := (g.RoundNumber - 1) % 4
	g.CardsDealtThisRound = cycle + 1

	// Ka (Spades=1), Chu (Diamonds=2), Fu (Clubs=3), L (Hearts=4)
	switch cycle {
	case 0:
		g.TrumpSuit = string(Spades)
	case 1:
		g.TrumpSuit = string(Diamonds)
	case 2:
		g.TrumpSuit = string(Clubs)
	case 3:
		g.TrumpSuit = string(Hearts)
	}

	hands := Deal(playerCount, g.RoundNumber)
	for i, p := range g.Players {
		sortHand(hands[i])
		p.Hand = hands[i]
		p.Bid = -1
		p.TricksWon = 0
	}

	g.CurrentTrick = nil
	g.LedSuit = ""
	g.Phase = PhaseBidding
	g.TurnIndex = (g.DealerIndex + 1) % playerCount
}

// SubmitBid validates and records a player's bid, enforcing the Golden Rule on the dealer.
func (g *Game) SubmitBid(playerIdx int, bid int) error {
	if g.Phase != PhaseBidding {
		return errors.New("not in bidding phase")
	}
	if playerIdx != g.TurnIndex {
		return errors.New("not your turn to bid")
	}
	if bid < 0 || bid > g.CardsDealtThisRound {
		return fmt.Errorf("bid must be between 0 and %d", g.CardsDealtThisRound)
	}

	// Golden Rule: dealer cannot make total bids equal to total cards dealt
	if playerIdx == g.DealerIndex {
		totalBids := bid
		for i, p := range g.Players {
			if i != playerIdx && p.Bid >= 0 {
				totalBids += p.Bid
			}
		}
		if totalBids == g.CardsDealtThisRound {
			return fmt.Errorf("Golden Rule: total bids cannot equal %d", g.CardsDealtThisRound)
		}
	}

	g.Players[playerIdx].Bid = bid

	if playerIdx == g.DealerIndex {
		g.Phase = PhasePlaying

		// Highest bidder leads the first trick; ties go to whoever bid it first
		playerCount := len(g.Players)
		startIdx := (g.DealerIndex + 1) % playerCount
		leaderIdx := startIdx
		highestBid := -1

		for i := 0; i < playerCount; i++ {
			idx := (startIdx + i) % playerCount
			if g.Players[idx].Bid > highestBid {
				highestBid = g.Players[idx].Bid
				leaderIdx = idx
			}
		}

		g.TurnIndex = leaderIdx
	} else {
		g.TurnIndex = (g.TurnIndex + 1) % len(g.Players)
	}

	return nil
}
