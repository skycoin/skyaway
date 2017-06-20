package main

import (
	"encoding/json"
	"log"
	"os"

	"gopkg.in/telegram-bot-api.v4"

	"db"
)

type SkyAwayConfig struct {
	Token string `json:"token"`
	ChatID int64 `json:"chat_id"`
	Database db.Config `json:"database"`
}

func loadJsonFromFile(filename string, result interface{}) error {
	infile, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer infile.Close()

	decoder := json.NewDecoder(infile)
	if err := decoder.Decode(result); err != nil {
		return err
	}

	return nil
}

func loadConfig(filename string) *SkyAwayConfig {
	var c SkyAwayConfig

	if err := loadJsonFromFile(filename, &c); err != nil {
		panic(err)
	}

	return &c
}

var config = loadConfig("config.json")

func main() {
	db.Init(&config.Database)

//	var users []db.User
//	sess := db.GetSession()
//	num, err := sess.Select("*").From("botuser").LoadStructs(&users)
//	if err != nil {
//		panic(err)
//	}
//
//	log.Printf("%d users: %#v", num, users)
//	return

	bot, err := tgbotapi.NewBotAPI(config.Token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.Chat.ID == config.ChatID {
			log.Printf("-------- my chat id: %d ---------", update.Message.Chat.ID)
		} else {
			log.Printf("-------- unknown chat id: %d ---------", update.Message.Chat.ID)
			//log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		text := update.Message.Text
		if text != "" {
			msg := tgbotapi.NewMessage(config.ChatID, update.Message.Text)
			bot.Send(msg)
		}

		//msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
		//msg.ReplyToMessageID = update.Message.MessageID

		//bot.Send(msg)
	}
}
