package ynodeproto

import "errors"

var (
	// ErrNilFrame sinaliza tentativa de serializar ou validar frame nulo.
	ErrNilFrame = errors.New("ynodeproto: frame nao pode ser nil")
	// ErrNilMessage sinaliza tentativa de serializar ou validar mensagem tipada nula.
	ErrNilMessage = errors.New("ynodeproto: message nao pode ser nil")
	// ErrUnsupportedVersion sinaliza versão de header não reconhecida.
	ErrUnsupportedVersion = errors.New("ynodeproto: versao de protocolo nao suportada")
	// ErrUnknownMessageType sinaliza message type não reconhecido pelo pacote.
	ErrUnknownMessageType = errors.New("ynodeproto: message type desconhecido")
	// ErrInvalidNodeID sinaliza node id vazio ou só com espaços.
	ErrInvalidNodeID = errors.New("ynodeproto: node id invalido")
	// ErrInvalidConnectionID sinaliza connection id vazio ou só com espaços.
	ErrInvalidConnectionID = errors.New("ynodeproto: connection id invalido")
	// ErrMissingPayload sinaliza mensagens tipadas que exigem payload bruto não vazio.
	ErrMissingPayload = errors.New("ynodeproto: payload obrigatorio")
	// ErrInvalidEpoch sinaliza epoch ausente ou zerado em mensagens que dependem de fencing.
	ErrInvalidEpoch = errors.New("ynodeproto: epoch invalido")
	// ErrInvalidNonce sinaliza nonce ausente ou zerado em mensagens correlacionáveis.
	ErrInvalidNonce = errors.New("ynodeproto: nonce invalido")
	// ErrInvalidPayloadLength sinaliza tamanho negativo ou inválido informado na API.
	ErrInvalidPayloadLength = errors.New("ynodeproto: payload length invalido")
	// ErrPayloadTooLarge sinaliza payload incompatível com o campo uint32 do header.
	ErrPayloadTooLarge = errors.New("ynodeproto: payload excede limite do header")
	// ErrIncompleteHeader sinaliza bytes insuficientes para ler o header fixo.
	ErrIncompleteHeader = errors.New("ynodeproto: header incompleto")
	// ErrIncompletePayload sinaliza bytes insuficientes para ler o payload anunciado.
	ErrIncompletePayload = errors.New("ynodeproto: payload incompleto")
	// ErrPayloadLengthMismatch sinaliza divergência entre header e payload recebido.
	ErrPayloadLengthMismatch = errors.New("ynodeproto: payload length diverge do header")
	// ErrTrailingPayloadBytes sinaliza bytes extras após o fim de um payload tipado.
	ErrTrailingPayloadBytes = errors.New("ynodeproto: payload tipado contem bytes excedentes")
	// ErrTrailingBytes sinaliza bytes extras após um frame isolado completo.
	ErrTrailingBytes = errors.New("ynodeproto: frame contem bytes excedentes")
)
