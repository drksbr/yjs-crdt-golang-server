// Package ynodeproto define o framing binário do protocolo inter-node do
// yjs-go-bridge.
//
// A versão inicial (v1) usa um header fixo de 8 bytes em big-endian:
// - 1 byte de versão do wire format;
// - 1 byte de message type;
// - 2 bytes de flags reservadas ao tipo da mensagem;
// - 4 bytes de payload length.
//
// O pacote expõe somente o codec e os tipos básicos do frame. O shape dos
// payloads de cada message type fica para etapas posteriores de integração com
// runtimes e providers.
package ynodeproto
