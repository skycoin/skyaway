package main

import (
	"encoding/json"
	"log"
	"fmt"
	"time"
	"strconv"
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

func (ctx *Context) OnStart() error {
	helpCommand := "/help"
	if !ctx.Message.Chat.IsPrivate() {
		helpCommand += "@" + ctx.Bot.Self.UserName
	}
	return ctx.Reply(fmt.Sprintf(
		`Hey, this is a skycoin giveaway bot!
Type %s for details.`,
		helpCommand,
	))
}

func (ctx *Context) OnHelp() error {
	return ctx.Reply(`
/start
/help - this text
/settings

/seteventduration [hours] - set duration of event (how long users have to claim coins)
/scheduleevent [coins] [ISO timestamp, or human readable] [surprise] - start an event at timestamp
/cancelevent - cancel a scheduled event
/stopevent - stop current event
/startevent [number of coins] - start an event immediately
/adduser [username] - force add user to eligible list
/banuser [username] - blacklist user from eligible list
/announce [msg] - send announcement
/announceevent - force send current scheduled or ongoing event announcement
/usercount - return number of users
/users - return all users in list
/bannedusers - return all users in banned list`)
}

func (ctx *Context) OnSetEventDuration(dur time.Duration) error {
	if dur <= 0 {
		return ctx.Reply("event duration has to be positive")
	}
	config.EventDuration = Duration{dur}
	return ctx.Reply(fmt.Sprintf("new event duration: %s", config.EventDuration))
}

func (ctx *Context) OnSettings() error {
	chat, err := ctx.Bot.GetChat(tgbotapi.ChatConfig{config.ChatID, ""})
	if err != nil {
		log.Printf("failed to get chat info: %v", err)
		return err
	}

	settings := map[string]interface{}{
		"event_duration": config.EventDuration,
		"bot": map[string]interface{}{
			"id": ctx.Bot.Self.ID,
			"name": ctx.Bot.Self.UserName,
		},
		"chat": map[string]interface{}{
			"id": chat.ID,
			"type": chat.Type,
			"title": chat.Title,
		},
	}
	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		log.Printf("failed to encode current settings into json: %v", err)
		return err
	}
	return ctx.Reply(fmt.Sprintf("current settings: %s", string(encoded)))
}

func (ctx *Context) OnCommand(command string, args string) error {
	switch command {
		case "help":
			return ctx.OnHelp()
		case "start":
			return ctx.OnStart()
		case "settings":
			return ctx.OnSettings()
		case "seteventduration":
			hours, err := strconv.ParseFloat(args, 64)
			if err == nil {
				return ctx.OnSetEventDuration(time.Second * time.Duration(hours * 3600))
			}

			dur, err := time.ParseDuration(args)
			if err != nil {
				ctx.Reply("malformed duration format: use something like 1.5, 1.5h, or 1h30m")
				return nil
			}

			return ctx.OnSetEventDuration(dur)
		default:
			log.Printf("command not found: %s", command)
	}
	return nil
}

func (ctx *Context) OnPrivateMessage() error {
	//log.Printf("private message from %s: %s", ctx.Message.From.UserName, ctx.Message.Text)
	if ctx.Message.IsCommand() {
		return ctx.OnCommand(ctx.Message.Command(), ctx.Message.CommandArguments())
	}
	return nil
}

func (ctx *Context) OnUserJoin(user *tgbotapi.User) error {
	dbuser := db.GetUser(user.ID)
	if dbuser == nil {
		dbuser = &db.User{
			ID: user.ID,
			UserName: user.UserName,
			FirstName: user.FirstName,
			LastName: user.LastName,
		}
	}
	dbuser.Enlisted = true
	if err := dbuser.Put(); err != nil {
		log.Printf("failed to save the user")
		return err
	}

	log.Printf("user joined: %s", dbuser.NameAndTags())
	return nil
}

func (ctx *Context) OnUserLeft(user *tgbotapi.User) error {
	dbuser := db.GetUser(user.ID)
	if dbuser != nil {
		dbuser.Enlisted = false
		if err := dbuser.Put(); err != nil {
			log.Printf("failed to save the user")
			return err
		}

		log.Printf("user left: %s", dbuser.NameAndTags())
	}
	return nil
}

func (ctx *Context) OnGroupMessage() error {
	var gerr error
	if u := ctx.Message.NewChatMember; u != nil {
		if err := ctx.OnUserJoin(u); err != nil {
			gerr = err
		}
	}
	if u := ctx.Message.LeftChatMember; u != nil {
		if err := ctx.OnUserLeft(u); err != nil {
			gerr = err
		}
	}
	//log.Printf("group message from %s: %s", ctx.Message.From.UserName, ctx.Message.Text)
	return gerr
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
	msg.ReplyToMessageID = ctx.Message.MessageID
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
