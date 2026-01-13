package controllers

import (
	"chat-integrated/models"
	"chat-integrated/services"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type WSController struct {
	BaseController
	lobbyService *services.LobbyService
	upgrader     websocket.Upgrader
}

func NewWSController(lobbyService *services.LobbyService) *WSController {
	return &WSController{
		lobbyService: lobbyService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// Add this public method to expose upgrader functionality
func (wsc *WSController) UpgradeConnection(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return wsc.upgrader.Upgrade(w, r, nil)
}

func (wsc *WSController) ReadPump(client *models.Client) {
	defer func() {
		wsc.lobbyService.Unregister <- client
		client.Conn.Close()
	}()

	for {
		var msg models.Message
		err := client.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		msg.Type = models.MessageTypeChat
		msg.Username = client.Email
		msg.LobbyID = client.LobbyID
		msg.Timestamp = time.Now()

		wsc.lobbyService.Broadcast <- services.BroadcastMessage{
			LobbyID: client.LobbyID,
			Message: msg,
		}
	}
}

func (wsc *WSController) WritePump(client *models.Client) {
	defer func() {
		client.Conn.Close()
		log.Printf("🔌 WritePump closed for: %s", client.Email)
	}()

	for message := range client.Send {
		err := client.Conn.WriteJSON(message)
		if err != nil {
			log.Printf("❌ Write error for %s: %v", client.Email, err)
			return
		}
	}
}
