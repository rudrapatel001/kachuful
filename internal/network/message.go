package network

// GameMessage is the standard JSON envelope for all socket communication.
type GameMessage struct {
	Action  string      `json:"action"` // e.g., "JOIN", "BID", "PLAY_CARD"
	Player  string      `json:"player,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
}
