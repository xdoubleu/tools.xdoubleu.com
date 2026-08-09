// Package config provides functions which can be used to
// extract environment variables and parse them to the right type.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// ProdEnv can be used as value when reading out the type of environment.
const ProdEnv string = "production"

// TestEnv can be used as value when reading out the type of environment.
const TestEnv string = "test"

// DevEnv can be used as value when reading out the type of environment.
const DevEnv string = "development"

const errorMessage = "can't convert env var '%s' with value '%s' to %s"

// Parser extracts environment variables and parses them to the right type.
type Parser struct {
	logger *slog.Logger
}

// New returns a new Parser and loads environment variables that
// could be provided using a .env file (particularly useful during development).
func New(logger *slog.Logger) Parser {
	_ = godotenv.Load()

	return Parser{logger: logger}
}

func (c Parser) baseEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return ""
	}

	return value
}

func (c Parser) logValue(valType, key string, value any) {
	c.logger.Info(
		fmt.Sprintf("loaded env var '%s'='%v' with type '%s'", key, value, valType),
	)
}

// EnvStr extracts a string environment variable.
func (c Parser) EnvStr(key, defaultValue string) string {
	value := c.baseEnv(key)
	if len(value) == 0 {
		value = defaultValue
	}

	c.logValue("string", key, value)
	return value
}

// EnvSecret behaves like EnvStr but never logs the actual value, only
// whether one was set.
func (c Parser) EnvSecret(key, defaultValue string) string {
	value := c.baseEnv(key)
	if len(value) == 0 {
		value = defaultValue
	}

	masked := "<unset>"
	if len(value) > 0 {
		masked = "<redacted>"
	}
	c.logValue("string", key, masked)
	return value
}

// EnvInt extracts an integer environment variable.
func (c Parser) EnvInt(key string, defaultValue int) int {
	value := defaultValue

	strVal := c.baseEnv(key)
	if len(strVal) != 0 {
		intVal, err := strconv.Atoi(strVal)
		if err != nil {
			panic(fmt.Sprintf(errorMessage, key, strVal, "int"))
		}
		value = intVal
	}

	c.logValue("int", key, value)
	return value
}
