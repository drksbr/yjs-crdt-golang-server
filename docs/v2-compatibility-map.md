# V2 compatibility map

## Current contract

The project now treats Update V2 as the canonical in-memory/persisted snapshot
form for the V2-aware runtime paths. V1 remains the compatibility boundary for
legacy APIs, default client egress and old storage rows. Valid V1 and V2 payloads
materialize into the same internal model and can emit either V1-compatible bytes
or canonical V2 bytes depending on the boundary.

Implemented today:

- format detection distinguishes V1, V2, empty payloads and ambiguous payloads;
- aggregate validation rejects mixed V1/V2 inputs before dispatching operations;
- internal `DecodeV2` covers the selected upstream Yjs fixtures for text insert, Unicode text, map content, nested `Any` object/array values, binary, embed, format, nested type, XML attributes/text, subdoc, delete set and multi-client updates;
- internal `EncodeV2` emits Yjs-compatible V2 bytes and matches upstream fixtures byte-for-byte for the covered single-update and `Y.mergeUpdatesV2` multi-update matrix;
- `DecodeUpdate`, `ConvertUpdateToV1`, `ConvertUpdatesToV1`, persisted snapshot constructors, direct snapshot extraction, state-vector extraction, content-id extraction, merge, diff and intersect can decode valid V2 payloads without first forcing a V2->V1 aggregate conversion;
- explicit APIs `ConvertUpdateToV2`, `ConvertUpdatesToV2`, `MergeUpdatesV2`, `DiffUpdateV2` and `IntersectUpdateWithContentIDsV2` emit canonical V2 bytes while existing non-`V2` APIs keep V1-compatible output for legacy callers;
- persisted snapshots store `UpdateV2` as canonical and `UpdateV1` as compatibility; `EncodePersistedSnapshotV1`/`DecodePersistedSnapshotV1` remain available for legacy rows, and `EncodePersistedSnapshotV2`/`DecodePersistedSnapshotV2` round-trip the canonical payload;
- `pkg/storage` memory/Postgres snapshot stores preserve both `snapshot_v2` and `snapshot_v1`, replay merges canonical V2, and update logs can append/preserve V2 through `UpdateLogStoreV2`/`AuthoritativeUpdateLogStoreV2` with V1 fallback;
- `pkg/yprotocol` exposes explicit V2 sync egress helpers for `SyncStep2` and incremental `Update`, plus session/provider options to reply or broadcast with V2 when the caller has negotiated that support;
- `pkg/yprotocol.Session` and `pkg/yprotocol.Provider` keep room/session state as canonical V2, derive V1 for compatibility APIs, and append V2 to stores that implement the V2 update-log contract;
- `pkg/ynodeproto` exposes negotiated dedicated V2 message types for both owner->edge egress and edge->owner forwarding without changing the frame version or overloading `UpdateV1` fields;
- `pkg/yhttp` passes valid V2 client sync payloads into the V2-canonical provider path, while `Request.SyncOutputFormat=UpdateFormatV2` controls per-client WebSocket sync egress. When the edge requests and the owner ack confirms `FlagSupportsUpdateV2`, inter-node sync responses/updates can travel as dedicated V2 message types in both directions; `SyncOutputFormatFromHTTPRequest` is the reference parser for query/header/subprotocol negotiation;
- upstream multi-update fixtures cover text append across clients, text delete after insert, formatting over deleted text, independent and overwritten map sets, nested-map child writes, XML element/text updates, array delete ranges and subdoc follow-up map updates, including `Y.mergeUpdatesV2` payloads;
- malformed V2 inputs cover side-channel truncation, truncated/invalid RLE and varint side-channel payloads, inconsistent string table lengths, delete-set overflow, invalid `parentInfo`, oversized top-level and nested `Any` collections, and unused `keyClock` values before or after key-cache consumption;
- malformed V2 inputs also reject unused values in consumed side-channel encoders such as `client`, `leftClock`, `rightClock`, `info`, `strings`, `parentInfo`, `typeRef` and `lengths`;
- `DecodePersistedSnapshotV1` explicitly rejects V2 restore payloads and remains the legacy codec; the V2 codec is the canonical persisted snapshot path.

## Remaining gaps

- default WebSocket/client output remains V1-compatible unless a caller uses explicit V2 helpers/options or HTTP negotiation;
- legacy inter-node V1 message types remain for peers that do not negotiate `FlagSupportsUpdateV2`;
- V2 support should keep expanding with more upstream equivalence cases before removing remaining V1 compatibility fallbacks.

## Next implementation block

The next safe V2 block is expanding equivalence coverage and hardening the
remaining compatibility boundaries, not removing compatibility paths.

1. Add upstream fixtures for any newly discovered transactional edge cases before changing V2 behavior.
2. Expand equivalence tests for `MergeUpdates`, `DiffUpdate` and `IntersectUpdateWithContentIDs` beyond the current fixture matrix.
3. Keep dedicated edge->owner and owner->edge V2 inter-node message types behind negotiated `FlagSupportsUpdateV2`.
4. Keep client V2 sync egress behind explicit negotiation (`*V2` helpers/options or `Request.SyncOutputFormat`) instead of changing current V1 defaults.
5. Keep sync-runtime regressions proving that accepted V2 input is not emitted to legacy clients as V2 bytes unless an egress option requested it.

## Acceptance criteria for enabling the first V2 slice

- fixtures are generated by upstream Yjs and checked into tests with a short description of the document operation that produced each fixture.
- V2-derived snapshot/state vector/content IDs match the same operation after `ConvertUpdateToV1`.
- V2 merge/diff/intersect match the same operation after `ConvertUpdateToV1`; V1 APIs return compatibility bytes and V2 APIs return canonical V2 bytes.
- V2 output APIs emit payloads detected as V2 and convert back to the expected compatibility V1 payload.
- malformed V2 inputs fail deterministically with parser errors, not panics.
- restore-only paths such as `DecodePersistedSnapshotV1` continue returning `ErrUnsupportedUpdateFormatV2` for V2 restore payloads.
