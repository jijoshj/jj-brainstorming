package services

import (
	"chat-integrated/config"
	"chat-integrated/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisService struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisService() *RedisService {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: "",
		DB:       config.RedisDB,
	})

	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("❌ Failed to connect to Redis: %v", err)
	}

	log.Println("✅ Connected to Redis successfully")
	return &RedisService{
		client: rdb,
		ctx:    ctx,
	}
}

func (rs *RedisService) PushMessage(username, content, lobbyID string, timestamp time.Time) error {
	messageID := fmt.Sprintf("msg_%s_%d_%s", lobbyID, timestamp.Unix(), username)

	redisMsg := models.RedisMessage{
		Username:  username,
		Content:   content,
		LobbyID:   lobbyID,
		Timestamp: timestamp,
		MessageID: messageID,
	}

	msgJSON, err := json.Marshal(redisMsg)
	if err != nil {
		log.Printf("❌ Failed to marshal message to JSON: %v", err)
		return err
	}

	// Push to lobby-specific queue
	queueKey := fmt.Sprintf("chat:lobby:%s:messages", lobbyID)
	err = rs.client.RPush(rs.ctx, queueKey, msgJSON).Err()
	if err != nil {
		log.Printf("❌ Failed to push message to Redis: %v", err)
		return err
	}

	log.Printf("✅ Message pushed to Redis queue [%s]: %s - %s", lobbyID, username, content)
	return nil
}

func (rs *RedisService) GetMessages(lobbyID string) ([]models.RedisMessage, error) {
	queueKey := fmt.Sprintf("chat:lobby:%s:messages", lobbyID)
	messages, err := rs.client.LRange(rs.ctx, queueKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	var redisMessages []models.RedisMessage
	for _, msgStr := range messages {
		var msg models.RedisMessage
		if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
			log.Printf("⚠️ Failed to unmarshal message: %v", err)
			continue
		}
		redisMessages = append(redisMessages, msg)
	}

	return redisMessages, nil
}

func (rs *RedisService) Close() {
	rs.client.Close()
}
