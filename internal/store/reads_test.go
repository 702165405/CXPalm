package store

import "testing"

func TestSharedReadsAreNotDeviceScopedAndFailuresAreVisible(t *testing.T) {
	a, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err = a.SetReadToken("chat", "reply"); err != nil {
		t.Fatal(err)
	}
	all, err := a.ReadTokens()
	if err != nil || all["chat"] != "reply" {
		t.Fatal(all, err)
	}
	if token, _ := b.ReadToken("chat"); token != "" {
		t.Fatal("other computer was marked read")
	}
	a.Close()
	if _, err = a.ReadTokens(); err == nil {
		t.Fatal("closed store silently succeeded")
	}
	if err = a.SetReadToken("chat", "new"); err == nil {
		t.Fatal("write failure hidden")
	}
}
