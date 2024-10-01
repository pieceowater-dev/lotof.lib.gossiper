package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Tools struct{}

// Split splits a string by the given separator rune
func (inst *Tools) Split(s string, sep rune) []string {
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

// SplitOnce splits a string by the given separator rune once
func (inst *Tools) SplitOnce(s string, sep rune) []string {
	parts := make([]string, 2)
	i := 0
	var part []rune
	for _, c := range s {
		if c == sep && i == 0 {
			parts[i] = string(part)
			part = nil
			i++
		} else {
			part = append(part, c)
		}
	}
	if i < 2 {
		parts[i] = string(part)
	}
	return parts
}

// ToString converts any value to a string
func (inst *Tools) ToString(val any) string {
	return fmt.Sprintf("%v", val)
}

// ToInt converts a string to an integer
func (inst *Tools) ToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// ToBool converts a string to a boolean
func (inst *Tools) ToBool(s string) (bool, error) {
	return strconv.ParseBool(s)
}

// Join concatenates elements of a slice into a single string with the given separator
func (inst *Tools) Join(arr []string, sep string) string {
	return strings.Join(arr, sep)
}

// StructToJSON converts a struct to JSON format
func (inst *Tools) StructToJSON(v any) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// JSONToStruct converts JSON to a struct
func (inst *Tools) JSONToStruct(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}
