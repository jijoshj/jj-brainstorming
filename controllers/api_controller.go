package controllers

import (
	"chat-integrated/services"
)

type APIController struct {
	BaseController
	lobbyService *services.LobbyService
}

func NewAPIController(lobbyService *services.LobbyService) *APIController {
	return &APIController{
		lobbyService: lobbyService,
	}
}
