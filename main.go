package main

import (
	"chat-integrated/config"
	"chat-integrated/controllers"
	"chat-integrated/handlers"
	"chat-integrated/services"
	"fmt"
	"log"
	"net/http"
)

func main() {
	// Initialize services
	redisService := services.NewRedisService()
	defer redisService.Close()

	lobbyService := services.NewLobbyService(redisService)
	go lobbyService.Run()

	// Initialize controllers
	apiController := controllers.NewAPIController(lobbyService)
	wsController := controllers.NewWSController(lobbyService)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(apiController, lobbyService)
	statusHandler := handlers.NewStatusHandler(apiController, lobbyService)
	wsHandler := handlers.NewWSHandler(wsController, lobbyService)

	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// API routes
	http.HandleFunc("/api/login", authHandler.Login)
	http.HandleFunc("/api/status", statusHandler.GetStatus)

	// WebSocket route
	http.HandleFunc("/ws", wsHandler.HandleWebSocket)

	fmt.Println("🚀 Integrated Chat Server starting on http://localhost:8080")
	fmt.Println("📱 Visit http://localhost:8080 to access the chat UI")
	fmt.Println("🔌 WebSocket endpoint: ws://localhost:8080/ws?email=user@example.com&lobby_id=lobby-123")
	log.Fatal(http.ListenAndServe(config.ServerPort, nil))
}
