package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"Ai_Bot/api"
	"Ai_Bot/database"
	"Ai_Bot/models"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gopkg.in/telebot.v4"
)

// test
const WebAppURL = "https://neonatal-leeanne-unroped.ngrok-free.dev"

var globalBot *telebot.Bot

func main() {
	// 🔥 .env ENG BOSHIDA
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ .env yuklanmadi (serverda normal holat)")
	}

	// DB init
	database.InitDB()

	// BotSender (image.go ishlatadi)
	api.BotSender = func(chatID int64, filePath, caption string) {
		if globalBot == nil {
			return
		}
		recipient := &telebot.Chat{ID: chatID}
		photo := &telebot.Photo{
			File:    telebot.FromDisk(filePath),
			Caption: caption,
		}
		if _, err := globalBot.Send(recipient, photo); err != nil {
			log.Println("[BOT] Rasm yuborishda xato:", err)
		}
	}

	// parallel ishga tushiramiz
	go startWebServer()
	startBot()
}

// 🌐 WEB SERVER
func startWebServer() {
	r := gin.Default()

	// 🔐 Proxy fix
	r.SetTrustedProxies(nil)

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type"},
	}))

	apiGroup := r.Group("/api")
	{
		apiGroup.POST("/chat", api.ChatHandler)
		apiGroup.GET("/sessions", api.GetSessionsHandler)
		apiGroup.GET("/history", api.GetHistoryHandler)
		apiGroup.DELETE("/session/:session_id", api.DeleteSessionHandler)

		apiGroup.POST("/generate-image", api.GenerateImageHandler)
		apiGroup.POST("/analyze-image", api.AnalyzeImageHandler)
	}

	// static
	r.Static("/generated", "./frontend/generated")

	r.StaticFile("/", "./frontend/index.html")
	r.StaticFile("/app.js", "./frontend/app.js")
	r.StaticFile("/style.css", "./frontend/style.css")

	r.NoRoute(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.File("./frontend" + c.Request.URL.Path)
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "topilmadi"})
		}
	})

	log.Println("✅ Web server: http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Server xato:", err)
	}
}

// TELEGRAM BOT
func startBot() {
	token := os.Getenv("TELEGRAM_TOKEN")

	if token == "" {
		log.Fatal("❌ TELEGRAM_TOKEN topilmadi (.env tekshir)")
	}

	pref := telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatal("❌ Bot xato:", err)
	}

	globalBot = bot
	log.Println("✅ Bot ishga tushdi...")

	// /start
	bot.Handle("/start", func(c telebot.Context) error {
		menu := &telebot.ReplyMarkup{ResizeKeyboard: true}
		contactBtn := menu.Contact("📞 Telefon raqamni yuborish")
		menu.Reply(menu.Row(contactBtn))

		return c.Send("Assalomu alaykum! Ro'yxatdan o'tish uchun telefon raqamingizni yuboring 👇", menu)
	})

	// Contact
	bot.Handle(telebot.OnContact, func(c telebot.Context) error {
		contact := c.Message().Contact

		var user models.User
		result := database.DB.Where(models.User{TelegramID: contact.UserID}).
			FirstOrCreate(&user, models.User{
				TelegramID: contact.UserID,
				FirstName:  contact.FirstName,
				LastName:   contact.LastName,
				Phone:      contact.PhoneNumber,
			})

		if result.Error != nil {
			log.Println("DB xato:", result.Error)
		}

		_ = c.Send("✅ Muvaffaqiyatli ro'yxatdan o'tdingiz!", &telebot.ReplyMarkup{RemoveKeyboard: true})

		inlineMenu := &telebot.ReplyMarkup{}
		webAppBtn := inlineMenu.WebApp("🤖 AI Chatni Ochish", &telebot.WebApp{URL: WebAppURL})
		inlineMenu.Inline(inlineMenu.Row(webAppBtn))

		return c.Send("Quyidagi tugmani bosing:", inlineMenu)
	})

	// Photo
	bot.Handle(telebot.OnPhoto, func(c telebot.Context) error {
		return c.Send("📷 Rasmni WebApp orqali yuboring 👇", &telebot.ReplyMarkup{
			InlineKeyboard: [][]telebot.InlineButton{
				{{Text: "🤖 WebApp ni Ochish", WebApp: &telebot.WebApp{URL: WebAppURL}}},
			},
		})
	})

	// Text
	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		inlineMenu := &telebot.ReplyMarkup{}
		webAppBtn := inlineMenu.WebApp("🤖 AI Chatni Ochish", &telebot.WebApp{URL: WebAppURL})
		inlineMenu.Inline(inlineMenu.Row(webAppBtn))

		return c.Send("AI bilan gaplashish uchun tugmani bosing 👇", inlineMenu)
	})

	bot.Start()
}
