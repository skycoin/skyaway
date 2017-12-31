package skyaway

type DatabaseConfig struct {
	Driver string `json:"driver"`
	Source string `json:"source"`
}

type WalletConfig struct {
	RPC       string `json:"rpc"`
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
	SecretKey string `json:"secret_key"`
}

type Config struct {
	Debug         bool           `json:"debug"`
	Token         string         `json:"token"`
	ChatID        int64          `json:"chat_id"`
	Database      DatabaseConfig `json:"database"`
	AnnounceEvery Duration       `json:"announce_every"`
}
