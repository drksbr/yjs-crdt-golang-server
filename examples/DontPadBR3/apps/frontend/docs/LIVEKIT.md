# LiveKit PoC - Setup

This project includes a small Proof-of-Concept for real-time voice chat using LiveKit.

Files added:

- `app/api/livekit/token/route.ts` - server endpoint that mints join tokens for a given `documentId`.
- `components/VoiceChat.tsx` - client UI that joins a room named after the `documentId`.
- `components/DocumentView.tsx` - small button to open the voice PoC overlay.

Environment variables required (set in `.env` / deployment env):

- `LIVEKIT_API_KEY` - LiveKit API key (server)
- `LIVEKIT_API_SECRET` - LiveKit API secret (server)
- `LIVEKIT_URL` - full LiveKit URL (e.g. `https://<your-livekit-host>`)

Notes about ports and TURN configuration:

- This PoC configures LiveKit to use signaling on TCP 7880 and media on UDP 50000–60000.
- The TURN server (coturn) relay range has been adjusted to 50000–60000 (see `turnserver.conf`) so the firewall rules are simpler and consistent with LiveKit.
- Ensure your firewall allows:
  - TCP 7880 (signaling proxy from NPM)
  - UDP 50000–60000 (LiveKit media)
  - UDP/TCP 3478 (TURN)
  - TCP 5349 (TURN TLS), if you enable it

Security notes:

- The LiveKit API key/secret in `livekit.yaml` have been generated for PoC; rotate them for production and keep them secret (use environment variables or secret manager).
- The coturn credentials are static in the PoC; prefer time-limited credentials for production.

Quick steps to run locally:

1. Install dependencies:

```bash
npm install
# or with bun
bun install
```

2. Install LiveKit (self-hosted) or sign up for LiveKit Cloud. For self-hosted see https://docs.livekit.io.

3. Set environment variables in `.env` (or in your deployment):

```
LIVEKIT_API_KEY=your_key
LIVEKIT_API_SECRET=your_secret
LIVEKIT_URL=https://your-livekit.example.com
```

4. Run the app and open a document page. Click the "Entrar com voz" button to test.

Notes:

- The PoC uses dynamic imports so the client only loads `livekit-client` when the user attempts to join.
- The server route dynamically imports `livekit-server-sdk` to mint tokens.
- This is a minimal PoC; for production you'll want to improve error handling, add auth integration (tie identity to your users), and secure token issuance.
