package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"Ai_Bot/database"
	"Ai_Bot/models"
	"github.com/gin-gonic/gin"
)

//  GROQ SOZLAMASI

const GroqAPIURL = "https://api.groq.com/openai/v1/chat/completions"
const GroqModel = "llama-3.3-70b-versatile"

// ==========================

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Model     string        `json:"model"`
	Messages  []groqMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

//  GROQ CALL

func callGroq(messages []groqMessage) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY topilmadi")
	}

	body, _ := json.Marshal(groqRequest{
		Model:     GroqModel,
		Messages:  messages,
		MaxTokens: 1024,
	})

	req, err := http.NewRequest("POST", GroqAPIURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 🔥 STATUS CHECK
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Groq xato %d: %s", resp.StatusCode, string(raw))
	}

	raw, _ := io.ReadAll(resp.Body)

	var gr groqResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return "", fmt.Errorf("json parse xato: %w", err)
	}

	if gr.Error != nil {
		return "", fmt.Errorf("groq xato: %s", gr.Error.Message)
	}

	if len(gr.Choices) == 0 {
		return "", fmt.Errorf("AI javob qaytarmadi")
	}

	return gr.Choices[0].Message.Content, nil
}

//  CHAT HANDLER

func ChatHandler(c *gin.Context) {
	var req struct {
		TelegramID int64  `json:"telegram_id"`
		SessionID  string `json:"session_id"`
		Message    string `json:"message"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "notogri sorov"})
		return
	}

	// Session topish yoki yaratish
	var session models.ChatSession
	res := database.DB.Where("session_id = ? AND telegram_id = ?", req.SessionID, req.TelegramID).First(&session)

	if res.Error != nil {
		title := req.Message
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		session = models.ChatSession{
			TelegramID: req.TelegramID,
			SessionID:  req.SessionID,
			Title:      title,
		}

		database.DB.Create(&session)
	}

	// User message save
	database.DB.Create(&models.ChatMessage{
		SessionID: req.SessionID,
		Role:      "user",
		Content:   req.Message,
	})

	// History olish
	var history []models.ChatMessage
	database.DB.Where("session_id = ?", req.SessionID).
		Order("created_at asc").
		Find(&history)

	// Groq messages
	msgs := []groqMessage{
		{
			Role:    "system",
			Content: "Siz foydali va dostona AI yordamchisiz. Uzbek tilida qisqa va aniq javob bering.",
		},
	}

	for _, m := range history {
		msgs = append(msgs, groqMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// AI call
	reply, err := callGroq(msgs)
	if err != nil {
		log.Println("Groq xato:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save AI
	database.DB.Create(&models.ChatMessage{
		SessionID: req.SessionID,
		Role:      "assistant",
		Content:   reply,
	})

	database.DB.Model(&session).Update("updated_at", time.Now())

	c.JSON(http.StatusOK, gin.H{
		"reply":      reply,
		"session_id": req.SessionID,
	})
}

//  SESSIONS

func GetSessionsHandler(c *gin.Context) {
	telegramID := c.Query("telegram_id")

	if telegramID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "telegram_id kerak"})
		return
	}

	var sessions []models.ChatSession
	database.DB.Where("telegram_id = ?", telegramID).
		Order("updated_at desc").
		Find(&sessions)

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

//  HISTORY

func GetHistoryHandler(c *gin.Context) {
	sessionID := c.Query("session_id")

	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id kerak"})
		return
	}

	var messages []models.ChatMessage
	database.DB.Where("session_id = ?", sessionID).
		Order("created_at asc").
		Find(&messages)

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

//  DELETE

func DeleteSessionHandler(c *gin.Context) {
	sessionID := c.Param("session_id")

	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id kerak"})
		return
	}

	database.DB.Where("session_id = ?", sessionID).Delete(&models.ChatMessage{})
	database.DB.Where("session_id = ?", sessionID).Delete(&models.ChatSession{})

	c.JSON(http.StatusOK, gin.H{"success": true})
}
