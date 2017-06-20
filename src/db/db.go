package db

import (
	_ "github.com/lib/pq"
	"github.com/gocraft/dbr"
)

type Config struct {
	Driver string `json:"driver"`
	Source string `json:"source"`
}

type User struct {
	ID        int64        `json:"id"`
	UserName  string       `db:"username" json:"username,omitempty"`
	FirstName string       `json:"first_name,omitempty"`
	LastName  string       `json:"last_name,omitempty"`
	JoinedAt  dbr.NullTime `json:"joined_at,omitempty"`
	LeftAt    dbr.NullTime `json:"left_at,omitempty"`
	BannedAt  dbr.NullTime `json:"banned_at,omitempty"`
	Admin     bool         `json:"admin"`
}

type Chat struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type Event struct {
	ID          int64        `json:"id"`
	Duration    int          `json:"duration"`
	ScheduledAt dbr.NullTime `json:"scheduled_at"`
	StartedAt   dbr.NullTime `json:"started_at"`
	EndedAt     dbr.NullTime `json:"ended_at"`
	Coins       int          `json:"coins"`
}

var config *Config
var conn *dbr.Connection

func GetConnection() *dbr.Connection {
	if conn == nil {
		if config == nil {
			panic("please call db.Init() before any other method")
		}

		// TODO: use a real log receiver here instead of `nil`
		var err error
		conn, err = dbr.Open(config.Driver, config.Source, nil)
		if err != nil {
			panic(err)
		}
	}

	return conn
}

func GetSession() *dbr.Session {
	// TODO: use a real log receiver here instead of `nil`
	return GetConnection().NewSession(nil)
}

func Init(newConfig *Config) {
	config = newConfig
}
