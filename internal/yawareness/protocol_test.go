package yawareness

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	ybinary "github.com/drksbr/yjs-crdt-golang-server/internal/binary"
	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
	"github.com/drksbr/yjs-crdt-golang-server/internal/yprotocol"
)

func TestEncodeDecodeUpdateRoundTrip(t *testing.T) {
	t.Parallel()

	update := &Update{
		Clients: []ClientState{
			{
				ClientID: 1,
				Clock:    7,
				State:    json.RawMessage(`{"name":"ramon","cursor":[1,2]}`),
			},
			{
				ClientID: 9,
				Clock:    3,
				State:    nil,
			},
		},
	}

	encoded, err := EncodeUpdate(update)
	if err != nil {
		t.Fatalf("EncodeUpdate() unexpected error: %v", err)
	}

	decoded, err := DecodeUpdate(encoded)
	if err != nil {
		t.Fatalf("DecodeUpdate() unexpected error: %v", err)
	}

	if len(decoded.Clients) != 2 {
		t.Fatalf("len(Clients) = %d, want 2", len(decoded.Clients))
	}

	if decoded.Clients[0].ClientID != 1 || decoded.Clients[0].Clock != 7 {
		t.Fatalf("Clients[0] = %+v, want client=1 clock=7", decoded.Clients[0])
	}
	if !bytes.Equal(decoded.Clients[0].State, []byte(`{"name":"ramon","cursor":[1,2]}`)) {
		t.Fatalf("Clients[0].State = %s", decoded.Clients[0].State)
	}

	if decoded.Clients[1].ClientID != 9 || decoded.Clients[1].Clock != 3 || !decoded.Clients[1].IsNull() {
		t.Fatalf("Clients[1] = %+v, want client=9 clock=3 state=null", decoded.Clients[1])
	}
}

func TestEncodeProtocolDecodeProtocolUpdateRoundTrip(t *testing.T) {
	t.Parallel()

	update := &Update{
		Clients: []ClientState{
			{ClientID: 5, Clock: 1, State: json.RawMessage(`{"online":true}`)},
		},
	}

	encoded, err := EncodeProtocolUpdate(update)
	if err != nil {
		t.Fatalf("EncodeProtocolUpdate() unexpected error: %v", err)
	}

	decoded, err := DecodeProtocolUpdate(encoded)
	if err != nil {
		t.Fatalf("DecodeProtocolUpdate() unexpected error: %v", err)
	}

	if len(decoded.Clients) != 1 || decoded.Clients[0].ClientID != 5 {
		t.Fatalf("decoded = %#v, want one client=5", decoded)
	}
}

func TestReadProtocolUpdateStreaming(t *testing.T) {
	t.Parallel()

	first, err := EncodeProtocolUpdate(&Update{
		Clients: []ClientState{{ClientID: 1, Clock: 1, State: json.RawMessage(`{"a":1}`)}},
	})
	if err != nil {
		t.Fatalf("EncodeProtocolUpdate() first unexpected error: %v", err)
	}
	second, err := EncodeProtocolUpdate(&Update{
		Clients: []ClientState{{ClientID: 2, Clock: 4, State: nil}},
	})
	if err != nil {
		t.Fatalf("EncodeProtocolUpdate() second unexpected error: %v", err)
	}

	reader := ybinary.NewReader(append(first, second...))

	gotFirst, err := ReadProtocolUpdate(reader)
	if err != nil {
		t.Fatalf("ReadProtocolUpdate() first unexpected error: %v", err)
	}
	if len(gotFirst.Clients) != 1 || gotFirst.Clients[0].ClientID != 1 {
		t.Fatalf("gotFirst = %#v", gotFirst)
	}

	gotSecond, err := ReadProtocolUpdate(reader)
	if err != nil {
		t.Fatalf("ReadProtocolUpdate() second unexpected error: %v", err)
	}
	if len(gotSecond.Clients) != 1 || gotSecond.Clients[0].ClientID != 2 || !gotSecond.Clients[0].IsNull() {
		t.Fatalf("gotSecond = %#v", gotSecond)
	}
}

func TestEncodeUpdateRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := EncodeUpdate(&Update{
		Clients: []ClientState{{ClientID: 1, Clock: 1, State: json.RawMessage(`{`)}},
	})
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("EncodeUpdate() error = %v, want ErrInvalidJSON", err)
	}
}

func TestDecodeUpdateRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	src := varint.Append(nil, 1)
	src = varint.Append(src, 7)
	src = varint.Append(src, 3)
	src = appendVarString(src, []byte(`not-json`))

	_, err := DecodeUpdate(src)
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("DecodeUpdate() error = %v, want ErrInvalidJSON", err)
	}
}

func TestDecodeProtocolUpdateRejectsWrongProtocol(t *testing.T) {
	t.Parallel()

	payload, err := EncodeUpdate(&Update{})
	if err != nil {
		t.Fatalf("EncodeUpdate() unexpected error: %v", err)
	}

	src := append(yprotocol.AppendProtocolType(nil, yprotocol.ProtocolTypeSync), payload...)

	_, err = DecodeProtocolUpdate(src)
	if !errors.Is(err, yprotocol.ErrUnexpectedProtocolType) {
		t.Fatalf("DecodeProtocolUpdate() error = %v, want yprotocol.ErrUnexpectedProtocolType", err)
	}
}

func TestDecodeUpdateRejectsTruncatedPayload(t *testing.T) {
	t.Parallel()

	src := varint.Append(nil, 1)
	src = varint.Append(src, 1)
	src = varint.Append(src, 2)
	src = varint.Append(src, 4)
	src = append(src, []byte(`nul`)...)

	_, err := DecodeUpdate(src)
	if !errors.Is(err, ybinary.ErrUnexpectedEOF) {
		t.Fatalf("DecodeUpdate() error = %v, want binary.ErrUnexpectedEOF", err)
	}
}

func TestDecodeUpdateRejectsTrailingBytes(t *testing.T) {
	t.Parallel()

	src, err := EncodeUpdate(&Update{})
	if err != nil {
		t.Fatalf("EncodeUpdate() unexpected error: %v", err)
	}
	src = append(src, 0xff)

	_, err = DecodeUpdate(src)
	if !errors.Is(err, ErrTrailingBytes) {
		t.Fatalf("DecodeUpdate() error = %v, want ErrTrailingBytes", err)
	}
}
