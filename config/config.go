package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Email    EmailConfig
	BankID   BankIDConfig
}

type AppConfig struct {
	Env                   string
	Port                  string
	BaseURL               string
	FrontendURL           string
	LaundryOpenHour       int
	LaundryCloseHour      int
	SaunaOpenHour         int
	SaunaCloseHour        int
	LaundryBookAheadWeeks int
}

type DatabaseConfig struct {
	URL             string
	Host            string
	Port            string
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		d.Host, d.Port, d.Name, d.User, d.Password, d.SSLMode)
}

func (d DatabaseConfig) MigrationURL() string {
	if d.URL != "" {
		return d.URL
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode)
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

type EmailConfig struct {
	Provider  string
	APIKey    string
	FromEmail string
	FromName  string
}

type BankIDConfig struct {
	Provider             string
	MockDelay            int
	SignicatClientID     string
	SignicatClientSecret string
	SignicatEndpoint     string
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	return &Config{
		App: AppConfig{
			Env: getEnv("APP_ENV", "development"), Port: getEnv("APP_PORT", "8080"),
			BaseURL: requireEnv("APP_BASE_URL"), FrontendURL: requireEnv("APP_FRONTEND_URL"),
			LaundryOpenHour: 7, LaundryCloseHour: 22, SaunaOpenHour: 16, SaunaCloseHour: 21,
			LaundryBookAheadWeeks: 8,
		},
		Database: DatabaseConfig{
			URL:  getEnv("DATABASE_URL", ""),
			Host: getEnv("DB_HOST", "localhost"), Port: getEnv("DB_PORT", "5432"),
			Name: requireEnvOrDefault("DB_NAME", "DATABASE_URL"), User: getEnv("DB_USER", "postgres"), Password: getEnv("DB_PASSWORD", ""),
			SSLMode: getEnv("DB_SSLMODE", "disable"), MaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_MIN", 5)) * time.Minute,
		},
		Redis:  RedisConfig{Addr: getEnv("REDIS_ADDR", "localhost:6379"), Password: getEnv("REDIS_PASSWORD", ""), DB: getEnvInt("REDIS_DB", 0)},
		JWT:    JWTConfig{AccessSecret: requireEnv("JWT_ACCESS_SECRET"), RefreshSecret: requireEnv("JWT_REFRESH_SECRET"), AccessTTL: 15 * time.Minute, RefreshTTL: 7 * 24 * time.Hour},
		Email:  EmailConfig{Provider: getEnv("EMAIL_PROVIDER", "mock"), APIKey: getEnv("RESEND_API_KEY", ""), FromEmail: getEnv("EMAIL_FROM", "noreply@hoas-booking.com"), FromName: getEnv("EMAIL_FROM_NAME", "HOAS Booking")},
		BankID: BankIDConfig{Provider: getEnv("BANK_ID_PROVIDER", "mock"), MockDelay: getEnvInt("BANK_ID_MOCK_DELAY_SEC", 2)},
	}, nil
}

func (c *Config) IsProd() bool { return c.App.Env == "production" }

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required env var %q not set", key))
	}
	return v
}

func requireEnvOrDefault(key, fallbackKey string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if os.Getenv(fallbackKey) != "" {
		return "" // will be ignored when DATABASE_URL is set
	}
	panic(fmt.Sprintf("required env var %q not set", key))
}
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
