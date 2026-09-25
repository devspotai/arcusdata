package util

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"

	"github.com/joho/godotenv"
)

// envNameRegex constrains APP_ENV so it cannot escape the working directory
// when it is concatenated into the dotenv filename.
var envNameRegex = regexp.MustCompile(`^[A-Za-z0-9_-]*$`)

// LoadEnvFile loads .env.<APP_ENV> into the process environment.
//
// It returns an error rather than terminating: a library has no business
// calling os.Exit on behalf of the program that imports it. Existing callers
// that invoke it as a statement continue to compile.
func LoadEnvFile() error {
	env := os.Getenv("APP_ENV")
	if !envNameRegex.MatchString(env) {
		return fmt.Errorf("invalid APP_ENV %q: expected only letters, digits, '-' or '_'", env)
	}
	envFile := ".env." + env
	log.Println("Loading env file: " + envFile)
	if err := godotenv.Load(envFile); err != nil {
		return fmt.Errorf("loading %s: %w", envFile, err)
	}
	return nil
}

func GetEnvName() string {
	return os.Getenv("APP_ENV")
}

func GetEnvAsInt(name string, defaultValue int) int {
	valueStr, ok := os.LookupEnv(name)
	if !ok {
		return defaultValue
	}
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
