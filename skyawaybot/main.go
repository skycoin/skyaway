package main

import (
	"os"
	"fmt"
	"encoding/json"
	"log"

	_ "github.com/lib/pq"
	"github.com/kvap/skyaway"
)

func loadJsonFromFile(filename string, result interface{}) error {
	infile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf(
			"failed to open json from file '%s': %v",
			filename, err,
		)
	}
	defer infile.Close()

	decoder := json.NewDecoder(infile)
	if err := decoder.Decode(result); err != nil {
		return fmt.Errorf(
			"failed to decode json from file '%s': %v",
			filename, err,
		)
	}

	return nil
}

func Private(bot *skyaway.Bot, ctx *skyaway.Context, text string) (bool, error) {
	log.Printf("private message from %s: %s", ctx.User.NameAndTags(), text)
	return true, nil
}

func Hello(bot *skyaway.Bot, ctx *skyaway.Context, command, args string) error {
	log.Printf("hello from %s", ctx.User.NameAndTags())
	return nil
}

func Goodbye(bot *skyaway.Bot, ctx *skyaway.Context, command, args string) error {
	log.Printf("goodbye from %s", ctx.User.NameAndTags())
	return nil
}

func main() {
	var config skyaway.Config
	if err := loadJsonFromFile("config.json", &config); err != nil {
		panic(err)
	}

	bot, err := skyaway.NewBot(config)
	if err != nil {
		panic(err)
	}

	bot.SetCommandHandler(false, "hello", Hello)
	bot.SetCommandHandler(true, "goodbye", Goodbye)
	bot.AddPrivateMessageHandler(Private)

	bot.Start()
}
