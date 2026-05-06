package documents

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/objectstore"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

const (
	legacyYSweetFileName      = "data.ysweet"
	legacyYSweetDocName       = "doc"
	legacyYSweetBincodeMaxLen = common.MaxDocumentUpdateBytes
)

type LegacyYSweetMigrator struct {
	objects objectstore.Store
	paths   common.StoragePaths
}

type legacyYSweetEntry struct {
	key   []byte
	value []byte
}

func NewLegacyYSweetMigrator(objects objectstore.Store, paths common.StoragePaths) *LegacyYSweetMigrator {
	return &LegacyYSweetMigrator{objects: objects, paths: paths}
}

func (s *Service) EnsureLegacyMigrated(ctx context.Context, documentID string) error {
	if s == nil || s.legacy == nil || s.store == nil {
		return nil
	}
	documentID, err := common.NormalizeDocumentID(documentID)
	if err != nil {
		return err
	}

	key := storage.DocumentKey{
		Namespace:  s.namespace,
		DocumentID: documentID,
	}
	hasState, err := s.hasPersistedDocumentState(ctx, key)
	if err != nil {
		return err
	}
	if hasState {
		return s.ensureLegacySubdocumentsMigrated(ctx, documentID)
	}

	legacyKey := s.legacy.documentKey(documentID)
	exists, err := s.legacy.exists(ctx, legacyKey)
	if err != nil {
		return fmt.Errorf("stat legacy ysweet %s: %w", legacyKey, err)
	}
	if !exists {
		return nil
	}

	unlock := s.lockLegacyMigration(documentID)
	defer unlock()

	hasState, err = s.hasPersistedDocumentState(ctx, key)
	if err != nil {
		return err
	}
	if hasState {
		return s.ensureLegacySubdocumentsMigrated(ctx, documentID)
	}

	update, err := s.legacy.readUpdate(ctx, legacyKey)
	if err != nil {
		return err
	}
	snapshot, err := yjsbridge.PersistedSnapshotFromUpdate(update)
	if err != nil {
		return fmt.Errorf("decode legacy ysweet update for %s: %w", documentID, err)
	}
	if _, err := s.store.SaveSnapshotCheckpoint(ctx, key, snapshot, 0); err != nil {
		return fmt.Errorf("save migrated legacy ysweet snapshot for %s: %w", documentID, err)
	}
	if err := s.migrateLegacySubdocuments(ctx, documentID, update); err != nil {
		return err
	}
	log.Printf("legacy ysweet migrated doc=%s bytes=%d source=%s", documentID, len(update), legacyKey)
	return nil
}

func (s *Service) hasPersistedDocumentState(ctx context.Context, key storage.DocumentKey) (bool, error) {
	if _, err := s.store.LoadSnapshot(ctx, key); err == nil {
		return true, nil
	} else if !errors.Is(err, storage.ErrSnapshotNotFound) {
		return false, err
	}

	records, err := s.store.ListUpdates(ctx, key, 0, 1)
	if err != nil {
		return false, err
	}
	return len(records) > 0, nil
}

func (s *Service) lockLegacyMigration(documentID string) func() {
	s.legacyMu.Lock()
	if s.legacyLocks == nil {
		s.legacyLocks = make(map[string]*sync.Mutex)
	}
	lock := s.legacyLocks[documentID]
	if lock == nil {
		lock = &sync.Mutex{}
		s.legacyLocks[documentID] = lock
	}
	s.legacyMu.Unlock()

	lock.Lock()
	return func() {
		lock.Unlock()
	}
}

func (m *LegacyYSweetMigrator) documentKey(documentID string) string {
	return m.paths.LegacyYSweetKey(documentID)
}

func (m *LegacyYSweetMigrator) exists(ctx context.Context, legacyKey string) (bool, error) {
	if m == nil || m.objects == nil {
		return false, nil
	}
	return m.objects.Exists(ctx, legacyKey)
}

func (m *LegacyYSweetMigrator) readUpdate(ctx context.Context, legacyKey string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m == nil || m.objects == nil {
		return nil, common.ErrNotFound
	}
	data, err := objectstore.ReadAll(ctx, m.objects, legacyKey, 0)
	if err != nil {
		return nil, fmt.Errorf("read legacy ysweet %s: %w", legacyKey, err)
	}
	entries, err := decodeLegacyYSweetBincodeMap(data)
	if err != nil {
		return nil, fmt.Errorf("decode legacy ysweet %s: %w", legacyKey, err)
	}
	update, err := legacyYSweetEntriesAsUpdate(ctx, entries)
	if err != nil {
		return nil, fmt.Errorf("extract legacy ysweet update %s: %w", legacyKey, err)
	}
	return update, nil
}

func decodeLegacyYSweetBincodeMap(data []byte) ([]legacyYSweetEntry, error) {
	decoder := legacyBincodeReader{data: data}
	count, err := decoder.readU64()
	if err != nil {
		return nil, err
	}
	if count > 1_000_000 {
		return nil, fmt.Errorf("entry count too large: %d", count)
	}

	entries := make([]legacyYSweetEntry, 0, int(count))
	for i := uint64(0); i < count; i++ {
		key, err := decoder.readVec()
		if err != nil {
			return nil, fmt.Errorf("entry %d key: %w", i, err)
		}
		value, err := decoder.readVec()
		if err != nil {
			return nil, fmt.Errorf("entry %d value: %w", i, err)
		}
		entries = append(entries, legacyYSweetEntry{key: key, value: value})
	}
	if decoder.remaining() != 0 {
		return nil, fmt.Errorf("trailing bytes: %d", decoder.remaining())
	}
	return entries, nil
}

func legacyYSweetEntriesAsUpdate(ctx context.Context, entries []legacyYSweetEntry) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	oid, ok, err := legacyYSweetDocOID(entries, []byte(legacyYSweetDocName))
	if err != nil || !ok {
		return yjsbridge.MergeUpdatesContext(ctx)
	}

	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].key, entries[j].key) < 0
	})

	updates := make([][]byte, 0, 1)
	docKey := legacyYSweetDocStateKey(oid)
	for _, entry := range entries {
		if bytes.Equal(entry.key, docKey) {
			updates = append(updates, append([]byte(nil), entry.value...))
			break
		}
	}

	updatePrefix := legacyYSweetUpdateKeyPrefix(oid)
	for _, entry := range entries {
		if len(entry.key) == len(updatePrefix)+5 &&
			bytes.HasPrefix(entry.key, updatePrefix) &&
			entry.key[len(entry.key)-1] == 0 {
			updates = append(updates, append([]byte(nil), entry.value...))
		}
	}

	return yjsbridge.MergeUpdatesContext(ctx, updates...)
}

func legacyYSweetDocOID(entries []legacyYSweetEntry, docName []byte) (uint32, bool, error) {
	key := legacyYSweetOIDKey(docName)
	for _, entry := range entries {
		if !bytes.Equal(entry.key, key) {
			continue
		}
		if len(entry.value) != 4 {
			return 0, false, fmt.Errorf("invalid oid length: %d", len(entry.value))
		}
		return binary.BigEndian.Uint32(entry.value), true, nil
	}
	return 0, false, nil
}

func legacyYSweetOIDKey(docName []byte) []byte {
	key := make([]byte, 0, len(docName)+3)
	key = append(key, 0, 0)
	key = append(key, docName...)
	key = append(key, 0)
	return key
}

func legacyYSweetDocStateKey(oid uint32) []byte {
	key := make([]byte, 7)
	key[0] = 0
	key[1] = 1
	binary.BigEndian.PutUint32(key[2:6], oid)
	key[6] = 0
	return key
}

func legacyYSweetUpdateKeyPrefix(oid uint32) []byte {
	key := make([]byte, 7)
	key[0] = 0
	key[1] = 1
	binary.BigEndian.PutUint32(key[2:6], oid)
	key[6] = 2
	return key
}

type legacyBincodeReader struct {
	data []byte
	off  int
}

func (r *legacyBincodeReader) readU64() (uint64, error) {
	if len(r.data)-r.off < 8 {
		return 0, errors.New("unexpected eof reading u64")
	}
	value := binary.LittleEndian.Uint64(r.data[r.off : r.off+8])
	r.off += 8
	return value, nil
}

func (r *legacyBincodeReader) readVec() ([]byte, error) {
	length, err := r.readU64()
	if err != nil {
		return nil, err
	}
	if length > uint64(legacyYSweetBincodeMaxLen) {
		return nil, fmt.Errorf("vec length too large: %d", length)
	}
	if uint64(len(r.data)-r.off) < length {
		return nil, errors.New("unexpected eof reading vec")
	}
	value := append([]byte(nil), r.data[r.off:r.off+int(length)]...)
	r.off += int(length)
	return value, nil
}

func (r *legacyBincodeReader) remaining() int {
	return len(r.data) - r.off
}
