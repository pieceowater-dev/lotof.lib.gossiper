package gossiper

import (
	"errors"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	tools "github.com/pieceowater-dev/lotof.lib.gossiper/internal/tools"
	"log"
	"os"
	"strings"
)

// EnvVars holds the mapped environment variables.
var EnvVars map[string]string

// Env provides environment handling methods such as loading, mapping, and validating environment variables.
type Env struct{}

// Get retrieves the value of the environment variable identified by 'key'.
// Returns an error if the variable is not found.
func (e *Env) Get(key string) (string, error) {
	value, exists := EnvVars[key]
	if !exists {
		return "", errors.New("environment variable not found: " + key)
	}
	return value, nil
}

// LoadEnv loads environment variables from the `.env` file using the godotenv package.
// It returns an error if the file is missing or cannot be loaded.
func (e *Env) LoadEnv() error {
	return godotenv.Load()
}

// MapEnv maps all environment variables into the EnvVars map by splitting each variable on `=`.
// It uses the `tools.Split` function to handle splitting.
func (e *Env) MapEnv() {
	EnvVars = make(map[string]string)
	t := &tools.Tools{} // Instance of Tools to use the Split method.
	for _, env := range os.Environ() {
		pair := t.SplitOnce(env, '=')
		if len(pair) == 2 {
			EnvVars[pair[0]] = pair[1] // Store the key-value pair in EnvVars.
		}
	}
}

// Init initializes the environment variables by loading them, mapping them, and validating required ones.
// It logs all user-defined environment variables and halts execution if required variables are missing.
func (e *Env) Init(required []string) {
	// Load environment variables from the .env file (if present)
	err := e.LoadEnv()
	if err != nil {
		log.Println("Error loading .env file:", err)
	}

	// Map system environment variables and .env values into EnvVars
	e.MapEnv()

	// List user-defined environment variables (ignoring system variables)
	var userEnvVars []string
	for key := range EnvVars {
		if e.isUserEnvVar(key) { // Only include user-defined variables
			userEnvVars = append(userEnvVars, key)
		}
	}

	// Log all user-defined environment variables in yellow
	color.Set(color.FgYellow)
	log.Println("Environment variables initialized:")
	log.Printf("[%s]", strings.Join(userEnvVars, ", "))

	// Validate the presence of required environment variables
	for _, envKey := range required {
		_, err := e.Get(envKey)
		if err != nil {
			// Log error in red if required variable is missing and stop execution
			color.Set(color.FgRed)
			log.Fatalf("Required environment variable %s not found: %v", envKey, err)
		}
	}
}

// isUserEnvVar checks if the provided key is a user-defined environment variable.
// It excludes common system variables.
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
	// Returns true if the key is not a common system variable
	return !systemVars[key]
}
