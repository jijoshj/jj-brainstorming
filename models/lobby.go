package models

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	Email    string
	LobbyID  string
	Conn     *websocket.Conn
	Send     chan Message
	JoinedAt time.Time
}

type Lobby struct {
	ID               string
	Users            map[string]*User
	Clients          map[string]*Client
	MaxUsers         int
	IsActive         bool
	CreatedAt        time.Time
	WebSocketStarted bool
	MessageHistory   []Message
	mu               sync.RWMutex
}

func NewLobby(id string, maxUsers int) *Lobby {
	return &Lobby{
		ID:               id,
		Users:            make(map[string]*User),
		Clients:          make(map[string]*Client),
		MaxUsers:         maxUsers,
		IsActive:         false,
		CreatedAt:        time.Now(),
		WebSocketStarted: false,
		MessageHistory:   make([]Message, 0),
	}
}

func (l *Lobby) AddUser(email string) *User {
	l.mu.Lock()
	defer l.mu.Unlock()

	if user, exists := l.Users[email]; exists {
		user.IsActive = true
		user.LastSeen = time.Now()
		return user
	}

	user := &User{
		Email:    email,
		LobbyID:  l.ID,
		JoinedAt: time.Now(),
		IsActive: true,
		LastSeen: time.Now(),
	}
	l.Users[email] = user
	return user
}

func (l *Lobby) AddClient(email string, client *Client) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Clients[email] = client
}

func (l *Lobby) RemoveClient(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.Clients, email)
}

func (l *Lobby) MarkUserInactive(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if user, exists := l.Users[email]; exists {
		user.IsActive = false
		user.LastSeen = time.Now()
	}
}

func (l *Lobby) GetActiveUserCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	count := 0
	for _, user := range l.Users {
		if user.IsActive {
			count++
		}
	}
	return count
}

func (l *Lobby) GetConnectedClientCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.Clients)
}

func (l *Lobby) GetActiveUserList() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	userList := make([]string, 0)
	for email, user := range l.Users {
		if user.IsActive {
			userList = append(userList, email)
		}
	}
	return userList
}

func (l *Lobby) GetAllClients() map[string]*Client {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return a copy to avoid concurrent map access
	clients := make(map[string]*Client)
	for k, v := range l.Clients {
		clients[k] = v
	}
	return clients
}

func (l *Lobby) GetMessageHistory() []Message {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return a copy
	history := make([]Message, len(l.MessageHistory))
	copy(history, l.MessageHistory)
	return history
}

func (l *Lobby) AddMessageToHistory(msg Message) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.MessageHistory = append(l.MessageHistory, msg)
}

func (l *Lobby) StartWebSocket() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.WebSocketStarted = true
	l.IsActive = true
}

func (l *Lobby) IsWebSocketStarted() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.WebSocketStarted
}

func (l *Lobby) CanAcceptNewUsers() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.Users) < l.MaxUsers
}

func (l *Lobby) IsUserInLobby(email string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, exists := l.Users[email]
	return exists
}

func (l *Lobby) IsFull() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.Users) >= l.MaxUsers
}

func (l *Lobby) GetUserCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.Users)
}
