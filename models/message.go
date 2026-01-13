package models

import "time"

type MessageType string

const (
	MessageTypeChat         MessageType = "message"
	MessageTypeSystemAction MessageType = "system_action"
)

type SystemActionType string

const (
	SystemActionWelcome    SystemActionType = "welcome"
	SystemActionUserJoined SystemActionType = "user_joined"
	SystemActionUserLeft   SystemActionType = "user_left"
	SystemActionError      SystemActionType = "error"
	SystemActionUserList   SystemActionType = "user_list"
)

type Message struct {
	Type         MessageType       `json:"type"`
	SystemAction *SystemActionType `json:"system_action,omitempty"`
	Username     string            `json:"username,omitempty"`
	Content      string            `json:"content"`
	LobbyID      string            `json:"lobby_id"`
	UserCount    int               `json:"user_count,omitempty"`
	MaxUsers     int               `json:"max_users,omitempty"`
	UserList     []string          `json:"user_list,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}

type RedisMessage struct {
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	LobbyID   string    `json:"lobby_id"`
	Timestamp time.Time `json:"timestamp"`
	MessageID string    `json:"message_id"`
}
