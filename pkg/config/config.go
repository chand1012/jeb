// internal/config/config.go
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	OpenAI      OpenAIConfig      `mapstructure:"openai"`
	Concurrency ConcurrencyConfig `mapstructure:"concurrency"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type OpenAIConfig struct {
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`

	MaxTokens       int           `mapstructure:"max_tokens"`
	ReasoningEffort string        `mapstructure:"reasoning_effort"`
	Temperature     *float64      `mapstructure:"temperature"`
	Timeout         time.Duration `mapstructure:"timeout"`
	MaxRetries      int           `mapstructure:"max_retries"`
}

type ConcurrencyConfig struct {
	MaxRequests int `mapstructure:"max_requests"`
}

func Load() (*Config, error) {
	return LoadWithOverrides(nil)
}

// LoadWithOverrides applies explicitly supplied command-line values after
// environment variables and configuration files, before decoding and validation.
func LoadWithOverrides(overrides map[string]any) (*Config, error) {
	v := viper.New()

	setDefaults(v)
	configureEnvironment(v)
	configureFiles(v)

	if err := readConfigFile(v); err != nil {
		return nil, err
	}
	for key, value := range overrides {
		v.Set(key, value)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate configuration: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 6102)

	v.SetDefault("openai.base_url", "http://localhost:11434/v1")
	v.SetDefault("openai.api_key", "")
	v.SetDefault("openai.model", "qwen3.5:9b")
	v.SetDefault("openai.max_tokens", 10)
	v.SetDefault("openai.reasoning_effort", "none")

	v.SetDefault("openai.timeout", "60s")
	v.SetDefault("openai.max_retries", 3)

	v.SetDefault("concurrency.max_requests", 1)
}

func configureEnvironment(v *viper.Viper) {
	// Bind OpenAI settings to the conventional OPENAI_* environment namespace.
	mustBindEnv(v, "openai.base_url", "OPENAI_BASE_URL")
	mustBindEnv(v, "openai.api_key", "OPENAI_API_KEY")
	mustBindEnv(v, "openai.model", "OPENAI_MODEL")
	mustBindEnv(v, "openai.max_tokens", "OPENAI_MAX_TOKENS")
	mustBindEnv(v, "openai.reasoning_effort", "OPENAI_REASONING_EFFORT")
	mustBindEnv(v, "openai.temperature", "OPENAI_TEMPERATURE")
	mustBindEnv(v, "openai.timeout", "OPENAI_TIMEOUT")
	mustBindEnv(v, "openai.max_retries", "OPENAI_MAX_RETRIES")

	// Keep application-specific settings namespaced.
	mustBindEnv(v, "server.host", "JEB_HOST")
	mustBindEnv(v, "server.port", "JEB_PORT")

	mustBindEnv(v, "concurrency.max_requests", "JEB_MAX_REQUESTS")
}

func mustBindEnv(v *viper.Viper, key, env string) {
	if err := v.BindEnv(key, env); err != nil {
		panic(fmt.Sprintf("bind environment variable %s: %v", env, err))
	}
}

func configureFiles(v *viper.Viper) {
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".config", "jeb"))
	}
}

func readConfigFile(v *viper.Viper) error {
	if path := os.Getenv("JEB_CONFIG_FILE"); path != "" {
		v.SetConfigFile(path)

		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("read config file %q: %w", path, err)
		}

		return nil
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError

		// The config file is optional because environment variables may provide
		// the complete configuration.
		if errors.As(err, &notFound) {
			return nil
		}

		return fmt.Errorf("read config file: %w", err)
	}

	return nil
}

func (c Config) Validate() error {
	var errs []error

	if strings.TrimSpace(c.Server.Host) == "" {
		errs = append(errs, errors.New("server.host must not be empty"))
	}

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(
			errs,
			fmt.Errorf(
				"server.port must be between 1 and 65535, got %d",
				c.Server.Port,
			),
		)
	}

	if strings.TrimSpace(c.OpenAI.BaseURL) == "" {
		errs = append(errs, errors.New("openai.base_url must not be empty"))
	}

	if strings.TrimSpace(c.OpenAI.Model) == "" {
		errs = append(errs, errors.New("openai.model must not be empty"))
	}

	if c.OpenAI.MaxTokens < 1 {
		errs = append(errs, errors.New("openai.max_tokens must be at least 1"))
	}

	switch c.OpenAI.ReasoningEffort {
	case "", "none", "minimal", "low", "medium", "high", "xhigh":
	default:
		errs = append(
			errs,
			fmt.Errorf(
				"openai.reasoning_effort has unsupported value %q",
				c.OpenAI.ReasoningEffort,
			),
		)
	}

	if c.OpenAI.Temperature != nil && *c.OpenAI.Temperature < 0 {
		errs = append(
			errs,
			fmt.Errorf(
				"openai.temperature must not be negative, got %f",
				*c.OpenAI.Temperature,
			),
		)
	}

	if c.OpenAI.Timeout <= 0 {
		errs = append(errs, errors.New("openai.timeout must be greater than zero"))
	}

	if c.OpenAI.MaxRetries < 0 {
		errs = append(errs, errors.New("openai.max_retries must not be negative"))
	}

	if c.Concurrency.MaxRequests < 1 {
		errs = append(
			errs,
			errors.New("concurrency.max_requests must be at least 1"),
		)
	}

	return errors.Join(errs...)
}

func (c Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
