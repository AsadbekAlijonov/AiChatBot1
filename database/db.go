package database

import (
	"log"

	"Ai_Bot/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("bot_database.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // "record not found" loglarni yashirish
	})
	if err != nil {
		log.Fatal("Bazaga ulanishda xato: ", err)
	}

	// Avtomatik jadvallarni yaratish
	err = DB.AutoMigrate(
		&models.User{},
		&models.ChatSession{},
		&models.ChatMessage{},
	)
	if err != nil {
		log.Fatal("Migratsiyada xato: ", err)
	}

	log.Println("Database muvaffaqiyatli ishga tushdi!")
}
