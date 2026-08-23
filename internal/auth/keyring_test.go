package auth

import (
	"testing"

	"github.com/zalando/go-keyring"
)

func TestKeyRoundTrip(t *testing.T) {
	keyring.MockInit()

	if err := SetKey("test-account", "pk_secret"); err != nil {
		t.Fatal(err)
	}
	got, err := GetKey("test-account")
	if err != nil {
		t.Fatal(err)
	}
	if got != "pk_secret" {
		t.Errorf("GetKey = %q", got)
	}
	if err := DeleteKey("test-account"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetKey("test-account"); err == nil {
		t.Error("expected error after delete")
	}
}
