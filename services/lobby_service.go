package services

import (
	"chat-integrated/config"
	"chat-integrated/models"
	"fmt"
	"log"
	"sync"
	"time"
)

type LobbyService struct {
	lobbies      map[string]*models.Lobby
	mu           sync.RWMutex
	Broadcast    chan BroadcastMessage
	Register     chan *models.Client
	Unregister   chan *models.Client
	redisService *RedisService
}

type BroadcastMessage struct {
	LobbyID string
	Message models.Message
}

func NewLobbyService(redisService *RedisService) *LobbyService {
	return &LobbyService{
		lobbies:      make(map[string]*models.Lobby),
		Broadcast:    make(chan BroadcastMessage),
		Register:     make(chan *models.Client),
		Unregister:   make(chan *models.Client),
		redisService: redisService,
	}
}

func (ls *LobbyService) GetOrCreateLobby() *models.Lobby {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// Find an available lobby that's not full
	for _, lobby := range ls.lobbies {
		if lobby.CanAcceptNewUsers() {
			log.Printf("📍 Using existing lobby: %s (Users: %d/%d)", lobby.ID, lobby.GetUserCount(), lobby.MaxUsers)
			return lobby
		}
	}

	// Check if there are any lobbies that are full (active session)
	// If yes, don't create new lobby - return nil
	for _, lobby := range ls.lobbies {
		if lobby.IsFull() {
			log.Printf("❌ Active session exists. Cannot create new lobby until current session ends.")
			return nil
		}
	}

	// Create new lobby only if NO lobbies exist
	lobbyID := fmt.Sprintf("lobby-%d", time.Now().Unix())
	lobby := models.NewLobby(lobbyID, config.MaxUsersPerLobby)
	ls.lobbies[lobbyID] = lobby
	log.Printf("🆕 Created new lobby: %s", lobbyID)
	return lobby
}

func (ls *LobbyService) GetLobby(lobbyID string) *models.Lobby {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.lobbies[lobbyID]
}

func (ls *LobbyService) GetMostRecentLobby() *models.Lobby {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	var currentLobby *models.Lobby
	var newestTime time.Time

	for _, lobby := range ls.lobbies {
		if lobby.CreatedAt.After(newestTime) {
			newestTime = lobby.CreatedAt
			currentLobby = lobby
		}
	}

	return currentLobby
}

func (ls *LobbyService) GetAvailableLobby() *models.Lobby {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	// Look for a lobby that can accept new users
	for _, lobby := range ls.lobbies {
		if lobby.CanAcceptNewUsers() {
			return lobby
		}
	}

	return nil
}

func (ls *LobbyService) FindLobbyByUserEmail(email string) *models.Lobby {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	for _, lobby := range ls.lobbies {
		if lobby.IsUserInLobby(email) {
			log.Printf("🔍 Found existing lobby for %s: %s", email, lobby.ID)
			return lobby
		}
	}

	log.Printf("🔍 No existing lobby found for: %s", email)
	return nil
}

func (ls *LobbyService) Run() {
	for {
		select {
		case client := <-ls.Register:
			ls.handleRegister(client)

		case client := <-ls.Unregister:
			ls.handleUnregister(client)

		case broadcastMsg := <-ls.Broadcast:
			ls.handleBroadcast(broadcastMsg)
		}
	}
}

func (ls *LobbyService) handleRegister(client *models.Client) {
	log.Printf("🔧 handleRegister called for: %s in lobby: %s", client.Email, client.LobbyID)

	lobby := ls.GetLobby(client.LobbyID)
	if lobby == nil {
		log.Printf("❌ Lobby not found: %s", client.LobbyID)
		client.Conn.Close()
		return
	}

	// Add client to lobby
	lobby.AddClient(client.Email, client)
	connectedCount := lobby.GetConnectedClientCount()

	log.Printf("✅ Client registered in handleRegister: %s (%d/%d)", client.Email, connectedCount, config.MaxUsersPerLobby)

	// Send welcome message to this client
	welcomeAction := models.SystemActionWelcome
	welcomeMsg := models.Message{
		Type:         models.MessageTypeSystemAction,
		SystemAction: &welcomeAction,
		Content:      fmt.Sprintf("Welcome back, %s! 🎉", client.Email),
		LobbyID:      client.LobbyID,
		UserCount:    lobby.GetActiveUserCount(),
		MaxUsers:     config.MaxUsersPerLobby,
		UserList:     lobby.GetActiveUserList(),
		Timestamp:    time.Now(),
	}

	log.Printf("📝 Sending welcome message to: %s (UserCount: %d)", client.Email, lobby.GetActiveUserCount())
	client.Send <- welcomeMsg
	log.Printf("✅ Welcome message queued for: %s", client.Email)

	// Send message history to reconnecting user
	messageHistory := lobby.GetMessageHistory()
	log.Printf("📚 Sending %d history messages to: %s", len(messageHistory), client.Email)
	for _, historyMsg := range messageHistory {
		client.Send <- historyMsg
	}

	// Check if all users are connected
	if connectedCount == config.MaxUsersPerLobby {
		lobby.StartWebSocket()
		log.Printf("🚀 WebSocket session started for lobby: %s (All %d users connected)", client.LobbyID, config.MaxUsersPerLobby)
	}

	// Broadcast user joined to all clients
	userJoinedAction := models.SystemActionUserJoined
	joinMsg := models.Message{
		Type:         models.MessageTypeSystemAction,
		SystemAction: &userJoinedAction,
		Username:     client.Email,
		Content:      fmt.Sprintf("%s joined the chat", client.Email),
		LobbyID:      client.LobbyID,
		UserCount:    lobby.GetActiveUserCount(),
		MaxUsers:     config.MaxUsersPerLobby,
		UserList:     lobby.GetActiveUserList(),
		Timestamp:    time.Now(),
	}

	log.Printf("📢 Broadcasting user joined for: %s", client.Email)

	// NON-BLOCKING send to avoid deadlock
	go func() {
		ls.Broadcast <- BroadcastMessage{
			LobbyID: client.LobbyID,
			Message: joinMsg,
		}
	}()

	log.Printf("✅ Join message queued for broadcast")
}

func (ls *LobbyService) handleUnregister(client *models.Client) {
	lobby := ls.GetLobby(client.LobbyID)
	if lobby == nil {
		return
	}

	// Remove client and mark user as inactive
	lobby.RemoveClient(client.Email)
	close(client.Send)
	lobby.MarkUserInactive(client.Email)

	connectedCount := lobby.GetConnectedClientCount()

	log.Printf("👋 Client disconnected from lobby %s: %s (%d/%d remaining)", client.LobbyID, client.Email, connectedCount, config.MaxUsersPerLobby)

	// Broadcast user left to remaining clients
	userLeftAction := models.SystemActionUserLeft
	leaveMsg := models.Message{
		Type:         models.MessageTypeSystemAction,
		SystemAction: &userLeftAction,
		Username:     client.Email,
		Content:      fmt.Sprintf("%s left the chat", client.Email),
		LobbyID:      client.LobbyID,
		UserCount:    lobby.GetActiveUserCount(),
		MaxUsers:     config.MaxUsersPerLobby,
		UserList:     lobby.GetActiveUserList(),
		Timestamp:    time.Now(),
	}

	// NON-BLOCKING send
	go func() {
		ls.Broadcast <- BroadcastMessage{
			LobbyID: client.LobbyID,
			Message: leaveMsg,
		}
	}()
}

func (ls *LobbyService) handleBroadcast(broadcastMsg BroadcastMessage) {
	log.Printf("📣 handleBroadcast called for lobby: %s, type: %s", broadcastMsg.LobbyID, broadcastMsg.Message.Type)

	lobby := ls.GetLobby(broadcastMsg.LobbyID)
	if lobby == nil {
		log.Printf("❌ Lobby not found in broadcast: %s", broadcastMsg.LobbyID)
		return
	}

	// Store message in history if it's a chat message
	if broadcastMsg.Message.Type == models.MessageTypeChat {
		lobby.AddMessageToHistory(broadcastMsg.Message)

		// Push to Redis
		err := ls.redisService.PushMessage(
			broadcastMsg.Message.Username,
			broadcastMsg.Message.Content,
			broadcastMsg.Message.LobbyID,
			broadcastMsg.Message.Timestamp,
		)
		if err != nil {
			log.Printf("⚠️ Failed to push message to Redis: %v", err)
		}
	}

	// Broadcast to all connected clients in this lobby
	clients := lobby.GetAllClients()
	log.Printf("📤 Broadcasting to %d clients in lobby %s", len(clients), broadcastMsg.LobbyID)

	for email, client := range clients {
		select {
		case client.Send <- broadcastMsg.Message:
			log.Printf("✅ Message delivered to: %s", email)
		default:
			log.Printf("❌ Failed to deliver message to: %s (channel full or closed)", email)
			lobby.RemoveClient(email)
			close(client.Send)
		}
	}
}
