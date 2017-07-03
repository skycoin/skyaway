package db

import (
	"strings"
	"fmt"
	"time"
	"errors"

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
	Enlisted  bool         `json:"enlisted"`
	Banned    bool         `json:"banned"`
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
	Duration    Duration     `json:"duration"`
	ScheduledAt NullTime     `json:"scheduled_at"`
	StartedAt   NullTime     `json:"started_at"`
	EndedAt     NullTime     `json:"ended_at"`
	Coins       int          `json:"coins"`
	Surprise    bool         `json:"surpruse"`
}

var config *Config
var conn *dbr.Connection

func ScheduleEvent(coins int, start time.Time, duration Duration, surprise bool) error {
	event := Event{
		Coins:       coins,
		ScheduledAt: NewNullTime(start),
		Duration:    duration,
		Surprise:    surprise,
	}
	sess := GetSession()
	_, err := sess.InsertInto("event").Columns(
		"coins", "duration", "scheduled_at", "surprise",
	).Record(&event).Exec()
	return err
}

func (e *Event) Start() error {
	if e.StartedAt.Valid {
		return errors.New("already started")
	}
	sess := GetSession()
	t := NewNullTime(time.Now())
	_, err := sess.Update("event").
		Set("started_at", t).
		Where("id = ?", e.ID).
		Exec()
	if err == nil {
		e.StartedAt = t
	}
	return err
}

func (e *Event) End() error {
	if e.EndedAt.Valid {
		return errors.New("already ended")
	}
	sess := GetSession()
	t := NewNullTime(time.Now())
	_, err := sess.Update("event").
		Set("ended_at", t).
		Where("id = ?", e.ID).
		Exec()
	if err == nil {
		e.EndedAt = t
	}
	return err
}

func GetCurrentEvent() *Event {
	var event Event
	sess := GetSession()
	err := sess.Select("*").From("event").Where("ended_at is null").LoadStruct(&event)

	if err == dbr.ErrNotFound {
		return nil
	}

	if err != nil {
		panic(err)
		return nil
	}

	return &event
}

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

func GetUserByName(name string) *User {
	var user User
	sess := GetSession()
	err := sess.Select("*").From("botuser").Where("username = ?", name).LoadStruct(&user)

	if err == dbr.ErrNotFound {
		return nil
	}

	if err != nil {
		return nil
	}

	user.exists = true
	return &user
}

func GetUsers(banned bool) ([]User, error) {
	sess := GetSession()

	all := sess.Select("*").From("botuser")
	filtered := all.Where("banned = ?", banned)
	ordered := filtered.OrderBy("username")
	var users []User
	_, err := ordered.LoadStructs(&users)

	if err != nil {
		return nil, err
	}

	return users, nil
}

func GetUserCount(banned bool) (int, error) {
	sess := GetSession()

	query := sess.Select("count(*)").From("botuser").Where("banned = ?", banned)
	var count int
	err := query.LoadValue(&count)

	if err != nil {
		return 0, err
	}

	return count, nil
}

func (u *User) Put() error {
	sess := GetSession()
	if u.exists {
		_, err := sess.Update("botuser").
			Set("username", u.UserName).
			Set("first_name", u.FirstName).
			Set("last_name", u.LastName).
			Set("banned", u.Banned).
			Set("admin", u.Admin).
			Where("id = ?", u.ID).
			Exec()
		return err
	} else {
		_, err := sess.InsertInto("botuser").Columns(
			"id", "username", "first_name", "last_name",
			"banned", "admin",
		).Record(u).Exec()
		if err != nil {
			u.exists = true
		}
		return err
	}
}

func (u *User) NameAndTags() string {
	var tags []string
	if u.Banned {
		tags = append(tags, "banned")
	}
	if u.Admin {
		tags = append(tags, "admin")
	}

	if len(tags) > 0 {
		return fmt.Sprintf("%s (%s)", u.UserName, strings.Join(tags, ", "))
	} else {
		return u.UserName
	}
}

func Init(newConfig *Config) {
	config = newConfig
}
