package tools

import (
	"encoding/json"
	"errors"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"reflect"
	"strconv"
	"time"
)

var validate = validator.New()

// setDefaultsAndConvert sets default values for fields in the destination struct that are zero values.
// It also converts string representations of default values to their appropriate types.
func (inst *Tools) setDefaultsAndConvert(dest any) error {
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
				if err := setDefaultValueForPointerField(field, elemType, defaultValue); err != nil {
					return err
				}
			default:
				if err := setDefaultValueForValueField(field, field.Kind(), defaultValue); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// setDefaultValueForPointerField sets the default value for pointer fields.
func setDefaultValueForPointerField(field reflect.Value, elemType reflect.Kind, defaultValue string) error {
	switch elemType {
	case reflect.String:
		field.Set(reflect.ValueOf(&defaultValue))
	case reflect.Int:
		if intValue, err := strconv.Atoi(defaultValue); err == nil {
			field.Set(reflect.ValueOf(&intValue))
		} else {
			logrus.WithFields(logrus.Fields{"field": field.Type().Name(), "default": defaultValue}).Error("Failed to convert default value to int")
			return errors.New("invalid default value for int")
		}
	case reflect.Float64:
		if floatValue, err := strconv.ParseFloat(defaultValue, 64); err == nil {
			field.Set(reflect.ValueOf(&floatValue))
		} else {
			logrus.WithFields(logrus.Fields{"field": field.Type().Name(), "default": defaultValue}).Error("Failed to convert default value to float64")
			return errors.New("invalid default value for float64")
		}
	case reflect.Bool:
		if boolValue, err := strconv.ParseBool(defaultValue); err == nil {
			field.Set(reflect.ValueOf(&boolValue))
		} else {
			logrus.WithFields(logrus.Fields{"field": field.Type().Name(), "default": defaultValue}).Error("Failed to convert default value to bool")
			return errors.New("invalid default value for bool")
		}
	case reflect.Struct:
		// Check if the field is a pointer to time.Time
		if field.Type().Elem() == reflect.TypeOf(time.Time{}) {
			if timeValue, err := time.Parse(time.RFC3339, defaultValue); err == nil {
				field.Set(reflect.ValueOf(&timeValue))
			} else {
				logrus.WithFields(logrus.Fields{"field": field.Type().Name(), "default": defaultValue}).Error("Failed to convert default value to time.Time")
				return errors.New("invalid default value for time.Time")
			}
		}
	default:
		logrus.WithFields(logrus.Fields{"field": field.Type().Name(), "default": defaultValue}).Error("Unhandled default case for pointer type")
		return errors.New("unhandled default case for pointer type")
	}
	return nil
}

// setDefaultValueForValueField sets the default value for non-pointer fields.
func setDefaultValueForValueField(field reflect.Value, kind reflect.Kind, defaultValue string) error {
	switch kind {
	case reflect.Int:
		if intValue, err := strconv.Atoi(defaultValue); err == nil {
			field.SetInt(int64(intValue))
		} else {
			logrus.WithFields(logrus.Fields{"field": field.Type().Name(), "default": defaultValue}).Error("Failed to convert default value to int")
			return errors.New("invalid default value for int")
		}
	case reflect.Float64:
		if floatValue, err := strconv.ParseFloat(defaultValue, 64); err == nil {
			field.SetFloat(floatValue)
		} else {
			logrus.WithFields(logrus.Fields{"field": field.Type().Name(), "default": defaultValue}).Error("Failed to convert default value to float64")
			return errors.New("invalid default value for float64")
		}
	case reflect.Bool:
		if boolValue, err := strconv.ParseBool(defaultValue); err == nil {
			field.SetBool(boolValue)
		} else {
			logrus.WithFields(logrus.Fields{"field": field.Type().Name(), "default": defaultValue}).Error("Failed to convert default value to bool")
			return errors.New("invalid default value for bool")
		}
	case reflect.Struct:
		if field.Type() == reflect.TypeOf(time.Time{}) {
			if timeValue, err := time.Parse(time.RFC3339, defaultValue); err == nil {
				field.Set(reflect.ValueOf(timeValue))
			} else {
				logrus.WithFields(logrus.Fields{"field": field.Type().Name(), "default": defaultValue}).Error("Failed to convert default value to time.Time")
				return errors.New("invalid default value for time.Time")
			}
		}
	default:
		logrus.WithFields(logrus.Fields{"field": field.Type().Name()}).Error("Unhandled default case for value type")
		return errors.New("unhandled default case for value type")
	}
	return nil
}

// convertFields converts string values in the provided data map to the appropriate types
// in the destination struct. It supports converting basic types and pointer types.
func (inst *Tools) convertFields(data map[string]any, dest any) error {
	v := reflect.ValueOf(dest).Elem()
	t := v.Type() // Get the type of the destination struct

	for key, value := range data {
		field := v.FieldByNameFunc(func(fieldName string) bool {
			// Use the type of the struct to get the field type
			fieldType, found := t.FieldByName(fieldName)
			if !found {
				return false
			}
			tag, _ := fieldType.Tag.Lookup("json")
			return tag == key
		})

		if !field.IsValid() || !field.CanSet() {
			logrus.WithFields(logrus.Fields{"key": key}).Error("Invalid field mapping")
			continue
		}

		if err := setValue(field, value); err != nil {
			return err
		}
	}

	return nil
}

// setValue sets the value for a given field based on the provided value.
func setValue(field reflect.Value, value any) error {
	switch field.Kind() {
	case reflect.String:
		if strVal, ok := value.(string); ok {
			field.SetString(strVal)
		}
	case reflect.Int:
		if strVal, ok := value.(float64); ok {
			field.SetInt(int64(strVal))
		}
	case reflect.Float64:
		if strVal, ok := value.(float64); ok {
			field.SetFloat(strVal)
		}
	case reflect.Uint:
		if strVal, ok := value.(float64); ok {
			field.SetUint(uint64(strVal))
		}
	case reflect.Bool:
		if strVal, ok := value.(bool); ok {
			field.SetBool(strVal)
		}
	case reflect.Struct:
		if field.Type() == reflect.TypeOf(time.Time{}) {
			if strVal, ok := value.(string); ok {
				if timeValue, err := time.Parse(time.RFC3339, strVal); err == nil {
					field.Set(reflect.ValueOf(timeValue))
				} else {
					return errors.New("invalid value for time.Time")
				}
			}
		}
	case reflect.Ptr:
		return setValueForPointerField(field, value)
	default:
		logrus.WithFields(logrus.Fields{"key": value}).Error("Unhandled default case")
		return errors.New("unhandled default case")
	}
	return nil
}

// setValueForPointerField sets the value for pointer fields.
func setValueForPointerField(field reflect.Value, value any) error {
	switch field.Type().Elem().Kind() {
	case reflect.String:
		if strVal, ok := value.(string); ok {
			field.Set(reflect.ValueOf(&strVal))
		}
	case reflect.Int:
		if strVal, ok := value.(float64); ok {
			intValue := int(strVal)
			field.Set(reflect.ValueOf(&intValue))
		}
	case reflect.Float64:
		if strVal, ok := value.(float64); ok {
			field.Set(reflect.ValueOf(&strVal))
		}
	case reflect.Uint:
		if strVal, ok := value.(float64); ok {
			uintValue := uint(strVal)
			field.Set(reflect.ValueOf(&uintValue))
		}
	case reflect.Bool:
		if strVal, ok := value.(bool); ok {
			field.Set(reflect.ValueOf(&strVal))
		}
	case reflect.Struct:
		if field.Type().Elem() == reflect.TypeOf(time.Time{}) {
			if strVal, ok := value.(string); ok {
				if timeValue, err := time.Parse(time.RFC3339, strVal); err == nil {
					field.Set(reflect.ValueOf(&timeValue))
				} else {
					return errors.New("invalid value for time.Time")
				}
			}
		}
	default:
		logrus.WithFields(logrus.Fields{"key": value}).Error("Unhandled default case for pointer type")
		return errors.New("unhandled default case for pointer type")
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

	if err = inst.setDefaultsAndConvert(dest); err != nil {
		return err
	}

	err = validate.Struct(dest)
	if err != nil {
		return err
	}

	return nil
}
