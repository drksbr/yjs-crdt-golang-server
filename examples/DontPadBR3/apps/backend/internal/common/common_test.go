package common

import (
	"bytes"
	"errors"
	"testing"
)

func TestReadLimitedPayload(t *testing.T) {
	t.Parallel()

	payload, err := ReadLimitedPayload(bytes.NewReader([]byte("abcdef")), 6)
	if err != nil {
		t.Fatalf("ReadLimitedPayload() unexpected error: %v", err)
	}
	if string(payload) != "abcdef" {
		t.Fatalf("payload = %q, want abcdef", payload)
	}

	_, err = ReadLimitedPayload(bytes.NewReader([]byte("abcdefg")), 6)
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("ReadLimitedPayload() err = %v, want ErrPayloadTooLarge", err)
	}
}
