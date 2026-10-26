package desktop

import (
	"errors"
	"testing"
)

func TestArchiveWriterRejectionRequiresExactOfficialResponse(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	api := &metadataError{Code: -32600, Message: "thread " + id + " already has an active writer"}
	var rejected *Rejected
	if !errors.As(archiveFailure(api, id), &rejected) || rejected.Code != "active_writer" {
		t.Fatal("explicit precondition rejection remained uncertain")
	}
	for _, err := range []error{
		errors.New(api.Error()),
		&metadataError{Code: -32000, Message: api.Message},
		&metadataError{Code: -32600, Message: "thread another already has an active writer"},
		errors.New("connection lost after dispatch"),
	} {
		if errors.As(archiveFailure(err, id), &rejected) {
			t.Fatal("uncertain or unrelated error released protection", err)
		}
	}
}
