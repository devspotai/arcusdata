package util

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func LoadEnvFile() {
	env := os.Getenv("APP_ENV")
	envFile := ".env." + env
	log.Println("Loading env file: " + envFile)
	err := godotenv.Load(envFile)
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func GetEnvName() string {
	return os.Getenv("APP_ENV")
}

func GetEnvAsInt(name string, defaultValue int) int {
	valueStr := GetEnv(name, "0")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func GetEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
