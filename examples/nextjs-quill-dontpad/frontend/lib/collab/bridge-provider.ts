import diff from "fast-diff"
import * as decoding from "lib0/decoding"
import * as encoding from "lib0/encoding"
import { Awareness } from "y-protocols/awareness.js"
import * as awarenessProtocol from "y-protocols/awareness.js"
import * as syncProtocol from "y-protocols/sync.js"
import * as Y from "yjs"

const messageSync = 0
const messageAwareness = 1
const messageAuth = 2
const messageQueryAwareness = 3

export type ProviderStatus =
  | "connecting"
  | "connected"
  | "disconnected"
  | "error"

export type UserProfile = {
  name: string
  color: string
}

export type UserSelection = {
  start: number
  end: number
}

export type UserPointer = {
  x: number
  y: number
}

export type AttachmentRecord = {
  id: string
  name: string
  size: number
  contentType: string
  url: string
  uploadedAt: string
  uploadedBy: string
}

export type PresenceSnapshot = {
  clientID: number
  name: string
  color: string
  selection: UserSelection | null
  pointer: UserPointer | null
  isLocal: boolean
}

type ProviderAwarenessState = {
  user?: UserProfile
  selection?: UserSelection | null
  pointer?: UserPointer | null
}

type ProviderOptions = {
  room: string
  profile: UserProfile
  wsURL: string
  persistOnClose: boolean
  reconnectDelayMs?: number
}

export class BridgeProvider {
  readonly doc: Y.Doc
  readonly awareness: Awareness

  private readonly sharedText: Y.Text
  private readonly attachments: Y.Array<AttachmentRecord>
  private readonly wsURL: string
  private readonly persistOnClose: boolean
  private readonly reconnectDelayMs: number
  private readonly remoteSyncOrigin = Symbol("remote-sync")
  private readonly remoteAwarenessOrigin = Symbol("remote-awareness")

  private socket: WebSocket | null = null
  private status: ProviderStatus = "connecting"
  private reconnectTimer: number | null = null
  private destroyed = false
  private room: string
  private profile: UserProfile
  private selection: UserSelection | null = null
  private pointer: UserPointer | null = null
  private readonly statusListeners = new Set<(status: ProviderStatus) => void>()

  constructor(options: ProviderOptions) {
    this.doc = new Y.Doc()
    this.awareness = new Awareness(this.doc)
    this.sharedText = this.doc.getText("content")
    this.attachments = this.doc.getArray<AttachmentRecord>("attachments")
    this.wsURL = options.wsURL
    this.persistOnClose = options.persistOnClose
    this.reconnectDelayMs = options.reconnectDelayMs ?? 1200
    this.room = options.room
    this.profile = options.profile

    this.publishLocalAwareness()
    this.doc.on("update", this.handleDocumentUpdate)
    this.awareness.on("update", this.handleAwarenessUpdate)
  }

  connect() {
    this.destroyed = false
    this.clearReconnectTimer()
    this.setStatus("connecting")

    const socket = new WebSocket(this.buildSocketURL())
    socket.binaryType = "arraybuffer"
    socket.addEventListener("open", this.handleOpen)
    socket.addEventListener("message", this.handleMessage)
    socket.addEventListener("close", this.handleClose)
    socket.addEventListener("error", this.handleError)
    this.socket = socket
  }

  destroy() {
    this.destroyed = true
    this.clearReconnectTimer()
    this.doc.off("update", this.handleDocumentUpdate)
    this.awareness.off("update", this.handleAwarenessUpdate)

    if (this.socket?.readyState === WebSocket.OPEN) {
      this.awareness.setLocalState(null)
    }

    this.awareness.destroy()
    if (this.socket) {
      this.socket.close()
      this.socket = null
    }
  }

  getText() {
    return this.sharedText
  }

  getAttachmentsCollection() {
    return this.attachments
  }

  getAttachments() {
    return this.attachments.toArray().map((value) => ({
      id: String(value.id ?? ""),
      name: String(value.name ?? "arquivo"),
      size: Number(value.size ?? 0),
      contentType: String(value.contentType ?? "application/octet-stream"),
      url: String(value.url ?? ""),
      uploadedAt: String(value.uploadedAt ?? new Date().toISOString()),
      uploadedBy: String(value.uploadedBy ?? "alguem"),
    }))
  }

  addAttachment(record: AttachmentRecord) {
    this.attachments.push([record])
  }

  removeAttachment(id: string) {
    const index = this.attachments
      .toArray()
      .findIndex((record) => String(record.id ?? "") === id)
    if (index >= 0) {
      this.attachments.delete(index, 1)
    }
  }

  getPresence(): PresenceSnapshot[] {
    return Array.from(this.awareness.getStates().entries())
      .map(([clientID, rawState]) => {
        const state = (rawState as ProviderAwarenessState | undefined) ?? {}
        return {
          clientID: Number(clientID),
          name: state.user?.name ?? `guest-${Number(clientID).toString(16)}`,
          color: state.user?.color ?? "#20283f",
          selection: state.selection ?? null,
          pointer: state.pointer ?? null,
          isLocal: Number(clientID) === this.clientID,
        }
      })
      .sort(
        (left, right) =>
          Number(right.isLocal) - Number(left.isLocal) ||
          left.name.localeCompare(right.name),
      )
  }

  setProfile(profile: UserProfile) {
    this.profile = profile
    this.publishLocalAwareness()
  }

  setSelection(selection: UserSelection | null) {
    this.selection = selection
    this.publishLocalAwareness()
  }

  setPointer(pointer: UserPointer | null) {
    this.pointer = pointer
    this.publishLocalAwareness()
  }

  subscribeStatus(listener: (status: ProviderStatus) => void) {
    this.statusListeners.add(listener)
    listener(this.status)
    return () => {
      this.statusListeners.delete(listener)
    }
  }

  private get clientID() {
    return Number(this.doc.clientID)
  }

  private buildSocketURL() {
    const url = new URL(this.wsURL)
    url.searchParams.set("doc", this.room)
    url.searchParams.set("client", String(this.clientID))
    url.searchParams.set("persist", this.persistOnClose ? "1" : "0")
    return url.toString()
  }

  private handleOpen = () => {
    this.setStatus("connected")
    this.publishLocalAwareness()
    this.sendSyncStep1()
    this.sendQueryAwareness()
    this.sendLocalAwareness()
  }

  private handleMessage = (event: MessageEvent<ArrayBuffer>) => {
    const decoder = decoding.createDecoder(new Uint8Array(event.data))
    while (decoding.hasContent(decoder)) {
      const messageType = decoding.readVarUint(decoder)
      switch (messageType) {
        case messageSync: {
          const encoder = encoding.createEncoder()
          encoding.writeVarUint(encoder, messageSync)
          syncProtocol.readSyncMessage(
            decoder,
            encoder,
            this.doc,
            this.remoteSyncOrigin,
          )
          const reply = encoding.toUint8Array(encoder)
          if (reply.length > 1) {
            this.send(reply)
          }
          break
        }
        case messageAwareness: {
          const update = decoding.readVarUint8Array(decoder)
          awarenessProtocol.applyAwarenessUpdate(
            this.awareness,
            update,
            this.remoteAwarenessOrigin,
          )
          break
        }
        case messageQueryAwareness:
          this.sendLocalAwareness()
          break
        case messageAuth:
          decoding.readVarString(decoder)
          break
        default:
          return
      }
    }
  }

  private handleClose = () => {
    this.socket = null
    if (this.destroyed) {
      return
    }
    this.setStatus("disconnected")
    this.scheduleReconnect()
  }

  private handleError = () => {
    this.setStatus("error")
  }

  private handleDocumentUpdate = (update: Uint8Array, origin: unknown) => {
    if (origin === this.remoteSyncOrigin) {
      return
    }
    const encoder = encoding.createEncoder()
    encoding.writeVarUint(encoder, messageSync)
    syncProtocol.writeUpdate(encoder, update)
    this.send(encoding.toUint8Array(encoder))
  }

  private handleAwarenessUpdate = (
    change: { added: number[]; updated: number[]; removed: number[] },
    origin: unknown,
  ) => {
    if (origin === this.remoteAwarenessOrigin) {
      return
    }
    const changedClients = change.added.concat(change.updated, change.removed)
    if (changedClients.length === 0) {
      return
    }
    const update = awarenessProtocol.encodeAwarenessUpdate(
      this.awareness,
      changedClients,
    )
    this.sendEnvelope(messageAwareness, update)
  }

  private publishLocalAwareness() {
    const nextState: ProviderAwarenessState = {
      user: this.profile,
      selection: this.selection,
      pointer: this.pointer,
    }
    this.awareness.setLocalState(nextState)
  }

  private sendSyncStep1() {
    const encoder = encoding.createEncoder()
    encoding.writeVarUint(encoder, messageSync)
    syncProtocol.writeSyncStep1(encoder, this.doc)
    this.send(encoding.toUint8Array(encoder))
  }

  private sendQueryAwareness() {
    const encoder = encoding.createEncoder()
    encoding.writeVarUint(encoder, messageQueryAwareness)
    this.send(encoding.toUint8Array(encoder))
  }

  private sendLocalAwareness() {
    const update = awarenessProtocol.encodeAwarenessUpdate(this.awareness, [
      this.clientID,
    ])
    this.sendEnvelope(messageAwareness, update)
  }

  private sendEnvelope(type: number, payload: Uint8Array) {
    const encoder = encoding.createEncoder()
    encoding.writeVarUint(encoder, type)
    encoding.writeVarUint8Array(encoder, payload)
    this.send(encoding.toUint8Array(encoder))
  }

  private send(payload: Uint8Array) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return
    }
    const buffer = new Uint8Array(payload.byteLength)
    buffer.set(payload)
    this.socket.send(buffer)
  }

  private scheduleReconnect() {
    this.clearReconnectTimer()
    this.reconnectTimer = window.setTimeout(() => {
      this.connect()
    }, this.reconnectDelayMs)
  }

  private clearReconnectTimer() {
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  private setStatus(status: ProviderStatus) {
    this.status = status
    for (const listener of this.statusListeners) {
      listener(status)
    }
  }
}

export function applyPlainTextDelta(
  text: Y.Text,
  previousValue: string,
  nextValue: string,
) {
  if (previousValue === nextValue) {
    return
  }

  let cursor = 0
  for (const [operation, fragment] of diff(previousValue, nextValue)) {
    if (operation === 0) {
      cursor += fragment.length
      continue
    }
    if (operation === -1) {
      text.delete(cursor, fragment.length)
      continue
    }
    text.insert(cursor, fragment)
    cursor += fragment.length
  }
}
