package skyaway

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"database/sql"

	"github.com/jmoiron/sqlx"
)

type DB struct {
	*sqlx.DB
}

var NotParticipating = errors.New("the user is not participating in the event")
var AlreadyClaimed = errors.New("the user has already claimed coins in the event")

func (db *DB) ScheduleEvent(coins int, start time.Time, duration Duration, surprise bool) error {
	_, err := db.Exec(db.Rebind(`
		insert into event (
			coins, duration, scheduled_at, surprise
		) values (?, ?, ?, ?)`),
		coins, duration, start, surprise,
	)
	return err
}

func (db *DB) StartNewEvent(coins int, duration Duration) error {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(tx.Rebind(`
		insert into event (
			coins, duration, started_at, surprise
		) values (?, ?, ?, ?)`),
		coins, duration, time.Now(), true,
	)
	if err != nil {
		return fmt.Errorf("failed to insert event: %v", err)
	}

	var event Event
	if err = tx.Get(&event, "SELECT * FROM event WHERE ended_at IS NULL"); err != nil {
		return fmt.Errorf("event inserted, but could not be found immediatly after: %v", err)
	}

	if err := event.addParticipants(tx); err != nil {
		return fmt.Errorf("failed to add participants: %v", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit the event: %v", err)
	}

	return nil
}

func (e *Event) addParticipants(tx *sqlx.Tx) error {
	var users []TempUser
	err := tx.Select(&users, "SELECT id, username FROM botuser WHERE NOT banned AND enlisted")
	if err != nil {
		return fmt.Errorf("failed to select eligible users for coin distribution: %v", err)
	}

	if len(users) == 0 {
		return nil
	}

	coinsPerUser := e.Coins / len(users)
	volatility := 0
	if e.Coins%len(users) != 0 {
		volatility = 1
	}

	for _, user := range users {
		coins := coinsPerUser + rand.Intn(volatility+1)
		_, err := tx.Exec(tx.Rebind(`
			insert into participant (
				event_id, user_id, username, coins
			) values (?, ?, ?, ?)`),
			e.ID, user.ID, user.UserName, coins,
		)
		if err != nil {
			return fmt.Errorf("failed to add user to event participants: %v", err)
		}
	}
	return nil
}

func (db *DB) CoinsClaimed(e *Event) (int, error) {
	var coins int
	err := db.Get(&coins, db.Rebind(`
		select sum(coins)
		from participant
		where event_id = ? and claimed_at is not null`),
		e.ID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to count claimed coins: %v", err)
	}
	return coins, nil
}

func (db *DB) CoinsUnclaimed(e *Event) (int, error) {
	claimed, err := db.CoinsClaimed(e)
	if err != nil {
		return 0, fmt.Errorf("failed to count unclaimed coins: %v", err)
	}
	return e.Coins - claimed, nil
}

func (db *DB) ClaimersLeft(e *Event) (int, error) {
	var claimers int
	err := db.Get(&claimers, db.Rebind(`
		select count(user_id)
		from participant
		where event_id = ? and claimed_at is null`),
		e.ID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to count claimers: %v", err)
	}
	return claimers, nil
}

func (db *DB) StartEvent(e *Event) error {
	if e.StartedAt.Valid {
		return errors.New("already started")
	}
	t := NewNullTime(time.Now())

	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		tx.Rebind("update event set started_at = ? where id = ?"),
		t, e.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update event status: %v", err)
	}

	if err := e.addParticipants(tx); err != nil {
		return fmt.Errorf("failed to add participants: %v", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit the event: %v", err)
	}

	e.StartedAt = t
	return nil
}

func (db *DB) EndEvent(e *Event) error {
	if e.EndedAt.Valid {
		return errors.New("already ended")
	}
	t := NewNullTime(time.Now())
	_, err := db.Exec(
		db.Rebind("update event set ended_at = ? where id = ?"),
		t, e.ID,
	)
	if err == nil {
		e.EndedAt = t
	}
	return err
}

func (db *DB) GetCurrentEvent() *Event {
	var event Event

	err := db.Get(&event, "SELECT * FROM event WHERE ended_at IS NULL")

	if err == sql.ErrNoRows {
		return nil
	}

	if err != nil {
		panic(err)
		return nil
	}

	return &event
}

func (db *DB) GetLastEvent() *Event {
	var event Event

	err := db.Get(&event, "SELECT * FROM event WHERE ended_at IS NOT NULL AND started_at IS NOT NULL ORDER BY id DESC LIMIT 1")

	if err == sql.ErrNoRows {
		return nil
	}

	if err != nil {
		panic(err)
		return nil
	}

	return &event
}

func NewDB(config *DatabaseConfig) (*DB, error) {
	if config == nil {
		errors.New("config should not be nil in NewDB()")
	}
	db, err := sqlx.Open(config.Driver, config.Source)
	if err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

func (db *DB) GetUser(id int) *User {
	var user User
	err := db.Get(&user, db.Rebind("select * from botuser where id=?"), id)
	if err == sql.ErrNoRows {
		return nil
	}

	if err != nil {
		return nil
	}

	user.exists = true
	return &user
}

func (db *DB) GetUserByName(name string) *User {
	var user User
	err := db.Get(&user, db.Rebind("select * from botuser where username=?"), name)

	if err == sql.ErrNoRows {
		return nil
	}

	if err != nil {
		return nil
	}

	user.exists = true
	return &user
}

func (db *DB) GetUserByNameOrId(identifier string) *User {
	var user User
	var err error

	err = db.Get(&user, db.Rebind("select * from botuser where username=? or id=?"), identifier, identifier)

	if err == sql.ErrNoRows {
		return nil
	}

	if err != nil {
		return nil
	}

	user.exists = true
	return &user
}

func (db *DB) GetUsers(banned bool) ([]User, error) {
	var users []User

	err := db.Select(&users, db.Rebind("select * from botuser where banned = ? order by username"), banned)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (db *DB) GetWinners(eventID int) ([]Participant, error) {
	var winners []Participant

	err := db.Select(&winners, db.Rebind("Select * from participant where event_id=?"), eventID)

	if err != nil {
		return []Participant{}, nil
	}

	return winners, nil
}

func (db *DB) ClaimCoins(user *User, event *Event) error {
	_, err := db.Exec(db.Rebind(`
		update participant
		set claimed_at = now()
		where
			user_id = ?
			and event_id = ?`),
		user.ID, event.ID,
	)
	return err
}

func (db *DB) GetCoinsToClaim(user *User, event *Event) (int, error) {
	var coins int
	var claimedAt NullTime
	err := db.QueryRowx(db.Rebind(`
		select coins, claimed_at
		from participant
		where
			user_id = ?
			and event_id = ?`),
		user.ID, event.ID,
	).Scan(&coins, &claimedAt)

	if err == sql.ErrNoRows {
		return 0, NotParticipating
	}

	if err != nil {
		return 0, err
	}

	if claimedAt.Valid {
		return coins, AlreadyClaimed
	}

	return coins, nil
}

func (db *DB) GetUserCount(banned bool) (int, error) {
	var count int

	err := db.Get(&count, db.Rebind("select count(*) from botuser where banned = ?"), banned)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (db *DB) PutUser(u *User) error {
	if u.exists {
		_, err := db.Exec(db.Rebind(`
			update botuser
				set username = ?,
				first_name = ?,
				last_name = ?,
				banned = ?,
				admin = ?
			where id = ?`),
			u.UserName,
			u.FirstName,
			u.LastName,
			u.Banned,
			u.Admin,
			u.ID,
		)
		return err
	} else {
		_, err := db.Exec(db.Rebind(`
			insert into botuser (
				id, username, first_name, last_name,
				banned, admin
			) values (?, ?, ?, ?, ?, ?)`),
			u.ID,
			u.UserName,
			u.FirstName,
			u.LastName,
			u.Banned,
			u.Admin,
		)
		if err == nil {
			u.exists = true
		}
		return err
	}
}
