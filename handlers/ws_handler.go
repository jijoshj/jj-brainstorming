package handlers

import (
	"chat-integrated/controllers"
	"chat-integrated/models"
	"chat-integrated/services"
	"log"
	"net/http"
	"time"
)

type WSHandler struct {
	controller   *controllers.WSController
	lobbyService *services.LobbyService
}

func NewWSHandler(controller *controllers.WSController, lobbyService *services.LobbyService) *WSHandler {
	return &WSHandler{
		controller:   controller,
		lobbyService: lobbyService,
	}
}

func (wh *WSHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	lobbyID := r.URL.Query().Get("lobby_id")

	if email == "" || lobbyID == "" {
		log.Println("❌ WebSocket connection rejected: email or lobby_id missing")
		http.Error(w, "Email and lobby_id are required", http.StatusBadRequest)
		return
	}

	// Get lobby
	lobby := wh.lobbyService.GetLobby(lobbyID)
	if lobby == nil {
		log.Printf("❌ Lobby not found: %s", lobbyID)
		http.Error(w, "Lobby not found", http.StatusNotFound)
		return
	}

	// Check if user is in the lobby
	if !lobby.IsUserInLobby(email) {
		log.Printf("❌ User not in lobby: %s", email)
		http.Error(w, "User not authorized for this lobby", http.StatusForbidden)
		return
	}

	log.Printf("🔌 Attempting WebSocket upgrade for user: %s in lobby: %s", email, lobbyID)

	// Upgrade connection to WebSocket
	conn, err := wh.controller.UpgradeConnection(w, r)
	if err != nil {
		log.Printf("❌ WebSocket upgrade failed for %s: %v", email, err)
		return
	}

	log.Printf("✅ WebSocket upgrade successful for user: %s", email)

	client := &models.Client{
		Email:    email,
		LobbyID:  lobbyID,
		Conn:     conn,
		Send:     make(chan models.Message, 256),
		JoinedAt: time.Now(),
	}

	// CRITICAL FIX: Start goroutines BEFORE registering
	// This ensures WritePump is listening when messages are sent
	go wh.controller.WritePump(client)
	go wh.controller.ReadPump(client)

	// Small delay to ensure goroutines are running
	time.Sleep(50 * time.Millisecond)

	// Now register the client (this will send welcome messages)
	wh.lobbyService.Register <- client
}
