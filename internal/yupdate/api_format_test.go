package yupdate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yjs-go-bridge/internal/varint"
)

func TestFormatFromUpdatesRejectsEmptyPayloads(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "a"),
			},
		},
	)
	_, err := FormatFromUpdates(left, nil)
	if !errors.Is(err, ErrUnknownUpdateFormat) {
		t.Fatalf("FormatFromUpdates() error = %v, want %v", err, ErrUnknownUpdateFormat)
	}
}

func TestFormatFromUpdatesReturnsCommonFormat(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				gc(1),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 4,
			clock:  1,
			structs: []structEncoding{
				itemString(rootParent("doc"), "x"),
			},
		},
	)

	got, err := FormatFromUpdates(left, right)
	if err != nil {
		t.Fatalf("FormatFromUpdates() unexpected error: %v", err)
	}
	if got != UpdateFormatV1 {
		t.Fatalf("FormatFromUpdates() = %s, want %s", got, UpdateFormatV1)
	}
}

func TestMergeUpdatesPreservesIndexedFormatError(t *testing.T) {
	t.Parallel()

	valid := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "a"),
			},
		},
	)

	_, err := MergeUpdates(valid, []byte{0x80})
	if err == nil {
		t.Fatalf("MergeUpdates() error = nil, want malformed update error")
	}
	if !strings.Contains(err.Error(), "update[1]") {
		t.Fatalf("MergeUpdates() error = %v, want update index 1", err)
	}
	if !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("MergeUpdates() error = %v, want %v", err, varint.ErrUnexpectedEOF)
	}
}

func TestFormatFromUpdatesContextRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	update := buildUpdate(
		clientBlock{
			client: 2,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "a"),
			},
		},
	)

	_, err := FormatFromUpdatesContext(ctx, update)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FormatFromUpdatesContext() error = %v, want context.Canceled", err)
	}
}

func TestMergeUpdatesContextRespectsCanceledContextDuringFormatValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	update := buildUpdate(
		clientBlock{
			client: 3,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "a"),
			},
		},
	)

	_, err := MergeUpdatesContext(ctx, update)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("MergeUpdatesContext() error = %v, want context.Canceled", err)
	}
}
