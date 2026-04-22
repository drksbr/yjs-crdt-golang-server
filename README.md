# Yjs Go Bridge

A Go-first compatibility layer for **Yjs** and **YHub** document updates and protocol primitives.

This repository focuses on low-level correctness first: parsing Yjs binary updates, extracting state metadata, and preparing the core blocks needed for a future fully compatible real-time server.

## Why this project exists

Most Yjs server implementations are tightly coupled to Node.js runtimes.
This project aims to provide a native Go alternative for the core synchronization pieces, so backends can:

- read and reason about Yjs updates without a JavaScript runtime,
- run deterministic, testable binary compatibility logic,
- prepare for high-throughput WebSocket collaboration services and persistence pipelines in Go.

## What is implemented

Current implementation includes the foundational compatibility pipeline:

- Safe binary read primitives with explicit bounds/error handling.
- Varint encoding/decoding compatible with Yjs/lib0 varuint usage.
- Minimal Yjs type model (`ID`, `Item`, `GC`, `Skip`) and supporting structures.
- Client-scoped `ID` range set (`IdSet`) with normalization.
- V1 update decoding/encoding with:
  - delete-set parsing,
  - state vector extraction,
  - content id extraction,
  - merge helpers,
  - diff helpers,
  - content-id based intersection.
- Minimal sync protocol wire format.
- Minimal awareness protocol wire format.
- Expanded test coverage for round-trips and invalid/corner-case inputs.

The project is currently focused on building the **binary core** and is intentionally narrow in scope for compatibility verification.

## Project structure

```text
internal/
  binary/      # Safe byte readers and cursor/offset helpers
  varint/      # Varint encoding/decoding for Yjs-style integers
  ytypes/      # Core structural types and low-level models
  yidset/      # Client-scoped ID range utilities
  yupdate/     # Update V1 decode/encode and update operations
  yprotocol/   # Sync protocol wire format
  yawareness/  # Awareness wire format
```

## Goals

This project is organized in three phases:

1. **Minimum compatibility core**
   - binary utilities, update parsing, state vectors, content IDs, sync/awareness basics.
2. **Binary document operations**
   - stronger merge/diff semantics, incremental merge, and format compatibility hardening.
3. **YHub-aligned capabilities**
   - content maps, attribution, rollback, activity, and changesets.

## Status snapshot

- Go module initialized (`go 1.26`).
- Core binary stack and metadata extraction features are in place.
- Documentation and task plans are synchronized with implementation.
- This is a foundation repo and not yet a complete production-ready collaboration server.

## Current constraints

- No editor UI is included.
- No full distributed storage layer is included yet.
- No full YHub reimplementation is included yet.
- Scope is intentionally incremental to avoid compatibility drift.

## Use cases

- Building or evolving Go-based Yjs-aware storage services.
- Running backend sync/validation pipelines independent from Node.js.
- Prototyping Yjs-compatible endpoints that need deterministic update handling.

## Development notes

- Written in Go, with compatibility-focused and deterministic behavior.
- Parsing errors are explicit and handled as errors (no silent panics on malformed input).
- The codebase follows incremental compatibility: no unsupported abstractions are introduced early.

## Quick start

```bash
go test ./...
```

## Documentation model

Project behavior and priorities are documented in:

- `AGENT.md` (implementation contract)
- `SPEC.md` (technical scope and architecture)
- `TASK.md` (current technical status)
- `docs/` (implementation notes and research findings)

## Roadmap (short)

- Add stronger, benchmarkable merge/diff/intersection compatibility.
- Improve lazy-write and incremental merge behavior.
- Expand toward snapshot handling and V2 compatibility conversions.
- Add YHub-inspired advanced server features over time.

## License

Add your preferred license in `LICENSE` (currently not included).

## Contributing

Contributions are welcome. Keep pull requests compatibility-first and tightly scoped:

- prefer small functional increments,
- include tests for each binary behavior change,
- document compatibility decisions clearly.
