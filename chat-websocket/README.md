# WebSocket Chat Application

A real-time chat application built with Go, WebSocket, and Redis.

## Features

- ✅ Real-time messaging using WebSocket
- ✅ Maximum 5 concurrent users
- ✅ User join/leave notifications
- ✅ Live user list sidebar
- ✅ Message history stored in Redis
- ✅ REST API endpoints
- ✅ Responsive UI

## Running this application

1. Make sure Redis is running:
```bash
redis-server
# OR
brew services start redis
```

2. Navigate to this directory:
```bash
cd chat-websocket
```

3. Install dependencies:
```bash
go mod tidy
```

4. Run the application:
```bash
go run main.go
```

5. Open browser: http://localhost:8080

## API Endpoints

- **GET** `/` - Web UI
- **WebSocket** `/ws?username=YourName` - WebSocket connection
- **GET** `/api/status` - Get current users online
- **GET** `/api/messages` - Get all messages from Redis
