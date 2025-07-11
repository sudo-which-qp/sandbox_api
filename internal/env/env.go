package env

import (
	"log"
	"os"
	"strconv"
)

func GetString(key, fallback string) string {
	value, ok := os.LookupEnv(key)

	if !ok {
		log.Printf("%s not found, defaulting to %s", key, fallback)
		return fallback
	}

	return value
}

func GetInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)

	if !ok {
		return fallback
	}

	valueAsInt, err := strconv.Atoi(value)

	if err != nil {
		return fallback
	}

	return valueAsInt
}

func GetBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)

	if !ok {
		return fallback
	}

	valueAsBool, err := strconv.ParseBool(value)

	if err != nil {
		return fallback
	}

	return valueAsBool
}
