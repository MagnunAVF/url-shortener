package logger

import (
	"os"
	"strconv"
	"strings"
)

func InitFromEnv() {
	cfg := Config{
		Level:     getenvDefault("LOG_LEVEL", "info"),
		Format:    getenvDefault("LOG_FORMAT", "json"),
		AddSource: getenvBool("LOG_ADD_SOURCE", false),
		Service:   os.Getenv("LOG_SERVICE"),
		Env:       getenvDefault("LOG_ENV", getenvDefault("ENV", os.Getenv("APP_ENV"))),
		Output:    getenvDefault("LOG_OUTPUT", "stdout"),
	}
	Init(cfg)
}

func getenvDefault(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func getenvBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
