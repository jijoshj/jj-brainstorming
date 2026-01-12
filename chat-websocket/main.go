package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const MaxConnections = 5

var ctx = context.Background()

type Message struct {
	Type      string    `json:"type"` // "welcome", "user_joined", "user_left", "message", "error", "user_list"
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	UserCount int       `json:"user_count,omitempty"`
	MaxUsers  int       `json:"max_users,omitempty"`
	UserList  []string  `json:"user_list,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type RedisMessage struct {
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	MessageID string    `json:"message_id"`
}

type Client struct {
	Username string
	Conn     *websocket.Conn
	Send     chan Message
}

type Hub struct {
	Clients     map[*Client]bool
	Broadcast   chan Message
	Register    chan *Client
	Unregister  chan *Client
	mu          sync.RWMutex
	redisClient *redis.Client
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local testing
	},
}

var hub Hub

func initRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// Test connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("❌ Failed to connect to Redis: %v", err)
	}

	log.Println("✅ Connected to Redis successfully")
	return rdb
}

func (h *Hub) pushMessageToRedis(username, content string, timestamp time.Time) error {
	messageID := fmt.Sprintf("msg_%d_%s", timestamp.Unix(), username)

	redisMsg := RedisMessage{
		Username:  username,
		Content:   content,
		Timestamp: timestamp,
		MessageID: messageID,
	}

	msgJSON, err := json.Marshal(redisMsg)
	if err != nil {
		log.Printf("❌ Failed to marshal message to JSON: %v", err)
		return err
	}

	// Push to Redis list (queue)
	err = h.redisClient.RPush(ctx, "chat:messages", msgJSON).Err()
	if err != nil {
		log.Printf("❌ Failed to push message to Redis: %v", err)
		return err
	}

	log.Printf("✅ Message pushed to Redis queue: %s - %s", username, content)
	return nil
}

func (h *Hub) getUserList() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userList := make([]string, 0, len(h.Clients))
	for client := range h.Clients {
		userList = append(userList, client.Username)
	}
	return userList
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			currentCount := len(h.Clients)

			if currentCount < MaxConnections {
				h.Clients[client] = true
				newCount := len(h.Clients)
				h.mu.Unlock()

				userList := h.getUserList()

				log.Printf("📝 Sending welcome message to: %s", client.Username)

				// Send welcome message to the new user only
				welcomeMsg := Message{
					Type:      "welcome",
					Username:  client.Username,
					Content:   fmt.Sprintf("Welcome to the chat, %s! 🎉", client.Username),
					UserCount: newCount,
					MaxUsers:  MaxConnections,
					UserList:  userList,
					Timestamp: time.Now(),
				}

				// Send welcome message immediately to this specific client
				select {
				case client.Send <- welcomeMsg:
					log.Printf("✅ Welcome message sent to: %s", client.Username)
				default:
					log.Printf("❌ Failed to send welcome message to: %s", client.Username)
				}

				// Small delay to ensure welcome is processed first
				time.Sleep(100 * time.Millisecond)

				// Broadcast to ALL users (including new one) that a new user joined
				log.Printf("📢 Broadcasting user joined message for: %s", client.Username)
				joinMsg := Message{
					Type:      "user_joined",
					Username:  client.Username,
					Content:   fmt.Sprintf("%s joined the chat", client.Username),
					UserCount: newCount,
					MaxUsers:  MaxConnections,
					UserList:  userList,
					Timestamp: time.Now(),
				}

				// Broadcast to all clients
				h.mu.RLock()
				for c := range h.Clients {
					select {
					case c.Send <- joinMsg:
						log.Printf("✅ Join notification sent to: %s", c.Username)
					default:
						log.Printf("❌ Failed to send join notification to: %s", c.Username)
					}
				}
				h.mu.RUnlock()

				fmt.Printf("✅ Client registered: %s (Total: %d/%d)\n", client.Username, newCount, MaxConnections)
			} else {
				h.mu.Unlock()
				log.Printf("❌ Rejecting client (room full): %s", client.Username)

				// Send rejection message DIRECTLY through the connection (not through channel)
				rejectionMsg := Message{
					Type:      "error",
					Content:   "Chat room is full. Maximum 5 users allowed. Please wait for someone to leave.",
					Timestamp: time.Now(),
				}

				// Write directly to WebSocket connection synchronously
				err := client.Conn.WriteJSON(rejectionMsg)
				if err != nil {
					log.Printf("❌ Failed to send rejection message to %s: %v", client.Username, err)
				} else {
					log.Printf("✅ Rejection message sent to: %s", client.Username)
				}

				// Small delay to ensure message is received before closing
				time.Sleep(200 * time.Millisecond)

				// Close the connection
				client.Conn.Close()
				close(client.Send)

				fmt.Printf("❌ Connection rejected: %s (Room full: %d/%d)\n", client.Username, currentCount, MaxConnections)
			}

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
				newCount := len(h.Clients)
				h.mu.Unlock()

				userList := h.getUserList()

				log.Printf("📢 Broadcasting user left message for: %s", client.Username)

				// Broadcast to all remaining users that someone left
				leaveMsg := Message{
					Type:      "user_left",
					Username:  client.Username,
					Content:   fmt.Sprintf("%s left the chat", client.Username),
					UserCount: newCount,
					MaxUsers:  MaxConnections,
					UserList:  userList,
					Timestamp: time.Now(),
				}

				h.mu.RLock()
				for c := range h.Clients {
					select {
					case c.Send <- leaveMsg:
						log.Printf("✅ Leave notification sent to: %s", c.Username)
					default:
						log.Printf("❌ Failed to send leave notification to: %s", c.Username)
					}
				}
				h.mu.RUnlock()

				fmt.Printf("👋 Client unregistered: %s (Total: %d/%d)\n", client.Username, newCount, MaxConnections)
			} else {
				h.mu.Unlock()
			}

		case message := <-h.Broadcast:
			log.Printf("📢 Broadcasting message from: %s", message.Username)

			// Push ONLY regular chat messages to Redis (not join/leave notifications)
			if message.Type == "message" {
				err := h.pushMessageToRedis(message.Username, message.Content, message.Timestamp)
				if err != nil {
					log.Printf("⚠️ Failed to push message to Redis, but continuing broadcast")
				}
			}

			h.mu.RLock()
			for client := range h.Clients {
				select {
				case client.Send <- message:
					log.Printf("✅ Message delivered to: %s", client.Username)
				default:
					log.Printf("❌ Failed to deliver message to: %s", client.Username)
					close(client.Send)
					delete(h.Clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (c *Client) ReadPump() {
	defer func() {
		hub.Unregister <- c
		c.Conn.Close()
	}()

	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		msg.Username = c.Username
		msg.Timestamp = time.Now()
		msg.Type = "message"

		hub.Broadcast <- msg
	}
}

func (c *Client) WritePump() {
	defer func() {
		c.Conn.Close()
		log.Printf("🔌 WritePump closed for: %s", c.Username)
	}()

	for message := range c.Send {
		log.Printf("✍️ Writing message to %s: type=%s", c.Username, message.Type)
		err := c.Conn.WriteJSON(message)
		if err != nil {
			log.Printf("❌ Write error for %s: %v", c.Username, err)
			return
		}
		log.Printf("✅ Message written successfully to: %s", c.Username)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		log.Println("❌ WebSocket connection rejected: no username provided")
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	log.Printf("🔌 Attempting WebSocket upgrade for user: %s", username)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade failed for %s: %v", username, err)
		return
	}

	log.Printf("✅ WebSocket upgrade successful for user: %s", username)

	client := &Client{
		Username: username,
		Conn:     conn,
		Send:     make(chan Message, 256),
	}

	hub.Register <- client

	go client.WritePump()
	go client.ReadPump()
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	hub.mu.RLock()
	clientCount := len(hub.Clients)
	usernames := make([]string, 0, clientCount)
	for client := range hub.Clients {
		usernames = append(usernames, client.Username)
	}
	hub.mu.RUnlock()

	response := map[string]interface{}{
		"current_connections": clientCount,
		"max_connections":     MaxConnections,
		"users":               usernames,
	}

	json.NewEncoder(w).Encode(response)
}

// New endpoint to retrieve messages from Redis
func messagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	// Get all messages from Redis list
	messages, err := hub.redisClient.LRange(ctx, "chat:messages", 0, -1).Result()
	if err != nil {
		http.Error(w, "Failed to retrieve messages", http.StatusInternalServerError)
		log.Printf("❌ Failed to retrieve messages from Redis: %v", err)
		return
	}

	// Parse messages
	var redisMessages []RedisMessage
	for _, msgStr := range messages {
		var msg RedisMessage
		if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
			log.Printf("⚠️ Failed to unmarshal message: %v", err)
			continue
		}
		redisMessages = append(redisMessages, msg)
	}

	response := map[string]interface{}{
		"total_messages": len(redisMessages),
		"messages":       redisMessages,
	}

	json.NewEncoder(w).Encode(response)
}

func main() {
	// Initialize Redis
	rdb := initRedis()

	// Initialize hub
	hub = Hub{
		Clients:     make(map[*Client]bool),
		Broadcast:   make(chan Message),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		redisClient: rdb,
	}

	// Start the hub
	go hub.Run()

	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// WebSocket endpoint
	http.HandleFunc("/ws", wsHandler)

	// Status endpoint
	http.HandleFunc("/api/status", statusHandler)

	// Messages endpoint (to retrieve Redis queue messages)
	http.HandleFunc("/api/messages", messagesHandler)

	fmt.Println("🚀 WebSocket server starting on http://localhost:8080")
	fmt.Println("📱 Visit http://localhost:8080 to access the chat UI")
	fmt.Println("🔌 WebSocket endpoint: ws://localhost:8080/ws?username=YourName")
	fmt.Println("📊 Messages endpoint: http://localhost:8080/api/messages")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
