package objectstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
)

// LocalStore guarda objetos em um diretorio local.
type LocalStore struct {
	root string
}

// NewLocal cria um object store local preso ao diretorio informado.
func NewLocal(root string) (*LocalStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("local object store root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local object store root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("ensure local object store root: %w", err)
	}
	return &LocalStore{root: abs}, nil
}

func (s *LocalStore) Put(ctx context.Context, key string, src io.Reader, opts PutOptions) (int64, error) {
	target, err := s.resolve(key)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return 0, err
	}
	temp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return 0, err
	}
	tempName := temp.Name()
	defer func() {
		_ = os.Remove(tempName)
	}()

	written, copyErr := copyLimited(ctx, temp, src, opts.MaxBytes)
	closeErr := temp.Close()
	if copyErr != nil {
		return written, copyErr
	}
	if closeErr != nil {
		return written, closeErr
	}
	if err := os.Rename(tempName, target); err != nil {
		return written, err
	}
	return written, nil
}

func (s *LocalStore) Get(ctx context.Context, key string) (*Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	target, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, common.ErrNotFound
		}
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return &Object{Body: file, Size: info.Size()}, nil
}

func (s *LocalStore) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	target, err := s.resolve(key)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *LocalStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *LocalStore) DeletePrefix(ctx context.Context, prefix string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(prefix) == "" {
		return fmt.Errorf("delete prefix is required")
	}
	target, err := s.resolve(prefix)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *LocalStore) String() string {
	return "local:" + s.root
}

func (s *LocalStore) resolve(key string) (string, error) {
	clean, err := cleanKey(key)
	if err != nil {
		return "", err
	}
	target := filepath.Join(s.root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(s.root, target)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("unsafe object key %q", key)
	}
	return target, nil
}

func cleanKey(key string) (string, error) {
	key = strings.TrimSpace(strings.ReplaceAll(key, "\\", "/"))
	key = strings.TrimPrefix(key, "/")
	for _, segment := range strings.Split(key, "/") {
		if segment == ".." {
			return "", fmt.Errorf("unsafe object key %q", key)
		}
	}
	clean := strings.TrimPrefix(path.Clean("/"+key), "/")
	if clean == "." || clean == "" || strings.Contains(clean, "\x00") {
		return "", fmt.Errorf("object key is required")
	}
	return clean, nil
}
