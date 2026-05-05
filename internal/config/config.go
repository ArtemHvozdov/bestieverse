package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config is the single source of truth for all application configuration.
// It is loaded once at startup and passed to all components.
type Config struct {
	Bot     BotConfig
	DB      DBConfig
	OpenAI  OpenAIConfig
	Media   MediaConfig
	Log     LogConfig
	Support SupportConfig
	Timings Timings
	Messages Messages
	Tasks   []Task    // sorted by Task.Order
	Game    GameMessages
	TestMode bool
}

// BotConfig holds Telegram bot credentials.
type BotConfig struct {
	Token string
}

// DBConfig holds MySQL connection parameters.
type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

// DSN returns the MySQL data source name.
func (d DBConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		d.User, d.Password, d.Host, d.Port, d.Name)
}

// OpenAIConfig holds credentials and model config for OpenAI.
type OpenAIConfig struct {
	APIKey string
	Model  string
}

// MediaConfig holds the path to the media assets directory.
type MediaConfig struct {
	Path string
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level string
	File  string
}

// SupportConfig holds the URL for the support Telegram account.
type SupportConfig struct {
	TelegramURL string
}

// Load reads all configuration from the environment and content files.
// It loads .env first (ignored if absent), then reads YAML content files.
func Load() (*Config, error) {
	_ = godotenv.Load() // no-op if .env is absent (production uses real env vars)

	cfg := &Config{}
	cfg.TestMode = os.Getenv("TEST_MODE") == "true"

	cfg.Bot.Token = os.Getenv("BOT_TOKEN")

	cfg.DB.Host = envOrDefault("DB_HOST", "localhost")
	cfg.DB.Port = envOrDefault("DB_PORT", "3306")
	cfg.DB.Name = envOrDefault("DB_NAME", "gamebot")
	cfg.DB.User = os.Getenv("DB_USER")
	cfg.DB.Password = os.Getenv("DB_PASSWORD")

	cfg.OpenAI.APIKey = os.Getenv("OPENAI_API_KEY")
	cfg.OpenAI.Model = envOrDefault("OPENAI_MODEL", "gpt-image-1")

	cfg.Media.Path = envOrDefault("MEDIA_PATH", "./assets/media")

	cfg.Log.Level = envOrDefault("LOG_LEVEL", "info")
	cfg.Log.File = os.Getenv("LOG_FILE")

	cfg.Support.TelegramURL = os.Getenv("SUPPORT_TELEGRAM_URL")

	var err error
	cfg.Timings, err = loadTimings(cfg.TestMode)
	if err != nil {
		return nil, fmt.Errorf("config.Load: timings: %w", err)
	}

	cfg.Messages, err = loadMessages("content/messages.yaml")
	if err != nil {
		return nil, fmt.Errorf("config.Load: messages: %w", err)
	}

	cfg.Game, err = loadGameMessages("content/game.yml")
	if err != nil {
		return nil, fmt.Errorf("config.Load: game messages: %w", err)
	}

	cfg.Tasks, err = loadTasks("content/tasks")
	if err != nil {
		return nil, fmt.Errorf("config.Load: tasks: %w", err)
	}

	return cfg, nil
}

// TaskByOrder returns the task with the given order number, or nil if not found.
func (c *Config) TaskByOrder(order int) *Task {
	for i := range c.Tasks {
		if c.Tasks[i].Order == order {
			return &c.Tasks[i]
		}
	}
	return nil
}

// TaskByID returns the task with the given ID string, or nil if not found.
func (c *Config) TaskByID(id string) *Task {
	for i := range c.Tasks {
		if c.Tasks[i].ID == id {
			return &c.Tasks[i]
		}
	}
	return nil
}

func loadMessages(path string) (Messages, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Messages{}, fmt.Errorf("read %s: %w", path, err)
	}
	var msgs Messages
	if err := yaml.Unmarshal(data, &msgs); err != nil {
		return Messages{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return msgs, nil
}

func loadGameMessages(path string) (GameMessages, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GameMessages{}, fmt.Errorf("read %s: %w", path, err)
	}
	var gm GameMessages
	if err := yaml.Unmarshal(data, &gm); err != nil {
		return GameMessages{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return gm, nil
}

func loadTasks(dir string) ([]Task, error) {
	pattern := filepath.Join(dir, "task_*.yaml")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	tasks := make([]Task, 0, len(paths))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var t Task
		if err := yaml.Unmarshal(data, &t); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		tasks = append(tasks, t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Order < tasks[j].Order
	})
	return tasks, nil
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
