package yjsbridge

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestWireConvertersHandleYjsV1FormatJSON(t *testing.T) {
	t.Parallel()

	// Emitted by upstream Yjs for:
	// ytext.applyDelta([{ insert: "hello", attributes: { bold: true } }])
	v1 := mustDecodeHexWire(t, "01039fecb8ca09000601047465787404626f6c640474727565849fecb8ca09000568656c6c6f869fecb8ca090504626f6c64046e756c6c00")

	v2, err := ConvertUpdateToV2YjsWire(v1)
	if err != nil {
		t.Fatalf("ConvertUpdateToV2YjsWire() unexpected error: %v", err)
	}

	roundTrip, err := ConvertUpdateToV1YjsWire(v2)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1YjsWire() unexpected error: %v", err)
	}

	if !bytes.Equal(roundTrip, v1) {
		t.Fatalf("V1 wire round-trip mismatch:\n got: %x\nwant: %x", roundTrip, v1)
	}
}

func TestWireConvertersRepairCanonicalV2FormatJSON(t *testing.T) {
	t.Parallel()

	// Emitted by upstream Yjs for:
	// ytext.applyDelta([{ insert: "hello", attributes: { bold: true } }])
	v1 := mustDecodeHexWire(t, "01039fecb8ca09000601047465787404626f6c640474727565849fecb8ca09000568656c6c6f869fecb8ca090504626f6c64046e756c6c00")

	canonicalV2, err := ConvertUpdateToV2(v1)
	if err != nil {
		t.Fatalf("ConvertUpdateToV2() unexpected error: %v", err)
	}
	wireV2, err := ConvertUpdateToV2YjsWire(canonicalV2)
	if err != nil {
		t.Fatalf("ConvertUpdateToV2YjsWire(canonical V2) unexpected error: %v", err)
	}
	roundTrip, err := ConvertUpdateToV1YjsWire(wireV2)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1YjsWire(wire V2) unexpected error: %v", err)
	}
	if !bytes.Equal(roundTrip, v1) {
		t.Fatalf("canonical V2 wire repair mismatch:\n got: %x\nwant: %x", roundTrip, v1)
	}
}

func TestWireConvertersHandleNumericAndEmbedJSON(t *testing.T) {
	t.Parallel()

	tests := []string{
		// header: 1 (legacy failure signature tag=1 in ContentFormat value path)
		"010388fadad80100060104746578740668656164657201318488fadad80100065469746c650a8688fadad8010606686561646572046e756c6c00",
		// ytext.insertEmbed(0, { img: "x" })
		"0101d3a08c5700050101740b7b22696d67223a2278227d00",
	}

	for _, encoded := range tests {
		v1 := mustDecodeHexWire(t, encoded)
		v2, err := ConvertUpdateToV2YjsWire(v1)
		if err != nil {
			t.Fatalf("ConvertUpdateToV2YjsWire(%s) unexpected error: %v", encoded, err)
		}
		roundTrip, err := ConvertUpdateToV1YjsWire(v2)
		if err != nil {
			t.Fatalf("ConvertUpdateToV1YjsWire(%s) unexpected error: %v", encoded, err)
		}
		if !bytes.Equal(roundTrip, v1) {
			t.Fatalf("wire round-trip mismatch for %s:\n got: %x\nwant: %x", encoded, roundTrip, v1)
		}
	}
}

func mustDecodeHexWire(t *testing.T, value string) []byte {
	t.Helper()
	data, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("hex.DecodeString(%q) unexpected error: %v", value, err)
	}
	return data
}
