package tools

import (
	"encoding/json"
	"github.com/go-playground/validator/v10"
	"reflect"
	"strconv"
	"time"
)

var validate = validator.New()

// setDefaultsAndConvert sets default values for fields in the destination struct that are zero values.
// It also converts string representations of default values to their appropriate types.
func (inst *Tools) setDefaultsAndConvert(dest any) {
	v := reflect.ValueOf(dest).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		defaultValue, ok := fieldType.Tag.Lookup("default")
		if ok && field.IsZero() {
			switch field.Kind() {
			case reflect.Ptr:
				elemType := fieldType.Type.Elem().Kind()
				switch elemType {
				case reflect.String:
					field.Set(reflect.ValueOf(&defaultValue))
				case reflect.Int:
					if intValue, err := strconv.Atoi(defaultValue); err == nil {
						field.Set(reflect.ValueOf(&intValue))
					}
				case reflect.Float64:
					if floatValue, err := strconv.ParseFloat(defaultValue, 64); err == nil {
						field.Set(reflect.ValueOf(&floatValue))
					}
				case reflect.Uint:
					if uintValue, err := strconv.ParseUint(defaultValue, 10, 64); err == nil {
						field.Set(reflect.ValueOf(&uintValue))
					}
				case reflect.Bool:
					if boolValue, err := strconv.ParseBool(defaultValue); err == nil {
						field.Set(reflect.ValueOf(&boolValue))
					}
				case reflect.Struct:
					if fieldType.Type.Elem() == reflect.TypeOf(time.Time{}) {
						if timeValue, err := time.Parse(time.RFC3339, defaultValue); err == nil {
							field.Set(reflect.ValueOf(&timeValue))
						}
					}
				default:
					panic("unhandled default case")
				}
			case reflect.Int:
				if intValue, err := strconv.Atoi(defaultValue); err == nil {
					field.SetInt(int64(intValue))
				}
			case reflect.Float64:
				if floatValue, err := strconv.ParseFloat(defaultValue, 64); err == nil {
					field.SetFloat(floatValue)
				}
			case reflect.Uint:
				if uintValue, err := strconv.ParseUint(defaultValue, 10, 64); err == nil {
					field.SetUint(uintValue)
				}
			case reflect.Bool:
				if boolValue, err := strconv.ParseBool(defaultValue); err == nil {
					field.SetBool(boolValue)
				}
			case reflect.Struct:
				if fieldType.Type == reflect.TypeOf(time.Time{}) {
					if timeValue, err := time.Parse(time.RFC3339, defaultValue); err == nil {
						field.Set(reflect.ValueOf(timeValue))
					}
				}
			default:
				panic("unhandled default case")
			}
		}
	}
}

// convertFields converts string values in the provided data map to the appropriate types
// in the destination struct. It supports converting basic types and pointer types.
func (inst *Tools) convertFields(data map[string]any, dest any) error {
	v := reflect.ValueOf(dest).Elem()

	for key, value := range data {
		field := v.FieldByNameFunc(func(fieldName string) bool {
			fieldType, _ := v.Type().FieldByName(fieldName)
			tag, _ := fieldType.Tag.Lookup("json")
			return tag == key
		})

		if !field.IsValid() || !field.CanSet() {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			if strVal, ok := value.(string); ok {
				field.SetString(strVal)
			}
		case reflect.Int:
			if strVal, ok := value.(string); ok {
				if intValue, err := strconv.Atoi(strVal); err == nil {
					field.SetInt(int64(intValue))
				}
			}
		case reflect.Float64:
			if strVal, ok := value.(string); ok {
				if floatValue, err := strconv.ParseFloat(strVal, 64); err == nil {
					field.SetFloat(floatValue)
				}
			}
		case reflect.Uint:
			if strVal, ok := value.(string); ok {
				if uintValue, err := strconv.ParseUint(strVal, 10, 64); err == nil {
					field.SetUint(uintValue)
				}
			}
		case reflect.Bool:
			if strVal, ok := value.(string); ok {
				if boolValue, err := strconv.ParseBool(strVal); err == nil {
					field.SetBool(boolValue)
				}
			}
		case reflect.Struct:
			if field.Type() == reflect.TypeOf(time.Time{}) {
				if strVal, ok := value.(string); ok {
					if timeValue, err := time.Parse(time.RFC3339, strVal); err == nil {
						field.Set(reflect.ValueOf(timeValue))
					}
				}
			}
		case reflect.Ptr:
			switch field.Type().Elem().Kind() {
			case reflect.String:
				if strVal, ok := value.(string); ok {
					field.Set(reflect.ValueOf(&strVal))
				}
			case reflect.Int:
				if strVal, ok := value.(string); ok {
					if intValue, err := strconv.Atoi(strVal); err == nil {
						field.Set(reflect.ValueOf(&intValue))
					}
				}
			case reflect.Float64:
				if strVal, ok := value.(string); ok {
					if floatValue, err := strconv.ParseFloat(strVal, 64); err == nil {
						field.Set(reflect.ValueOf(&floatValue))
					}
				}
			case reflect.Uint:
				if strVal, ok := value.(string); ok {
					if uintValue, err := strconv.ParseUint(strVal, 10, 64); err == nil {
						field.Set(reflect.ValueOf(&uintValue))
					}
				}
			case reflect.Bool:
				if strVal, ok := value.(string); ok {
					if boolValue, err := strconv.ParseBool(strVal); err == nil {
						field.Set(reflect.ValueOf(&boolValue))
					}
				}
			case reflect.Struct:
				if field.Type().Elem() == reflect.TypeOf(time.Time{}) {
					if strVal, ok := value.(string); ok {
						if timeValue, err := time.Parse(time.RFC3339, strVal); err == nil {
							field.Set(reflect.ValueOf(&timeValue))
						}
					}
				}
			default:
				panic("unhandled default case")
			}
		default:
			panic("unhandled default case")
		}
	}

	return nil
}

// Satisfies handles default setting, type conversion, and validation for the provided data
// against the destination struct. It takes in a map or any type, converts it, sets defaults,
// and validates the result.
func (inst *Tools) Satisfies(data any, dest any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = json.Unmarshal(jsonData, dest)
	if err != nil {
		return err
	}

	if dataMap, ok := data.(map[string]any); ok {
		err = inst.convertFields(dataMap, dest)
		if err != nil {
			return err
		}
	}

	inst.setDefaultsAndConvert(dest)

	err = validate.Struct(dest)
	if err != nil {
		return err
	}

	return nil
}
