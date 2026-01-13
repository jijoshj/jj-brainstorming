# Lobby Service Documentation

## Overview
The `LobbyService` is the central component managing the lifecycle of chat lobbies and user sessions. It handles concurrency safely using a mix of Mutexes and Go channels to ensure that multiple users interacting simultaneously (joining, leaving, chatting) do not cause race conditions.

## File Location
`services/lobby_service.go`

---

## 1. Public Methods (External API)

### `NewLobbyService(redisService *RedisService) *LobbyService`
*   **Purpose**: Constructor for the service. Is executed only once during application startup.
*   **Logic**: Initializes the in-memory `lobbies` map and the three control channels (`Broadcast`, `Register`, `Unregister`).
*   **Usage**: Called in `main.go` to create the singleton instance.

### `GetOrCreateLobby() *models.Lobby`
*   **Purpose**: Determines which lobby a NEW user should join.
*   **Logic**:
    1.  **Check Existing**: Iterates through active lobbies. If one exists that is **active** but **not full** (users < 5), it returns that lobby.
    2.  **Check Full**: If it finds a lobby that is **full**, it returns `nil` (blocking the creation of a new lobby to enforce the "Single Session" rule).
    3.  **Create New**: If no lobbies exist at all, it creates a new one with a timestamp-based ID.
*   **Usage**: Called by `AuthHandler.Login` when a user attempts to sign in.

### `GetLobby(lobbyID string) *models.Lobby`
*   **Purpose**: Retrieves a specific lobby instance by ID.
*   **Logic**: Simple read-locked map lookup (`ls.lobbies[lobbyID]`).
*   **Usage**: Used by `WSHandler` to validate connection requests and inside internal service methods.

### `GetAvailableLobby() *models.Lobby`
*   **Purpose**: Finds a lobby that has space for new users.
*   **Logic**: Iterates through lobbies and returns the first one where `CanAcceptNewUsers()` is true. Returns `nil` if all are full or none exist.
*   **Usage**: Used by `StatusHandler.GetStatus` to report current room capacity.

### `FindLobbyByUserEmail(email string) *models.Lobby`
*   **Purpose**: Checks if a specific user is already part of an existing lobby.
*   **Logic**: Iterates through all lobbies and calls `lobby.IsUserInLobby(email)`.
*   **Usage**: Called by `AuthHandler.Login` to handle **reconnections**. If found, the user rejoins their previous session instead of creating a new one.

### `Run()`
*   **Purpose**: The main event loop for the service.
*   **Logic**: Runs an infinite loop using `select` to listen on `Register`, `Unregister`, and `Broadcast` channels. This ensures that state-modifying operations are serialized or handled safely.
*   **Usage**: Started as a goroutine (`go lobbyService.Run()`) in `main.go`.

---

## 2. Internal / Private Handler Methods
These methods are called by the `Run()` loop in response to channel events.

### `handleRegister(client *models.Client)`
*   **Purpose**: Finalizes a WebSocket connection and sets up the user in the lobby.
*   **Logic**:
    1.  Validates the lobby exists.
    2.  Adds the client to the `Lobby` struct.
    3.  **Welcomes**: Sends a "Welcome back" message and **replays message history** from memory.
    4.  **Game Start Check**: If the user count reaches the maximum (5), it calls `lobby.StartWebSocket()` to officially "start" the session.
    5.  **Broadcast**: Queues a "User Joined" message to be sent to all other peers.
*   **Usage**: Triggered when `WSHandler` sends a client to the `ls.Register` channel.

### `handleUnregister(client *models.Client)`
*   **Purpose**: Handles user disconnection.
*   **Logic**:
    1.  Removes the client from the lobby's active connections list.
    2.  Closes the write channel to prevent leaks.
    3.  Marks the user object as "Inactive".
    4.  Broadcasts a "User Left" message to remaining users.
*   **Usage**: Triggered when a WebSocket connection breaks or is closed.

### `handleBroadcast(broadcastMsg BroadcastMessage)`
*   **Purpose**: Distributes a message to all users in a specific lobby.
*   **Logic**:
    1.  **Persistence**: If it's a chat message, it saves it to the in-memory history and pushes it to **Redis**.
    2.  **Delivery**: Iterates through all connected clients in that lobby and attempts to send the message.
    3.  **Cleanup**: If sending fails (e.g., client disconnected unexpectedly), it removes that client.
*   **Usage**: Triggered whenever a user sends a message or the system generates a notification.

---

## 3. Unused / Dead Methods

The following methods are defined in the codebase but verify as **unused** based on current project analysis:

### `GetMostRecentLobby() *models.Lobby`
*   **Status**: **UNUSED / DEPRECATED**
*   **Logic**: Iterates through all lobbies to find the one created most recently.
*   **Context**: This appears to have been replaced by `GetAvailableLobby()` in the `StatusHandler`. The current logic prefers finding *any* open lobby rather than just the newest one.
