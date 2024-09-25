package environment

import (
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/pieceowater-dev/lotof.lib.gossiper/tools"
	"log"
	"os"
	"strings"
)

// Vars Global variable to store environment variables
var Vars map[string]string

// Get returns the value of the environment variable or an empty string if not found
func Get(key string) string {
	return Vars[key]
}

func LoadEnv() error {
	return godotenv.Load()
}

// MapEnv - Load environment variables into the global map - Vars
func MapEnv() {
	Vars = make(map[string]string)
	for _, env := range os.Environ() {
		pair := tools.Split(env, '=')
		if len(pair) == 2 {
			Vars[pair[0]] = pair[1]
		}
	}
}

// Init - Load env and populate global Vars object
func Init() {
	err := LoadEnv()
	if err != nil {
		log.Println("Error loading .env file:", err)
	}
	MapEnv()

	// List user-defined environment variables
	var userEnvVars []string
	for key := range Vars {
		if isUserEnvVar(key) { // Filter out only user-defined env vars
			userEnvVars = append(userEnvVars, key)
		}
	}

	// Log environment variables
	color.Set(color.FgYellow)
	log.Println("Environment variables initialized:")
	log.Printf("[%s]", strings.Join(userEnvVars, ", "))
}

// isUserEnvVar checks if the environment variable is user-defined
func isUserEnvVar(key string) bool {
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
