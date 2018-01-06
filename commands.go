package skyaway

type Command struct {
	Admin       bool
	Command     string
	Handlerfunc CommandHandler
}

type Commands []Command

func (bot *Bot) setCommandHandlers() {
	for _, command := range commands {
		bot.SetCommandHandler(command.Admin, command.Command, command.Handlerfunc)
	}

	bot.AddPrivateMessageHandler((*Bot).handleDirectMessageFallback)
	bot.AddGroupMessageHandler((*Bot).handleDirectMessageFallback)
}

var commands = Commands{
	Command{
		false,
		"help",
		(*Bot).handleCommandHelp,
	},
	Command{
		false,
		"start",
		(*Bot).handleCommandStart,
	},
	Command{
		true,
		"adduser",
		(*Bot).handleCommandAddUser,
	},
	Command{
		true,
		"makeadmin",
		(*Bot).handleCommandMakeAdmin,
	},
	Command{
		true,
		"removeadmin",
		(*Bot).handleCommandRemoveAdmin,
	},
	Command{
		true,
		"announce",
		(*Bot).handleCommandAnnounce,
	},
	Command{
		true,
		"announceevent",
		(*Bot).handleCommandAnnounceEvent,
	},
	Command{
		false,
		"listevent",
		(*Bot).handleCommandListEvent,
	},
	Command{
		true,
		"banuser",
		(*Bot).handleCommandBanUser,
	},
	Command{
		true,
		"unbanuser",
		(*Bot).handleCommandUnBanUser,
	},
	Command{
		true,
		"cancelevent",
		(*Bot).handleCommandCancelEvent,
	},
	Command{
		true,
		"scheduleevent",
		(*Bot).handleCommandScheduleEvent,
	},
	Command{
		true,
		"settings",
		(*Bot).handleCommandSettings,
	},
	Command{
		true,
		"startevent",
		(*Bot).handleCommandStartEvent,
	},
	Command{
		true,
		"stopevent",
		(*Bot).handleCommandStopEvent,
	},
	Command{
		true,
		"usercount",
		(*Bot).handleCommandUserCount,
	},
	Command{
		true,
		"users",
		func(bot *Bot, ctx *Context, command, args string) error {
			banned := false
			return bot.handleCommandUsersParsed(ctx, banned)
		},
	},
	Command{
		true,
		"bannedusers",
		func(bot *Bot, ctx *Context, command, args string) error {
			banned := true
			return bot.handleCommandUsersParsed(ctx, banned)
		},
	},
	Command{
		true,
		"listwinners",
		(*Bot).handleCommandListWinners,
	},
}
