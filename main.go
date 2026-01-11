package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type User struct {
	Email    string    `json:"email"`
	LobbyID  string    `json:"lobby_id"`
	JoinedAt time.Time `json:"joined_at"`
}

type LoginRequest struct {
	Email string `json:"email"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	LobbyID string `json:"lobby_id,omitempty"`
	Email   string `json:"email,omitempty"`
}

type Lobby struct {
	Users    []User
	MaxUsers int
	mu       sync.RWMutex
}

var currentLobby *Lobby

func init() {
	currentLobby = &Lobby{
		Users:    make([]User, 0),
		MaxUsers: 5,
	}
}

func generateLobbyID() string {
	return fmt.Sprintf("lobby-%d", time.Now().Unix())
}

func (l *Lobby) AddUser(email string) (*LoginResponse, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if user already exists
	for _, user := range l.Users {
		if user.Email == email {
			return &LoginResponse{
				Success: true,
				Message: "User already in lobby",
				LobbyID: user.LobbyID,
				Email:   email,
			}, nil
		}
	}

	// Check if lobby is full
	if len(l.Users) >= l.MaxUsers {
		return &LoginResponse{
			Success: false,
			Message: "Lobby is full. Please wait for the current chat to complete.",
		}, nil
	}

	// Generate lobby ID for first user, reuse for others
	var lobbyID string
	if len(l.Users) == 0 {
		lobbyID = generateLobbyID()
	} else {
		lobbyID = l.Users[0].LobbyID
	}

	// Add new user
	newUser := User{
		Email:    email,
		LobbyID:  lobbyID,
		JoinedAt: time.Now(),
	}
	l.Users = append(l.Users, newUser)

	return &LoginResponse{
		Success: true,
		Message: "User registered successfully",
		LobbyID: lobbyID,
		Email:   email,
	}, nil
}

func (l *Lobby) GetUsers() []User {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.Users
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for local testing
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	response, err := currentLobby.AddUser(req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if response.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(response)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	users := currentLobby.GetUsers()

	response := map[string]interface{}{
		"current_users": len(users),
		"max_users":     currentLobby.MaxUsers,
		"users":         users,
	}

	json.NewEncoder(w).Encode(response)
}

func activeUsersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	users := currentLobby.GetUsers()

	// Build user details list
	userDetails := make([]map[string]interface{}, 0)
	for _, user := range users {
		userDetails = append(userDetails, map[string]interface{}{
			"username":         user.Email,
			"lobby_id":         user.LobbyID,
			"user_joined_time": user.JoinedAt.Format(time.RFC3339),
		})
	}

	response := map[string]interface{}{
		"total_count": len(users),
		"users":       userDetails,
	}

	json.NewEncoder(w).Encode(response)
}

func main() {
	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// API endpoints
	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/status", statusHandler)
	http.HandleFunc("/api/activeusers", activeUsersHandler)

	fmt.Println("Server starting on http://localhost:8080")
	fmt.Println("Visit http://localhost:8080 to access the UI")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
