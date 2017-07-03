package main

import (
	"encoding/json"
	"log"
	"fmt"
	"time"
	"strconv"
	"os"
	"strings"

	"gopkg.in/telegram-bot-api.v4"
	"github.com/bcampbell/fuzzytime"

	"db"
)

type SkyAwayConfig struct {
	Token         string      `json:"token"`
	ChatID        int64       `json:"chat_id"`
	Database      db.Config   `json:"database"`
	EventDuration db.Duration `json:"event_duration"`
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
	Bot     *tgbotapi.BotAPI
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
	config.EventDuration = db.Duration{Duration: dur, Valid: true}
	return ctx.Reply(fmt.Sprintf("new event duration: %s", config.EventDuration))
}

func (ctx *Context) OnUserCount(banned bool) error {
	count, err := db.GetUserCount(banned)

	if err != nil {
		log.Printf("failed to get user count from db: %v", err)
		return err
	}

	return ctx.Reply(strconv.Itoa(count))
}

func (ctx *Context) OnUsers(banned bool) error {
	users, err := db.GetUsers(banned)

	if err != nil {
		log.Printf("failed to get users from db: %v", err)
		return err
	}

	var lines []string
	for i, user := range users {
		lines = append(lines, fmt.Sprintf(
			"%d. %d: %s", i, user.ID, user.NameAndTags(),
		))
	}
	if len(lines) > 0 {
		return ctx.Reply(strings.Join(lines, "\n"))
	} else {
		return ctx.Reply("no users in the list")
	}
}

func (ctx *Context) OnAddUser(name string) error {
	user := db.GetUserByName(name)
	if user == nil {
		return ctx.Reply("no user by that name")
	}
	var actions []string
	if user.Banned {
		user.Banned = false
		actions = append(actions, "unbanned")
	}
	if !user.Enlisted {
		user.Enlisted = true
		actions = append(actions, "enlisted")
	}
	if len(actions) > 0 {
		if err := user.Put(); err != nil {
			log.Printf("failed to save the user to db: %v", err)
			return err
		}
		return ctx.Reply(strings.Join(actions, ", "))
	}
	return ctx.Reply("no action required")
}

func (ctx *Context) OnSetBanned(name string, banned bool) error {
	user := db.GetUserByName(name)
	if user == nil {
		return ctx.Reply("no user by that name")
	}
	if user.Banned != banned {
		user.Banned = banned
		if err := user.Put(); err != nil {
			log.Printf("failed to save the user to db: %v", err)
			return err
		}
	}
	return ctx.Reply(user.NameAndTags())
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

func NiceDuration(d time.Duration) string {
	if d < 0 {
		return d.String()
	}

	var hours, minutes, seconds int
	seconds = int(d.Seconds())
	hours, seconds = seconds / 3600, seconds % 3600
	minutes, seconds = seconds / 60, seconds % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		} else {
			return fmt.Sprintf("%dh", hours)
		}
	} else {
		if minutes > 0 {
			if seconds > 0 {
				return fmt.Sprintf("%dm%ds", minutes, seconds)
			} else {
				return fmt.Sprintf("%dm", minutes)
			}
		} else {
			return fmt.Sprintf("%ds", seconds)
		}
	}
}

func (ctx *Context) OnScheduleEvent(coins int, t time.Time, surprise bool) error {
	if event := db.GetCurrentEvent(); event != nil {
		if event.StartedAt.Valid {
			return ctx.ReplyInMarkdown(fmt.Sprintf(
				"Already have an active event\n" +
				"*Coins:* %d\n" +
				"*Started:* %s (%s ago)\n" +
				"*Duration:* %s\n",
				event.Coins,
				event.StartedAt.Time.Format("Jan 2 2006, 15:04:05 -0700"),
				NiceDuration(time.Since(event.StartedAt.Time)),
				NiceDuration(event.Duration.Duration),
			))
		} else {
			return ctx.ReplyInMarkdown(fmt.Sprintf(
				"Already have an event in schedule\n" +
				"*Coins:* %d\n" +
				"*Start:* %s (%s from now)\n" +
				"*Duration:* %s\n",
				event.Coins,
				event.ScheduledAt.Time.Format("Jan 2 2006, 15:04:05 -0700"),
				NiceDuration(time.Until(event.ScheduledAt.Time)),
				NiceDuration(event.Duration.Duration),
			))
		}
	}

	ctx.ReplyInMarkdown(fmt.Sprintf(
		"Scheduling an event\n" +
		"*Coins:* %d\n" +
		"*Start:* %s (%s from now)\n" +
		"*Duration:* %s\n" +
		"*Surprise:* %t",
		coins,
		t.Format("Jan 2 2006, 15:04:05 -0700"),
		NiceDuration(time.Until(t)),
		NiceDuration(config.EventDuration.Duration),
		surprise,
	))
	err := db.ScheduleEvent(coins, t, config.EventDuration, surprise)
	if err != nil {
		log.Printf("failed to schedule event: %#v", err)
	}
	return err
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
				return ctx.Reply("malformed duration format: use something like 1.5, 1.5h, or 1h30m")
			}

			return ctx.OnSetEventDuration(dur)
		case "usercount":
			return ctx.OnUserCount(false)
		case "users":
			return ctx.OnUsers(false)
		case "bannedusers":
			return ctx.OnUsers(true)
		case "adduser":
			return ctx.OnAddUser(args)
		case "banuser":
			return ctx.OnSetBanned(args, true)
		case "scheduleevent":
			words := strings.Fields(args)
			if len(words) < 2 {
				return ctx.Reply(
					"insufficient arguments to /scheduleevent:" +
					"use something like `/scheduleevent 5 23:00 surprise",
				)
			}

			coins, err := strconv.Atoi(words[0])
			if err != nil {
				return ctx.Reply(
					"insufficient arguments to /scheduleevent:" +
					"use something like `/scheduleevent 5 23:00 surprise",
				)
			}

			surprise := words[len(words)-1] == "surprise"
			if surprise {
				// cut out the first and last word
				words = words[1:len(words)-1]
			} else {
				// cut out the first word
				words = words[1:len(words)]
			}

			timestr := strings.Join(words, " ")
			ft, _, err := fuzzytime.Extract(timestr)
			if ft.Empty() {
				return ctx.Reply("wrong datetime format")
			}

			var hour, minute, second int
			var loc *time.Location
			if ft.Time.HasHour() {
				hour = ft.Time.Hour()
			}
			if ft.Time.HasMinute() {
				minute = ft.Time.Minute()
			}
			if ft.Time.HasSecond() {
				second = ft.Time.Second()
			}
			if ft.Time.HasTZOffset() {
				loc = time.FixedZone("", ft.Time.TZOffset())
			} else {
				loc = time.UTC
			}

			var t time.Time
			if ft.HasFullDate() {
				t = time.Date(
					ft.Date.Year(),
					time.Month(ft.Date.Month()),
					ft.Date.Day(),
					hour, minute, second, 0,
					loc,
				)
			} else {
				year, month, day := time.Now().In(loc).Date()
				t = time.Date(
					year, month, day,
					hour, minute, second, 0,
					loc,
				)
				if t.Before(time.Now()) {
					t = t.AddDate(0, 0, 1)
				}
			}

			if t.Before(time.Now()) {
				return ctx.Reply(fmt.Sprintf("%s is in the past", t.String()))
			}

			return ctx.OnScheduleEvent(coins, t, surprise)
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

func (ctx *Context) OnUserActivity(u *tgbotapi.User) error {
	if u.ID == ctx.Bot.Self.ID {
		return nil
	}
	dbuser := db.GetUser(u.ID)
	if dbuser == nil {
		dbuser = &db.User{
			ID: u.ID,
			UserName: u.UserName,
			FirstName: u.FirstName,
			LastName: u.LastName,
		}
		if err := dbuser.Put(); err != nil {
			log.Printf("failed to save the user")
			return err
		}

		log.Printf("activity from untracked user: %s", dbuser.NameAndTags())
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
	if u := ctx.Message.From; u != nil {
		if err := ctx.OnUserActivity(u); u != nil {
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

func (ctx *Context) ReplyInParseMode(text, parseMode string) error {
	msg := tgbotapi.NewMessage(ctx.Message.Chat.ID, text)
	switch parseMode {
		case "markdown":
			msg.ParseMode = "Markdown"
		case "html":
			msg.ParseMode = "HTML"
		case "text":
			msg.ParseMode = ""
		default:
			return fmt.Errorf("unsupported parse mode: %s", parseMode)
	}
	msg.ReplyToMessageID = ctx.Message.MessageID
	_, err := ctx.Bot.Send(msg)
	return err

}

func (ctx *Context) ReplyInMarkdown(text string) error {
	return ctx.ReplyInParseMode(text, "markdown")
}

func (ctx *Context) Reply(text string) error {
	return ctx.ReplyInParseMode(text, "text")
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
