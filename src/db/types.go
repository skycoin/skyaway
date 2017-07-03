package db

import (
	"time"
	"encoding/json"
	"database/sql/driver"
	"fmt"
	"log"
	"bytes"

	"github.com/lib/pq"
)

type Duration struct {
	time.Duration
	Valid bool
}

var nullString = []byte("null")

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
	log.Printf("should save time %s as %s", n.Time, pq.FormatTimestamp(n.Time))
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
