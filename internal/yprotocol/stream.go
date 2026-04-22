package yprotocol

import (
	"fmt"

	ybinary "yjs-go-bridge/internal/binary"
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
