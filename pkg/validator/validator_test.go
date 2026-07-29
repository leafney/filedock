package validator

import (
	"testing"
)

type TestStruct struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0,lte=130"`
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		s       interface{}
		wantErr bool
	}{
		{
			name: "valid struct",
			s: TestStruct{
				Name:  "John",
				Email: "john@example.com",
				Age:   30,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			s: TestStruct{
				Email: "john@example.com",
				Age:   30,
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			s: TestStruct{
				Name:  "John",
				Email: "invalid-email",
				Age:   30,
			},
			wantErr: true,
		},
		{
			name: "invalid age",
			s: TestStruct{
				Name:  "John",
				Email: "john@example.com",
				Age:   150,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.s); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
