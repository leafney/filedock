package validator

import (
	"strings"
	"testing"

	"github.com/leafney/filedock/pkg/i18n"
)

type testStruct struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0,lte=130"`
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		wantErr bool
	}{
		{name: "valid", value: testStruct{Name: "John", Email: "john@example.com", Age: 30}},
		{name: "missing name", value: testStruct{Email: "john@example.com", Age: 30}, wantErr: true},
		{name: "invalid email", value: testStruct{Name: "John", Email: "invalid-email", Age: 30}, wantErr: true},
		{name: "invalid age", value: testStruct{Name: "John", Email: "john@example.com", Age: 150}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.value); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFirstMessageUsesRequestedLanguageAndJSONFieldName(t *testing.T) {
	err := Validate(testStruct{Name: "John", Email: "invalid", Age: 30})
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	zhMessage := FirstMessage(err, i18n.LocaleZhCN)
	if !strings.Contains(zhMessage, "email") || !strings.Contains(zhMessage, "邮箱") {
		t.Fatalf("Chinese message = %q", zhMessage)
	}
	enMessage := FirstMessage(err, i18n.LocaleEn)
	if !strings.Contains(enMessage, "email") || !strings.Contains(enMessage, "valid email") {
		t.Fatalf("English message = %q", enMessage)
	}
}

func TestFirstMessageReturnsOnlyFirstError(t *testing.T) {
	err := Validate(testStruct{Email: "invalid", Age: 150})
	message := FirstMessage(err, i18n.LocaleEn)
	if !strings.Contains(message, "name") {
		t.Fatalf("first validation message = %q", message)
	}
}
