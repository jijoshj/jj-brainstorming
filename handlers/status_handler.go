package handlers

import (
	"chat-integrated/config"
	"chat-integrated/controllers"
	"chat-integrated/services"
	"net/http"
)

type StatusHandler struct {
	controller   *controllers.APIController
	lobbyService *services.LobbyService
}

func NewStatusHandler(controller *controllers.APIController, lobbyService *services.LobbyService) *StatusHandler {
	return &StatusHandler{
		controller:   controller,
		lobbyService: lobbyService,
	}
}

func (sh *StatusHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	sh.controller.SetCommonHeaders(w)

	// Get a lobby that can accept new users
	availableLobby := sh.lobbyService.GetAvailableLobby()

	if availableLobby == nil {
		// No available lobby - either all are full or no lobbies exist
		response := map[string]interface{}{
			"current_users": 0,
			"max_users":     config.MaxUsersPerLobby,
			"lobby_id":      "",
			"users":         []string{},
			"message":       "No active lobby available. A session may be in progress.",
		}
		sh.controller.RespondJSON(w, http.StatusOK, response)
		return
	}

	response := map[string]interface{}{
		"current_users": availableLobby.GetActiveUserCount(),
		"max_users":     config.MaxUsersPerLobby,
		"lobby_id":      availableLobby.ID,
		"users":         availableLobby.GetActiveUserList(),
	}

	sh.controller.RespondJSON(w, http.StatusOK, response)
}
