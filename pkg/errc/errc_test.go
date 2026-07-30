package errc

import (
	"net/http"
	"testing"

	"github.com/leafney/filedock/pkg/i18n"
)

func TestDefinitionsAreValid(t *testing.T) {
	definitions := Definitions()
	if len(definitions) == 0 {
		t.Fatal("Definitions() is empty")
	}
	seenCodes := make(map[int]struct{}, len(definitions))
	seenKeys := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if _, exists := seenCodes[definition.Code]; exists {
			t.Fatalf("duplicate code %d", definition.Code)
		}
		seenCodes[definition.Code] = struct{}{}
		if definition.MessageKey == "" {
			t.Fatalf("code %d has empty message key", definition.Code)
		}
		if _, exists := seenKeys[definition.MessageKey]; exists {
			t.Fatalf("duplicate message key %q", definition.MessageKey)
		}
		seenKeys[definition.MessageKey] = struct{}{}
		if definition.HTTPStatus < 100 || definition.HTTPStatus > 599 {
			t.Fatalf("code %d has invalid HTTP status %d", definition.Code, definition.HTTPStatus)
		}
		if definition.Code != Success && (definition.Code < 10000 || definition.Code > 99999) {
			t.Fatalf("code %d is not a five-digit error code", definition.Code)
		}
		if definition.Code != Success && definition.Code/100 != definition.HTTPStatus {
			t.Fatalf("code %d does not map to HTTP status %d", definition.Code, definition.HTTPStatus)
		}
	}
}

func TestDefinitionsReferenceExistingTranslations(t *testing.T) {
	catalog, err := i18n.NewCatalog()
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	for _, definition := range Definitions() {
		for _, locale := range i18n.SupportedLocales() {
			if !catalog.Has(locale, definition.MessageKey) {
				t.Fatalf("code %d references missing %s translation %q", definition.Code, locale, definition.MessageKey)
			}
		}
	}
}

func TestHTTPStatus(t *testing.T) {
	tests := map[int]int{
		Success:             http.StatusOK,
		ErrParams:           http.StatusBadRequest,
		ErrUnAuthorized:     http.StatusUnauthorized,
		ErrForbidden:        http.StatusForbidden,
		ErrNotFound:         http.StatusNotFound,
		ErrMethodNotAllowed: http.StatusMethodNotAllowed,
		ErrConflict:         http.StatusConflict,
		ErrServer:           http.StatusInternalServerError,
		999999:              http.StatusInternalServerError,
	}
	for code, want := range tests {
		if got := HTTPStatus(code); got != want {
			t.Fatalf("HTTPStatus(%d) = %d, want %d", code, got, want)
		}
	}
}
