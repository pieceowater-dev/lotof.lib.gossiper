package gossiper

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Tools struct{}

// Split splits a string by the given separator rune
func (t *Tools) Split(s string, sep rune) []string {
	var parts []string
	var part []rune
	for _, c := range s {
		if c == sep {
			if len(part) > 0 {
				parts = append(parts, string(part))
				part = nil
			}
		} else {
			part = append(part, c)
		}
	}
	if len(part) > 0 {
		parts = append(parts, string(part))
	}
	return parts
}

// ToString converts any value to a string
func (t *Tools) ToString(val interface{}) string {
	return fmt.Sprintf("%v", val)
}

// ToInt converts a string to an integer
func (t *Tools) ToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// ToBool converts a string to a boolean
func (t *Tools) ToBool(s string) (bool, error) {
	return strconv.ParseBool(s)
}

// Join concatenates elements of a slice into a single string with the given separator
func (t *Tools) Join(arr []string, sep string) string {
	return strings.Join(arr, sep)
}

// StructToJSON converts a struct to JSON format
func (t *Tools) StructToJSON(v interface{}) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// JSONToStruct converts JSON to a struct
func (t *Tools) JSONToStruct(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}