package yupdate

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

type v2Fixture struct {
	name string
	v1   string
	v2   string
}

type v2MultiUpdateFixture struct {
	name        string
	description string
	updates     []v2Fixture
	mergedV1    string
	mergedV2    string
}

// yjsV2Fixtures validate decoding of updates emitted by Yjs and conversion to
// this package's canonical V1 encoding. ContentEmbed and ContentFormat keep the
// legacy project V1 representation instead of Yjs' JSON-string V1 representation.
var yjsV2Fixtures = []v2Fixture{
	{
		name: "empty",
		v1:   "0000",
		v2:   "00000000000001000000000000",
	},
	{
		name: "text_insert",
		v1:   "010165000401017402686900",
		v2:   "000002a50100000104060374686901020101000001010000",
	},
	{
		name: "map_any",
		v1:   "010165002801016d016b01760101617d0100",
		v2:   "000002a5010000012805026d6b41000101000101010100760101617d0100",
	},
	{
		name: "text_delete_full",
		v1:   "0103650004010174016881650002846502026c6f0165010102",
		v2:   "000003e5010102000400050400810084080474686c6f41000201010001020103000165010101",
	},
	{
		name: "delete_only",
		v1:   "000165010102",
		v2:   "0000000000000100000000000165010101",
	},
	{
		name: "multi_client",
		v1:   "0201ca010008010161027d017701780165000401017402686900",
		v2:   "0000048a03a50100000308000408046174686941000201010001020201007d01770178010000",
	},
	{
		name: "binary_array",
		v1:   "01016500030101610301020300",
		v2:   "000002a5010000010303016101010100000101000301020300",
	},
	{
		name: "text_unicode",
		v1:   "010165000401017407686920f09f8c8d00",
		v2:   "000002a501000001040b0874686920f09f8c8d01050101000001010000",
	},
	{
		name: "array_numbers_booleans_null",
		v1:   "0101650008010161067d017d4278797e77017300",
		v2:   "000002a501000001080301610101010001060101007d017d4278797e77017300",
	},
	{
		name: "array_nested_any",
		v1:   "0101650008010161027602026f6b78016e7d0775027701787d0300",
		v2:   "000002a501000001080301610101010001020101007602026f6b78016e7d0775027701787d0300",
	},
	{
		name: "map_object_array_any",
		v1:   "010265002801016d036f626a01760201617d0101627502787e2801016d0361727201750277017a7d0200",
		v2:   "000002a501000001280d086d6f626a6d61727201030103010100024100010200760201617d0101627502787e750277017a7d0200",
	},
	{
		name: "text_embed",
		v1:   "0101650005010174760103696d6777017800",
		v2:   "000002a501000001050301740101010000010100760103696d6777017800",
	},
	{
		name: "text_format",
		v1:   "0103650004010174017846650004626f6c647886650004626f6c647e00",
		v2:   "0002000203e50101010001000504004600860f0a7478626f6c64626f6c644100440001010000010300787e00",
	},
	{
		name: "nested_map_type",
		v1:   "01026500070101610128006500016b0177017600",
		v2:   "000003e50100010000030700280502616b4100030100000101010101020077017600",
	},
	{
		name: "xml_fragment_text_attrs",
		v1:   "01046500070103786d6c0301702800650005636c6173730177046c6561640700650006040065020568656c6c6f00",
		v2:   "00010003e5010203010004000707002800070004130e786d6c70636c61737368656c6c6f0301450003010000020306010101040077046c65616400",
	},
	{
		name: "subdoc",
		v1:   "0101650009010161057375622d31760000",
		v2:   "000002a501000001090906617375622d31010501010000010100760000",
	},
}

// yjsV2MultiUpdateFixtures are generated with upstream Yjs using incremental
// updates plus Y.mergeUpdates/Y.mergeUpdatesV2. They protect the public V2
// contract that accepts V2 input but emits canonical V1 output.
var yjsV2MultiUpdateFixtures = []v2MultiUpdateFixture{
	{
		name:        "text_two_clients_append",
		description: "client 101 inserts text, then client 202 appends to the same Y.Text",
		updates: []v2Fixture{
			{v1: "010165000401017402686900", v2: "000002a50100000104060374686901020101000001010000"},
			{v1: "0101ca0100846501012100", v2: "0000048a03a50101020001840301210100000001010000"},
		},
		mergedV1: "0201ca010084650101210165000401017402686900",
		mergedV2: "0000058a03e501000102000384000408042174686941000201010000020100010000",
	},
	{
		name:        "text_delete_after_insert",
		description: "client 101 inserts text, then deletes a middle range",
		updates: []v2Fixture{
			{v1: "01016500040101740568656c6c6f00", v2: "000002a5010000010409067468656c6c6f01050101000001010000"},
			{v1: "000165010102", v2: "0000000000000100000000000165010101"},
		},
		mergedV1: "01016500040101740568656c6c6f0165010102",
		mergedV2: "000002a5010000010409067468656c6c6f0105010100000101000165010101",
	},
	{
		name:        "map_set_two_clients",
		description: "client 101 sets one Y.Map key, then client 202 sets another key",
		updates: []v2Fixture{
			{v1: "010165002801016d016b017702763100", v2: "000002a5010000012805026d6b410001010001010101007702763100"},
			{v1: "0101ca01002801016d056f74686572017d2a00", v2: "0000028a030000012809066d6f74686572010501010001010101007d2a00"},
		},
		mergedV1: "0201ca01002801016d056f74686572017d2a0165002801016d016b017702763100",
		mergedV2: "0000048a03a501000001280d086d6f746865726d6b010541000101000241000201007d2a01007702763100",
	},
	{
		name:        "nested_map_then_child_set",
		description: "client 101 inserts a nested Y.Map, then sets a key inside that child map",
		updates: []v2Fixture{
			{v1: "01016500270104726f6f74056368696c640100", v2: "000002a501000001270c09726f6f746368696c640405010101010001010000"},
			{v1: "0101650128006500046c65616601770576616c756500", v2: "000003e50100010000012806046c656166040100000101010101770576616c756500"},
		},
		mergedV1: "01026500270104726f6f74056368696c640128006500046c65616601770576616c756500",
		mergedV2: "000003e5010001000003270028110d726f6f746368696c646c6561660405040301000001010101010200770576616c756500",
	},
	{
		name:        "map_overwrite_same_key",
		description: "client 101 sets one Y.Map key, then client 202 overwrites the same key",
		updates: []v2Fixture{
			{v1: "010165002801016d016b0177036f6c6400", v2: "000002a5010000012805026d6b4100010100010101010077036f6c6400"},
			{v1: "0101ca01008865000177036e65770165010001", v2: "0000048a03a50101000001a801000000010101010077036e65770165010000"},
		},
		mergedV1: "0201ca01008865000177036e65770165002801016d016b0177036f6c640165010001",
		mergedV2: "0000058a03e501000100000388002805026d6b410001010002410002010077036e6577010077036f6c640165010000",
	},
	{
		name:        "xml_element_then_text",
		description: "client 101 inserts a Y.XmlElement, then appends XmlText inside it",
		updates: []v2Fixture{
			{v1: "01016500070103786d6c03017000", v2: "00010002a501000001070704786d6c700301010101030001010000"},
			{v1: "0102650107006500060400650102686900", v2: "000003e5010102000200030700040402686902010001060001020100"},
		},
		mergedV1: "01036500070103786d6c03017007006500060400650102686900",
		mergedV2: "00010003e5010102000200030701040a06786d6c706869030102030100000203060001030000",
	},
	{
		name:        "array_delete_middle",
		description: "client 101 inserts three Y.Array values, then deletes the middle value",
		updates: []v2Fixture{
			{v1: "01016500080101610377016177016277016300", v2: "000002a5010000010803016101010100010301010077016177016277016300"},
			{v1: "000165010101", v2: "0000000000000100000000000165010100"},
		},
		mergedV1: "0101650008010161037701617701627701630165010101",
		mergedV2: "000002a501000001080301610101010001030101007701617701627701630165010100",
	},
	{
		name:        "text_format_then_delete_overlap",
		description: "client 101 inserts text, formats a middle range, then deletes overlapping text",
		updates: []v2Fixture{
			{v1: "01016500040101740661626364656600", v2: "000002a501000001040a077461626364656601060101000001010000"},
			{v1: "01026506c66500650104626f6c6478c66504650504626f6c647e00", v2: "0002000203e5010302000802020801c60b08626f6c64626f6c644400000000010206787e00"},
			{v1: "000165010202", v2: "0000000000000100000000000165010201"},
		},
		mergedV1: "010365000401017406616263646566c66500650104626f6c6478c66504650504626f6c647e0165010202",
		mergedV2: "0002000203e50103020008020208030400c6140f74616263646566626f6c64626f6c640106440001010000010300787e0165010201",
	},
	{
		name:        "subdoc_then_map_update",
		description: "client 101 inserts a subdoc into a Y.Map, then updates another key in the parent map",
		updates: []v2Fixture{
			{v1: "010165002901016d056368696c640a7375622d666f6c6c6f77760000", v2: "000002a5010000012914106d6368696c647375622d666f6c6c6f7701050a01010000010100760000"},
			{v1: "010165012801016d06737461747573017705726561647900", v2: "000002a501000001280a076d737461747573010601010001010101017705726561647900"},
		},
		mergedV1: "010265002901016d056368696c640a7375622d666f6c6c6f7776002801016d06737461747573017705726561647900",
		mergedV2: "000002a5010000032900281d176d6368696c647375622d666f6c6c6f776d73746174757301050a0106010100010101020076007705726561647900",
	},
}

func TestDecodeV2AndConvertUpdateToV1UseYjsFixtures(t *testing.T) {
	t.Parallel()

	for _, fixture := range yjsV2Fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()

			v1 := mustDecodeHex(t, fixture.v1)
			v2 := mustDecodeHex(t, fixture.v2)

			format, err := DetectUpdateFormatWithReason(v2)
			if err != nil {
				t.Fatalf("DetectUpdateFormatWithReason() unexpected error: %v", err)
			}
			if format != UpdateFormatV2 {
				t.Fatalf("DetectUpdateFormatWithReason() = %s, want %s", format, UpdateFormatV2)
			}

			decoded, err := DecodeV2(v2)
			if err != nil {
				t.Fatalf("DecodeV2() unexpected error: %v", err)
			}

			encoded, err := EncodeV1(decoded)
			if err != nil {
				t.Fatalf("EncodeV1(DecodeV2()) unexpected error: %v", err)
			}
			if !bytes.Equal(encoded, v1) {
				t.Fatalf("EncodeV1(DecodeV2()) = %x, want %x", encoded, v1)
			}

			converted, err := ConvertUpdateToV1(v2)
			if err != nil {
				t.Fatalf("ConvertUpdateToV1() unexpected error: %v", err)
			}
			if !bytes.Equal(converted, v1) {
				t.Fatalf("ConvertUpdateToV1() = %x, want %x", converted, v1)
			}

			merged, err := ConvertUpdatesToV1(nil, v2, []byte{})
			if err != nil {
				t.Fatalf("ConvertUpdatesToV1() unexpected error: %v", err)
			}
			if !bytes.Equal(merged, v1) {
				t.Fatalf("ConvertUpdatesToV1() = %x, want %x", merged, v1)
			}

			assertV2DerivedAPIsMatchConvertedV1(t, v2, v1)
			assertV2MutatingAPIsMatchConvertedV1(t, v2, v1)
		})
	}
}

func TestV2MultiUpdateFixturesMatchCanonicalV1(t *testing.T) {
	t.Parallel()

	for _, fixture := range yjsV2MultiUpdateFixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()

			if fixture.description == "" {
				t.Fatal("fixture description is empty")
			}

			v1Updates := make([][]byte, 0, len(fixture.updates))
			v2Updates := make([][]byte, 0, len(fixture.updates))
			for i, update := range fixture.updates {
				v1 := mustDecodeHex(t, update.v1)
				v2 := mustDecodeHex(t, update.v2)
				converted, err := ConvertUpdateToV1(v2)
				if err != nil {
					t.Fatalf("ConvertUpdateToV1(update[%d]) unexpected error: %v", i, err)
				}
				if !bytes.Equal(converted, v1) {
					t.Fatalf("ConvertUpdateToV1(update[%d]) = %x, want %x", i, converted, v1)
				}
				v1Updates = append(v1Updates, v1)
				v2Updates = append(v2Updates, v2)
			}

			mergedV1 := mustDecodeHex(t, fixture.mergedV1)
			mergedV2 := mustDecodeHex(t, fixture.mergedV2)
			convertedMergedV2, err := ConvertUpdateToV1(mergedV2)
			if err != nil {
				t.Fatalf("ConvertUpdateToV1(mergedV2) unexpected error: %v", err)
			}
			if !bytes.Equal(convertedMergedV2, mergedV1) {
				t.Fatalf("ConvertUpdateToV1(mergedV2) = %x, want %x", convertedMergedV2, mergedV1)
			}

			gotMerged, err := MergeUpdates(v2Updates...)
			if err != nil {
				t.Fatalf("MergeUpdates(v2...) unexpected error: %v", err)
			}
			wantMerged, err := MergeUpdatesV1(v1Updates...)
			if err != nil {
				t.Fatalf("MergeUpdatesV1(v1...) unexpected error: %v", err)
			}
			if !bytes.Equal(wantMerged, mergedV1) {
				t.Fatalf("MergeUpdatesV1(v1...) = %x, want upstream merged V1 %x", wantMerged, mergedV1)
			}
			if !bytes.Equal(gotMerged, mergedV1) {
				t.Fatalf("MergeUpdates(v2...) = %x, want %x", gotMerged, mergedV1)
			}

			convertedUpdates, err := ConvertUpdatesToV1(v2Updates...)
			if err != nil {
				t.Fatalf("ConvertUpdatesToV1(v2...) unexpected error: %v", err)
			}
			if !bytes.Equal(convertedUpdates, mergedV1) {
				t.Fatalf("ConvertUpdatesToV1(v2...) = %x, want %x", convertedUpdates, mergedV1)
			}

			stateVector, err := EncodeStateVectorFromUpdate(v1Updates[0])
			if err != nil {
				t.Fatalf("EncodeStateVectorFromUpdate(v1[0]) unexpected error: %v", err)
			}
			gotDiff, err := DiffUpdate(mergedV2, stateVector)
			if err != nil {
				t.Fatalf("DiffUpdate(mergedV2) unexpected error: %v", err)
			}
			wantDiff, err := DiffUpdateV1(mergedV1, stateVector)
			if err != nil {
				t.Fatalf("DiffUpdateV1(mergedV1) unexpected error: %v", err)
			}
			if !bytes.Equal(gotDiff, wantDiff) {
				t.Fatalf("DiffUpdate(mergedV2) = %x, want %x", gotDiff, wantDiff)
			}

			contentIDs, err := CreateContentIDsFromUpdate(mergedV1)
			if err != nil {
				t.Fatalf("CreateContentIDsFromUpdate(mergedV1) unexpected error: %v", err)
			}
			gotIntersection, err := IntersectUpdateWithContentIDs(mergedV2, contentIDs)
			if err != nil {
				t.Fatalf("IntersectUpdateWithContentIDs(mergedV2) unexpected error: %v", err)
			}
			wantIntersection, err := IntersectUpdateWithContentIDsV1(mergedV1, contentIDs)
			if err != nil {
				t.Fatalf("IntersectUpdateWithContentIDsV1(mergedV1) unexpected error: %v", err)
			}
			if !bytes.Equal(gotIntersection, wantIntersection) {
				t.Fatalf("IntersectUpdateWithContentIDs(mergedV2) = %x, want %x", gotIntersection, wantIntersection)
			}

			assertV2DerivedAPIsMatchConvertedV1(t, mergedV2, mergedV1)
		})
	}
}

func TestMergeUpdatesV2MultiplePayloadsReturnCanonicalV1(t *testing.T) {
	t.Parallel()

	first := mustDecodeHex(t, "000002a50100000104060374686901020101000001010000")
	second := mustDecodeHex(t, "0000028a0300000104060374686901020101000001010000")

	firstV1, err := ConvertUpdateToV1(first)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1(first) unexpected error: %v", err)
	}
	secondV1, err := ConvertUpdateToV1(second)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1(second) unexpected error: %v", err)
	}

	got, err := MergeUpdates(first, second)
	if err != nil {
		t.Fatalf("MergeUpdates(v2, v2) unexpected error: %v", err)
	}
	want, err := MergeUpdatesV1(firstV1, secondV1)
	if err != nil {
		t.Fatalf("MergeUpdatesV1(converted...) unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("MergeUpdates(v2, v2) = %x, want %x", got, want)
	}
}

func TestDecodeV2RejectsMalformedInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "minimal_header_without_rest",
			data: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name: "truncated_valid_fixture",
			data: mustDecodeHex(t, "000002a501000001040603746869010201010000010100"),
		},
		{
			name: "string_table_length_overflow",
			data: mustDecodeHex(t, "000002a5010000010404017401020101000001010000"),
		},
		{
			name: "delete_set_clock_length_overflow",
			data: mustDecodeHex(t, "000000000000010000000000010101ffffffff0f00"),
		},
		{
			name: "unsupported_feature_flag",
			data: []byte{1},
		},
		{
			name: "unused_client_encoder_value",
			data: appendByteToV2EncoderSection(t, mustDecodeHex(t, "000002a50100000104060374686901020101000001010000"), 1, 0x00),
		},
		{
			name: "unused_info_rle_count",
			data: appendByteToV2EncoderSection(t, mustDecodeHex(t, "000002a50100000104060374686901020101000001010000"), 4, 0x01),
		},
		{
			name: "unused_key_clock_without_consumed_keys",
			data: appendByteToV2EncoderSection(t, mustDecodeHex(t, "000002a50100000104060374686901020101000001010000"), 0, 0x00),
		},
		{
			name: "invalid_parent_info_value",
			data: replaceV2EncoderSection(t, mustDecodeHex(t, "000002a50100000104060374686901020101000001010000"), 6, []byte{0x02}),
		},
		{
			name: "content_any_length_too_large",
			data: replaceV2EncoderSection(t, mustDecodeHex(t, "000002a501000001080301610101010001060101007d017d4278797e77017300"), 8, appendVarUintV1(nil, ^uint32(0))),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := DecodeV2(tt.data); err == nil {
				t.Fatal("DecodeV2() error = nil, want malformed input error")
			}
		})
	}
}

func TestDecodeUpdateDispatchesValidV2(t *testing.T) {
	t.Parallel()

	v2 := mustDecodeHex(t, "000002a50100000104060374686901020101000001010000")
	decoded, err := DecodeUpdate(v2)
	if err != nil {
		t.Fatalf("DecodeUpdate() unexpected error: %v", err)
	}
	if len(decoded.Structs) != 1 {
		t.Fatalf("DecodeUpdate() structs len = %d, want 1", len(decoded.Structs))
	}

	item, ok := decoded.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("DecodeUpdate() struct type = %T, want *ytypes.Item", decoded.Structs[0])
	}
	content, ok := item.Content.(ParsedContent)
	if !ok {
		t.Fatalf("DecodeUpdate() content type = %T, want ParsedContent", item.Content)
	}
	if item.ID() != (ytypes.ID{Client: 101, Clock: 0}) {
		t.Fatalf("item ID = %+v, want client=101 clock=0", item.ID())
	}
	if item.Parent.Kind() != ytypes.ParentRoot || item.Parent.Root() != "t" {
		t.Fatalf("item parent = kind %d root %q, want root t", item.Parent.Kind(), item.Parent.Root())
	}
	if content.ContentRef() != itemContentString || content.Text != "hi" || content.Length() != 2 {
		t.Fatalf("content = ref=%d text=%q len=%d, want string hi len 2", content.ContentRef(), content.Text, content.Length())
	}
}

func TestDecodeV2ParsesMapParentSubAndDeleteSet(t *testing.T) {
	t.Parallel()

	mapUpdate := mustDecodeHex(t, "000002a5010000012805026d6b41000101000101010100760101617d0100")
	decodedMap, err := DecodeV2(mapUpdate)
	if err != nil {
		t.Fatalf("DecodeV2(map) unexpected error: %v", err)
	}
	mapItem, ok := decodedMap.Structs[0].(*ytypes.Item)
	if !ok {
		t.Fatalf("DecodeV2(map) struct type = %T, want *ytypes.Item", decodedMap.Structs[0])
	}
	mapContent, ok := mapItem.Content.(ParsedContent)
	if !ok {
		t.Fatalf("DecodeV2(map) content type = %T, want ParsedContent", mapItem.Content)
	}
	if mapItem.ParentSub != "k" {
		t.Fatalf("map parentSub = %q, want k", mapItem.ParentSub)
	}
	if mapContent.ContentRef() != itemContentAny || len(mapContent.Any) != 1 {
		t.Fatalf("map content = ref=%d anyLen=%d, want ContentAny len 1", mapContent.ContentRef(), len(mapContent.Any))
	}

	deleteUpdate := mustDecodeHex(t, "0000000000000100000000000165010101")
	decodedDelete, err := DecodeV2(deleteUpdate)
	if err != nil {
		t.Fatalf("DecodeV2(delete) unexpected error: %v", err)
	}
	if len(decodedDelete.Structs) != 0 {
		t.Fatalf("DecodeV2(delete) structs len = %d, want 0", len(decodedDelete.Structs))
	}
	ranges := decodedDelete.DeleteSet.Ranges(101)
	if len(ranges) != 1 || ranges[0] != (ytypes.DeleteRange{Clock: 1, Length: 2}) {
		t.Fatalf("DecodeV2(delete) ranges = %+v, want clock=1 length=2", ranges)
	}
}

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()

	data, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("hex.DecodeString(%q) unexpected error: %v", value, err)
	}
	return data
}

func appendByteToV2EncoderSection(t *testing.T, data []byte, section int, value byte) []byte {
	t.Helper()

	if section < 0 || section >= 9 {
		t.Fatalf("invalid V2 encoder section index %d", section)
	}

	offset := skipV2VarUint(t, data, 0)
	for i := 0; i < 9; i++ {
		lengthStart := offset
		length, next := readV2VarUintAt(t, data, offset)
		payloadStart := next
		payloadEnd := payloadStart + int(length)
		if payloadEnd < payloadStart || payloadEnd > len(data) {
			t.Fatalf("section %d length %d exceeds data length", i, length)
		}
		if i == section {
			out := make([]byte, 0, len(data)+1)
			out = append(out, data[:lengthStart]...)
			out = appendVarUintV1(out, length+1)
			out = append(out, data[payloadStart:payloadEnd]...)
			out = append(out, value)
			out = append(out, data[payloadEnd:]...)
			return out
		}
		offset = payloadEnd
	}

	t.Fatalf("section %d not found", section)
	return nil
}

func replaceV2EncoderSection(t *testing.T, data []byte, section int, payload []byte) []byte {
	t.Helper()

	if section < 0 || section >= 9 {
		t.Fatalf("invalid V2 encoder section index %d", section)
	}

	offset := skipV2VarUint(t, data, 0)
	for i := 0; i < 9; i++ {
		lengthStart := offset
		length, next := readV2VarUintAt(t, data, offset)
		payloadStart := next
		payloadEnd := payloadStart + int(length)
		if payloadEnd < payloadStart || payloadEnd > len(data) {
			t.Fatalf("section %d length %d exceeds data length", i, length)
		}
		if i == section {
			out := make([]byte, 0, len(data)-int(length)+len(payload)+5)
			out = append(out, data[:lengthStart]...)
			out = appendVarUintV1(out, uint32(len(payload)))
			out = append(out, payload...)
			out = append(out, data[payloadEnd:]...)
			return out
		}
		offset = payloadEnd
	}

	t.Fatalf("section %d not found", section)
	return nil
}

func skipV2VarUint(t *testing.T, data []byte, offset int) int {
	t.Helper()

	_, next := readV2VarUintAt(t, data, offset)
	return next
}

func readV2VarUintAt(t *testing.T, data []byte, offset int) (uint32, int) {
	t.Helper()

	var value uint64
	shift := uint(0)
	for i := offset; i < len(data); i++ {
		b := data[i]
		value |= uint64(b&0x7f) << shift
		if value > uint64(^uint32(0)) {
			t.Fatalf("varuint at %d overflows uint32", offset)
		}
		if b&0x80 == 0 {
			return uint32(value), i + 1
		}
		shift += 7
		if shift > 35 {
			t.Fatalf("varuint at %d is too long", offset)
		}
	}
	t.Fatalf("varuint at %d reached EOF", offset)
	return 0, 0
}

func assertV2DerivedAPIsMatchConvertedV1(t *testing.T, v2 []byte, v1 []byte) {
	t.Helper()

	gotStateVector, err := StateVectorFromUpdate(v2)
	if err != nil {
		t.Fatalf("StateVectorFromUpdate(v2) unexpected error: %v", err)
	}
	wantStateVector, err := StateVectorFromUpdate(v1)
	if err != nil {
		t.Fatalf("StateVectorFromUpdate(v1) unexpected error: %v", err)
	}
	if !equalStateVectors(gotStateVector, wantStateVector) {
		t.Fatalf("StateVectorFromUpdate(v2) = %#v, want %#v", gotStateVector, wantStateVector)
	}

	gotEncodedStateVector, err := EncodeStateVectorFromUpdate(v2)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdate(v2) unexpected error: %v", err)
	}
	wantEncodedStateVector, err := EncodeStateVectorFromUpdate(v1)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdate(v1) unexpected error: %v", err)
	}
	if !bytes.Equal(gotEncodedStateVector, wantEncodedStateVector) {
		t.Fatalf("EncodeStateVectorFromUpdate(v2) = %x, want %x", gotEncodedStateVector, wantEncodedStateVector)
	}

	gotContentIDs, err := CreateContentIDsFromUpdate(v2)
	if err != nil {
		t.Fatalf("CreateContentIDsFromUpdate(v2) unexpected error: %v", err)
	}
	wantContentIDs, err := CreateContentIDsFromUpdate(v1)
	if err != nil {
		t.Fatalf("CreateContentIDsFromUpdate(v1) unexpected error: %v", err)
	}
	if !IsSubsetContentIDs(gotContentIDs, wantContentIDs) || !IsSubsetContentIDs(wantContentIDs, gotContentIDs) {
		t.Fatalf("CreateContentIDsFromUpdate(v2) = %#v, want %#v", gotContentIDs, wantContentIDs)
	}

	gotSnapshot, err := SnapshotFromUpdate(v2)
	if err != nil {
		t.Fatalf("SnapshotFromUpdate(v2) unexpected error: %v", err)
	}
	wantSnapshot, err := SnapshotFromUpdate(v1)
	if err != nil {
		t.Fatalf("SnapshotFromUpdate(v1) unexpected error: %v", err)
	}
	if !equalStateVectors(gotSnapshot.StateVector, wantSnapshot.StateVector) {
		t.Fatalf("SnapshotFromUpdate(v2).StateVector = %#v, want %#v", gotSnapshot.StateVector, wantSnapshot.StateVector)
	}
	if !sameContentIDs(deleteSetContentIDs(gotSnapshot), deleteSetContentIDs(wantSnapshot)) {
		t.Fatalf("SnapshotFromUpdate(v2).DeleteSet = %#v, want %#v", gotSnapshot.DeleteSet, wantSnapshot.DeleteSet)
	}
}

func deleteSetContentIDs(snapshot *Snapshot) *ContentIDs {
	out := NewContentIDs()
	if snapshot == nil || snapshot.DeleteSet == nil {
		return out
	}
	for _, client := range snapshot.DeleteSet.Clients() {
		for _, r := range snapshot.DeleteSet.Ranges(client) {
			_ = out.Deletes.Add(client, r.Clock, r.Length)
		}
	}
	return out
}

func sameContentIDs(a, b *ContentIDs) bool {
	return IsSubsetContentIDs(a, b) && IsSubsetContentIDs(b, a)
}

func assertV2MutatingAPIsMatchConvertedV1(t *testing.T, v2 []byte, v1 []byte) {
	t.Helper()

	gotMerged, err := MergeUpdates(v2)
	if err != nil {
		t.Fatalf("MergeUpdates(v2) unexpected error: %v", err)
	}
	wantMerged, err := MergeUpdatesV1(v1)
	if err != nil {
		t.Fatalf("MergeUpdatesV1(v1) unexpected error: %v", err)
	}
	if !bytes.Equal(gotMerged, wantMerged) {
		t.Fatalf("MergeUpdates(v2) = %x, want %x", gotMerged, wantMerged)
	}

	emptyStateVector := encodeStateVectorEntry()
	gotDiff, err := DiffUpdate(v2, emptyStateVector)
	if err != nil {
		t.Fatalf("DiffUpdate(v2) unexpected error: %v", err)
	}
	wantDiff, err := DiffUpdateV1(v1, emptyStateVector)
	if err != nil {
		t.Fatalf("DiffUpdateV1(v1) unexpected error: %v", err)
	}
	if !bytes.Equal(gotDiff, wantDiff) {
		t.Fatalf("DiffUpdate(v2) = %x, want %x", gotDiff, wantDiff)
	}

	contentIDs, err := CreateContentIDsFromUpdate(v1)
	if err != nil {
		t.Fatalf("CreateContentIDsFromUpdate(v1) unexpected error: %v", err)
	}
	gotIntersection, err := IntersectUpdateWithContentIDs(v2, contentIDs)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDs(v2) unexpected error: %v", err)
	}
	wantIntersection, err := IntersectUpdateWithContentIDsV1(v1, contentIDs)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV1(v1) unexpected error: %v", err)
	}
	if !bytes.Equal(gotIntersection, wantIntersection) {
		t.Fatalf("IntersectUpdateWithContentIDs(v2) = %x, want %x", gotIntersection, wantIntersection)
	}
}
