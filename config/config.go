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
	Sender struct {
		Provider         string         `toml:"provider"`
		ProviderSettings map[string]any `toml:"settings"`
	} `toml:"sender"`
}

func (c *Config) GetProviderSetting(k string) (any, bool) {
	v, ok := c.Sender.ProviderSettings[k]
	return v, ok
}

func (c *Config) GetProviderStringSetting(k string) (string, bool) {
	v, ok := c.Sender.ProviderSettings[k]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func (c *Config) GetProviderIntSetting(k string) (int, bool) {
	v, ok := c.Sender.ProviderSettings[k]
	if !ok {
		return 0, false
	}
	i, ok := v.(int)
	return i, ok
}

func (c *Config) GetProviderBoolSetting(k string) (bool, bool) {
	v, ok := c.Sender.ProviderSettings[k]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}
