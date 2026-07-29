package errx

import (
	"errors"
	"testing"

	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/i18n"
)

func TestXError(t *testing.T) {
	cause := errors.New("database password leaked")
	err := Wrap(errc.ErrDataBase, cause, i18n.Params{"resource": "room"})
	if !errors.Is(err, cause) {
		t.Fatal("wrapped cause is not discoverable")
	}
	if got := Code(err); got != errc.ErrDataBase {
		t.Fatalf("Code() = %d", got)
	}
	if got := Params(err)["resource"]; got != "room" {
		t.Fatalf("Params()[resource] = %v", got)
	}
}

func TestUnknownErrorUsesServerCode(t *testing.T) {
	if got := Code(errors.New("internal detail")); got != errc.ErrServer {
		t.Fatalf("Code() = %d, want %d", got, errc.ErrServer)
	}
	if got := Code(New(999999, nil)); got != errc.ErrServer {
		t.Fatalf("unknown business code = %d, want %d", got, errc.ErrServer)
	}
}
