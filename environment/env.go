package gossiper

import (
	"errors"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	tools "github.com/pieceowater-dev/lotof.lib.gossiper/tools"
	"log"
	"os"
	"strings"
)

type Env struct {
	Vars map[string]string
}

func (e *Env) Get(key string) (string, error) {
	value, exists := e.Vars[key]
	if !exists {
		return "", errors.New("environment variable not found: " + key)
	}
	return value, nil
}

func (e *Env) LoadEnv() error {
	return godotenv.Load()
}

func (e *Env) MapEnv() {
	e.Vars = make(map[string]string)
	t := &tools.Tools{} // Create an instance of Tools
	for _, env := range os.Environ() {
		pair := t.Split(env, '=')
		if len(pair) == 2 {
			e.Vars[pair[0]] = pair[1]
		}
	}
}

func (e *Env) Init(required []string) {
	err := e.LoadEnv()
	if err != nil {
		log.Println("Error loading .env file:", err)
	}
	e.MapEnv()

	// List user-defined environment variables
	var userEnvVars []string
	for key := range e.Vars {
		if e.isUserEnvVar(key) { // Filter out only user-defined env vars
			userEnvVars = append(userEnvVars, key)
		}
	}

	// Log environment variables
	color.Set(color.FgYellow)
	log.Println("Environment variables initialized:")
	log.Printf("[%s]", strings.Join(userEnvVars, ", "))

	// Validate required environment variables
	for _, envKey := range required {
		_, err := e.Get(envKey)
		if err != nil {
			color.Set(color.FgRed)
			log.Fatalf("Required environment variable %s not found: %v", envKey, err)
		}
	}
}

func (e *Env) isUserEnvVar(key string) bool {
	systemVars := map[string]bool{
		"PATH":                 true,
		"HOME":                 true,
		"USER":                 true,
		"PWD":                  true,
		"SHELL":                true,
		"XPC_FLAGS":            true,
		"HOMEBREW_REPOSITORY":  true,
		"LC_CTYPE":             true,
		"HOMEBREW_PREFIX":      true,
		"SSH_AUTH_SOCK":        true,
		"OLDPWD":               true,
		"__CFBundleIdentifier": true,
		"HOMEBREW_CELLAR":      true,
		"TMPDIR":               true,
	}
	return !systemVars[key]
}
