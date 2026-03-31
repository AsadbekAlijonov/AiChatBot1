package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

//  SOZLAMALAR

const HFModel = "black-forest-labs/FLUX.1-schnell"
const ImagesDir = "./frontend/generated"

// BotSender — main.go dan set qilinadi
var BotSender func(chatID int64, filePath, caption string)

//  IMAGE GENERATION

func GenerateImage(prompt string) (string, error) {
	apiKey := os.Getenv("HFAPI_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("HFAPI_KEY topilmadi")
	}

	if err := os.MkdirAll(ImagesDir, 0755); err != nil {
		return "", fmt.Errorf("papka yaratishda xato: %w", err)
	}

	body, _ := json.Marshal(map[string]string{"inputs": prompt})

	url := "https://router.huggingface.co/hf-inference/models/" + HFModel
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 🔥 status check
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HF API xato %d: %s", resp.StatusCode, string(raw))
	}

	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("img_%d.png", time.Now().UnixMilli())
	filePath := filepath.Join(ImagesDir, filename)

	if err := os.WriteFile(filePath, imgData, 0644); err != nil {
		return "", fmt.Errorf("faylga yozishda xato: %w", err)
	}

	return filename, nil
}

// HANDLER: GENERATE IMAGE

func GenerateImageHandler(c *gin.Context) {
	var req struct {
		Prompt     string `json:"prompt"`
		TelegramID int64  `json:"telegram_id"`
		SendToBot  bool   `json:"send_to_bot"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.Prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt kerak"})
		return
	}

	log.Printf("[IMG] Generatsiya boshlandi: %s", req.Prompt)

	filename, err := GenerateImage(req.Prompt)
	if err != nil {
		log.Println("[IMG] Xato:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	imageURL := "/generated/" + filename

	if req.SendToBot && req.TelegramID != 0 && BotSender != nil {
		go BotSender(req.TelegramID, filepath.Join(ImagesDir, filename), "🎨 "+req.Prompt)
	}

	c.JSON(http.StatusOK, gin.H{
		"image_url": imageURL,
		"filename":  filename,
	})
}

// HANDLER: ANALYZE IMAGE

func AnalyzeImageHandler(c *gin.Context) {
	var req struct {
		ImageBase64 string `json:"image_base64"`
		Question    string `json:"question"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.ImageBase64 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rasm kerak"})
		return
	}

	question := req.Question
	if question == "" {
		question = "Bu rasmda nima ko'ryapsiz? Batafsil tushuntiring."
	}

	analysis, err := analyzeWithGroqVision(req.ImageBase64, question)
	if err != nil {
		log.Println("[VISION] Xato:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"analysis": analysis})
}

//  GROQ VISION

func analyzeWithGroqVision(imageBase64, question string) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY topilmadi")
	}

	b64 := imageBase64
	mediaType := "image/jpeg"

	if idx := strings.Index(b64, ";base64,"); idx != -1 {
		mediaType = strings.TrimPrefix(b64[:idx], "data:")
		b64 = b64[idx+8:]
	}

	// base64 validatsiya
	if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
		if _, err2 := base64.RawStdEncoding.DecodeString(b64); err2 != nil {
			return "", fmt.Errorf("noto'g'ri base64")
		}
	}

	payload := map[string]any{
		"model":      "meta-llama/llama-4-scout-17b-16e-instruct",
		"max_tokens": 1024,
		"messages": []map[string]any{
			{
				"role":    "system",
				"content": "Siz rasmlarni tahlil qiluvchi AI yordamchisiz. O'zbek tilida aniq va batafsil javob bering.",
			},
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": fmt.Sprintf("data:%s;base64,%s", mediaType, b64),
						},
					},
					{"type": "text", "text": question},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", GroqAPIURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 🔥 status check
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
		return "", fmt.Errorf("groq vision xato: %s", gr.Error.Message)
	}

	if len(gr.Choices) == 0 {
		return "", fmt.Errorf("bo'sh javob")
	}

	return gr.Choices[0].Message.Content, nil
}
