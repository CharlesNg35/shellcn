package validator

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

type testPayload struct {
	Username string `json:"username" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Age      int    `json:"age" validate:"gte=18"`
}

func TestValidateStructSuccess(t *testing.T) {
	payload := testPayload{
		Username: "alice",
		Email:    "alice@example.com",
		Age:      20,
	}

	if err := ValidateStruct(payload); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateStructFailures(t *testing.T) {
	payload := testPayload{
		Username: "",
		Email:    "invalid",
		Age:      10,
	}

	err := ValidateStruct(payload)
	if err == nil {
		t.Fatal("expected validation error")
	}

	vErrs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	if len(vErrs) != 3 {
		t.Fatalf("expected 3 validation errors, got %d", len(vErrs))
	}

	foundEmail := false
	for _, v := range vErrs {
		if v.Field == "email" {
			foundEmail = true
		}
	}

	if !foundEmail {
		t.Fatal("expected email field to be present in validation errors")
	}
}

func TestRegisterValidation(t *testing.T) {
	err := RegisterValidation("shellcn", func(fl validator.FieldLevel) bool {
		return fl.Field().String() == "shellcn"
	})
	if err != nil {
		t.Fatalf("register validation: %v", err)
	}

	type custom struct {
		Value string `validate:"shellcn"`
	}

	if err := ValidateStruct(custom{Value: "shellcn"}); err != nil {
		t.Fatalf("expected validation to pass, got %v", err)
	}
	if err := ValidateStruct(custom{Value: "other"}); err == nil {
		t.Fatal("expected validation to fail for non-matching value")
	}
}
