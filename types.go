package skyaway

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"strconv"
)

var nullString = []byte("null")

type Duration struct {
	time.Duration
	Valid bool
}

func NewDuration(d time.Duration) Duration {
	return Duration{d, true}
}

type User struct {
	ID        int    `json:"id"`
	UserName  string `db:"username" json:"username,omitempty"`
	FirstName string `db:"first_name" json:"first_name,omitempty"`
	LastName  string `db:"last_name" json:"last_name,omitempty"`
	Enlisted  bool   `json:"enlisted"`
	Banned    bool   `json:"banned"`
	Admin     bool   `json:"admin"`

	exists bool
}

type Participant struct {
	EventID   int      `db:"event_id" json:"event_id"`
	UserID    int      `db:"user_id" json:"user_id"`
	UserName  string   `db:"username" json:"username,omitempty"`
	Coins     int      `db:"coins" json:"coins"`
	ClaimedAt NullTime `db:"claimed_at" json:"claimed_at,omitempty"`
}

type TempUser struct {
	ID       int    `db:"id"`
	UserName string `db:"username"`
}

func (u *User) NameAndTags() string {
	var tags []string
	if u.Banned {
		tags = append(tags, "banned")
	}
	if u.Admin {
		tags = append(tags, "admin")
	}

	// If username is hidden use userid
	identifier := u.UserName
	if identifier == "" {
		identifier = strconv.Itoa(u.ID)
	}

	if len(tags) > 0 {
		return fmt.Sprintf("%s (%s)", identifier, strings.Join(tags, ", "))
	}

	return identifier
}

func (u *User) Exists() bool {
	return u.exists
}

type Chat struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type Event struct {
	ID          int      `json:"id"`
	Duration    Duration `json:"duration"`
	ScheduledAt NullTime `db:"scheduled_at" json:"scheduled_at"`
	StartedAt   NullTime `db:"started_at" json:"started_at"`
	EndedAt     NullTime `db:"ended_at" json:"ended_at"`
	Coins       int      `json:"coins"`
	Surprise    bool     `json:"surpruse"`
}

func (d Duration) Value() (driver.Value, error) {
	if !d.Valid {
		return nil, nil
	}
	return int64(d.Duration), nil
}

func (d *Duration) Scan(value interface{}) error {
	if value == nil {
		d.Valid = false
		return nil
	}
	i, ok := value.(int64)
	if !ok {
		return fmt.Errorf("cannot cast %T to int64 during duration scan", value)
	}
	d.Valid = true
	d.Duration = time.Duration(i)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	if d.Valid {
		return json.Marshal(d.String())
	}
	return nullString, nil
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, nullString) {
		return d.Scan(nil)
	}

	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if dur, err := time.ParseDuration(s); err != nil {
		return err
	} else {
		*d = Duration{Duration: dur, Valid: true}
		return nil
	}
}

type NullTime struct {
	Time  time.Time
	Valid bool
}

func (n NullTime) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Time, nil
}

func (n *NullTime) Scan(value interface{}) error {
	if value == nil {
		n.Time, n.Valid = time.Time{}, false
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		n.Time, n.Valid = v, true
		return nil
	}

	n.Valid = false
	return nil
}

func (n NullTime) MarshalJSON() ([]byte, error) {
	if n.Valid {
		return json.Marshal(n.Time)
	}
	return nullString, nil
}

func (n *NullTime) UnmarshalJSON(b []byte) error {
	// scan for null
	if bytes.Equal(b, nullString) {
		n.Time, n.Valid = time.Time{}, false
		return nil
	}
	// scan for JSON timestamp
	if err := json.Unmarshal(b, &n.Time); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

func NewNullTime(t time.Time) NullTime {
	return NullTime{Time: t, Valid: true}
}
