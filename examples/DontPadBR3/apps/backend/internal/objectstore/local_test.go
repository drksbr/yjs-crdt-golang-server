package objectstore

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
)

func TestLocalStorePutGetDelete(t *testing.T) {
	ctx := context.Background()
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocal() unexpected error: %v", err)
	}

	written, err := store.Put(ctx, "documents/doc/files/a.txt", bytes.NewReader([]byte("hello")), PutOptions{
		ContentType: "text/plain",
		MaxBytes:    10,
	})
	if err != nil {
		t.Fatalf("Put() unexpected error: %v", err)
	}
	if written != 5 {
		t.Fatalf("written = %d, want 5", written)
	}

	got, err := ReadAll(ctx, store, "documents/doc/files/a.txt", 10)
	if err != nil {
		t.Fatalf("ReadAll() unexpected error: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadAll() = %q, want hello", got)
	}

	exists, err := store.Exists(ctx, "documents/doc/files/a.txt")
	if err != nil || !exists {
		t.Fatalf("Exists() = %v, %v, want true, nil", exists, err)
	}
	if err := store.Delete(ctx, "documents/doc/files/a.txt"); err != nil {
		t.Fatalf("Delete() unexpected error: %v", err)
	}
	if _, err := store.Get(ctx, "documents/doc/files/a.txt"); !errors.Is(err, common.ErrNotFound) {
		t.Fatalf("Get() err = %v, want ErrNotFound", err)
	}
}

func TestLocalStoreRejectsTraversal(t *testing.T) {
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocal() unexpected error: %v", err)
	}
	if _, err := store.Put(context.Background(), "../outside", bytes.NewReader(nil), PutOptions{}); err == nil {
		t.Fatal("Put() err = nil, want traversal rejection")
	}
}

func TestLocalStoreLimitRemovesTempFile(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocal(root)
	if err != nil {
		t.Fatalf("NewLocal() unexpected error: %v", err)
	}

	_, err = store.Put(context.Background(), "documents/doc/file.bin", bytes.NewReader([]byte("toolarge")), PutOptions{
		MaxBytes: 3,
	})
	if !errors.Is(err, common.ErrPayloadTooLarge) {
		t.Fatalf("Put() err = %v, want ErrPayloadTooLarge", err)
	}
	if _, err := os.Stat(filepath.Join(root, "documents", "doc", "file.bin")); !os.IsNotExist(err) {
		t.Fatalf("target stat err = %v, want not exist", err)
	}
}
