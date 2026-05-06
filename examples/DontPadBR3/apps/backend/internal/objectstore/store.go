package objectstore

import (
	"context"
	"io"
)

// Store guarda objetos binarios por chave relativa.
type Store interface {
	Put(ctx context.Context, key string, src io.Reader, opts PutOptions) (int64, error)
	Get(ctx context.Context, key string) (*Object, error)
	Exists(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, key string) error
	DeletePrefix(ctx context.Context, prefix string) error
	String() string
}

// PutOptions descreve metadados e limites para uma escrita.
type PutOptions struct {
	ContentType string
	MaxBytes    int64
}

// Object e o resultado de uma leitura de objeto.
type Object struct {
	Body        io.ReadCloser
	Size        int64
	ContentType string
}

// ReadAll le um objeto inteiro respeitando um limite opcional.
func ReadAll(ctx context.Context, store Store, key string, maxBytes int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	obj, err := store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = obj.Body.Close()
	}()

	reader := io.Reader(obj.Body)
	if maxBytes > 0 {
		reader = io.LimitReader(obj.Body, maxBytes+1)
	}
	data, err := io.ReadAll(&contextReader{ctx: ctx, r: reader})
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, errPayloadTooLarge()
	}
	return data, nil
}

func copyLimited(ctx context.Context, dst io.Writer, src io.Reader, maxBytes int64) (int64, error) {
	reader := io.Reader(&contextReader{ctx: ctx, r: src})
	if maxBytes > 0 {
		reader = io.LimitReader(reader, maxBytes+1)
	}
	written, err := io.Copy(dst, reader)
	if err != nil {
		return written, err
	}
	if maxBytes > 0 && written > maxBytes {
		return written, errPayloadTooLarge()
	}
	return written, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
