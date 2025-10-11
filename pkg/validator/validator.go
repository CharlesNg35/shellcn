package validator

import (
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	once     sync.Once
	validate *validator.Validate
)

// ValidationError represents a single field validation failure.
type ValidationError struct {
	Field string `json:"field"`
	Tag   string `json:"tag"`
	Param string `json:"param"`
}

// ValidationErrors collects multiple validation failures.
type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	if len(v) == 0 {
		return "validation failed"
	}

	parts := make([]string, len(v))
	for i, err := range v {
		if err.Param != "" {
			parts[i] = err.Field + " failed on " + err.Tag + "=" + err.Param
		} else {
			parts[i] = err.Field + " failed on " + err.Tag
		}
	}
	return strings.Join(parts, "; ")
}

// ValidateStruct validates a struct using registered rules.
func ValidateStruct(s interface{}) error {
	err := getValidator().Struct(s)
	if err == nil {
		return nil
	}

	if ve, ok := err.(validator.ValidationErrors); ok {
		failures := make(ValidationErrors, 0, len(ve))
		for _, fe := range ve {
			failures = append(failures, ValidationError{
				Field: fe.Field(),
				Tag:   fe.Tag(),
				Param: fe.Param(),
			})
		}
		return failures
	}

	return err
}

// RegisterValidation exposes underlying validator custom rules.
func RegisterValidation(tag string, fn validator.Func) error {
	return getValidator().RegisterValidation(tag, fn)
}

func getValidator() *validator.Validate {
	once.Do(func() {
		validate = validator.New()
		validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := fld.Tag.Get("json")
			if name == "" {
				return fld.Name
			}

			comma := strings.Index(name, ",")
			if comma != -1 {
				name = name[:comma]
			}

			if name == "-" || name == "" {
				return fld.Name
			}
			return name
		})
	})
	return validate
}
