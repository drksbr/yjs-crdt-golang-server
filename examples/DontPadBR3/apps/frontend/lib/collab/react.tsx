"use client";

import * as decoding from "lib0/decoding";
import * as encoding from "lib0/encoding";
import { useEffect, useMemo, useRef, useState, createContext, useContext } from "react";
import * as awarenessProtocol from "y-protocols/awareness.js";
import { Awareness } from "y-protocols/awareness.js";
import * as Y from "yjs";

const messageSync = 0;
const messageAwareness = 1;
const messageAuth = 2;
const messageQueryAwareness = 3;

const messageYjsSyncStep1 = 0;
const messageYjsSyncStep2 = 1;
const messageYjsUpdate = 2;

type LocalProviderStatus =
  | "connecting"
  | "handshaking"
  | "connected"
  | "disconnected"
  | "error";

type ConnectionStatus = LocalProviderStatus | "offline";

type DocumentToken = {
  documentId: string;
  canEdit: boolean;
  persist: boolean;
  wsBaseUrl: string;
  token: string;
};

type YDocProviderProps = {
  docId: string;
  authEndpoint: () => Promise<unknown>;
  showDebuggerLink?: boolean;
  warnOnClose?: boolean;
  children: React.ReactNode;
};

type ContextValue = {
  doc: Y.Doc;
  awareness: Awareness;
  status: LocalProviderStatus;
  hasLocalChanges: boolean;
  canEdit: boolean;
};

type ProviderSnapshot = {
  status: LocalProviderStatus;
  hasLocalChanges: boolean;
  canEdit: boolean;
};

class RealtimeDocProvider {
  readonly doc: Y.Doc;
  readonly awareness: Awareness;

  private readonly authResolver: () => Promise<unknown>;
  private readonly room: string;
  private readonly remoteSyncOrigin = Symbol("remote-sync");
  private readonly remoteAwarenessOrigin = Symbol("remote-awareness");
  private readonly listeners = new Set<() => void>();

  private socket: WebSocket | null = null;
  private reconnectTimer: number | null = null;
  private connectSeq = 0;
  private destroyed = false;
  private status: LocalProviderStatus = "connecting";
  private hasLocalChanges = false;
  private canEdit = true;
  private persistOnClose = true;
  private wsBaseURL = "";
  private wsToken = "";
  private pendingSyncUpdates: Uint8Array[] = [];

  constructor(room: string, authResolver: () => Promise<unknown>) {
    this.doc = new Y.Doc();
    this.awareness = new Awareness(this.doc);
    this.room = room;
    this.authResolver = authResolver;

    this.doc.on("updateV2", this.handleDocumentUpdateV2);
    this.awareness.on("update", this.handleAwarenessUpdate);
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  snapshot(): ProviderSnapshot {
    return {
      status: this.status,
      hasLocalChanges: this.hasLocalChanges,
      canEdit: this.canEdit,
    };
  }

  async connect(): Promise<void> {
    if (this.destroyed) {
      return;
    }
    const seq = ++this.connectSeq;
    this.clearReconnectTimer();
    this.setStatus("connecting");
    try {
      const tokenPayload = await this.authResolver();
      if (this.destroyed || seq !== this.connectSeq) {
        return;
      }
      const token = this.parseTokenPayload(tokenPayload);
      if (this.destroyed || seq !== this.connectSeq) {
        return;
      }
      this.canEdit = token.canEdit;
      this.persistOnClose = token.persist;
      this.wsBaseURL = token.wsBaseUrl;
      this.wsToken = token.token;
      this.emitChange();
      this.openSocket(seq);
    } catch {
      if (this.destroyed || seq !== this.connectSeq) {
        return;
      }
      this.setStatus("error");
      this.scheduleReconnect();
    }
  }

  destroy() {
    this.destroyed = true;
    this.connectSeq++;
    this.clearReconnectTimer();
    this.pendingSyncUpdates = [];
    this.doc.off("updateV2", this.handleDocumentUpdateV2);
    this.awareness.off("update", this.handleAwarenessUpdate);
    this.awareness.destroy();
    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }
  }

  private emitChange() {
    this.listeners.forEach((listener) => {
      listener();
    });
  }

  private setStatus(next: LocalProviderStatus) {
    if (this.status === next) return;
    this.status = next;
    this.emitChange();
  }

  private setLocalChanges(next: boolean) {
    if (this.hasLocalChanges === next) return;
    this.hasLocalChanges = next;
    this.emitChange();
  }

  private openSocket(seq: number) {
    if (this.destroyed || seq !== this.connectSeq) {
      return;
    }
    const wsURL = this.buildSocketURL();
    this.setStatus("connecting");

    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }

    const socket = new WebSocket(wsURL);
    socket.binaryType = "arraybuffer";
    socket.addEventListener("open", this.handleOpen);
    socket.addEventListener("message", this.handleMessage);
    socket.addEventListener("close", this.handleClose);
    socket.addEventListener("error", this.handleError);
    this.socket = socket;
  }

  private parseTokenPayload(payload: unknown): DocumentToken {
    const raw = (payload || {}) as Partial<DocumentToken>;
    const wsBaseUrl = String(raw.wsBaseUrl || "");
    const token = String(raw.token || "");
    if (!wsBaseUrl || !token) {
      throw new Error("invalid token payload");
    }
    return {
      documentId: String(raw.documentId || this.room),
      canEdit: raw.canEdit !== false,
      persist: raw.persist !== false,
      wsBaseUrl,
      token,
    };
  }

  private buildSocketURL() {
    const url = new URL(this.wsBaseURL);
    url.searchParams.set("doc", this.room);
    url.searchParams.set("client", String(this.clientID));
    url.searchParams.set("persist", this.persistOnClose ? "1" : "0");
    url.searchParams.set("sync", "v2");
    url.searchParams.set("token", this.wsToken);
    return url.toString();
  }

  private get clientID() {
    return Number(this.doc.clientID);
  }

  private handleOpen = () => {
    if (this.destroyed) {
      this.socket?.close();
      return;
    }
    this.setStatus("handshaking");
    this.sendSyncStep1();
    this.sendQueryAwareness();
    this.sendLocalAwareness();
    this.flushPendingSyncUpdates();
    this.setStatus("connected");
  };

  private handleMessage = (event: MessageEvent<ArrayBuffer>) => {
    if (this.destroyed) {
      return;
    }
    const decoder = decoding.createDecoder(new Uint8Array(event.data));
    while (decoding.hasContent(decoder)) {
      const messageType = decoding.readVarUint(decoder);
      switch (messageType) {
        case messageSync: {
          const encoder = encoding.createEncoder();
          encoding.writeVarUint(encoder, messageSync);
          this.readSyncMessageV2(decoder, encoder);
          const reply = encoding.toUint8Array(encoder);
          if (reply.length > 1) {
            this.send(reply);
          }
          break;
        }
        case messageAwareness: {
          const update = decoding.readVarUint8Array(decoder);
          awarenessProtocol.applyAwarenessUpdate(
            this.awareness,
            update,
            this.remoteAwarenessOrigin,
          );
          break;
        }
        case messageQueryAwareness:
          this.sendLocalAwareness();
          break;
        case messageAuth:
          decoding.readVarString(decoder);
          break;
        default:
          return;
      }
    }
  };

  private handleClose = () => {
    this.socket = null;
    if (this.destroyed) {
      return;
    }
    this.setStatus("disconnected");
    this.scheduleReconnect();
  };

  private handleError = () => {
    if (this.destroyed) {
      return;
    }
    this.setStatus("error");
  };

  private handleDocumentUpdateV2 = (update: Uint8Array, origin: unknown) => {
    if (origin === this.remoteSyncOrigin) {
      this.setLocalChanges(false);
      return;
    }
    if (!this.canEdit) {
      return;
    }
    this.setLocalChanges(true);
    if (!this.isSocketOpen()) {
      this.queuePendingSyncUpdate(update);
      window.setTimeout(() => this.setLocalChanges(false), 250);
      return;
    }
    this.sendSyncUpdate(update);
    window.setTimeout(() => this.setLocalChanges(false), 250);
  };

  private sendSyncUpdate(update: Uint8Array) {
    const encoder = encoding.createEncoder();
    encoding.writeVarUint(encoder, messageSync);
    encoding.writeVarUint(encoder, messageYjsUpdate);
    encoding.writeVarUint8Array(encoder, update);
    this.send(encoding.toUint8Array(encoder));
  }

  private queuePendingSyncUpdate(update: Uint8Array) {
    const copy = new Uint8Array(update.byteLength);
    copy.set(update);
    this.pendingSyncUpdates.push(copy);
    if (this.pendingSyncUpdates.length > 50) {
      this.pendingSyncUpdates = [Y.mergeUpdatesV2(this.pendingSyncUpdates)];
    }
  }

  private flushPendingSyncUpdates() {
    if (!this.isSocketOpen() || this.pendingSyncUpdates.length === 0 || !this.canEdit) {
      return;
    }
    const updates = this.pendingSyncUpdates;
    this.pendingSyncUpdates = [];
    for (const update of updates) {
      this.sendSyncUpdate(update);
    }
  }

  private readSyncMessageV2(
    decoder: decoding.Decoder,
    encoder: encoding.Encoder,
  ) {
    const messageType = decoding.readVarUint(decoder);
    switch (messageType) {
      case messageYjsSyncStep1: {
        const stateVector = decoding.readVarUint8Array(decoder);
        encoding.writeVarUint(encoder, messageYjsSyncStep2);
        encoding.writeVarUint8Array(
          encoder,
          Y.encodeStateAsUpdateV2(this.doc, stateVector),
        );
        return;
      }
      case messageYjsSyncStep2:
      case messageYjsUpdate: {
        const update = decoding.readVarUint8Array(decoder);
        Y.applyUpdateV2(this.doc, update, this.remoteSyncOrigin);
        return;
      }
      default:
        throw new Error(`Unknown Yjs sync message type: ${messageType}`);
    }
  }

  private handleAwarenessUpdate = (
    change: { added: number[]; updated: number[]; removed: number[] },
    origin: unknown,
  ) => {
    if (origin === this.remoteAwarenessOrigin) {
      this.emitChange();
      return;
    }
    const changedClients = change.added.concat(change.updated, change.removed);
    if (changedClients.length === 0) {
      return;
    }
    const update = awarenessProtocol.encodeAwarenessUpdate(
      this.awareness,
      changedClients,
    );
    this.sendEnvelope(messageAwareness, update);
    this.emitChange();
  };

  private sendSyncStep1() {
    const encoder = encoding.createEncoder();
    encoding.writeVarUint(encoder, messageSync);
    encoding.writeVarUint(encoder, messageYjsSyncStep1);
    encoding.writeVarUint8Array(encoder, Y.encodeStateVector(this.doc));
    this.send(encoding.toUint8Array(encoder));
  }

  private sendQueryAwareness() {
    const encoder = encoding.createEncoder();
    encoding.writeVarUint(encoder, messageQueryAwareness);
    this.send(encoding.toUint8Array(encoder));
  }

  private sendLocalAwareness() {
    const update = awarenessProtocol.encodeAwarenessUpdate(this.awareness, [
      this.clientID,
    ]);
    this.sendEnvelope(messageAwareness, update);
  }

  private sendEnvelope(type: number, payload: Uint8Array) {
    const encoder = encoding.createEncoder();
    encoding.writeVarUint(encoder, type);
    encoding.writeVarUint8Array(encoder, payload);
    this.send(encoding.toUint8Array(encoder));
  }

  private send(payload: Uint8Array) {
    const socket = this.socket;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return;
    }
    const bytes = new Uint8Array(payload.byteLength);
    bytes.set(payload);
    socket.send(bytes);
  }

  private isSocketOpen() {
    return this.socket?.readyState === WebSocket.OPEN;
  }

  private scheduleReconnect() {
    if (this.destroyed || this.reconnectTimer !== null) {
      return;
    }
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      if (this.destroyed) {
        return;
      }
      void this.connect();
    }, 1200);
  }

  private clearReconnectTimer() {
    if (this.reconnectTimer === null) return;
    window.clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
  }
}

const ProviderContext = createContext<ContextValue | null>(null);

function useProviderContext(): ContextValue {
  const context = useContext(ProviderContext);
  if (!context) {
    throw new Error("YDocProvider context missing");
  }
  return context;
}

export function YDocProvider({
  docId,
  authEndpoint,
  warnOnClose,
  children,
}: YDocProviderProps) {
  const authEndpointRef = useRef(authEndpoint);
  useEffect(() => {
    authEndpointRef.current = authEndpoint;
  }, [authEndpoint]);

  const providerRef = useRef<RealtimeDocProvider | null>(null);
  const [snapshot, setSnapshot] = useState<ProviderSnapshot>({
    status: "connecting",
    hasLocalChanges: false,
    canEdit: true,
  });

  useEffect(() => {
    const provider = new RealtimeDocProvider(docId, () => authEndpointRef.current());
    providerRef.current = provider;
    setSnapshot(provider.snapshot());
    const unsubscribe = provider.subscribe(() => setSnapshot(provider.snapshot()));
    void provider.connect();
    return () => {
      unsubscribe();
      provider.destroy();
      providerRef.current = null;
    };
  }, [docId]);

  useEffect(() => {
    if (!warnOnClose) return;
    const handler = (event: BeforeUnloadEvent) => {
      if (!snapshot.hasLocalChanges) return;
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [snapshot.hasLocalChanges, warnOnClose]);

  const value = useMemo<ContextValue | null>(() => {
    const provider = providerRef.current;
    if (!provider) return null;
    return {
      doc: provider.doc,
      awareness: provider.awareness,
      status: snapshot.status,
      hasLocalChanges: snapshot.hasLocalChanges,
      canEdit: snapshot.canEdit,
    };
  }, [snapshot]);

  if (!value) {
    return null;
  }
  return (
    <ProviderContext.Provider value={value}>{children}</ProviderContext.Provider>
  );
}

export function useYDoc() {
  return useProviderContext().doc;
}

export function useAwareness() {
  return useProviderContext().awareness;
}

export function usePresence() {
  const awareness = useAwareness();
  const [, setTick] = useState(0);
  useEffect(() => {
    if (!awareness) return;
    const handler = () => setTick((v) => v + 1);
    awareness.on("change", handler);
    awareness.on("update", handler);
    return () => {
      awareness.off("change", handler);
      awareness.off("update", handler);
    };
  }, [awareness]);

  return awareness.getStates();
}

export function useConnectionStatus(): ConnectionStatus {
  const context = useContext(ProviderContext);
  const [online, setOnline] = useState(true);

  useEffect(() => {
    setOnline(navigator.onLine);
    const handleOnline = () => setOnline(true);
    const handleOffline = () => setOnline(false);
    window.addEventListener("online", handleOnline);
    window.addEventListener("offline", handleOffline);
    return () => {
      window.removeEventListener("online", handleOnline);
      window.removeEventListener("offline", handleOffline);
    };
  }, []);

  if (!online) return "offline";
  return context?.status ?? "connecting";
}

export function useHasLocalChanges() {
  return useProviderContext().hasLocalChanges;
}

export function useYjsProvider() {
  const context = useProviderContext();
  return {
    hasLocalChanges: context.hasLocalChanges,
    canEdit: context.canEdit,
  };
}

export function useMap(name: string) {
  const doc = useYDoc();
  const [, setTick] = useState(0);

  const yMap = useMemo(() => doc.getMap(name), [doc, name]);

  useEffect(() => {
    if (!yMap) return;
    const handler = () => setTick((v) => v + 1);
    yMap.observe(handler);
    return () => yMap.unobserve(handler);
  }, [yMap]);

  return yMap;
}

export function useArray(name: string) {
  const doc = useYDoc();
  const [, setTick] = useState(0);

  const yArray = useMemo(() => doc.getArray(name), [doc, name]);

  useEffect(() => {
    if (!yArray) return;
    const handler = () => setTick((v) => v + 1);
    yArray.observe(handler);
    return () => yArray.unobserve(handler);
  }, [yArray]);

  return yArray;
}

export function useText(
  name: string,
  options?: { observe?: "deep" | "none" },
) {
  const doc = useYDoc();
  const [, setTick] = useState(0);

  const yText = useMemo(() => doc.getText(name), [doc, name]);

  useEffect(() => {
    if (!yText || options?.observe === "none") return;
    const handler = () => setTick((v) => v + 1);
    yText.observe(handler);
    return () => yText.unobserve(handler);
  }, [options?.observe, yText]);

  return yText;
}
