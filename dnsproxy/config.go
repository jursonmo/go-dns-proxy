package dnsproxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/BurntSushi/toml"
)

var (
	defaultLevel = "info"
	defaultDay   = 3
	defaultPath  = fmt.Sprintf("%s.log", os.Args[0])
)

type Config struct {
	Proxy  *ProxyConfig  `toml:"dns"`
	Policy *PolicyConfig `toml:"policy"`
	Cache  *CacheConfig  `toml:"cache"`
	Log    *LogConfig    `toml:"log"`
}

type LogConfig struct {
	Level  string `toml:"level"`
	MaxDay int64  `toml:"max_day"`
	Path   string `toml:"path"`
}

func ParseConfig(path string) (*Config, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return parseConfig(content)
}

func parseConfig(bytes []byte) (*Config, error) {
	var cfg Config
	err := toml.Unmarshal(bytes, &cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Log == nil {
		cfg.Log = &LogConfig{
			Level:  defaultLevel,
			MaxDay: int64(defaultDay),
			Path:   defaultPath,
		}
	}

	return &cfg, err
}

func (c *Config) String() string {
	bytes, _ := json.Marshal(c)
	return string(bytes)
}
