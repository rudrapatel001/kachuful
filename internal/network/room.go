package network

import (
	"sync"

	"backend/internal/game"

	"github.com/gorilla/websocket"
)

// Client wraps a WebSocket connection with a write mutex to prevent concurrent write panics.
type Client struct {
	Conn    *websocket.Conn
	ID      string
	WriteMu sync.Mutex
}

func (c *Client) WriteJSON(v interface{}) error {
	c.WriteMu.Lock()
	defer c.WriteMu.Unlock()
	return c.Conn.WriteJSON(v)
}

func (c *Client) WritePing() error {
	c.WriteMu.Lock()
	defer c.WriteMu.Unlock()
	return c.Conn.WriteMessage(websocket.PingMessage, nil)
}

type LobbyView struct {
	YourID  string   `json:"yourId"`
	HostID  string   `json:"hostId"`
	Players []string `json:"players"`
}

type PublicPlayerInfo struct {
	ID          string `json:"id"`
	Bid         int    `json:"bid"`
	TricksWon   int    `json:"tricksWon"`
	Score       int    `json:"score"`
	CardCount   int    `json:"cardCount"`
	RoundScores []int  `json:"roundScores"`
}

type GameStateView struct {
	YourID           string             `json:"yourId"`
	YourIndex        int                `json:"yourIndex"`
	YourHand         []game.Card        `json:"yourHand"`
	Phase            game.GamePhase     `json:"phase"`
	RoundNumber      int                `json:"roundNumber"`
	TotalRounds      int                `json:"totalRounds"`
	CardsDealt       int                `json:"cardsDealt"`
	TrumpSuit        string             `json:"trumpSuit"`
	TurnIndex        int                `json:"turnIndex"`
	DealerIndex      int                `json:"dealerIndex"`
	TrickWinnerIndex int                `json:"trickWinnerIndex"`
	CurrentTrick     []game.PlayedCard  `json:"currentTrick"`
	Players          []PublicPlayerInfo `json:"players"`
}

type Room struct {
	ID      string
	HostID  string
	Clients map[*websocket.Conn]*Client
	Game    *game.Game
	Mutex   sync.RWMutex
}

type RoomManager struct {
	Rooms map[string]*Room
	Mutex sync.RWMutex
}

func NewRoomManager() *RoomManager {
	return &RoomManager{
		Rooms: make(map[string]*Room),
	}
}

func (rm *RoomManager) CreateOrGetRoom(roomID string) *Room {
	rm.Mutex.Lock()
	defer rm.Mutex.Unlock()

	if room, exists := rm.Rooms[roomID]; exists {
		return room
	}

	newRoom := &Room{
		ID:      roomID,
		Clients: make(map[*websocket.Conn]*Client),
	}
	rm.Rooms[roomID] = newRoom
	return newRoom
}

func (r *Room) Broadcast(msg GameMessage) {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()

	for _, client := range r.Clients {
		if err := client.WriteJSON(msg); err != nil {
			client.Conn.Close()
		}
	}
}

func (r *Room) BroadcastLobby() {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()

	playerList := make([]string, 0, len(r.Clients))
	for _, client := range r.Clients {
		playerList = append(playerList, client.ID)
	}

	for _, client := range r.Clients {
		msg := GameMessage{
			Action: "LOBBY_UPDATE",
			Payload: LobbyView{
				YourID:  client.ID,
				HostID:  r.HostID,
				Players: playerList,
			},
		}
		if err := client.WriteJSON(msg); err != nil {
			client.Conn.Close()
		}
	}
}

func (r *Room) BroadcastGameState() {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()

	if r.Game == nil {
		return
	}

	publicPlayers := make([]PublicPlayerInfo, len(r.Game.Players))
	for i, p := range r.Game.Players {
		publicPlayers[i] = PublicPlayerInfo{
			ID:          p.ID,
			Bid:         p.Bid,
			TricksWon:   p.TricksWon,
			Score:       p.Score,
			CardCount:   len(p.Hand),
			RoundScores: p.RoundScores,
		}
	}

	trickWinnerIdx := r.Game.GetTrickWinnerIndex()

	for _, client := range r.Clients {
		playerIdx := -1
		var hand []game.Card

		for i, p := range r.Game.Players {
			if p.ID == client.ID {
				playerIdx = i
				hand = p.Hand
				break
			}
		}

		view := GameStateView{
			YourID:           client.ID,
			YourIndex:        playerIdx,
			YourHand:         hand,
			Phase:            r.Game.Phase,
			RoundNumber:      r.Game.RoundNumber,
			TotalRounds:      r.Game.TotalRounds,
			CardsDealt:       r.Game.CardsDealtThisRound,
			TrumpSuit:        r.Game.TrumpSuit,
			TurnIndex:        r.Game.TurnIndex,
			DealerIndex:      r.Game.DealerIndex,
			TrickWinnerIndex: trickWinnerIdx,
			CurrentTrick:     r.Game.CurrentTrick,
			Players:          publicPlayers,
		}

		err := client.WriteJSON(GameMessage{
			Action:  "GAME_STATE",
			Payload: view,
		})
		if err != nil {
			client.Conn.Close()
		}
	}
}

func (r *Room) RemoveClient(conn *websocket.Conn) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	client, exists := r.Clients[conn]
	if !exists {
		return
	}
	delete(r.Clients, conn)

	if client.ID == r.HostID {
		r.HostID = ""
		for _, c := range r.Clients {
			r.HostID = c.ID
			break
		}
	}
}
