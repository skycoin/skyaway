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
	EventDuration Duration `json:"event_duration"`
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

type Context struct {
	Bot *tgbotapi.BotAPI
	Message *tgbotapi.Message
}

func (ctx *Context) OnPrivateMessage() error {
	log.Printf("private message from %s: %s", ctx.Message.From.UserName, ctx.Message.Text)
	return nil
}

func (ctx *Context) OnGroupMessage() error {
	log.Printf("group message from %s: %s", ctx.Message.From.UserName, ctx.Message.Text)
	return nil
}

func (ctx *Context) Yell(text string) error {
	msg := tgbotapi.NewMessage(config.ChatID, text)
	_, err := ctx.Bot.Send(msg)
	return err
}

func (ctx *Context) Whisper(text string) error {
	msg := tgbotapi.NewMessage(int64(ctx.Message.From.ID), text)
	_, err := ctx.Bot.Send(msg)
	return err
}

func (ctx *Context) Reply(text string) error {
	msg := tgbotapi.NewMessage(ctx.Message.Chat.ID, text)
	_, err := ctx.Bot.Send(msg)
	return err
}

func (ctx *Context) OnMessage(m *tgbotapi.Message) error {
	ctx.Message = m

	if ctx.Message.Chat.IsGroup() && ctx.Message.Chat.ID == config.ChatID {
		return ctx.OnGroupMessage()
	} else if ctx.Message.Chat.IsPrivate() {
		return ctx.OnPrivateMessage()
	} else {
		log.Printf("unknown chat %d (%s)", ctx.Message.Chat.ID, ctx.Message.Chat.UserName)
		return nil
	}
//
//	log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
//
//	text := update.Message.Text
//	if text != "" {
//		msg := tgbotapi.NewMessage(config.ChatID, update.Message.Text)
//		bot.Send(msg)
//	}

	//msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
	//msg.ReplyToMessageID = update.Message.MessageID

	//bot.Send(msg)
}

func main() {
	db.Init(&config.Database)

	bot, err := tgbotapi.NewBotAPI(config.Token)
	if err != nil {
		log.Panic(err)
	}

	chat, err := bot.GetChat(tgbotapi.ChatConfig{config.ChatID, ""})
	if err != nil {
		log.Panic(err)
	}
	if !chat.IsGroup() {
		log.Panic("only group chats supported")
	}

	bot.Debug = true

	log.Printf("user: %d %s", bot.Self.ID, bot.Self.UserName)
	log.Printf("chat: %s %d %s", chat.Type, chat.ID, chat.Title)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		ctx := Context{Bot: bot}
		if err := ctx.OnMessage(update.Message); err != nil {
			log.Printf("error: %v", err)
		}
	}
}
