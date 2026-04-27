# Yjs Go Bridge

`yjs-go-bridge` é uma camada de compatibilidade Go para snapshots do ecossistema Yjs/YHub.
Na fase implementada hoje, a API pública está centrada em:

- snapshots persistíveis em V1 (canônicos),
- contratos de armazenamento de snapshots,
- formato, merge/diff, state vector e content ids para updates com suporte operacional em V1,
- implementações de store em memória e PostgreSQL,
- protocolo y-protocols (`pkg/yprotocol`), incluindo runtime in-process e provider local mínimo,
- payload/estado local de awareness (`pkg/yawareness`),
- borda HTTP/WebSocket genérica em `net/http` (`pkg/yhttp`) para acoplamento em servidores e frameworks,
- hooks opcionais de observabilidade no transporte HTTP/WebSocket, com adapter Prometheus em `pkg/yhttp/prometheus`.

Acima do núcleo binário e do provider local em `pkg/yprotocol`, o projeto já expõe
uma primeira borda pública de transporte em `pkg/yhttp`, mantendo o escopo em
single-process, V1-only e sem coordenação distribuída entre nós.

A fase distribuída já entrou no branch atual: o wire inter-node agora tem
mensagens tipadas em `pkg/ynodeproto`, o `pkg/yprotocol.Provider` já faz
bootstrap/recovery via `snapshot + update log`, e `pkg/yhttp` já expõe uma
borda owner-aware que só materializa o room quando o owner resolvido é local.

## Fase distribuída em andamento

- qualquer nó já pode expor a borda HTTP/WS e resolver ownership antes de abrir o provider local;
- `pkg/yhttp.OwnerAwareServer` autentica/resolve owner e só abre o room localmente quando o owner é o nó atual;
- apenas o owner ativo de cada documento/shard materializa `Session`/`Provider` e processa o room;
- o tráfego inter-node já tem mensagens tipadas e versionadas acima de `pkg/ynodeproto.MessageType`;
- o owner local já pode ser reidratado a partir de snapshot base + replay do tail do `update log`, com awareness mantido como estado efêmero fora do recovery durável;
- o lifecycle de lease no control plane já sobe `epoch` monotônico: acquire inicial em `1`, renew preserva `epoch/token` e takeover após expiração incrementa o epoch;
- a resposta owner-aware remota já devolve `epoch` do owner junto dos metadados de roteamento;
- lease, `epoch` e fencing continuam sendo o próximo fechamento crítico para handoff, failover e prevenção de split-brain;
- o wire interno permanece separado do `y-protocols`, que continua restrito à borda cliente.

## Epochs 1-3 já entregues

O branch atual já entrega a base operacional inicial e o segundo corte de
integração da fase distribuída, ainda sem ligar um runtime multi-nó completo:

- `pkg/storage` agora expõe contratos para snapshot distribuído, append log por documento, placement e lease, além de replay/recovery públicos em cima de `snapshot + update log`;
- `pkg/storage` já expõe `RecoverSnapshot`, `ReplayUpdateLog` e `CompactUpdateLog` para bootstrap, catch-up e checkpoint/compaction do runtime autoritativo;
- `pkg/storage/memory` e `pkg/storage/postgres` já implementam `DistributedStore` com snapshot, update log, placement e lease;
- `pkg/ycluster` expõe tipos, resolver determinístico de shard, lookup storage-backed de owner e adapter de lease sobre `pkg/storage`;
- `pkg/ynodeproto` agora expõe o framing binário versionado e os payloads tipados do protocolo inter-node (`handshake`, `document-sync-*`, `document-update`, `awareness-update`, `ping/pong`);
- `pkg/yprotocol.Provider` já trata `snapshot + update log` como bootstrap/recovery do owner local, registrando updates no log e compactando o tail em `Persist`;
- `pkg/yhttp` já expõe `OwnerAwareServer`, que resolve owner antes do provider local e responde com metadados retryable ou hook customizado quando o owner é remoto;
- `pkg/ycluster.StorageLeaseStore` já endurece o lifecycle básico de ownership com `epoch` monotônico em lease ativa, renew preservando `token`/`epoch` e takeover pós-expiração incrementando o epoch persistido;
- `examples/owner-aware-http-edge` e os novos testes de integração cobrem o wiring owner-aware, o replay via `snapshot + update log` e o recovery do provider;
- handoff, cutover, forwarding inter-node e fencing autoritativo ainda seguem como etapa posterior.

As APIs de update abaixo estão disponíveis na camada pública e seguem validação de formato antes de executar as operações.

## API pública atual

### `pkg/yjsbridge`

#### Snapshot

- `type Snapshot = yupdate.Snapshot`
- `type PersistedSnapshot = yupdate.PersistedSnapshot`
- `func NewSnapshot() *Snapshot`
- `func NewPersistedSnapshot() *PersistedSnapshot`
- `func PersistedSnapshotFromUpdate(update []byte) (*PersistedSnapshot, error)`
- `func PersistedSnapshotFromUpdates(updates ...[]byte) (*PersistedSnapshot, error)`
- `func PersistedSnapshotFromUpdatesContext(ctx context.Context, updates ...[]byte) (*PersistedSnapshot, error)`
- `func EncodePersistedSnapshotV1(snapshot *PersistedSnapshot) ([]byte, error)`
- `func DecodePersistedSnapshotV1(payload []byte) (*PersistedSnapshot, error)`
- `func DecodePersistedSnapshotV1Context(ctx context.Context, payload []byte) (*PersistedSnapshot, error)`
- `var ErrUnsupportedUpdateFormatV2 error`
- `var ErrUnknownUpdateFormat error`
- `var ErrInconsistentPersistedSnapshot error`
- `var ErrMismatchedUpdateFormats error`

#### Formato de update

- `type UpdateFormat uint8`
- `const UpdateFormatUnknown UpdateFormat`
- `const UpdateFormatV1 UpdateFormat`
- `const UpdateFormatV2 UpdateFormat`
- `func FormatFromUpdate(update []byte) (UpdateFormat, error)`
- `func FormatFromUpdates(updates ...[]byte) (UpdateFormat, error)`
- `func FormatFromUpdatesContext(ctx context.Context, updates ...[]byte) (UpdateFormat, error)`

#### Merge e diff

- `func MergeUpdates(updates ...[]byte) ([]byte, error)`
- `func MergeUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error)`
- `func DiffUpdate(update, stateVector []byte) ([]byte, error)`
- `func DiffUpdateContext(ctx context.Context, update, stateVector []byte) ([]byte, error)`

#### State vector

- `func StateVectorFromUpdate(update []byte) (map[uint32]uint32, error)`
- `func EncodeStateVectorFromUpdate(update []byte) ([]byte, error)`
- `func StateVectorFromUpdates(updates ...[]byte) (map[uint32]uint32, error)`
- `func StateVectorFromUpdatesContext(ctx context.Context, updates ...[]byte) (map[uint32]uint32, error)`
- `func EncodeStateVectorFromUpdates(updates ...[]byte) ([]byte, error)`
- `func EncodeStateVectorFromUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error)`

#### Content IDs

- `type ContentIDs`
- `func NewContentIDs() *ContentIDs`
- `func CreateContentIDsFromUpdate(update []byte) (*ContentIDs, error)`
- `func ContentIDsFromUpdates(updates ...[]byte) (*ContentIDs, error)`
- `func ContentIDsFromUpdatesContext(ctx context.Context, updates ...[]byte) (*ContentIDs, error)`
- `func EncodeContentIDs(contentIDs *ContentIDs) ([]byte, error)`
- `func DecodeContentIDs(payload []byte) (*ContentIDs, error)`
- `func MergeContentIDs(a *ContentIDs, b ...*ContentIDs) *ContentIDs`
- `func IntersectContentIDs(a, b *ContentIDs) *ContentIDs`
- `func DiffContentIDs(subject, remove *ContentIDs) *ContentIDs`
- `func IsSubsetContentIDs(subject, container *ContentIDs) bool`
- `func IntersectUpdateWithContentIDs(update []byte, contentIDs *ContentIDs) ([]byte, error)`
- `func IntersectUpdateWithContentIDsContext(ctx context.Context, update []byte, contentIDs *ContentIDs) ([]byte, error)`

#### Notas de comportamento

- `PersistedSnapshotFromUpdates` aceita zero ou mais updates; updates vazios não alteram estado e retornam snapshot vazio.
- `DecodePersistedSnapshotV1` aceita payload vazio como documento vazio.
- `MergeUpdates` sem argumentos e com todos os updates vazios retorna update V1 vazio.
- operações de update que aceitam `context.Context` respeitam cancelamento.
- `FormatFromUpdate/FormatFromUpdates` identificam V2; a execução funcional ainda é V1 primeiro.

### `pkg/yprotocol`

API pública de envelope/binário para mensagens do Yjs e runtime in-process mínimo de sessão.

#### Tipos e constantes

- `type ProtocolType = internal.ProtocolType`
- `type SyncMessageType = internal.SyncMessageType`
- `type AuthMessageType = internal.AuthMessageType`
- `type SyncMessage = internal.SyncMessage`
- `type AuthMessage = internal.AuthMessage`
- `type QueryAwarenessMessage = internal.QueryAwarenessMessage`
- `type ProtocolMessage struct { Protocol ProtocolType; Sync *SyncMessage; Awareness *AwarenessMessage; Auth *AuthMessage; QueryAwareness *QueryAwarenessMessage }`
- `type AwarenessMessage = yawareness.Update`
- `type AwarenessClient = yawareness.ClientState`
- `const ProtocolTypeSync = internal.ProtocolTypeSync`
- `const ProtocolTypeAwareness = internal.ProtocolTypeAwareness`
- `const ProtocolTypeAuth = internal.ProtocolTypeAuth`
- `const ProtocolTypeQueryAwareness = internal.ProtocolTypeQueryAwareness`
- `const SyncMessageTypeStep1 = internal.SyncMessageTypeStep1`
- `const SyncMessageTypeStep2 = internal.SyncMessageTypeStep2`
- `const SyncMessageTypeUpdate = internal.SyncMessageTypeUpdate`
- `const AuthMessageTypePermissionDenied = internal.AuthMessageTypePermissionDenied`

#### Erros

- `var ErrUnknownProtocolType = internal.ErrUnknownProtocolType`
- `var ErrUnexpectedProtocolType = internal.ErrUnexpectedProtocolType`
- `var ErrUnknownSyncMessageType = internal.ErrUnknownSyncMessageType`
- `var ErrUnknownAuthMessageType = internal.ErrUnknownAuthMessageType`
- `var ErrInvalidAwarenessJSON = internal.ErrInvalidAwarenessJSON`
- `var ErrTrailingBytes = internal.ErrTrailingBytes`
- `var ErrProtocolStreamByteLimitExceeded = internal.ErrProtocolStreamByteLimitExceeded`
- `type ParseError = internal.ParseError`

#### Funções

- `func EncodeProtocolMessage(protocol ProtocolType, payload []byte) ([]byte, error)`
- `func DecodeProtocolMessage(src []byte) (*ProtocolMessage, error)`
- `func DecodeProtocolMessages(src []byte) ([]*ProtocolMessage, error)`
- `func ReadProtocolMessagesFromStream(ctx context.Context, stream io.Reader) ([]*ProtocolMessage, error)`
- `func ReadProtocolMessagesFromStreamN(ctx context.Context, stream io.Reader, n int) ([]*ProtocolMessage, error)`
- `func ReadProtocolMessagesFromStreamNWithLimit(ctx context.Context, stream io.Reader, n int, limitBytes int) ([]*ProtocolMessage, error)`

- `func EncodeSyncMessage(typ SyncMessageType, payload []byte) ([]byte, error)`
- `func DecodeSyncMessage(src []byte) (*SyncMessage, error)`
- `func DecodeProtocolSyncMessage(src []byte) (*SyncMessage, error)`
- `func EncodeSyncStep1(stateVector []byte) []byte`
- `func EncodeSyncStep1FromUpdate(update []byte) ([]byte, error)`
- `func EncodeSyncStep1FromUpdates(updates ...[]byte) ([]byte, error)`
- `func EncodeSyncStep1FromUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error)`
- `func EncodeSyncStep2(update []byte) []byte`
- `func EncodeSyncStep2FromUpdates(updates ...[]byte) ([]byte, error)`
- `func EncodeSyncStep2FromUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error)`
- `func EncodeSyncUpdate(update []byte) []byte`

- `func EncodeProtocolSyncMessage(typ SyncMessageType, payload []byte) ([]byte, error)`
- `func EncodeProtocolSyncStep1(stateVector []byte) []byte`
- `func EncodeProtocolSyncStep1FromUpdate(update []byte) ([]byte, error)`
- `func EncodeProtocolSyncStep1FromUpdates(updates ...[]byte) ([]byte, error)`
- `func EncodeProtocolSyncStep1FromUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error)`
- `func EncodeProtocolSyncStep2(update []byte) []byte`
- `func EncodeProtocolSyncStep2FromUpdates(updates ...[]byte) ([]byte, error)`
- `func EncodeProtocolSyncStep2FromUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error)`
- `func EncodeProtocolSyncUpdate(update []byte) []byte`

- `func EncodeAuthMessage(typ AuthMessageType, reason string) ([]byte, error)`
- `func EncodeAuthPermissionDenied(reason string) []byte`
- `func EncodeProtocolAuthMessage(typ AuthMessageType, reason string) ([]byte, error)`
- `func EncodeProtocolAuthPermissionDenied(reason string) []byte`
- `func DecodeAuthMessage(src []byte) (*AuthMessage, error)`
- `func DecodeProtocolAuthMessage(src []byte) (*AuthMessage, error)`
- `func EncodeProtocolQueryAwareness() []byte`
- `func DecodeProtocolQueryAwareness(src []byte) (*QueryAwarenessMessage, error)`

- `func EncodeProtocolAwarenessUpdate(update *yawareness.Update) ([]byte, error)`
- `func DecodeProtocolAwarenessUpdate(src []byte) (*yawareness.Update, error)`

#### Observação de comportamento

- `context.Context` pode ser `nil`; a implementação usa `context.Background()` nesses pontos.
- A leitura por stream é incremental e pode respeitar cancelamento.
- `EncodeProtocolAwarenessUpdate` e `DecodeProtocolAwarenessUpdate` delegam para `pkg/yawareness`.

#### Runtime in-process de sessão

Corte público atual em `pkg/yprotocol`, restrito a runtime local em processo:

- `type Session`
- `func NewSession(localClientID uint32) *Session`
- `func (s *Session) Awareness() *yawareness.StateManager`
- `func (s *Session) UpdateV1() []byte`
- `func (s *Session) LoadUpdate(update []byte) error`
- `func (s *Session) LoadPersistedSnapshot(snapshot *yjsbridge.PersistedSnapshot) error`
- `func (s *Session) PersistedSnapshot() (*yjsbridge.PersistedSnapshot, error)`
- `func (s *Session) HandleProtocolMessage(message *ProtocolMessage) ([]*ProtocolMessage, error)`
- `func (s *Session) HandleProtocolMessages(messages ...*ProtocolMessage) ([]*ProtocolMessage, error)`
- `func (s *Session) HandleEncodedMessages(src []byte) ([]byte, error)`
- `func EncodeProtocolEnvelope(message *ProtocolMessage) ([]byte, error)`
- `func EncodeProtocolEnvelopes(messages ...*ProtocolMessage) ([]byte, error)`
- `var ErrNilProtocolMessage error`
- `var ErrInvalidProtocolMessage error`

Contrato esperado desse runtime:

- mantém estado do documento em V1 dentro do processo;
- integra o estado local de awareness via `pkg/yawareness`;
- aceita hidratação a partir de update V1 e `PersistedSnapshot`;
- responde a envelopes `sync`, `awareness`, `auth` e `query-awareness` no escopo mínimo do protocolo;
- não implementa provider distribuído, transporte de rede, coordenação entre nós ou persistência automática;
- continua sem suporte operacional a V2.

#### Provider local em processo

- `type ProviderConfig struct { Store storage.SnapshotStore }`
- `type DispatchResult struct { Direct, Broadcast []byte }`
- `type Provider`
- `type Connection`
- `var ErrInvalidConnectionID error`
- `var ErrConnectionClosed error`
- `var ErrConnectionExists error`
- `var ErrClientIDExists error`
- `var ErrPersistenceDisabled error`
- `func NewProvider(cfg ProviderConfig) *Provider`
- `func (p *Provider) Open(ctx context.Context, key storage.DocumentKey, connectionID string, localClientID uint32) (*Connection, error)`
- `func (c *Connection) ID() string`
- `func (c *Connection) ClientID() uint32`
- `func (c *Connection) DocumentKey() storage.DocumentKey`
- `func (c *Connection) HandleEncodedMessages(src []byte) (*DispatchResult, error)`
- `func (c *Connection) Persist(ctx context.Context) (*storage.SnapshotRecord, error)`
- `func (c *Connection) Close() (*DispatchResult, error)`

Contrato esperado dessa camada:

- mantém um snapshot autoritativo por documento dentro do processo;
- replica updates e awareness apenas entre conexões do mesmo processo;
- permite hidratação/persistência opcional via `pkg/storage`;
- não implementa transporte distribuído, ownership entre nós ou V2.

Próxima etapa planejada acima dessa camada:

- tratar `Provider`/`Connection` como runtime local do futuro owner distribuído;
- abrir o runtime autoritativo a partir de snapshot base + replay do tail do `update log`, reaproveitando `pkg/storage` para recovery/checkpoint e mantendo awareness fora do estado durável;
- introduzir resolução de owner, lease/`epoch`/fencing e aceitar `Session`/`Provider` apenas no owner local;
- adicionar protocolo inter-node tipado para forwarding, handoff e recovery, mantendo `y-protocols` apenas na borda cliente.

### `pkg/yhttp`

API pública de transporte HTTP/WebSocket acima de `pkg/yprotocol.Provider`.

#### Tipos, erros e construção

- `var ErrNilProvider error`
- `var ErrNilResolveRequest error`
- `type Request struct { DocumentKey storage.DocumentKey; ConnectionID string; ClientID uint32; PersistOnClose bool }`
- `type ResolveRequestFunc func(r *http.Request) (Request, error)`
- `type ErrorHandler func(r *http.Request, req Request, err error)`
- `type Metrics interface { ConnectionOpened(Request); ConnectionClosed(Request); FrameRead(Request, int); FrameWritten(Request, string, int); Handle(Request, time.Duration, error); Persist(Request, time.Duration, error); Error(Request, string, error) }`
- `type ServerConfig struct { Provider *yprotocol.Provider; ResolveRequest ResolveRequestFunc; AcceptOptions *websocket.AcceptOptions; ReadLimitBytes int64; WriteTimeout time.Duration; PersistTimeout time.Duration; Metrics Metrics; OnError ErrorHandler }`
- `type Server`
- `func NewServer(cfg ServerConfig) (*Server, error)`
- `func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)`

#### Observações de comportamento

- `Server` implementa `http.Handler`, então pode ser usado diretamente em `net/http`.
- `pkg/yhttp/gin` expõe `Handler(http.Handler) gin.HandlerFunc`.
- `pkg/yhttp/echo` expõe `Handler(http.Handler) echo.HandlerFunc`.
- `pkg/yhttp/chi` expõe `Mount(chi.Router, pattern string, http.Handler)`.
- `pkg/yhttp/prometheus` expõe um adapter opcional de métricas para `prometheus/client_golang`.
- `ClientID` precisa bater com o client id usado pelo peer Yjs nos payloads de awareness.
- `ConnectionID` pode ser omitido; o handler gera um identificador local estável para a conexão.
- o handler aceita apenas frames binários do `y-protocols`.
- o fanout continua local ao processo, reaproveitando o `DispatchResult` do provider.
- no roadmap distribuído, `pkg/yhttp` continua como borda pública em qualquer nó, mas entra em modo edge owner-aware: o nó não-owner aceita e autentica a conexão, resolve owner, encaminha frames/respostas pelo wire inter-node e não materializa o room localmente.

#### Adapters por framework

##### `pkg/yhttp/gin`

- `func Handler(handler http.Handler) gin.HandlerFunc`

##### `pkg/yhttp/echo`

- `func Handler(handler http.Handler) echo.HandlerFunc`

##### `pkg/yhttp/chi`

- `func Mount(router chi.Router, pattern string, handler http.Handler)`

##### `pkg/yhttp/prometheus`

- `type Config struct { Namespace, Subsystem string; Registerer prometheus.Registerer; HandleDurationBuckets []float64; PersistDurationBucket []float64 }`
- `type Metrics`
- `func New(cfg Config) (*Metrics, error)`

Esses adapters mantêm o acoplamento específico de framework fora de `pkg/yhttp`
e evitam duplicar a lógica de transporte do protocolo Yjs. No caso de
`pkg/yhttp/prometheus`, o pacote implementa a interface `yhttp.Metrics` e pode
ser registrado em um `prometheus.Registry` próprio, enquanto o endpoint
`/metrics` segue a montagem padrão do `promhttp`.

### `pkg/yawareness`

API de payload awareness e estado local:

#### Tipos e estado

- `type ClientState = internal.ClientState`
- `type Update = internal.Update`
- `type ClientMeta = internal.ClientMeta`
- `type StateManager = internal.StateManager`
- `type ParseError = internal.ParseError`
- `const OutdatedTimeout = internal.OutdatedTimeout`
- `var ErrInvalidJSON = internal.ErrInvalidJSON`
- `var ErrTrailingBytes = internal.ErrTrailingBytes`
- `var ErrLocalClientIDNotConfigured = internal.ErrLocalClientIDNotConfigured`

#### Funções

- `func NewStateManager(localClientID uint32) *StateManager`
- `func AppendUpdate(dst []byte, update *Update) ([]byte, error)`
- `func EncodeUpdate(update *Update) ([]byte, error)`
- `func DecodeUpdate(src []byte) (*Update, error)`
- `func EncodeProtocolUpdate(update *Update) ([]byte, error)`
- `func DecodeProtocolUpdate(src []byte) (*Update, error)`

#### `StateManager`

- `func (m *StateManager) SetLocalClientID(clientID uint32)`
- `func (m *StateManager) Apply(update *Update)`
- `func (m *StateManager) ApplyAt(update *Update, now time.Time)`
- `func (m *StateManager) ApplyJSON(src []byte)`
- `func (m *StateManager) Snapshot() *Update`
- `func (m *StateManager) UpdateForClients(clientIDs []uint32) *Update`
- `func (m *StateManager) Get(clientID uint32) (ClientState, bool)`
- `func (m *StateManager) Meta(clientID uint32) (ClientMeta, bool)`
- `func (m *StateManager) SetLocalState(state json.RawMessage) error`
- `func (m *StateManager) SetLocalStateAt(state json.RawMessage, now time.Time) error`
- `func (m *StateManager) RenewLocalIfDue(timeout time.Duration) (bool, error)`
- `func (m *StateManager) RenewLocalIfDueAt(now time.Time, timeout time.Duration) (bool, error)`
- `func (m *StateManager) ExpireStale(timeout time.Duration) []uint32`
- `func (m *StateManager) ExpireStaleAt(now time.Time, timeout time.Duration) []uint32`

#### Observação de escopo

- `pkg/yawareness` não implementa transporte, provider ou sincronização distribuída.

### `pkg/storage`

- `type DocumentKey struct { Namespace, DocumentID string }`
- `func (k DocumentKey) Validate() error`
- `type SnapshotRecord struct { Key DocumentKey; Snapshot *yjsbridge.PersistedSnapshot; StoredAt time.Time }`
- `func (r *SnapshotRecord) Clone() *SnapshotRecord`
- `type SnapshotStore interface { SaveSnapshot(ctx context.Context, key DocumentKey, snapshot *yjsbridge.PersistedSnapshot) (*SnapshotRecord, error); LoadSnapshot(ctx context.Context, key DocumentKey) (*SnapshotRecord, error) }`
- `ErrSnapshotNotFound`
- `ErrPlacementNotFound`
- `ErrLeaseNotFound`
- `ErrInvalidDocumentKey`
- `ErrInvalidShardID`
- `ErrInvalidNodeID`
- `ErrInvalidOwnerInfo`
- `ErrInvalidUpdatePayload`
- `ErrInvalidLeaseToken`
- `ErrInvalidLeaseExpiry`
- `ErrNilPersistedSnapshot`

`DocumentKey.Validate` exige `DocumentID` não vazio. `Namespace` é opcional.

#### Fundamentos distribuídos já expostos

Sem alterar o fluxo single-process atual, `pkg/storage` agora também expõe os
contratos-base que sustentam a próxima fase distribuída:

- `type ShardID string`
- `type NodeID string`
- `type UpdateOffset uint64`
- `type UpdateLogRecord`
- `type PlacementRecord`
- `type OwnerInfo`
- `type LeaseRecord`
- `type UpdateLogStore interface`
- `type PlacementStore interface`
- `type LeaseStore interface`
- `type DistributedStore interface`
- `type SnapshotLogStore interface`
- `type RecoveryResult struct { Snapshot *yjsbridge.PersistedSnapshot; Updates []*UpdateLogRecord; LastOffset UpdateOffset }`
- `type UpdateLogReplayResult struct { Snapshot *yjsbridge.PersistedSnapshot; Through UpdateOffset; Applied int }`
- `type UpdateLogCompactionResult struct { Snapshot *yjsbridge.PersistedSnapshot; Record *SnapshotRecord; Through UpdateOffset; Applied int }`
- `func ReplaySnapshot(ctx context.Context, base *yjsbridge.PersistedSnapshot, updates ...*UpdateLogRecord) (*yjsbridge.PersistedSnapshot, error)`
- `func ReplayUpdateLog(store UpdateLogStore, key DocumentKey, base *yjsbridge.PersistedSnapshot, after UpdateOffset, limit int) (*UpdateLogReplayResult, error)`
- `func RecoverSnapshot(ctx context.Context, snapshots SnapshotStore, updates UpdateLogStore, key DocumentKey, after UpdateOffset, limit int) (*RecoveryResult, error)`
- `func CompactUpdateLog(store SnapshotLogStore, key DocumentKey, base *yjsbridge.PersistedSnapshot, after UpdateOffset, limit int) (*UpdateLogCompactionResult, error)`

Esses contratos ainda não substituem `SnapshotStore`; eles abrem o caminho para
`snapshot + update log`, placement por shard e ownership com lease/fencing.
No corte atual:

- `UpdateLogStore` modela append/list/trim de updates V1 por `DocumentKey`, com `UpdateOffset` monotônico;
- `PlacementStore` separa a resolução persistida `documento -> shard`;
- `LeaseStore` separa ownership efêmero por shard via `OwnerInfo` + token opaco para renew/release;
- `DistributedStore` agrega snapshot, log, placement e lease em um backend opcional completo;
- `ReplaySnapshot` aplica um tail incremental de `UpdateLogRecord` sobre um snapshot base;
- `ReplayUpdateLog` pagina o tail persistido e devolve `Through` para o runtime reaproveitar como high-water mark de recovery/checkpoint;
- `RecoverSnapshot` carrega snapshot base, lista batches do `update log` e reconstrói o estado consolidado com `LastOffset` observável para recovery/checkpoint;
- `CompactUpdateLog` persiste um novo snapshot consolidado e poda o log até o offset aplicado.

No corte atual, `pkg/yprotocol.Provider` já usa esses helpers para bootstrap e
recovery do owner local: snapshot base + replay do tail do log para recuperar o
documento autoritativo, seguido de checkpoint/trim em `Persist`. Awareness
continua fora desse recovery durável.

### `pkg/ynodeproto`

Pacote público inicial do protocolo binário inter-node, separado do
`y-protocols` usado no perímetro com clientes.

- `const CurrentVersion`
- `const Version1`
- `const HeaderSize`
- `type Flags uint16`
- `type MessageType uint8`
- `type Header struct { Version uint8; Type MessageType; Flags Flags; PayloadLength uint32 }`
- `type Frame struct { Header Header; Payload []byte }`
- `func NewHeader(typ MessageType, flags Flags, payloadLength int) (Header, error)`
- `func EncodeHeader(header Header) ([]byte, error)`
- `func DecodeHeader(src []byte) (Header, error)`
- `func NewFrame(typ MessageType, flags Flags, payload []byte) (*Frame, error)`
- `func EncodeFrame(frame *Frame) ([]byte, error)`
- `func DecodeFrame(src []byte) (*Frame, error)`
- `func DecodeFramePrefix(src []byte) (*Frame, int, error)`
- `var ErrUnsupportedVersion error`
- `var ErrUnknownMessageType error`
- `var ErrIncompleteHeader error`
- `var ErrIncompletePayload error`
- `var ErrTrailingBytes error`

Escopo atual:

- framing fixo e versionado;
- enum de tipos de mensagem para handshake, catch-up de documento, update de documento, awareness e ping/pong;
- payloads tipados/validados por tipo para handshake, sync request/response, document update, awareness update e ping/pong;
- encode/decode estrito para frame único e decode por prefixo para stream concatenado;
- `ParseError` com offset para falhas de decode de payloads tipados.

Próximo passo acima desse wire:

- ligar essas mensagens ao forwarding real entre edge e owner;
- acrescentar fluxos de handoff/cutover e respostas de fencing;
- manter `Header`/`Frame` estáveis enquanto a semântica distribuída sobe de nível.

### `pkg/ycluster`

Pacote público inicial do control plane distribuído.

- `type NodeID string`
- `type ShardID uint32`
- `type Placement`
- `type Lease`
- `type LeaseRequest`
- `type OwnerLookupRequest`
- `type OwnerResolution`
- `type ShardResolver interface`
- `type PlacementStore interface`
- `type LeaseStore interface`
- `type OwnerLookup interface`
- `type Runtime interface`
- `type DeterministicShardResolver`
- `func NewDeterministicShardResolver(shardCount uint32) (*DeterministicShardResolver, error)`
- `type StaticLocalNode`
- `type PlacementOwnerLookup`
- `func NewPlacementOwnerLookup(localNode NodeID, resolver ShardResolver, placements PlacementStore) (*PlacementOwnerLookup, error)`
- `func StorageShardID(id ShardID) storage.ShardID`
- `func StorageNodeID(id NodeID) storage.NodeID`
- `func ParseStorageShardID(id storage.ShardID) (ShardID, error)`
- `func ParseStorageNodeID(id storage.NodeID) (NodeID, error)`
- `func LeaseFromStorageRecord(record *storage.LeaseRecord) (*Lease, error)`
- `type StorageOwnerLookup`
- `func NewStorageOwnerLookup(localNode NodeID, resolver ShardResolver, placements storage.PlacementStore, leases storage.LeaseStore) (*StorageOwnerLookup, error)`
- `type StorageLeaseStore`
- `func NewStorageLeaseStore(store storage.LeaseStore) (*StorageLeaseStore, error)`

Escopo atual:

- resolução determinística `DocumentKey -> shard`;
- tipos estáveis para placement, lease e owner lookup;
- lookup de owner em cima de `ShardResolver + PlacementStore`, ainda sem transporte entre nós;
- lookup storage-backed combinando `documento -> shard` e `shard -> lease owner` sobre `pkg/storage`;
- adapter de lease do control plane sobre `storage.LeaseStore` para acquire/renew/release sem reimplementar backend;
- separação explícita entre identidade local (`StaticLocalNode`), hashing de shard (`DeterministicShardResolver`) e consulta de owner (`PlacementOwnerLookup`/`StorageOwnerLookup`).

Esse pacote é scaffolding de control plane: ele ainda não faz eleição,
rebalanceamento, renovação de lease ou cutover, mas fixa os contratos que o
runtime distribuído vai consumir para decidir localidade, roteamento e fencing.

## Stores disponíveis

### `pkg/storage/memory`

- Implementação `memory.Store` das interfaces `SnapshotStore` e `DistributedStore`.
- `func New() *Store`
- `func (s *Store) SaveSnapshot(ctx context.Context, key storage.DocumentKey, snapshot *yjsbridge.PersistedSnapshot) (*storage.SnapshotRecord, error)`
- `func (s *Store) LoadSnapshot(ctx context.Context, key storage.DocumentKey) (*storage.SnapshotRecord, error)`
- `func (s *Store) AppendUpdate(ctx context.Context, key storage.DocumentKey, update []byte) (*storage.UpdateLogRecord, error)`
- `func (s *Store) ListUpdates(ctx context.Context, key storage.DocumentKey, after storage.UpdateOffset, limit int) ([]*storage.UpdateLogRecord, error)`
- `func (s *Store) TrimUpdates(ctx context.Context, key storage.DocumentKey, through storage.UpdateOffset) error`
- `func (s *Store) SavePlacement(ctx context.Context, placement storage.PlacementRecord) (*storage.PlacementRecord, error)`
- `func (s *Store) LoadPlacement(ctx context.Context, key storage.DocumentKey) (*storage.PlacementRecord, error)`
- `func (s *Store) SaveLease(ctx context.Context, lease storage.LeaseRecord) (*storage.LeaseRecord, error)`
- `func (s *Store) LoadLease(ctx context.Context, shardID storage.ShardID) (*storage.LeaseRecord, error)`
- `func (s *Store) ReleaseLease(ctx context.Context, shardID storage.ShardID, token string) error`

Uso recomendado para:

- testes locais,
- dev/ci sem dependência externa,
- cache simples de documentos.

### `pkg/storage/postgres`

- Implementação `postgres.Store` das interfaces `SnapshotStore` e `DistributedStore`.
- `type Config struct { ConnectionString, Schema, ApplicationName string; MinConns, MaxConns int32; SkipMigrations bool }`
- `func New(ctx context.Context, cfg Config) (*Store, error)` com automigration por padrão.
- `func (s *Store) SaveSnapshot(ctx context.Context, key storage.DocumentKey, snapshot *yjsbridge.PersistedSnapshot) (*storage.SnapshotRecord, error)`
- `func (s *Store) LoadSnapshot(ctx context.Context, key storage.DocumentKey) (*storage.SnapshotRecord, error)`
- `func (s *Store) AppendUpdate(ctx context.Context, key storage.DocumentKey, update []byte) (*storage.UpdateLogRecord, error)`
- `func (s *Store) ListUpdates(ctx context.Context, key storage.DocumentKey, after storage.UpdateOffset, limit int) ([]*storage.UpdateLogRecord, error)`
- `func (s *Store) TrimUpdates(ctx context.Context, key storage.DocumentKey, through storage.UpdateOffset) error`
- `func (s *Store) SavePlacement(ctx context.Context, placement storage.PlacementRecord) (*storage.PlacementRecord, error)`
- `func (s *Store) LoadPlacement(ctx context.Context, key storage.DocumentKey) (*storage.PlacementRecord, error)`
- `func (s *Store) SaveLease(ctx context.Context, lease storage.LeaseRecord) (*storage.LeaseRecord, error)`
- `func (s *Store) LoadLease(ctx context.Context, shardID storage.ShardID) (*storage.LeaseRecord, error)`
- `func (s *Store) ReleaseLease(ctx context.Context, shardID storage.ShardID, token string) error`
- `func (s *Store) Close()`
- `func (s *Store) AutoMigrate(ctx context.Context) error`

Observações de configuração:

- `Schema` padrão: `yjs_bridge`.
- `ApplicationName` padrão: `yjs-go-bridge`.
- `SkipMigrations` desativa automigration inicial (útil quando a migration é gerenciada por outro processo).
- `New` valida conexão e limites (`MinConns`, `MaxConns`) antes de abrir o pool.
- `Close` deve sempre ser chamado no shutdown da aplicação.

## Automigration (Postgres)

A store PostgreSQL aplica migrations versionadas na inicialização quando `SkipMigrations` está `false` (padrão).

Fluxo:

1. `New` valida `Config` e abre `pgxpool`.
2. A store chama `AutoMigrate(ctx)` e:
   1. cria/adiciona schema,
   2. cria tabela de versionamento (`schema_migrations`),
   3. aplica migrations pendentes em ordem,
   4. grava versão aplicada.
3. Caso queira controle externo, inicialize com `SkipMigrations: true` e execute `AutoMigrate` num ponto separado.

Comportamento de concorrência:

- `AutoMigrate` usa advisory lock interno para serializar múltiplas aplicações simultâneas.

## Exemplos simples

### Snapshot + store em memória

```go
package main

import (
	"context"
	"fmt"
	"log"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/yjsbridge"
)

func main() {
	ctx := context.Background()
	store := memory.New()

	key := storage.DocumentKey{
		Namespace:  "team-a",
		DocumentID: "doc-1",
	}

	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates()
	if err != nil {
		log.Fatalf("criando snapshot: %v", err)
	}

	saved, err := store.SaveSnapshot(ctx, key, snapshot)
	if err != nil {
		log.Fatalf("salvando snapshot: %v", err)
	}

	loaded, err := store.LoadSnapshot(ctx, key)
	if err != nil {
		log.Fatalf("carregando snapshot: %v", err)
	}

	payload, err := yjsbridge.EncodePersistedSnapshotV1(loaded.Snapshot)
	if err != nil {
		log.Fatalf("codificando snapshot: %v", err)
	}

	restored, err := yjsbridge.DecodePersistedSnapshotV1(payload)
	if err != nil {
		log.Fatalf("decodificando snapshot: %v", err)
	}

	fmt.Println("stored_at:", saved.StoredAt.UTC())
	fmt.Println("restored_empty:", restored.IsEmpty())
}
```

### Snapshot + Postgres (automigration ativa)

```go
package main

import (
	"context"
	"log"
	"os"
	"strings"

	"yjs-go-bridge/pkg/storage"
	pgstore "yjs-go-bridge/pkg/storage/postgres"
	"yjs-go-bridge/pkg/yjsbridge"
)

func main() {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("YJSBRIDGE_POSTGRES_DSN"))
	if dsn == "" {
		log.Fatal("defina YJSBRIDGE_POSTGRES_DSN")
	}

	store, err := pgstore.New(ctx, pgstore.Config{
		ConnectionString: dsn,
		Schema:           "yjs_bridge_example",
	})
	if err != nil {
		log.Fatalf("abrindo store postgres: %v", err)
	}
	defer store.Close()

	key := storage.DocumentKey{
		Namespace:  "team-a",
		DocumentID: "doc-1",
	}

	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates()
	if err != nil {
		log.Fatalf("criando snapshot: %v", err)
	}

	if _, err := store.SaveSnapshot(ctx, key, snapshot); err != nil {
		log.Fatalf("salvando snapshot: %v", err)
	}

	loaded, err := store.LoadSnapshot(ctx, key)
	if err != nil {
		log.Fatalf("carregando snapshot: %v", err)
	}

	log.Printf("namespace=%s document=%s stored_at=%s", loaded.Key.Namespace, loaded.Key.DocumentID, loaded.StoredAt.UTC())
}
```

### Controlando automigration manualmente

```go
store, err := pgstore.New(ctx, pgstore.Config{
	ConnectionString: dsn,
	Schema:           "tenant_app",
	SkipMigrations:   true,
})
if err != nil {
	log.Fatalf("abrindo store: %v", err)
}
if err := store.AutoMigrate(ctx); err != nil {
	log.Fatalf("executando migrations: %v", err)
}
```

## Observações de uso

- `SaveSnapshot` e `LoadSnapshot` são canceláveis por `context.Context`.
- A leitura retorna cópias; alterar o `Snapshot` retornado não altera dados persistidos.
- Para produção, prefira `pkg/storage/postgres`; para testes e desenvolvimento simples, `pkg/storage/memory`.
- Erros de chave inválida e snapshot nulo seguem contratos de `pkg/storage`.

## Como executar

```bash
go test ./...
```

Exemplos:

```bash
go run ./examples/memory
go run ./examples/chi-memory
YJSBRIDGE_POSTGRES_DSN='postgres://user:pass@localhost:5432/dbname?sslmode=disable' go run ./examples/chi-postgres
go run ./examples/echo-memory
YJSBRIDGE_POSTGRES_DSN='postgres://user:pass@localhost:5432/dbname?sslmode=disable' go run ./examples/echo-postgres
go run ./examples/gin-memory
YJSBRIDGE_POSTGRES_DSN='postgres://user:pass@localhost:5432/dbname?sslmode=disable' go run ./examples/gin-postgres
YJSBRIDGE_POSTGRES_DSN='postgres://user:pass@localhost:5432/dbname?sslmode=disable' go run ./examples/gin-react-tailwind-postgres
go run ./examples/http-memory
YJSBRIDGE_POSTGRES_DSN='postgres://user:pass@localhost:5432/dbname?sslmode=disable' go run ./examples/http-postgres
go run ./examples/merge-state-contentids
YJSBRIDGE_POSTGRES_DSN='postgres://user:pass@localhost:5432/dbname?sslmode=disable' go run ./examples/postgres
go run ./examples/protocol-awareness
go run ./examples/protocol-session
go run ./examples/provider-memory
```

O example `examples/gin-react-tailwind-postgres` adiciona um demo full-stack com
frontend `vite` + `react` + `tailwindcss`, backend `gin`, WebSocket em
`pkg/yhttp/gin` e persistência PostgreSQL, incluindo editor colaborativo com
sync de conteúdo e awareness.

O example `examples/provider-memory` é a referência local mais próxima do fluxo
de recovery planejado: ele publica update e awareness, persiste snapshot
explicitamente, reabre o documento em um novo provider e demonstra restore do
documento sem reidratar presença efêmera. Na fase distribuída, esse restore por
snapshot será complementado pelo replay do tail do `update log`.

Smoke tests opt-in com Postgres efêmero em Docker:

```bash
YJSBRIDGE_RUN_DOCKER_SMOKE=1 go test -v ./integration
```

Matriz opt-in de performance por framework/backend:

```bash
YJSBRIDGE_RUN_PERF_MATRIX=1 go test -run TestWebSocketPerformanceMatrix -v ./integration
```

Esse pacote sobe um container PostgreSQL efêmero, inicializa um servidor HTTP/WebSocket em memória, abre duas conexões WebSocket e cobre:

- fluxo funcional de sync e awareness;
- persistência com restart do servidor;
- smoke de performance com throughput básico de updates.

A matriz de performance reaproveita o mesmo harness para `net/http`, `gin`,
`echo` e `chi`, comparando stores em memória e PostgreSQL. O log inclui
throughput, latência média, `p50`, `p95`, `p99` e tempo de restore após restart.

## Status atual e limite de escopo

- Persistência de snapshots V1 está operacional com stores em memória e PostgreSQL.
- A superfície pública de update agora cobre formato, merge/diff, state vector e content ids para V1.
- V2 já é detectado, mas ainda não é suportado operacionalmente nas APIs de update; `DecodeUpdate`, `MergeUpdates`, `DiffUpdate`, `StateVectorFrom*`, `Create/ContentIDsFrom*` e `IntersectUpdateWithContentIDs` retornam erro explícito (`ErrUnsupportedUpdateFormatV2`) e misturas de formato retornam `ErrMismatchedUpdateFormats`.
- Em APIs agregadas, a validação preserva índice do payload relevante no erro (inclusive após prefixes vazios), e rejeita entradas vazias misturadas a V2 conforme contrato.
- `pkg/yprotocol` e `pkg/yawareness` expõem os codecs do envelope `y-protocols`, o runtime in-process mínimo de sessão, o provider local por documento e o estado local de awareness.
- `pkg/yhttp` agora também expõe `OwnerAwareServer` para resolver owner antes do provider local, além dos hooks opcionais de observabilidade e dos subpacotes `pkg/yhttp/gin`, `pkg/yhttp/echo`, `pkg/yhttp/chi` e `pkg/yhttp/prometheus`.
- `pkg/storage`, `pkg/ycluster` e `pkg/ynodeproto` já expõem a base pública distribuída com replay/recovery, control plane mínimo e wire inter-node tipado; o próximo passo é ligar essas superfícies ao forwarding/handoff do runtime.
- Ainda não há transporte distribuído, coordenação entre nós ou replicação horizontal entre processos.
- Recovery operacional agora já cobre `snapshot + update log` via helpers públicos, stores concretos e bootstrap do `Provider`; o control plane já materializa `epoch` básico de lease, mas handoff, cutover, append log por epoch, fencing autoritativo e forwarding inter-node seguem como etapa posterior.
- A próxima fase do roadmap fecha owner único por room/documento/shard com coordenação segura de `lease/epoch/fencing`, forwarding edge->owner e failover/handoff seguros, mantendo HTTP/WS acessível em qualquer nó.

## Referências do projeto

- `AGENT.md`
- `SPEC.md`
- `TASK.md`
- `docs/`
