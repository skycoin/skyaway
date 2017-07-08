# skyaway - A telegram bot for skycoin giveaway events

This is a telegram bot which sits in a group and tracks its members. Admins
can start, end, schedule, and cancel events, and also manage the users.
The admin configures the event duration and specifies the total number of coins
to be given away during each event.

When an event starts, the bot copies the list of current users in the chat.
Everyone on that list will be able to claim some skycoins during the event.
Each user is able to claim `total_coins` / `number_of_users`. If this number is
not round, then the user will receive the rounded in a random direction number
of coins. E.g. if there is 10 coins and 3 users, each of them will receive 3 or
4 coins. The event ends earlier if no coins remain or when all users on the
list have made claims.

The bot will then listen for @replies or direct messages from users. If the
user is on the list of users that may receive coins, the bot asks their skycoin
address and then sends the coins there. If the user is not on the list, the bot
tells them to wait for the next event.

The bot is able to countdown to events.

## Install

`go get github.com/kvap/skyaway`

## Docs

Detailed docs are available at <http://godoc.org/github.com/kvap/skyaway>.

## Example

1. Build the example with `go build github.com/kvap/skyaway/skyawaybot`.
2. Set up the database (a schema for postgres is provided in `schema.postgres.sql`).
3. Create `config.json` in the current director (you can base upon `config.example.json`).
4. Run `./skyawaybot`.
