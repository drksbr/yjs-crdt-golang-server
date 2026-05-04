# Known gaps by function

## `internal/yupdate` / `pkg/yjsbridge`

- `DecodeV2`/reader V2: implemented for the fixture-backed subset using upstream Yjs fixtures across text, Unicode text, map, nested `Any` object/array values, binary, embed, format, nested type, XML attributes/text, subdoc, delete set and multi-client updates.
- `EncodeV2`/writer V2: implemented for the same fixture-backed subset and validated byte-for-byte against upstream single-update and `Y.mergeUpdatesV2` fixtures.
- `ConvertUpdateToV1`/`ConvertUpdatesToV1` for valid V2 payloads: implemented as V2 reader -> internal model -> canonical V1 encode.
- `ConvertUpdateToV2`/`ConvertUpdatesToV2`/`MergeUpdatesV2`/`DiffUpdateV2`/`IntersectUpdateWithContentIDsV2`: implemented as explicit V2 output APIs; existing non-`V2` APIs keep V1-compatible output for callers that have not opted into V2 egress.
- `StateVectorFromUpdate`/`CreateContentIDsFromUpdate`/`SnapshotFromUpdate`/`MergeUpdates`/`DiffUpdate`/`IntersectUpdateWithContentIDs` accept valid V2 inputs through the shared decoded model; V1 output APIs derive compatibility bytes only at the boundary.
- V2 multi-update coverage now includes upstream `Y.mergeUpdatesV2` fixtures for text append across clients, delete-after-insert, formatting over deleted text, independent/overwritten map sets, nested-map child writes, XML element/text updates, array delete ranges and subdoc follow-up map updates.
- Persisted snapshots now keep `UpdateV2` as canonical and materialize `UpdateV1` for compatibility. Memory/Postgres stores preserve snapshot V2 and fall back to V1 for older rows.
- Update logs can preserve canonical V2 through `UpdateLogStoreV2`/`AuthoritativeUpdateLogStoreV2`; V1 append APIs and rows remain supported for compatibility and replay fallback.
- V2 malformed-input coverage includes side-channel truncation, truncated/invalid RLE and varint side-channel payloads, inconsistent string table lengths, delete-set overflow, invalid `parentInfo`, oversized top-level and nested `Any` collections and unused values in consumed side-channel encoders (`client`, `leftClock`, `rightClock`, `info`, `strings`, `parentInfo`, `typeRef`, `lengths`, `keyClock`). XML element/hook names now consume the key-cache path and `keyClock` is drained strictly.
- V1 structural coverage: current regression suite covers multi-client `merge -> diff -> intersect`, JSON/Any/String slicing, synthetic skips, delete sets and lazy writer round trips. V1 parsing also rejects invalid `parentInfo` and oversized JSON/Any collection lengths before large allocations. Keep adding regressions for every new malformed or upstream-divergent fixture.

## `pkg/yawareness`

- Wire format, state manager, clocks, tombstones, local protection, deltas and local field operators are implemented.
- Provider-level event fanout remains intentionally outside `pkg/yawareness`; `pkg/yprotocol` decides broadcast/direct routing.

## `pkg/yprotocol`

- Sync update/step2 payloads can enter as valid V2 and are normalized into the room/session V2 canonical state before mutation, broadcast, update-log append or snapshot persistence.
- V2 sync output is available through explicit helpers (`EncodeSyncStep2FromUpdatesV2`, `EncodeProtocolSyncStep2FromUpdatesV2`, `EncodeSyncUpdateV2`, `EncodeProtocolSyncUpdateV2`), `SessionHandleOptions{SyncOutputFormat: UpdateFormatV2}` and provider connection handle options. Default client egress remains V1-compatible unless the caller negotiates V2.

## `pkg/storage`

- `PlacementListStore` exists for memory/Postgres and feeds rebalance control loops.
- Cluster membership/health is intentionally outside the storage contract. `pkg/ycluster` now exposes health-driven target selection without requiring storage co-location.

## `pkg/ycluster`

- Document-level rebalance, planner/executor, periodic controller and placement-backed document source exist.
- Dynamic target selection from membership/health is implemented through `HealthyRebalanceTargetSource` and `RebalanceControllerConfig.TargetSource`.
- Controller decisions can trigger immediate edge authority revalidation through `yhttp.NewRebalanceAuthorityRevalidationCallback`.

## `pkg/yhttp`

- Owner-aware routing, relay, inter-node handshake auth seam, epoch validation and retryable cutover are implemented.
- Local write-time authority loss now signals the same handoff/rebind path used by periodic/forced revalidation, and stale remote-owner epoch rebind retries are bounded by timeout.
- Automatic cutover/rebind can be initiated by wiring `RebalanceControllerConfig.OnResult` to the yhttp rebalance authority revalidation callback.
- Client sync payloads follow the `pkg/yprotocol` contract: valid V2 input may be accepted at the edge, downstream owner state is V2 canonical, V1 compatibility bytes are derived at legacy boundaries, and WebSocket sync egress emits V2 only when `Request.SyncOutputFormat` is explicitly set to `UpdateFormatV2`. `SyncOutputFormatFromHTTPRequest` handles the reference query/header/subprotocol negotiation. Inter-node sync can use dedicated negotiated V2 message types in both owner->edge and edge->owner directions.
- HTTP/WebSocket security is deliberately opt-in: `Authenticator`, `Authorizer`, `RateLimiter`, `QuotaLimiter`, `OriginPolicy` and `RequestRedactor` run around request resolution and before local provider open or remote-owner lookup/forwarding where applicable.
- Reference security helpers exist for local wiring: `BearerTokenAuthenticator`, `TenantAuthorizer`, `FixedWindowRateLimiter`, `RateLimitByPrincipalOrRemoteAddr`, `RateLimitByTenant`, `RateLimitByDocument`, `LocalQuotaLimiter`, `StaticOriginPolicy` and `HashingRequestRedactor`.
- `RemoteOwnerEndpoint` exposes `RemoteOwnerAuthenticator`, validates handshake route fields/epoch and has bearer/HMAC helpers for node auth; HMAC supports `key_id`, multiple accepted secrets and nonce replay protection, while cluster-wide token distribution and mTLS remain deployment policy.

## Observability and operations

- Node-level oracle dashboards and Prometheus adapters exist.
- Reference Prometheus SLO recording/alert rules now exist for `env`/`region`/`tenant`/`deployment_role`; real deployments still need threshold tuning against traffic profile and error budget.

## Security

- HTTP/WebSocket auth is now opt-in through `pkg/yhttp.Authenticator`/`Authorizer`, including reference bearer-token authentication and tenant boundary enforcement against `DocumentKey.Namespace`.
- HTTP/WebSocket rate limiting is opt-in through `pkg/yhttp.RateLimiter`; the in-memory fixed-window implementation is a local reference, not a distributed quota system.
- HTTP/WebSocket quotas are opt-in through `pkg/yhttp.QuotaLimiter`; `LocalQuotaLimiter` covers local connection counts and per-connection frame/byte budgets, but distributed enforcement remains open.
- HTTP/WebSocket Origin/CORS policy is opt-in through `pkg/yhttp.OriginPolicy`; `StaticOriginPolicy` provides exact allowlists, preflight handling and method/header validation, and the HTTP/WebSocket server derives compatible `websocket.AcceptOptions.OriginPatterns` when possible.
- Metrics/error redaction is opt-in through `pkg/yhttp.RequestRedactor`; `HashingRequestRedactor` hashes request document ids, namespaces, connection ids and principals before `Metrics`/`OnError`.
- Inter-node handshake auth seam exists through `RemoteOwnerEndpointConfig.Authenticate`; reference bearer and HMAC `key_id`/timestamp/nonce helpers exist, but production deployments still need mandatory node identity policy, mTLS where required, scoped credentials and secure secret distribution.
- Production fail-closed checks exist through `ValidateProductionServerConfig`, `ValidateProductionOwnerAwareConfig` and `ValidateProductionRemoteOwnerEndpointConfig`; applications still need to call them during boot.
- Remaining quota gaps: distributed counters, owner lookup/forwarding budgets, storage/replay cost budgets and audit trails.
- Redaction rollout remains open for external logs, error payloads and operational dashboards; document ids, namespaces, principals, tokens and connection ids should not be emitted raw in production telemetry.
- Remaining public multi-tenant hardening: distributed quotas, production-grade key management, mandatory inter-node identity, operational secret rotation and audited defaults that fail closed when security hooks are absent.
