package generic

import (
	"reflect"
	"strings"
)

// IsFieldValid checks if the field exists in the struct, including embedded fields.
func IsFieldValid(model any, field string) bool {
	t := reflect.TypeOf(model).Elem()
	return checkFields(t, field)
}

// checkFields recursively checks fields, including embedded structs.
func checkFields(t reflect.Type, field string) bool {
	for i := 0; i < t.NumField(); i++ {
		structField := t.Field(i)
		// Check if the field matches directly
		dbName := structField.Name
		if dbName == field || ToSnakeCase(dbName) == strings.ToLower(field) {
			return true
		}
		// If the field is embedded, recursively check its fields
		if structField.Anonymous {
			if checkFields(structField.Type, field) {
				return true
			}
		}
	}
	return false
}

// ToSnakeCase converts "PascalCase" or "camelCase" to "snake_case".
func ToSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}
