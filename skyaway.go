package skyaway

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"gopkg.in/telegram-bot-api.v4"
)

type Bot struct {
	config                 *Config
	db                     *DB
	telegram               *tgbotapi.BotAPI
	commandHandlers        map[string]CommandHandler
	adminCommandHandlers   map[string]CommandHandler
	privateMessageHandlers []MessageHandler
	groupMessageHandlers   []MessageHandler
	rescheduleChan         chan int
}

type Context struct {
	message *tgbotapi.Message
	User    *User
}

type CommandHandler func(*Bot, *Context, string, string) error
type MessageHandler func(*Bot, *Context, string) (bool, error)

var EventExists = errors.New("already have a current event")
var EventDoesNotExist = errors.New("no current event")

// Starts the current event immediately and return the event, if it exists.
// Returns `EventDoesNotExist` otherwise.
func (bot *Bot) StartCurrentEvent() (*Event, error) {
	event := bot.db.GetCurrentEvent()
	if event == nil {
		return nil, EventDoesNotExist
	}

	err := bot.db.StartEvent(event)
	if err != nil {
		return nil, fmt.Errorf("failed to start current event: %v", err)
	}
	defer bot.Reschedule()

	bot.AnnounceEventWithTitle(event, "Event has started!")

	return event, nil
}

// Unconditionally ends the current event immediately and return the event, if
// it exists.  Returns `EventDoesNotExist` otherwise.
func (bot *Bot) EndCurrentEvent() (*Event, error) {
	event := bot.db.GetCurrentEvent()
	if event == nil {
		return nil, EventDoesNotExist
	}

	err := bot.db.EndEvent(event)
	if err != nil {
		return nil, fmt.Errorf("failed to end current event: %v", err)
	}
	defer bot.Reschedule()

	switch {
	case event.StartedAt.Valid:
		bot.AnnounceEventWithTitle(event, "Event has ended!")
	case event.ScheduledAt.Valid:
		// Make a cancel announcement only if it is a public event
		if !event.Surprise {
			bot.AnnounceEventWithTitle(event, "The scheduled event has been cancelled")
		}
	default:
		log.Printf("the ended event was neither started, nor scheduled")
	}

	return event, nil
}

// Ends the current event immediately and return the event, if it exists and
// needs to be ended (no more coins or claimers).
// Returns err == `EventDoesNotExist` if no current event.
func (bot *Bot) EndCurrentEventIfNeeded() (event *Event, ended bool, err error) {
	event = bot.db.GetCurrentEvent()
	if event == nil {
		err = EventDoesNotExist
		return
	}

	var coins, claimers int

	if coins, err = bot.db.CoinsUnclaimed(event); err != nil {
		return
	}

	if claimers, err = bot.db.ClaimersLeft(event); err != nil {
		return
	}

	if coins > 0 && claimers > 0 {
		return
	}

	err = bot.db.EndEvent(event)
	if err != nil {
		err = fmt.Errorf("failed to end current event: %v", err)
		return
	}
	bot.AnnounceEventWithTitle(event, "Event has ended!")
	defer bot.Reschedule()
	ended = true
	return
}

// Starts an event immediately with given number of `coins` and `duration`.
// Returns the current event and `EventExists` error if there already is a
// current event (scheduled or started). Returns the new event if started
// successfully
func (bot *Bot) StartNewEvent(coins int, duration Duration) (*Event, error) {
	event := bot.db.GetCurrentEvent()
	if event != nil {
		return event, EventExists
	}

	err := bot.db.StartNewEvent(coins, duration)
	if err != nil {
		return nil, fmt.Errorf("failed to start event: %v", err)
	}
	defer bot.Reschedule()

	event = bot.db.GetCurrentEvent()
	if event == nil {
		return nil, fmt.Errorf("event did not start due to reasons unknown")
	}

	bot.AnnounceEventWithTitle(event, "Event has started!")
	return event, nil
}

func (bot *Bot) enableUser(u *User) ([]string, error) {
	var actions []string
	if !u.Exists() {
		actions = append(actions, "created")
	}
	if u.Banned {
		u.Banned = false
		actions = append(actions, "unbanned")
	}
	if !u.Enlisted {
		u.Enlisted = true
		actions = append(actions, "enlisted")
	}
	if len(actions) > 0 {
		if err := bot.db.PutUser(u); err != nil {
			return nil, fmt.Errorf("failed to change user status: %v", err)
		}
	}
	return actions, nil
}

func (bot *Bot) handleForwardedMessageFrom(ctx *Context, id int) error {
	args := tgbotapi.ChatConfigWithUser{bot.config.ChatID, "", id}
	member, err := bot.telegram.GetChatMember(args)
	if err != nil {
		return fmt.Errorf("failed to get chat member from telegram: %v", err)
	}

	if !member.IsMember() && !member.IsCreator() && !member.IsAdministrator() {
		return bot.Reply(ctx, "that user is not a member of the chat")
	}

	user := member.User
	log.Printf("forwarded from user: %#v", user)
	dbuser := bot.db.GetUser(user.ID)
	if dbuser == nil {
		dbuser = &User{
			ID:        user.ID,
			UserName:  user.UserName,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		}
	}

	return bot.enableUserVerbosely(ctx, dbuser)
}

func (bot *Bot) handleCommand(ctx *Context, command, args string) error {
	if !ctx.User.Banned {
		handler, found := bot.commandHandlers[command]
		if found {
			return handler(bot, ctx, command, args)
		}
	}

	if ctx.User.Admin {
		handler, found := bot.adminCommandHandlers[command]
		if found {
			return handler(bot, ctx, command, args)
		}
	}

	return fmt.Errorf("command not found: %s", command)
}

func (bot *Bot) handlePrivateMessage(ctx *Context) error {
	if ctx.User.Admin {
		// let admin force add users by forwarding their messages
		if u := ctx.message.ForwardFrom; u != nil {
			if err := bot.handleForwardedMessageFrom(ctx, u.ID); err != nil {
				return fmt.Errorf("failed to add user %s: %v", u.String(), err)
			}
			return nil
		}
	}

	if ctx.message.IsCommand() {
		cmd, args := ctx.message.Command(), ctx.message.CommandArguments()
		err := bot.handleCommand(ctx, cmd, args)
		if err != nil {
			log.Printf("command '/%s %s' failed: %v", cmd, args, err)
			return bot.Reply(ctx, fmt.Sprintf("command failed: %v", err))
		}
		return nil
	}

	for i := len(bot.privateMessageHandlers) - 1; i >= 0; i-- {
		handler := bot.privateMessageHandlers[i]
		next, err := handler(bot, ctx, ctx.message.Text)
		if err != nil {
			return fmt.Errorf("private message handler failed: %v", err)
		}
		if !next {
			break
		}
	}

	return nil
}

func (bot *Bot) handleUserJoin(ctx *Context, user *tgbotapi.User) error {
	if user.ID == bot.telegram.Self.ID {
		log.Printf("i have joined the group")
		return nil
	}
	dbuser := bot.db.GetUser(user.ID)
	if dbuser == nil {
		dbuser = &User{
			ID:        user.ID,
			UserName:  user.UserName,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		}
	}
	dbuser.Enlisted = true
	if err := bot.db.PutUser(dbuser); err != nil {
		log.Printf("failed to save the user")
		return err
	}

	log.Printf("user joined: %s", dbuser.NameAndTags())
	return nil
}

func (bot *Bot) handleUserLeft(ctx *Context, user *tgbotapi.User) error {
	if user.ID == bot.telegram.Self.ID {
		log.Printf("i have left the group")
		return nil
	}
	dbuser := bot.db.GetUser(user.ID)
	if dbuser != nil {
		dbuser.Enlisted = false
		if err := bot.db.PutUser(dbuser); err != nil {
			log.Printf("failed to save the user")
			return err
		}

		log.Printf("user left: %s", dbuser.NameAndTags())
	}
	return nil
}

func (bot *Bot) removeMyName(text string) (string, bool) {
	var removed bool
	var words []string
	for _, word := range strings.Fields(text) {
		if word == "@"+bot.telegram.Self.UserName {
			removed = true
			continue
		}
		words = append(words, word)
	}
	return strings.Join(words, " "), removed
}

func (bot *Bot) isReplyToMe(ctx *Context) bool {
	if re := ctx.message.ReplyToMessage; re != nil {
		if u := re.From; u != nil {
			if u.ID == bot.telegram.Self.ID {
				return true
			}
		}
	}
	return false
}

func (bot *Bot) handleGroupMessage(ctx *Context) error {
	var gerr error
	if u := ctx.message.NewChatMembers; u != nil {
		for _, user := range *u {
			if err := bot.handleUserJoin(ctx, &user); err != nil {
				gerr = err
			}
		}
	}
	if u := ctx.message.LeftChatMember; u != nil {
		if err := bot.handleUserLeft(ctx, u); err != nil {
			gerr = err
		}
	}

	if ctx.User != nil {
		msgWithoutName, mentioned := bot.removeMyName(ctx.message.Text)

		if mentioned || bot.isReplyToMe(ctx) {
			for i := len(bot.groupMessageHandlers) - 1; i >= 0; i-- {
				handler := bot.groupMessageHandlers[i]
				next, err := handler(bot, ctx, msgWithoutName)
				if err != nil {
					return fmt.Errorf("group message handler failed: %v", err)
				}
				if !next {
					break
				}
			}
		}
	}
	return gerr
}

func (bot *Bot) Send(ctx *Context, mode, format, text string) error {
	var msg tgbotapi.MessageConfig
	switch mode {
	case "whisper":
		msg = tgbotapi.NewMessage(int64(ctx.message.From.ID), text)
	case "reply":
		msg = tgbotapi.NewMessage(ctx.message.Chat.ID, text)
		msg.ReplyToMessageID = ctx.message.MessageID
	case "yell":
		msg = tgbotapi.NewMessage(bot.config.ChatID, text)
	default:
		return fmt.Errorf("unsupported message mode: %s", mode)
	}
	switch format {
	case "markdown":
		msg.ParseMode = "Markdown"
	case "html":
		msg.ParseMode = "HTML"
	case "text":
		msg.ParseMode = ""
	default:
		return fmt.Errorf("unsupported message format: %s", format)
	}
	_, err := bot.telegram.Send(msg)
	return err

}

func (bot *Bot) ReplyAboutEvent(ctx *Context, text string, event *Event) error {
	return bot.Send(ctx, "reply", "markdown", fmt.Sprintf(
		"%s\n%s", text, formatEventAsMarkdown(event, false),
	))
}

func (bot *Bot) Ask(ctx *Context, text string) error {
	msg := tgbotapi.NewMessage(ctx.message.Chat.ID, text)
	msg.ReplyMarkup = tgbotapi.ForceReply{
		ForceReply: true,
		Selective:  true,
	}
	msg.ReplyToMessageID = ctx.message.MessageID
	_, err := bot.telegram.Send(msg)
	return err
}

func (bot *Bot) Reply(ctx *Context, text string) error {
	return bot.Send(ctx, "reply", "text", text)
}

func (bot *Bot) handleMessage(ctx *Context) error {
	if (ctx.message.Chat.IsGroup() || ctx.message.Chat.IsSuperGroup()) && ctx.message.Chat.ID == bot.config.ChatID {
		return bot.handleGroupMessage(ctx)
	} else if ctx.message.Chat.IsPrivate() {
		return bot.handlePrivateMessage(ctx)
	} else {
		log.Printf("unknown chat %d (%s)", ctx.message.Chat.ID, ctx.message.Chat.UserName)
		return nil
	}
}

func NewBot(config Config) (*Bot, error) {
	var bot = Bot{
		config:               &config,
		commandHandlers:      make(map[string]CommandHandler),
		adminCommandHandlers: make(map[string]CommandHandler),
	}
	var err error

	if bot.db, err = NewDB(&config.Database); err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if bot.telegram, err = tgbotapi.NewBotAPI(config.Token); err != nil {
		return nil, fmt.Errorf("failed to initialize telegram api: %v", err)
	}

	bot.telegram.Debug = config.Debug

	chat, err := bot.telegram.GetChat(tgbotapi.ChatConfig{config.ChatID, ""})
	if err != nil {
		return nil, fmt.Errorf("failed to get chat info from telegram: %v", err)
	}
	if !chat.IsGroup() && !chat.IsSuperGroup() {
		return nil, fmt.Errorf("only group and supergroups are supported")
	}

	log.Printf("user: %d %s", bot.telegram.Self.ID, bot.telegram.Self.UserName)
	log.Printf("chat: %s %d %s", chat.Type, chat.ID, chat.Title)

	bot.setCommandHandlers()

	return &bot, nil
}

func (bot *Bot) handleUpdate(update *tgbotapi.Update) error {
	if update.Message == nil {
		return nil
	}

	ctx := Context{message: update.Message}

	if u := ctx.message.From; u != nil {
		dbuser := bot.db.GetUser(u.ID)
		if dbuser == nil {
			log.Printf("message from untracked user: %s, adding to db", u.String())

			dbuser = &User{
				ID:        u.ID,
				UserName:  u.UserName,
				FirstName: u.FirstName,
				LastName:  u.LastName,
			}
			if err := bot.db.PutUser(dbuser); err != nil {
				return fmt.Errorf("failed to save the user: %v", err)
			}
		}
		ctx.User = dbuser
	}

	return bot.handleMessage(&ctx)
}

func (bot *Bot) AnnounceEventWithTitle(event *Event, title string) error {
	md := formatEventAsMarkdown(event, true)
	md = fmt.Sprintf("*%s*\n%s", title, md)
	return bot.Send(&Context{}, "yell", "markdown", md)
}

func (bot *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.telegram.GetUpdatesChan(u)
	if err != nil {
		return fmt.Errorf("failed to create telegram updates channel: %v", err)
	}

	go bot.maintain()

	for update := range updates {
		if err := bot.handleUpdate(&update); err != nil {
			log.Printf("error: %v", err)
		}
	}
	log.Printf("stopped")
	return nil
}
