package yprotocol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	ybinary "github.com/drksbr/yjs-crdt-golang-server/internal/binary"
	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

// ReadProtocolMessages decodifica mensagens protocoladas ate o fim do fluxo.
func ReadProtocolMessages(r *ybinary.Reader) ([]*ProtocolMessage, error) {
	return readProtocolMessagesN(r, -1)
}

// DecodeProtocolMessages decodifica um fluxo completo de mensagens protocoladas.
//
// A funcao aceita apenas fluxos compostos por mensagens consecutivas completas.
// Qualquer byte residual parcial/invalido e tratado como erro durante a leitura.
func DecodeProtocolMessages(src []byte) ([]*ProtocolMessage, error) {
	reader := ybinary.NewReader(src)
	messages, err := ReadProtocolMessages(reader)
	if err != nil {
		return nil, err
	}

	// Se nao sobrar nenhuma mensagem parcial, nao ha bytes residuais validos.
	return messages, nil
}

// ReadProtocolMessagesN le no maximo n mensagens do fluxo.
func ReadProtocolMessagesN(r *ybinary.Reader, n int) ([]*ProtocolMessage, error) {
	if n < 0 {
		return nil, wrapError("ReadProtocolMessagesN.count", 0, fmt.Errorf("n deve ser nao negativo"))
	}
	return readProtocolMessagesN(r, n)
}

// ReadProtocolMessagesFromStream le mensagens protocoladas a partir de um fluxo com contexto de cancelamento.
func ReadProtocolMessagesFromStream(ctx context.Context, stream io.Reader) ([]*ProtocolMessage, error) {
	return ReadProtocolMessagesFromStreamNWithLimit(ctx, stream, -1, 0)
}

// ReadProtocolMessagesFromStreamN le no maximo n mensagens protocoladas de um fluxo.
func ReadProtocolMessagesFromStreamN(ctx context.Context, stream io.Reader, n int) ([]*ProtocolMessage, error) {
	return ReadProtocolMessagesFromStreamNWithLimit(ctx, stream, n, 0)
}

// ReadProtocolMessagesFromStreamNWithLimit adiciona limite opcional de bytes no buffer incremental.
//
// limiteBytes=0 significa sem limite de memória para a janela de leitura.
func ReadProtocolMessagesFromStreamNWithLimit(ctx context.Context, stream io.Reader, n int, limitBytes int) ([]*ProtocolMessage, error) {
	if n < -1 {
		return nil, wrapError("ReadProtocolMessagesFromStreamNWithLimit.count", 0, fmt.Errorf("n deve ser nao negativo"))
	}
	if limitBytes < 0 {
		return nil, wrapError("ReadProtocolMessagesFromStreamNWithLimit.limitBytes", 0, fmt.Errorf("limite de bytes deve ser nao negativo"))
	}

	const readChunkSize = 4096

	messages := make([]*ProtocolMessage, 0, 0)
	if n > 0 {
		messages = make([]*ProtocolMessage, 0, n)
	}
	streamBuffer := make([]byte, 0)

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		reader := ybinary.NewReader(streamBuffer)
		needMore := false
		var incompleteErr error

		for n < 0 || len(messages) < n {
			if reader.Remaining() == 0 {
				break
			}

			start := reader.Offset()
			message, err := ReadProtocolMessage(reader)
			if err == nil {
				messages = append(messages, message)
				continue
			}

			if isRecoverableStreamEOF(err) {
				needMore = true
				streamBuffer = append([]byte(nil), streamBuffer[start:]...)
				incompleteErr = err
				break
			}

			return nil, err
		}

		if len(messages) == n {
			return messages, nil
		}

		if !needMore {
			// Descarta bytes já consumidos e mantem apenas o que ainda nao foi lido.
			streamBuffer = streamBuffer[reader.Offset():]
		}

		if len(streamBuffer) == 0 {
			if err := readAndAppendStreamBytes(ctx, stream, &streamBuffer, limitBytes, readChunkSize); err != nil {
				if errors.Is(err, io.EOF) {
					return messages, nil
				}
				return nil, err
			}
			continue
		}

		if len(streamBuffer) >= limitBytes && limitBytes > 0 {
			return nil, ErrProtocolStreamByteLimitExceeded
		}

		if needMore {
			if limitBytes > 0 && len(streamBuffer) > limitBytes {
				return nil, ErrProtocolStreamByteLimitExceeded
			}

			if err := readAndAppendStreamBytes(ctx, stream, &streamBuffer, limitBytes, readChunkSize); err != nil {
				if errors.Is(err, io.EOF) {
					return nil, incompleteErr
				}
				return nil, err
			}
			continue
		}

		return messages, nil
	}
}

func readAndAppendStreamBytes(
	ctx context.Context,
	stream io.Reader,
	target *[]byte,
	limitBytes int,
	chunkSize int,
) error {
	readLimit := chunkSize
	if limitBytes > 0 {
		available := limitBytes - len(*target)
		if available <= 0 {
			return ErrProtocolStreamByteLimitExceeded
		}
		if available < readLimit {
			readLimit = available
		}
	}

	chunk := make([]byte, readLimit)
	read, err := readFromStream(ctx, stream, chunk)
	if read > 0 {
		*target = append(*target, chunk[:read]...)
	}

	if err != nil {
		return err
	}
	if read == 0 {
		return io.ErrNoProgress
	}

	return nil
}

func readFromStream(ctx context.Context, stream io.Reader, buf []byte) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if reader, ok := stream.(interface {
		ReadContext(context.Context, []byte) (int, error)
	}); ok {
		return reader.ReadContext(ctx, buf)
	}

	if closer, ok := stream.(interface{ Close() error }); ok {
		return readWithCancel(ctx, stream, closer, buf)
	}

	if reader, ok := stream.(interface{ SetReadDeadline(time.Time) error }); ok {
		if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
			return readWithDeadline(stream, reader, deadline, buf)
		}
	}

	// Readers sem contrato de cancelamento explícito continuam funcionando,
	// mas o contexto só é avaliado fora do Read bloqueante.
	return stream.Read(buf)
}

func readWithDeadline(
	stream io.Reader,
	reader interface{ SetReadDeadline(time.Time) error },
	deadline time.Time,
	buf []byte,
) (int, error) {
	if err := reader.SetReadDeadline(deadline); err != nil {
		return 0, err
	}

	read, err := stream.Read(buf)

	if resetErr := reader.SetReadDeadline(time.Time{}); resetErr != nil {
		if err == nil {
			return read, resetErr
		}
		return read, err
	}

	return read, err
}

func readWithCancel(
	ctx context.Context,
	stream io.Reader,
	closer interface{ Close() error },
	buf []byte,
) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	type readResult struct {
		n   int
		err error
	}

	resultCh := make(chan readResult, 1)
	go func() {
		n, err := stream.Read(buf)
		resultCh <- readResult{n: n, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = closer.Close()
		return 0, ctx.Err()
	case result := <-resultCh:
		return result.n, result.err
	}
}

func isRecoverableStreamEOF(err error) bool {
	return errors.Is(err, ybinary.ErrUnexpectedEOF) || errors.Is(err, varint.ErrUnexpectedEOF)
}

func readProtocolMessagesN(r *ybinary.Reader, n int) ([]*ProtocolMessage, error) {
	capacity := 0
	if n > 0 {
		capacity = n
	}
	messages := make([]*ProtocolMessage, 0, capacity)

	for count := 0; n < 0 || count < n; count++ {
		if r.Remaining() == 0 {
			return messages, nil
		}

		message, err := ReadProtocolMessage(r)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	return messages, nil
}
