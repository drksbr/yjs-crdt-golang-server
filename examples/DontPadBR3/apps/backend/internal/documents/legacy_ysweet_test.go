package documents

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"os"
	"os/exec"
	"sort"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/objectstore"
	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
	"github.com/drksbr/yjs-crdt-golang-server/internal/yupdate"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func TestLegacyYSweetMigratorReadUpdateFromBincodeStore(t *testing.T) {
	storeDir := t.TempDir()
	objects, err := objectstore.NewLocal(storeDir)
	if err != nil {
		t.Fatalf("NewLocal() unexpected error: %v", err)
	}
	paths := common.StoragePaths{Root: storeDir}

	documentID := "legacy-fixture"
	const oid uint32 = 1
	baseUpdate := mustDecodeLegacyHex(t, "010165000401017402686900")
	tailUpdate := mustDecodeLegacyHex(t, "0101ca0100846501012100")
	legacyKey := paths.LegacyYSweetKey(documentID)
	if _, err := objects.Put(context.Background(), legacyKey, bytes.NewReader(encodeLegacyBincodeMap(t, map[string][]byte{
		string(legacyYSweetOIDKey([]byte(legacyYSweetDocName))):         legacyTestOIDBytes(oid),
		string(legacyYSweetDocStateKey(oid)):                            baseUpdate,
		string(append(legacyYSweetUpdateKeyPrefix(oid), 0, 0, 0, 1, 0)): tailUpdate,
	})), objectstore.PutOptions{}); err != nil {
		t.Fatalf("Put() unexpected error: %v", err)
	}

	migrator := NewLegacyYSweetMigrator(objects, paths)
	got, err := migrator.readUpdate(context.Background(), legacyKey)
	if err != nil {
		t.Fatalf("readUpdate() unexpected error: %v", err)
	}
	stateVector, err := yjsbridge.StateVectorFromUpdate(got)
	if err != nil {
		t.Fatalf("StateVectorFromUpdate() unexpected error: %v", err)
	}
	if stateVector[101] != 2 || stateVector[202] != 1 {
		t.Fatalf("stateVector = %#v, want clients 101:2 and 202:1", stateVector)
	}
}

func TestLegacyYSweetBincodeDecoderRejectsTrailingBytes(t *testing.T) {
	payload := append(encodeLegacyBincodeMap(t, nil), 0xff)
	if _, err := decodeLegacyYSweetBincodeMap(payload); err == nil {
		t.Fatal("decodeLegacyYSweetBincodeMap() err = nil, want trailing bytes error")
	}
}

func TestDiscoverLegacySubdocumentsFromLegacyRoots(t *testing.T) {
	update := legacySubdocumentsFixture(t)
	specs, err := discoverLegacySubdocuments(update)
	if err != nil {
		t.Fatalf("discoverLegacySubdocuments() unexpected error: %v", err)
	}

	got := make(map[string]legacySubdocumentSpec)
	for _, spec := range specs {
		got[spec.slug] = spec
	}
	for slug, wantType := range map[string]string{
		"maria20miranda": "texto",
		"markdown20note": "markdown",
		"tasks":          "checklist",
		"board":          "kanban",
		"empty-note":     "texto",
	} {
		spec, ok := got[slug]
		if !ok {
			t.Fatalf("missing migrated subdocument slug %q in %#v", slug, specs)
		}
		if spec.typ != wantType {
			t.Fatalf("subdocument %q type = %q, want %q", slug, spec.typ, wantType)
		}
	}
	if got["board"].roots["kanban-cols:board"] != "kanban-cols" ||
		got["board"].roots["kanban-items:board"] != "kanban-items" {
		t.Fatalf("kanban roots = %#v, want columns and items roots", got["board"].roots)
	}
}

func TestExtractLegacySubdocumentUpdateRenamesRoot(t *testing.T) {
	update := legacySubdocumentsFixture(t)
	extracted, err := extractLegacySubdocumentUpdate(update, map[string]string{
		"text:maria20miranda": "text",
	})
	if err != nil {
		t.Fatalf("extractLegacySubdocumentUpdate() unexpected error: %v", err)
	}

	decoded, err := yupdate.DecodeUpdate(extracted)
	if err != nil {
		t.Fatalf("DecodeUpdate() unexpected error: %v", err)
	}
	var sawCurrentRoot bool
	for _, current := range decoded.Structs {
		item, ok := current.(*ytypes.Item)
		if !ok || item.Parent.Kind() != ytypes.ParentRoot {
			continue
		}
		if item.Parent.Root() == "text:maria20miranda" {
			t.Fatalf("legacy root was not rewritten: %q", item.Parent.Root())
		}
		if item.Parent.Root() == "text" {
			sawCurrentRoot = true
		}
	}
	if !sawCurrentRoot {
		t.Fatal("extracted update did not contain current text root")
	}
}

func TestExtractLegacySubdocumentUpdateAppliesInYjs(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not available")
	}
	if _, err := os.Stat("../../../frontend/node_modules/yjs"); err != nil {
		t.Skip("frontend yjs dependency is not available")
	}

	update := legacySubdocumentsFixture(t)
	extracted, err := extractLegacySubdocumentUpdate(update, map[string]string{
		"text:maria20miranda": "text",
	})
	if err != nil {
		t.Fatalf("extractLegacySubdocumentUpdate() unexpected error: %v", err)
	}

	script := `
const Y = require('../../../frontend/node_modules/yjs')
const update = Uint8Array.from(Buffer.from(process.argv[1], 'hex'))
const doc = new Y.Doc()
Y.applyUpdate(doc, update)
const got = doc.getText('text').toString()
if (got !== 'sub note') {
  throw new Error('text root = ' + JSON.stringify(got))
}
`
	cmd := exec.Command("node", "-e", script, hex.EncodeToString(extracted))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node applyUpdate failed: %v\n%s", err, output)
	}
}

func encodeLegacyBincodeMap(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return bytes.Compare([]byte(keys[i]), []byte(keys[j])) < 0
	})

	var out []byte
	out = binary.LittleEndian.AppendUint64(out, uint64(len(keys)))
	for _, key := range keys {
		out = appendLegacyBincodeVec(out, []byte(key))
		out = appendLegacyBincodeVec(out, entries[key])
	}
	return out
}

func appendLegacyBincodeVec(dst []byte, value []byte) []byte {
	dst = binary.LittleEndian.AppendUint64(dst, uint64(len(value)))
	return append(dst, value...)
}

func legacyTestOIDBytes(oid uint32) []byte {
	var out [4]byte
	binary.BigEndian.PutUint32(out[:], oid)
	return out[:]
}

func mustDecodeLegacyHex(t *testing.T, value string) []byte {
	t.Helper()
	data, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("hex.DecodeString(%q) unexpected error: %v", value, err)
	}
	return data
}

func legacySubdocumentsFixture(t *testing.T) []byte {
	t.Helper()
	return mustDecodeLegacyHex(t, "0107f79e98f509000401047465787404726f6f74040113746578743a6d6172696132306d6972616e646108737562206e6f74650401116d643a6d61726b646f776e32306e6f74650723207469746c6528010f636865636b6c6973743a7461736b73066974656d2d3101760802696477066974656d2d31047465787477045461736b07636865636b65647908706172656e7449647e056f726465727da80f09636f6c6c617073656479096372656174656441747d01097570646174656441747d012801116b616e62616e2d636f6c733a626f61726405636f6c2d310176050269647705636f6c2d31057469746c657704546f646f0b6465736372697074696f6e770005636f6c6f72770723363437343862056f726465727da80f2801126b616e62616e2d6974656d733a626f617264066974656d2d3101760802696477066974656d2d31057469746c657704436172640b6465736372697074696f6e770008636f6c756d6e49647705636f6c2d31056f726465727da80f05636f6c6f727700106c696e6b6564446f63756d656e7449647700156c696e6b6564537562646f63756d656e744e616d6577002801046d6574611e737562646f633a656d7074792d6e6f74653a6c6173744d6f646966696564017d0100")
}
