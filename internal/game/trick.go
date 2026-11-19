package game

import (
	"errors"
	"fmt"
)

// PlayCard validates and plays a card from the player's hand into the current trick.
// Returns (true, nil) when the last card of the trick has been played and is ready to resolve.
func (g *Game) PlayCard(playerIdx int, cardIdx int) (bool, error) {
	if g.Phase != PhasePlaying {
		return false, errors.New("not in playing phase")
	}
	if len(g.CurrentTrick) >= len(g.Players) {
		return false, errors.New("waiting for trick to clear")
	}
	if playerIdx != g.TurnIndex {
		return false, errors.New("not your turn to play")
	}

	player := g.Players[playerIdx]
	if cardIdx < 0 || cardIdx >= len(player.Hand) {
		return false, errors.New("invalid card index")
	}

	card := player.Hand[cardIdx]

	// Enforce follow-suit rule if not leading the trick
	if len(g.CurrentTrick) > 0 && string(card.Suit) != g.LedSuit {
		for _, c := range player.Hand {
			if string(c.Suit) == g.LedSuit {
				return false, fmt.Errorf("must follow led suit: %s", g.LedSuit)
			}
		}
	}

	if len(g.CurrentTrick) == 0 {
		g.LedSuit = string(card.Suit)
	}

	// Remove card from hand and place on table
	player.Hand = append(player.Hand[:cardIdx], player.Hand[cardIdx+1:]...)
	g.CurrentTrick = append(g.CurrentTrick, PlayedCard{
		PlayerIndex: playerIdx,
		Card:        card,
	})

	if len(g.CurrentTrick) == len(g.Players) {
		return true, nil
	}

	g.TurnIndex = (g.TurnIndex + 1) % len(g.Players)
	return false, nil
}

// GetTrickWinnerIndex returns the winning player's index when all cards in the trick are played, or -1 otherwise.
func (g *Game) GetTrickWinnerIndex() int {
	if len(g.CurrentTrick) == 0 || len(g.CurrentTrick) < len(g.Players) {
		return -1
	}

	winningPlay := g.CurrentTrick[0]

	for _, play := range g.CurrentTrick[1:] {
		playIsTrump := string(play.Card.Suit) == g.TrumpSuit
		winIsTrump := string(winningPlay.Card.Suit) == g.TrumpSuit

		if playIsTrump && !winIsTrump {
			winningPlay = play
		} else if playIsTrump && winIsTrump {
			if play.Card.Rank > winningPlay.Card.Rank {
				winningPlay = play
			}
		} else if !playIsTrump && !winIsTrump {
			if string(play.Card.Suit) == g.LedSuit && play.Card.Rank > winningPlay.Card.Rank {
				winningPlay = play
			}
		}
	}

	return winningPlay.PlayerIndex
}

// ResolveTrick awards the trick to the winner and advances to the next trick or round.
func (g *Game) ResolveTrick() {
	winnerIdx := g.GetTrickWinnerIndex()
	if winnerIdx == -1 {
		return
	}

	g.Players[winnerIdx].TricksWon++
	g.CurrentTrick = nil
	g.LedSuit = ""

	// Check if all cards for the round have been played
	if len(g.Players[0].Hand) == 0 {
		g.calculateScores()
		return
	}

	g.TurnIndex = winnerIdx
}

// calculateScores records per-round points, updates totals, and advances rounds.
func (g *Game) calculateScores() {
	for _, p := range g.Players {
		roundPoints := 0
		if p.Bid == p.TricksWon {
			roundPoints = 10 + p.TricksWon
			p.Score += roundPoints
		}
		p.RoundScores = append(p.RoundScores, roundPoints)
	}

	if g.RoundNumber >= g.TotalRounds {
		g.Phase = PhaseGameOver
		return
	}

	g.RoundNumber++
	g.DealerIndex = (g.DealerIndex + 1) % len(g.Players)
	g.StartRound()
}
