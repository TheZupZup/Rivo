package postgres

import "testing"

func TestIsUUIDAcceptsCanonicalIdentifiers(t *testing.T) {
	for _, value := range []string{
		"6f9619ff-8b86-d011-b42d-00c04fc964ff",
		"6F9619FF-8B86-D011-B42D-00C04FC964FF",
		"00000000-0000-0000-0000-000000000000",
	} {
		if !isUUID(value) {
			t.Fatalf("expected %q to be accepted", value)
		}
	}
}

// These reach the store from a request body. Passing them straight to Postgres
// would raise an invalid-input error, which callers would surface as a 500 for what
// is really a client mistake.
func TestIsUUIDRejectsAnythingElse(t *testing.T) {
	for _, value := range []string{
		"",
		"not-a-uuid",
		"6f9619ff8b86d011b42d00c04fc964ff",
		"6f9619ff-8b86-d011-b42d-00c04fc964f",
		"6f9619ff-8b86-d011-b42d-00c04fc964fff",
		"6f9619ff-8b86-d011-b42d-00c04fc964fg",
		"6f9619ff_8b86-d011-b42d-00c04fc964ff",
		"'; DROP TABLE reports; --          ",
	} {
		if isUUID(value) {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}
