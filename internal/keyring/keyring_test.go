package keyring

import "testing"

func TestRoundTrip(t *testing.T) {
	MockInit()
	if err := Set("prod", "pk_secret"); err != nil {
		t.Fatal(err)
	}
	if key, err := Get("prod"); err != nil || key != "pk_secret" {
		t.Errorf("key = %q, err = %v", key, err)
	}
	if err := Delete("prod"); err != nil {
		t.Fatal(err)
	}
	if _, err := Get("prod"); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteMissingIsNotAnError(t *testing.T) {
	MockInit()
	if err := Delete("never-stored"); err != nil {
		t.Errorf("err = %v", err)
	}
}
