package skyaway

type DatabaseConfig struct {
	Driver string `json:"driver"`
	Source string `json:"source"`
}

type Config struct {
	Debug         bool           `json:"debug"`
	Token         string         `json:"token"`
	ChatID        int64          `json:"chat_id"`
	Database      DatabaseConfig `json:"database"`
	EventDuration Duration       `json:"event_duration"`
}
