# Generic Go Concepts Documentation

This document explains key Go language concepts used in this project, specifically focusing on concurrency primitives. These tools are what make Go particularly powerful for backend systems like chat servers.

---

## 1. Mutex (`sync.Mutex` / `sync.RWMutex`)
### What is it?
A **Mutex** (Mutual Exclusion) is a synchronization primitive that acts like a **lock**. It ensures that only **one goroutine (thread)** can access a specific piece of code or data at a time.

### The Problem it Solves
**Race Conditions**.
Imagine two users join a lobby at the exact same nanosecond.
1.  Thread A reads `UserCount = 4`.
2.  Thread B reads `UserCount = 4`.
3.  Thread A adds a user -> `UserCount` becomes 5.
4.  Thread B adds a user -> `UserCount` becomes 6.
**Result**: You have 6 users in a 5-person lobby. Broken logic.

### How it Works
*   **`Lock()`**: "I am claiming this data. Everyone else wait."
*   **`Unlock()`**: "I am done. Next person can go."

In this project (`RWMutex`), we use a special variation:
*   **`Lock()` / `Unlock()`**: Checking out the key for **Writing**. No one else can read OR write.
*   **`RLock()` / `RUnlock()`**: Checking out the key for **Reading**. Multiple people can read at the same time, but no one can write.

**Example from `LobbyService`**:
```go
func (ls *LobbyService) GetOrCreateLobby() {
    ls.mu.Lock()         // LOCK: No one else can add/checking lobbies right now
    defer ls.mu.Unlock() // UNLOCK: Automatically unlock when function finishes
    // ... safe to modify lobbies map ...
}
```

---

## 2. Channels (`chan`)
### What is it?
A **Channel** is a typed conduit (pipe) used to send and receive values between goroutines. It is Go's primary method for communication between threads.

### The Problem it Solves
**Safe Communication**.
Instead of using checks and locks to share memory (which is complex and error-prone), Go encourages "Sharing memory by communicating". Channels allow one thread to pass data to another implementation-safely.

### How it Works
*   **Send**: `channel <- value` (Drop item into the pipe)
*   **Receive**: `value := <-channel` (Pull item out of the pipe)

**Example from `LobbyService`**:
The `Run()` loop acts as a central event processor.
```go
// Sender (WSHandler)
ls.Register <- client // Drops a client into the 'Register' pipe

// Receiver (LobbyService)
case client := <-ls.Register: // Pulls it out and processes it
    ls.handleRegister(client)
```
This forces all registration events to happen **one by one**, preventing race conditions without needing complex locks inside the handler logic itself.

---

## 3. Goroutines (`go func()`)
### What is it?
A **Goroutine** is a lightweight thread managed by the Go runtime.

### The Problem it Solves
**Concurrency**.
It allows your program to do multiple things at once. In a web server, you want to handle User A's chat messages while simultaneously waiting for User B to login.

### How it Works
Putting the keyword `go` before a function call starts it in a new background thread.

**Example**:
```go
go ls.Broadcast(msg) // Send this in the background, don't stop my current work
```

---

## 4. Select Statement (`select`)
### What is it?
The `select` statement is like a `switch` statement, but for **channels**.

### The Problem it Solves
**Waiting on Multiple Things**.
If you are waiting for a User to Join OR a User to Leave OR a Message to Broadcast, you need a way to say "Wake me up if **ANY** of these happen."

### How it Works
It listens to multiple channel operations and blocks until one of them is ready.

**Example from `LobbyService.Run()`**:
```go
select {
case client := <-ls.Register:
    // Handle join
case client := <-ls.Unregister:
    // Handle leave
case msg := <-ls.Broadcast:
    // Handle message
}
```
 Whichever event happens first gets processed. If multiple happen at once, it picks one randomly.

---

## 5. Defer (`defer`)
### What is it?
A statement that schedules a function call to be run immediately before the function returns.

### The Problem it Solves
**Cleanup**.
for example, if you `Lock()` a mutex, you MUST `Unlock()` it, even if an error occurs halfway through your function. Forgetting to unlock causes a **Deadlock** (the program freezes forever).

### How it Works
```go
ls.mu.Lock()
defer ls.mu.Unlock() // This will ALWAYS run at the end, no matter what.

if error {
    return // Unlock runs here
}
return // Unlock runs here
```
