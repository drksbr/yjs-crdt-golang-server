package yupdate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

func TestValidateUpdatesFormatWithReasonContextPreservesMalformedIndex(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valid := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "a"),
			},
		},
	)

	_, err := ValidateUpdatesFormatWithReasonContext(ctx, valid, []byte{0x80})
	if err == nil {
		t.Fatal("ValidateUpdatesFormatWithReasonContext() error = nil, want malformed update error")
	}
	if !strings.Contains(err.Error(), "update[1]") {
		t.Fatalf("ValidateUpdatesFormatWithReasonContext() error = %v, want update index 1", err)
	}
	if !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("ValidateUpdatesFormatWithReasonContext() error = %v, want %v", err, varint.ErrUnexpectedEOF)
	}
}
