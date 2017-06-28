package db

import (
	"time"

	_ "github.com/lib/pq"
	"github.com/gocraft/dbr"
)

type Config struct {
	Driver string `json:"driver"`
	Source string `json:"source"`
}

type User struct {
	ID        int          `json:"id"`
	UserName  string       `db:"username" json:"username,omitempty"`
	FirstName string       `json:"first_name,omitempty"`
	LastName  string       `json:"last_name,omitempty"`
	JoinedAt  dbr.NullTime `json:"joined_at,omitempty"`
	LeftAt    dbr.NullTime `json:"left_at,omitempty"`
	BannedAt  dbr.NullTime `json:"banned_at,omitempty"`
	Admin     bool         `json:"admin"`

	exists    bool
}

type Chat struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type Event struct {
	ID          int          `json:"id"`
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

func GetUser(id int) *User {
	var user User
	sess := GetSession()
	err := sess.Select("*").From("botuser").Where("id = ?", id).LoadStruct(&user)

	if err == dbr.ErrNotFound {
		return nil
	}

	if err != nil {
		return nil
	}

	user.exists = true
	return &user
}

func inThePast(t dbr.NullTime) bool {
	return t.Valid && t.Time.Before(time.Now())
}

func (u *User) Eligible() bool {
	if inThePast(u.BannedAt) {
		return false
	}
	if inThePast(u.JoinedAt) {
		if inThePast(u.LeftAt) {
			// re-joined after leaving
			return u.JoinedAt.Time.After(u.LeftAt.Time)
		} else {
			// joined and not left
			return true
		}
	} else {
		// never joined (should not happen normally)
		return false
	}
}

func (u *User) Put() error {
	sess := GetSession()
	if u.exists {
		_, err := sess.Update("botuser").
			Set("username", u.UserName).
			Set("first_name", u.FirstName).
			Set("last_name", u.LastName).
			Set("joined_at", u.JoinedAt).
			Set("left_at", u.LeftAt).
			Set("banned_at", u.BannedAt).
			Set("admin", u.Admin).
			Where("id = ?", u.ID).
			Exec()
		return err
	} else {
		_, err := sess.InsertInto("botuser").Columns(
			"id", "username", "first_name", "last_name",
			"joined_at", "left_at", "banned_at",
			"admin",
		).Record(u).Exec()
		if err != nil {
			u.exists = true
		}
		return err
	}
}

func Init(newConfig *Config) {
	config = newConfig
}
