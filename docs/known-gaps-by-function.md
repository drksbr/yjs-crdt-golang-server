# Known gaps by function

## `internal/yupdate` / `pkg/yjsbridge`

- `DecodeV2`/reader V2: implemented for the fixture-backed subset using upstream Yjs fixtures across text, Unicode text, map, nested `Any` object/array values, binary, embed, format, nested type, XML attributes/text, subdoc, delete set and multi-client updates.
- `ConvertUpdateToV1`/`ConvertUpdatesToV1` for valid V2 payloads: implemented as V2 reader -> internal model -> canonical V1 encode.
- `StateVectorFromUpdate`/`CreateContentIDsFromUpdate`/`SnapshotFromUpdate`/`MergeUpdates`/`DiffUpdate`/`IntersectUpdateWithContentIDs` for valid V2 inputs now derive results through canonical V1 conversion.
- V2 multi-update coverage now includes upstream `Y.mergeUpdatesV2` fixtures for text append across clients, delete-after-insert, formatting over deleted text, independent/overwritten map sets, nested-map child writes, XML element/text updates, array delete ranges and subdoc follow-up map updates.
- V2 encoder and V2-preserving public output are not implemented; storage and protocol remain canonical V1.
- V2 malformed-input coverage includes side-channel truncation, inconsistent string table lengths, delete-set overflow, invalid `parentInfo`, oversized decoded collections and unused values in consumed side-channel encoders. `keyClock` drain validation now rejects no-key payloads and remains deferred only for XML/format key-cache paths that already consumed keys.
- V1 structural coverage: current regression suite covers multi-client `merge -> diff -> intersect`, JSON/Any/String slicing, synthetic skips, delete sets and lazy writer round trips. V1 parsing also rejects invalid `parentInfo` and oversized JSON/Any collection lengths before large allocations. Keep adding regressions for every new malformed or upstream-divergent fixture.

## `pkg/yawareness`

- Wire format, state manager, clocks, tombstones, local protection, deltas and local field operators are implemented.
- Provider-level event fanout remains intentionally outside `pkg/yawareness`; `pkg/yprotocol` decides broadcast/direct routing.

## `pkg/yprotocol`

- Sync update/step2 payloads can enter as valid V2 and are normalized to canonical V1 before room state mutation, broadcast, update-log append or snapshot persistence.
- V2-preserving sync output is intentionally not implemented; replay and late-join bootstrap remain V1 canonical.

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
- Client sync payloads follow the `pkg/yprotocol` contract: valid V2 input may be accepted at the edge, but downstream owner state, inter-node `UpdateV1` messages and responses remain V1 canonical.

## Observability and operations

- Node-level oracle dashboards and Prometheus adapters exist.
- Reference Prometheus SLO recording/alert rules now exist for `env`/`region`/`tenant`/`deployment_role`; real deployments still need threshold tuning against traffic profile and error budget.

## Security

- Auth seams exist for HTTP/inter-node integration.
- Public multi-tenant hardening remains a separate phase after the current roadmap closure.
