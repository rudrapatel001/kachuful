package network

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"backend/internal/game"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func ServeWS(rm *RoomManager, w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		http.Error(w, "Room ID is required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	room := rm.CreateOrGetRoom(roomID)

	rawName := strings.TrimSpace(r.URL.Query().Get("name"))
	if rawName == "" {
		rawName = "Player_" + conn.RemoteAddr().String()
	}

	room.Mutex.Lock()
	var playerID string
	reconnected := false

	// If a game is active and this name belongs to a player in the match, restore their seat
	if room.Game != nil && findPlayerIndex(room.Game, rawName) != -1 {
		playerID = rawName
		reconnected = true

		// Close and replace any stale socket still holding this player's name
		for oldConn, existingClient := range room.Clients {
			if existingClient.ID == rawName {
				delete(room.Clients, oldConn)
				oldConn.Close()
				break
			}
		}
	} else {
		playerID = rawName
		suffix := 2
		for isNameTaken(room, playerID) {
			playerID = fmt.Sprintf("%s_%d", rawName, suffix)
			suffix++
		}
	}

	client := &Client{
		Conn: conn,
		ID:   playerID,
	}
	room.Clients[conn] = client
	if room.HostID == "" {
		room.HostID = playerID
	}
	room.Mutex.Unlock()

	if reconnected {
		log.Printf("Client %s reconnected to active game in room [%s]", playerID, roomID)
		room.BroadcastGameState()
	} else {
		log.Printf("Client %s connected to room [%s]", playerID, roomID)
		room.BroadcastLobby()
	}

	go handleClientMessages(room, client)
}

func sendError(client *Client, message string) {
	_ = client.WriteJSON(GameMessage{
		Action:  "ERROR",
		Payload: message,
	})
}

func handleClientMessages(room *Room, client *Client) {
	done := make(chan struct{})

	// 30-second keep-alive ping to prevent cloud proxy idle disconnects
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := client.WritePing(); err != nil {
					client.Conn.Close()
					return
				}
			case <-done:
				return
			}
		}
	}()

	defer func() {
		close(done)
		room.RemoveClient(client.Conn)
		client.Conn.Close()
		room.Mutex.RLock()
		inLobby := room.Game == nil
		room.Mutex.RUnlock()
		if inLobby {
			room.BroadcastLobby()
		}
		log.Printf("Client %s left room [%s]", client.ID, room.ID)
	}()

	for {
		var msg GameMessage
		if err := client.Conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message from %s: %v", client.ID, err)
			}
			break
		}

		log.Printf("Room %s | %s -> %s", room.ID, client.ID, msg.Action)

		switch msg.Action {
		case "START_GAME":
			roundsFloat, ok := msg.Payload.(float64)
			totalRounds := 4
			if ok {
				totalRounds = int(roundsFloat)
			}
			if totalRounds < 4 || totalRounds > 40 || totalRounds%4 != 0 {
				sendError(client, "Total rounds must be a multiple of 4 between 4 and 40")
				continue
			}

			room.Mutex.Lock()
			if client.ID != room.HostID {
				room.Mutex.Unlock()
				sendError(client, "Only the host can start the game")
				continue
			}
			if len(room.Clients) < 2 || len(room.Clients) > 13 {
				room.Mutex.Unlock()
				sendError(client, "Need between 2 and 13 players to start")
				continue
			}

			playerIDs := make([]string, 0, len(room.Clients))
			for _, c := range room.Clients {
				playerIDs = append(playerIDs, c.ID)
			}
			room.Game = game.NewGame(playerIDs, totalRounds)
			room.Mutex.Unlock()

			room.BroadcastGameState()

		case "BID":
			bidFloat, ok := msg.Payload.(float64)
			if !ok {
				sendError(client, "Invalid bid format")
				continue
			}

			room.Mutex.Lock()
			if room.Game == nil {
				room.Mutex.Unlock()
				sendError(client, "Game has not started")
				continue
			}

			playerIdx := findPlayerIndex(room.Game, client.ID)
			err := room.Game.SubmitBid(playerIdx, int(bidFloat))
			room.Mutex.Unlock()

			if err != nil {
				sendError(client, err.Error())
				continue
			}
			room.BroadcastGameState()

		case "PLAY_CARD":
			cardIdxFloat, ok := msg.Payload.(float64)
			if !ok {
				sendError(client, "Invalid card index format")
				continue
			}

			room.Mutex.Lock()
			if room.Game == nil {
				room.Mutex.Unlock()
				sendError(client, "Game has not started")
				continue
			}

			playerIdx := findPlayerIndex(room.Game, client.ID)
			trickComplete, err := room.Game.PlayCard(playerIdx, int(cardIdxFloat))
			room.Mutex.Unlock()

			if err != nil {
				sendError(client, err.Error())
				continue
			}
			room.BroadcastGameState()

			if trickComplete {
				go func(r *Room) {
					time.Sleep(1500 * time.Millisecond)
					r.Mutex.Lock()
					if r.Game != nil && len(r.Game.CurrentTrick) == len(r.Game.Players) {
						r.Game.ResolveTrick()
					}
					r.Mutex.Unlock()
					r.BroadcastGameState()
				}(room)
			}

		case "BACK_TO_LOBBY":
			room.Mutex.Lock()
			if room.Game != nil && room.Game.Phase == game.PhaseGameOver {
				room.Game = nil
			}
			room.Mutex.Unlock()

			room.Broadcast(GameMessage{Action: "RETURN_TO_LOBBY"})
			room.BroadcastLobby()
		}
	}
}

func findPlayerIndex(g *game.Game, playerID string) int {
	for i, p := range g.Players {
		if p.ID == playerID {
			return i
		}
	}
	return -1
}

func isNameTaken(room *Room, name string) bool {
	for _, existing := range room.Clients {
		if existing.ID == name {
			return true
		}
	}
	return false
}
