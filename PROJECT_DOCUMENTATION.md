# Project Documentation: Single Session Chat App

## 1. Architecture Overview

This project is a **single-session chat application** designed for exactly 5 users. It is built using **Go (Golang)** for the backend and **Vanilla JavaScript/HTML** for the frontend.

### Key Architectural Components:
-   **Backend**: Pure Go using `net/http` for HTTP and WebSocket handling.
-   **Concurrency**: Extensive use of Go routines and Channels (`Register`, `Unregister`, `Broadcast`) for handling real-time events without blocking.
-   **State Management**:
    -   **In-Memory**: Active lobbies and user sessions are managed in-memory via `LobbyService`.
    -   **Persistence**: **Redis** is used for persisting message history and arguably for pub/sub capabilities (referenced in `RedisService`).
-   **Communication**:
    -   **REST API**: For initial authentication (`/login`) and system status (`/status`).
    -   **WebSockets**: For real-time bi-directional chat communication.

### Data Flow
1.  **Login**: User hits `/api/login` -> assigns/creates a Lobby -> returns `lobby_id`.
2.  **Connect**: User connects to WebSocket `/ws` with `lobby_id`.
3.  **Chat**: Messages sent via WebSocket are processed by `WSHandler` -> `LobbyService` -> Broadcast to all active clients in the lobby -> Saved to Redis.

---

## 2. Directory Structure

```
/
├── config/          # Configuration constants (port, max users, etc.)
├── controllers/     # Helper logic for HTTP responses and WS connection upgrades
├── handlers/        # HTTP Request Handlers (Entry points for API & WS)
├── middleware/      # HTTP Middlewares (CORS, Logging, etc.)
├── models/          # Data structures (User, Lobby, Message)
├── services/        # Business Logic (Lobby management, Redis interaction)
├── static/          # Frontend assets (index.html, css, js) - Served by FileServer
└── main.go          # Application Entry Point & Route Definitions
```

### Detailed Breakdown:

-   **`handlers/`**: Contains `AuthHandler` (login), `StatusHandler` (system info), and `WSHandler` (WebSocket connection initiation). These are the first line of code that runs when a request hits the server.
-   **`services/`**:
    -   `LobbyService`: The "brain" of the application. Manages the lifecycle of a game lobby (`GetOrCreateLobby`), handles user registration/deregistration, and broadcasts messages.
    -   `RedisService`: Handles interaction with the Redis database.
-   **`models/`**: Defines the shape of data, e.g., `Lobby` struct which holds connected clients, and `Message` struct for chat payloads.
-   **`controllers/`**: Abstracts common tasks like JSON responses (`APIController`) and WebSocket upgrading (`WSController`) to keep handlers clean.

---

## 3. Critical Functions

### `main.go`
-   **`main()`**: Initializes services (Redis, Lobby), controllers, and handlers. Sets up HTTP routes and starts the `LobbyService` run loop in a goroutine (`go lobbyService.Run()`), then starts the HTTP server.

### `services/lobby_service.go`
-   **`GetOrCreateLobby()`**: Core logic for session management.
    -   Checks for existing lobbies that aren't full.
    -   If a full lobby exists (active session), it **blocks** creation of a new one (Single Session rule).
    -   If no lobby exists, creates a new one.
-   **`handleRegister(client)`**:
    -   Adds a new WebSocket client to the lobby.
    -   Sends a "Welcome" message and **Message History** to the new user.
    -   If the lobby becomes full (5/5), it triggers `lobby.StartWebSocket()`.
    -   Broadcasts a "User Joined" system message.
-   **`Run()`**: The main event loop consuming channels (`Register`, `Unregister`, `Broadcast`) to ensure thread-safety when modifying lobby state.

### `handlers/auth_handler.go`
-   **`Login()`**:
    -   Validates email.
    -   Checks if user is reconnecting to an existing lobby.
    -   If new user, calls `GetOrCreateLobby`.
    -   Denies login if a session is currently in progress (Lobby full).

### `handlers/ws_handler.go`
-   **`HandleWebSocket()`**:
    -   Upgrades standard HTTP request to a WebSocket connection.
    -   Initilizes `ReadPump` and `WritePump` goroutines for the connection.
    -   Registers the client with `LobbyService`.

---

## 4. API & WebSocket Specification

### REST API

#### 1. Login
**Endpoint**: `POST /api/login`
**Description**: Authenticates a user and assigns them to a lobby.

**Request Body**:
```json
{
  "email": "user@example.com"
}
```

**Response (Success - 200 OK)**:
```json
{
  "success": true,
  "message": "User registered successfully",
  "lobby_id": "lobby-1700000000",
  "email": "user@example.com"
}
```

**Response (Error - 400/503)**:
```json
{
  "success": false,
  "message": "Lobby is full. Please wait for the current session to complete."
}
```

#### 2. System Status
**Endpoint**: `GET /api/status`
**Description**: Returns validation info about the current state of the server/lobby.

**Response**:
```json
{
  "current_users": 2,
  "max_users": 5,
  "lobby_id": "lobby-1700000000",
  "users": ["user1@example.com", "user2@example.com"],
  "message": "..." // Optional status message
}
```

---

### WebSocket API

**Endpoint**: `ws://localhost:8080/ws`
**Query Parameters**:
-   `email`: User's email (must match login)
-   `lobby_id`: The lobby ID returned from login

#### Message Protocol
All WebSocket messages follow a JSON structure.

**Data Structure (`Message`)**:
```json
{
  "type": "message" | "system_action",
  "system_action": "welcome" | "user_joined" | "user_left" | "error" | "user_list", // Optional
  "username": "user@example.com",
  "content": "Hello World",
  "lobby_id": "lobby-1700000000",
  "timestamp": "2024-01-01T12:00:00Z",
  "user_count": 3,
  "max_users": 5,
  "user_list": [...]
}
```

#### Message Types

1.  **Chat Message** (Client -> Server -> Broadcast):
    -   `type`: "message"
    -   `content`: The actual text message.

2.  **System Action** (Server -> Client):
    -   `type`: "system_action"
    -   `system_action`:
        -   `welcome`: Sent immediately on connection.
        -   `user_joined`: Sent when a new user enters.
        -   `user_left`: Sent when a user disconnects.

### Example Flow
1.  **Connect**: Server sends `type: "system_action", system_action: "welcome"`.
2.  **User Sends**: Client sends `{"content": "Hello"}`.
3.  **Broadcast**: Server receives, saves to Redis, and sends `{"type": "message", "username": "...", "content": "Hello"}` to all clients.
