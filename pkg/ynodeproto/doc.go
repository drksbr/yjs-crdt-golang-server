// Package ynodeproto define o framing binário e os payloads tipados do
// protocolo inter-node do yjs-go-bridge.
//
// A versão inicial (v1) usa um header fixo de 8 bytes em big-endian:
// - 1 byte de versão do wire format;
// - 1 byte de message type;
// - 2 bytes de flags reservadas ao tipo da mensagem;
// - 4 bytes de payload length.
//
// Acima do framing, o pacote também expõe structs tipadas para handshake,
// sync de documento, update de documento, awareness, query-awareness,
// disconnect/close e ping/pong, além de helpers para encode/decode entre
// mensagens tipadas e frames brutos.
//
// Strings são codificadas como `varuint(length) + bytes`, usando o codec
// canônico de `internal/varint`. Campos `epoch` e `nonce` usam uint64 fixo em
// big-endian; `clientID` usa uint32 fixo em big-endian. Mensagens com payload
// opaco de documento usam os bytes restantes do payload tipado como corpo bruto
// após os metadados roteáveis.
package ynodeproto
