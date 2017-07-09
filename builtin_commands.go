package skyaway

import (
	"time"
	"fmt"
	"log"
	"strconv"
	"strings"
	"encoding/json"

	"github.com/bcampbell/fuzzytime"
	"github.com/skycoin/skycoin/src/cipher"
	"gopkg.in/telegram-bot-api.v4"
)

func (bot *Bot) handleCommandStart(ctx *Context, command, args string) error {
	helpCommand := "/help"
	if !ctx.message.Chat.IsPrivate() {
		helpCommand += "@" + bot.telegram.Self.UserName
	}
	return bot.Reply(ctx, fmt.Sprintf(
		`Hey, this is a skycoin giveaway bot!
Type %s for details.`,
		helpCommand,
	))
}

func (bot *Bot) handleCommandHelp(ctx *Context, command, args string) error {
	return bot.Reply(ctx, `
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

func parseDuration(args string) (time.Duration, error) {
	hours, err := strconv.ParseFloat(args, 64)
	if err == nil {
		return time.Second * time.Duration(hours * 3600), nil
	}

	return time.ParseDuration(args)
}

func (bot *Bot) handleCommandSetEventDuration(ctx *Context, command, args string) error {
	dur, err := parseDuration(args)
	if err != nil {
		return bot.Reply(ctx, "malformed duration format: use something like 1.5, 1.5h, or 1h30m")
	}
	if dur <= 0 {
		return bot.Reply(ctx, "event duration has to be positive")
	}

	bot.config.EventDuration = NewDuration(dur)
	return bot.Reply(ctx, fmt.Sprintf("new event duration: %s", bot.config.EventDuration))
}

func (bot *Bot) handleCommandUserCount(ctx *Context, command, args string) error {
	banned := false
	count, err := bot.db.GetUserCount(banned)

	if err != nil {
		return fmt.Errorf("failed to get user count from db: %v", err)
	}

	return bot.Reply(ctx, strconv.Itoa(count))
}

func (bot *Bot) handleCommandUsersParsed(ctx *Context, banned bool) error {
	users, err := bot.db.GetUsers(banned)

	if err != nil {
		return fmt.Errorf("failed to get users from db: %v", err)
	}

	var lines []string
	for i, user := range users {
		lines = append(lines, fmt.Sprintf(
			"%d. %d: %s", i, user.ID, user.NameAndTags(),
		))
	}
	if len(lines) > 0 {
		return bot.Reply(ctx, strings.Join(lines, "\n"))
	} else {
		return bot.Reply(ctx, "no users in the list")
	}
}

func (bot *Bot) handleCommandAddUser(ctx *Context, command, args string) error {
	name := args
	dbuser := bot.db.GetUserByName(name)
	if dbuser == nil {
		return bot.Reply(ctx, "no user by that name")
	}
	return bot.enableUserVerbosely(ctx, dbuser)
}

func (bot *Bot) enableUserVerbosely(ctx *Context, dbuser *User) error {
	actions, err := bot.enableUser(dbuser)
	if err != nil {
		return fmt.Errorf("failed to enable user: %v", err)
	}
	if len(actions) > 0 {
		return bot.Reply(ctx, strings.Join(actions, ", "))
	}
	return bot.Reply(ctx, "no action required")
}

func (bot *Bot) handleCommandBanUser(ctx *Context, command, args string) error {
	name := args
	user := bot.db.GetUserByName(name)
	if user == nil {
		return bot.Reply(ctx, "no user by that name")
	}
	if !user.Banned {
		user.Banned = true
		if err := bot.db.PutUser(user); err != nil {
			return fmt.Errorf("failed to change user status: %v", err)
		}
	}
	return bot.Reply(ctx, user.NameAndTags())
}

func (bot *Bot) handleCommandSettings(ctx *Context, command, args string) error {
	chat, err := bot.telegram.GetChat(tgbotapi.ChatConfig{bot.config.ChatID, ""})
	if err != nil {
		return fmt.Errorf("failed to get chat info: %v", err)
	}

	settings := map[string]interface{}{
		"event_duration": bot.config.EventDuration,
		"bot": map[string]interface{}{
			"id": bot.telegram.Self.ID,
			"name": bot.telegram.Self.UserName,
		},
		"chat": map[string]interface{}{
			"id": chat.ID,
			"type": chat.Type,
			"title": chat.Title,
		},
	}
	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode current settings into json: %v", err)
	}
	return bot.Reply(ctx, fmt.Sprintf("current settings: %s", string(encoded)))
}

func (bot *Bot) complainIfHaveCurrentEvent(ctx *Context) (bool, error) {
	if event := bot.db.GetCurrentEvent(); event != nil {
		if event.StartedAt.Valid {
			return true, bot.ReplyAboutEvent(ctx, "already have an active event", event)
		} else {
			return true, bot.ReplyAboutEvent(ctx, "already have an event in schedule", event)
		}
	}
	return false, nil
}

func (bot *Bot) handleCommandStartEvent(ctx *Context, command, args string) error {
	coins, err := strconv.Atoi(args)
	if err != nil {
		return bot.Reply(ctx, "malformed coins format: use an integer number")
	}

	event, err := bot.StartNewEvent(coins, bot.config.EventDuration)
	if err == EventExists {
		return bot.ReplyAboutEvent(ctx, "already have an event", event)
	}
	if err != nil {
		return err
	}

	bot.AnnounceEventWithTitle(event, "Event has started!")
	bot.Reschedule()
	return bot.ReplyAboutEvent(ctx, "event started", event)
}

func (bot *Bot) handleCommandCancelEvent(ctx *Context, command, args string) error {
	event := bot.db.GetCurrentEvent()
	if event == nil {
		return bot.Reply(ctx, "nothing to cancel")
	}

	if event.StartedAt.Valid {
		return bot.ReplyAboutEvent(
			ctx,
			"the event has already started, use /stopevent instead",
			event,
		)
	}

	if err := bot.db.EndEvent(event); err != nil {
		return fmt.Errorf("failed to cancel event: %v", err)
	}

	bot.AnnounceEventWithTitle(event, "The scheduled event has been cancelled")
	bot.Reschedule()
	return bot.ReplyAboutEvent(ctx, "event cancelled", event)
}

func (bot *Bot) handleCommandStopEvent(ctx *Context, command, args string) error {
	event := bot.db.GetCurrentEvent()
	if event == nil {
		return bot.Reply(ctx, "nothing to stop")
	}

	if !event.StartedAt.Valid {
		return bot.ReplyAboutEvent(
			ctx,
			"the event has not started yet, use /cancelevent instead",
			event,
		)
	}

	if err := bot.db.EndEvent(event); err != nil {
		return fmt.Errorf("failed to stop event: %v", err)
	}

	bot.AnnounceEventWithTitle(event, "Event has ended!")
	bot.Reschedule()
	return bot.ReplyAboutEvent(ctx, "event stopped", event)
}

func parseScheduleEventArgs(args string) (coins int, start time.Time, surprise bool, err error) {
	words := strings.Fields(args)
	if len(words) < 2 {
		err = fmt.Errorf("insufficient arguments")
		return
	}

	coins, err = strconv.Atoi(words[0])
	if err != nil {
		err = fmt.Errorf("could not parse the number of coins: %v", err)
		return
	}

	surprise = words[len(words)-1] == "surprise"
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
		err = fmt.Errorf("unsupported datetime format: %v", timestr)
		return
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

	if ft.HasFullDate() {
		start = time.Date(
			ft.Date.Year(),
			time.Month(ft.Date.Month()),
			ft.Date.Day(),
			hour, minute, second, 0,
			loc,
		)
	} else {
		year, month, day := time.Now().In(loc).Date()
		start = time.Date(
			year, month, day,
			hour, minute, second, 0,
			loc,
		)
		if start.Before(time.Now()) {
			start = start.AddDate(0, 0, 1)
		}
	}

	if start.Before(time.Now()) {
		err = fmt.Errorf("%s is in the past", start.String())
		return
	}

	return
}

func (bot *Bot) handleCommandScheduleEvent(ctx *Context, command, args string) error {
	coins, start, surprise, err := parseScheduleEventArgs(args)
	if err != nil {
		return fmt.Errorf("could not understand: %v", err)
	}

	haveCurrent, err := bot.complainIfHaveCurrentEvent(ctx)
	if haveCurrent || err != nil {
		return err
	}

	err = bot.db.ScheduleEvent(coins, start, bot.config.EventDuration, surprise)
	if err != nil {
		return fmt.Errorf("failed to schedule event: %v", err)
	}

	event := bot.db.GetCurrentEvent()
	if event == nil {
		return fmt.Errorf("event was not scheduled due to reasons unknown")
	}

	if !surprise {
		bot.AnnounceEventWithTitle(event, "A new event has been scheduled!")
	}
	bot.Reschedule()
	return bot.ReplyAboutEvent(ctx, "event scheduled", event)
}

func (bot *Bot) handleCommandAnnounce(ctx *Context, command, args string) error {
	msg := strings.TrimSpace(args)
	if msg == "" {
		return fmt.Errorf("cannot announce an empty message")
	}
	if err := bot.Send(ctx, "yell", "text", msg); err != nil {
		return fmt.Errorf("failed to announce: %v", err)
	}

	return bot.Reply(ctx, "done")
}

func (bot *Bot) handleCommandAnnounceEvent(ctx *Context, command, args string) error {
	event := bot.db.GetCurrentEvent()
	if event == nil {
		return bot.Reply(ctx, "nothing to announce")
	}

	md := formatEventAsMarkdown(event, true)
	if err := bot.Send(ctx, "yell", "markdown", md); err != nil {
		return fmt.Errorf("failed to announce event: %v", err)
	}

	return bot.Reply(ctx, "done")
}

func (bot *Bot) handlePrivateMessageFallback(ctx *Context, text string) (bool, error) {
	event := bot.db.GetCurrentEvent()

	started := event != nil && event.StartedAt.Valid
	canTellWhen := event != nil && !event.Surprise

	if !started {
		if canTellWhen {
			return true, bot.Reply(ctx, fmt.Sprintf(
				"event has not started yet, come back in %s",
				niceDuration(time.Until(event.ScheduledAt.Time)),
			))
		} else {
			return true, bot.Reply(ctx, "event has not started yet, come back later")
		}
	}

	coins, err := bot.db.GetCoinsToClaim(ctx.User, event)

	if ctx.User.Banned || err == NotParticipating {
		return true, bot.Reply(ctx, "you are not eligible to claim coins in this event")
	}

	if err == AlreadyClaimed {
		return true, bot.Reply(ctx, "you have already claimed your coins in this event")
	}

	if err != nil {
		log.Printf("failed to get the number of coins to claim: %v", err)
		return true, bot.Reply(ctx, "sorry, something went wrong")
	}

	addr, err := cipher.DecodeBase58Address(text)
	if err != nil {
		log.Printf("not a skycoin address: %v", err)
		return true, bot.Reply(ctx, "please send your skycoin address to get coins")
	}

	if err := bot.db.ClaimCoins(ctx.User, event); err != nil {
		log.Printf("failed to store that the coins have been claimed: %v", err)
		return true, bot.Reply(ctx, "sorry, something went wrong")
	}

	// TODO: implement skycoin sending
	log.Printf("send %d coins to %s", coins, addr.String())
	return true, bot.Reply(ctx, fmt.Sprintf("%d coins are on the way to your wallet", coins))
}

func (bot *Bot) setBuiltInCommandHandlers() {
	bot.SetCommandHandler(false, "help",             (*Bot).handleCommandHelp)
	bot.SetCommandHandler(false, "start",            (*Bot).handleCommandStart)
	bot.SetCommandHandler(true,  "adduser",          (*Bot).handleCommandAddUser)
	bot.SetCommandHandler(true,  "announce",         (*Bot).handleCommandAnnounce)
	bot.SetCommandHandler(true,  "announceevent",    (*Bot).handleCommandAnnounceEvent)
	bot.SetCommandHandler(true,  "banuser",          (*Bot).handleCommandBanUser)
	bot.SetCommandHandler(true,  "cancelevent",      (*Bot).handleCommandCancelEvent)
	bot.SetCommandHandler(true,  "scheduleevent",    (*Bot).handleCommandScheduleEvent)
	bot.SetCommandHandler(true,  "seteventduration", (*Bot).handleCommandSetEventDuration)
	bot.SetCommandHandler(true,  "settings",         (*Bot).handleCommandSettings)
	bot.SetCommandHandler(true,  "startevent",       (*Bot).handleCommandStartEvent)
	bot.SetCommandHandler(true,  "stopevent",        (*Bot).handleCommandStopEvent)
	bot.SetCommandHandler(true,  "usercount",        (*Bot).handleCommandUserCount)
	bot.SetCommandHandler(true,  "users", func(bot *Bot, ctx *Context, command, args string) error {
		banned := false
		return bot.handleCommandUsersParsed(ctx, banned)
	})
	bot.SetCommandHandler(true, "bannedusers", func(bot *Bot, ctx *Context, command, args string) error {
		banned := true
		return bot.handleCommandUsersParsed(ctx, banned)
	})
	bot.AddPrivateMessageHandler((*Bot).handlePrivateMessageFallback)
}
