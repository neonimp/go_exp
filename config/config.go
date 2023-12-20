package config

type Config struct {
	DispatchInterval int  `toml:"dispatch_interval"`
	DryMode          bool `toml:"dry_mode"`
	Smtp             struct {
		Net  string `toml:"net"`
		Host string `toml:"host"`
		Port int    `toml:"port"`
	} `toml:"smtp"`
	Auth struct {
		AuthUsers []string `toml:"auth_users"`
		AllowAnon bool     `toml:"allow_anon"`
	} `toml:"auth"`
	Ses struct {
		Region string `toml:"region"`
	} `toml:"ses"`
}
