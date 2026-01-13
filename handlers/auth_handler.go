package handlers

import (
	"chat-integrated/config"
	"chat-integrated/controllers"
	"chat-integrated/services"
	"encoding/json"
	"log"
	"net/http"
)

type AuthHandler struct {
	controller   *controllers.APIController
	lobbyService *services.LobbyService
}

func NewAuthHandler(controller *controllers.APIController, lobbyService *services.LobbyService) *AuthHandler {
	return &AuthHandler{
		controller:   controller,
		lobbyService: lobbyService,
	}
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

func (ah *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if ah.controller.HandlePreflight(w, r) {
		return
	}

	if r.Method != "POST" {
		ah.controller.RespondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ah.controller.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Email == "" {
		ah.controller.RespondError(w, http.StatusBadRequest, "Email is required")
		return
	}

	log.Printf("📧 Login request from: %s", req.Email)

	// FIRST: Check if user already exists in any lobby (for reconnection)
	existingLobby := ah.lobbyService.FindLobbyByUserEmail(req.Email)

	if existingLobby != nil {
		// User is reconnecting to their existing lobby
		user := existingLobby.AddUser(req.Email) // This will reactivate the user
		log.Printf("🔄 User reconnecting to existing lobby: %s → %s", req.Email, existingLobby.ID)

		response := LoginResponse{
			Success: true,
			Message: "Reconnecting to your lobby. You'll see all previous messages.",
			LobbyID: existingLobby.ID,
			Email:   user.Email,
		}
		ah.controller.RespondJSON(w, http.StatusOK, response)
		return
	}

	// User is joining for the first time - get or create a lobby
	lobby := ah.lobbyService.GetOrCreateLobby()

	// If lobby is nil, it means there's an active session and we can't create new lobby
	if lobby == nil {
		log.Printf("❌ No available lobby for: %s (Active session in progress)", req.Email)
		response := LoginResponse{
			Success: false,
			Message: "A chat session is currently in progress. Please wait for it to complete or try again later.",
		}
		ah.controller.RespondJSON(w, http.StatusServiceUnavailable, response)
		return
	}

	log.Printf("📦 Got lobby for new user: %s (Current users: %d/%d)", lobby.ID, lobby.GetUserCount(), config.MaxUsersPerLobby)

	// Check if lobby can accept new users
	if !lobby.CanAcceptNewUsers() {
		log.Printf("❌ Lobby full, rejecting: %s", req.Email)
		response := LoginResponse{
			Success: false,
			Message: "Lobby is full. Please wait for the current session to complete.",
		}
		ah.controller.RespondJSON(w, http.StatusServiceUnavailable, response)
		return
	}

	// Add user to lobby
	user := lobby.AddUser(req.Email)
	log.Printf("✅ New user added to lobby: %s (Now: %d/%d users)", req.Email, lobby.GetUserCount(), config.MaxUsersPerLobby)

	response := LoginResponse{
		Success: true,
		Message: "User registered successfully",
		LobbyID: lobby.ID,
		Email:   user.Email,
	}

	ah.controller.RespondJSON(w, http.StatusOK, response)
}
