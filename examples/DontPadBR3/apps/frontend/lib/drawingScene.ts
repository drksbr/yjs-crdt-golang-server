const DRAWING_SCHEMA_VERSION = 2;
const DRAWING_META_KEY = "__scene_meta__";
const DRAWING_ELEMENT_PREFIX = "element:";
const DRAWING_FILE_PREFIX = "file:";

const PERSISTED_APP_STATE_KEYS = [
  "viewBackgroundColor",
  "scrollX",
  "scrollY",
  "zoom",
  "gridSize",
  "gridStep",
  "currentItemStrokeColor",
  "currentItemBackgroundColor",
  "currentItemFillStyle",
  "currentItemStrokeWidth",
  "currentItemStrokeStyle",
  "currentItemRoughness",
  "currentItemOpacity",
  "currentItemFontFamily",
  "currentItemFontSize",
  "currentItemTextAlign",
  "currentItemRoundness",
  "currentItemStartArrowhead",
  "currentItemEndArrowhead",
  "currentChartType",
  "frameRendering",
  "theme",
];

interface StoredDrawingMeta {
  schemaVersion: number;
  updatedAt: number;
  elementOrder: string[];
  appState: Record<string, unknown>;
}

export interface DrawingSceneSnapshot {
  schemaVersion: number;
  updatedAt: number;
  elements: any[];
  appState: Record<string, unknown>;
  files: Record<string, any>;
}

function safeParseJson<T>(value: unknown): T | null {
  if (typeof value !== "string") {
    return null;
  }

  try {
    return JSON.parse(value) as T;
  } catch {
    return null;
  }
}

function cloneJsonValue<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function getElementStorageKey(id: string) {
  return `${DRAWING_ELEMENT_PREFIX}${id}`;
}

function getFileStorageKey(id: string) {
  return `${DRAWING_FILE_PREFIX}${id}`;
}

function isReservedDrawingKey(key: string) {
  return (
    key === DRAWING_META_KEY ||
    key.startsWith(DRAWING_ELEMENT_PREFIX) ||
    key.startsWith(DRAWING_FILE_PREFIX)
  );
}

function isLegacyElementKey(key: string) {
  return !isReservedDrawingKey(key);
}

function sanitizePersistedAppState(appState: Record<string, any> | null | undefined) {
  const nextState: Record<string, unknown> = {};
  if (!appState) {
    return nextState;
  }

  for (const key of PERSISTED_APP_STATE_KEYS) {
    if (appState[key] !== undefined) {
      nextState[key] = cloneJsonValue(appState[key]);
    }
  }

  return nextState;
}

function readDrawingMeta(yMap: any): StoredDrawingMeta {
  const parsed = safeParseJson<Partial<StoredDrawingMeta>>(yMap?.get(DRAWING_META_KEY));

  return {
    schemaVersion:
      typeof parsed?.schemaVersion === "number"
        ? parsed.schemaVersion
        : DRAWING_SCHEMA_VERSION,
    updatedAt: typeof parsed?.updatedAt === "number" ? parsed.updatedAt : 0,
    elementOrder: Array.isArray(parsed?.elementOrder)
      ? parsed.elementOrder.filter((id): id is string => typeof id === "string")
      : [],
    appState:
      parsed?.appState && typeof parsed.appState === "object"
        ? parsed.appState
        : {},
  };
}

function sortDrawingElements(elements: any[], elementOrder: string[]) {
  const orderIndex = new Map(elementOrder.map((id, index) => [id, index]));

  return [...elements].sort((a, b) => {
    const aIndex = orderIndex.get(a.id);
    const bIndex = orderIndex.get(b.id);

    if (aIndex !== undefined && bIndex !== undefined && aIndex !== bIndex) {
      return aIndex - bIndex;
    }

    if (aIndex !== undefined) {
      return -1;
    }

    if (bIndex !== undefined) {
      return 1;
    }

    if ((a.updated ?? 0) !== (b.updated ?? 0)) {
      return (a.updated ?? 0) - (b.updated ?? 0);
    }

    if ((a.version ?? 0) !== (b.version ?? 0)) {
      return (a.version ?? 0) - (b.version ?? 0);
    }

    return String(a.id).localeCompare(String(b.id));
  });
}

export function hasPersistedDrawingContent(scene: DrawingSceneSnapshot) {
  return (
    scene.elements.length > 0 ||
    Object.keys(scene.files).length > 0 ||
    Object.keys(scene.appState).length > 0
  );
}

export function readDrawingSceneFromMap(yMap: any): DrawingSceneSnapshot {
  if (!yMap) {
    return {
      schemaVersion: DRAWING_SCHEMA_VERSION,
      updatedAt: 0,
      elements: [],
      appState: {},
      files: {},
    };
  }

  const meta = readDrawingMeta(yMap);
  const elementsById = new Map<string, any>();
  const files: Record<string, any> = {};

  yMap.forEach((rawValue: unknown, key: string) => {
    if (key === DRAWING_META_KEY) {
      return;
    }

    if (key.startsWith(DRAWING_FILE_PREFIX)) {
      const fileId = key.slice(DRAWING_FILE_PREFIX.length);
      const parsedFile = safeParseJson<any>(rawValue);
      if (parsedFile && fileId) {
        files[fileId] = parsedFile;
      }
      return;
    }

    if (key.startsWith(DRAWING_ELEMENT_PREFIX) || isLegacyElementKey(key)) {
      const parsedElement = safeParseJson<any>(rawValue);
      if (!parsedElement || typeof parsedElement.id !== "string") {
        return;
      }

      const existing = elementsById.get(parsedElement.id);
      if (
        !existing ||
        (parsedElement.version ?? 0) >= (existing.version ?? 0) ||
        key.startsWith(DRAWING_ELEMENT_PREFIX)
      ) {
        elementsById.set(parsedElement.id, parsedElement);
      }
    }
  });

  const elements = sortDrawingElements(Array.from(elementsById.values()), meta.elementOrder);

  return {
    schemaVersion: meta.schemaVersion,
    updatedAt: meta.updatedAt,
    elements,
    appState: meta.appState,
    files,
  };
}

export function createDrawingSceneSnapshot(
  elements: readonly any[],
  appState: Record<string, any> | null | undefined,
  files: Record<string, any> | null | undefined,
): DrawingSceneSnapshot {
  return {
    schemaVersion: DRAWING_SCHEMA_VERSION,
    updatedAt: Date.now(),
    elements: cloneJsonValue(Array.from(elements ?? [])),
    appState: sanitizePersistedAppState(appState),
    files: cloneJsonValue(files ?? {}),
  };
}

function getReferencedFileIds(elements: readonly any[]) {
  const ids = new Set<string>();
  for (const element of elements) {
    if (typeof element?.fileId === "string") {
      ids.add(element.fileId);
    }
  }
  return ids;
}

export function getDrawingSceneComparableSignature(scene: DrawingSceneSnapshot) {
  const visibleElements = scene.elements
    .filter((element) => !element?.isDeleted)
    .map((element) => `${element.id}:${element.version ?? 0}:${element.updated ?? 0}`)
    .join("|");
  const appState = JSON.stringify(scene.appState ?? {});
  const files = Object.keys(scene.files)
    .sort()
    .map((fileId) => {
      const file = scene.files[fileId] ?? {};
      return `${fileId}:${file.mimeType ?? ""}:${file.created ?? file.lastRetrieved ?? 0}`;
    })
    .join("|");

  return `${visibleElements}::${appState}::${files}`;
}

export function writeDrawingSceneToMap(
  yMap: any,
  elements: readonly any[],
  appState: Record<string, any> | null | undefined,
  files: Record<string, any> | null | undefined,
) {
  if (!yMap) {
    return createDrawingSceneSnapshot(elements, appState, files);
  }

  const nextScene = createDrawingSceneSnapshot(elements, appState, files);
  const currentKeys = Array.from(yMap.keys() as Iterable<string>);
  const currentElementIds = new Set<string>();
  const nextElementIds = new Set(nextScene.elements.map((element) => element.id as string));
  const referencedFileIds = getReferencedFileIds(nextScene.elements);

  for (const key of currentKeys) {
    if (key.startsWith(DRAWING_ELEMENT_PREFIX)) {
      currentElementIds.add(key.slice(DRAWING_ELEMENT_PREFIX.length));
    } else if (isLegacyElementKey(key)) {
      currentElementIds.add(key);
    }
  }

  yMap.doc?.transact(() => {
    yMap.set(
      DRAWING_META_KEY,
      JSON.stringify({
        schemaVersion: DRAWING_SCHEMA_VERSION,
        updatedAt: nextScene.updatedAt,
        elementOrder: nextScene.elements
          .filter((element) => !element?.isDeleted)
          .map((element) => element.id),
        appState: nextScene.appState,
      }),
    );

    for (const element of nextScene.elements) {
      const key = getElementStorageKey(element.id);
      const rawStored =
        yMap.get(key) ?? (currentElementIds.has(element.id) ? yMap.get(element.id) : undefined);
      const storedElement = safeParseJson<any>(rawStored);

      if (
        !storedElement ||
        (element.version ?? 0) >= (storedElement.version ?? 0) ||
        storedElement.isDeleted
      ) {
        yMap.set(key, JSON.stringify(element));
      }

      if (yMap.has(element.id)) {
        yMap.delete(element.id);
      }
    }

    for (const existingId of Array.from(currentElementIds)) {
      if (nextElementIds.has(existingId)) {
        continue;
      }

      const prefixedKey = getElementStorageKey(existingId);
      const rawStored = yMap.get(prefixedKey) ?? yMap.get(existingId);
      const storedElement = safeParseJson<any>(rawStored);
      if (!storedElement) {
        continue;
      }

      if (!storedElement.isDeleted) {
        yMap.set(prefixedKey, JSON.stringify({ ...storedElement, isDeleted: true }));
      }

      if (yMap.has(existingId)) {
        yMap.delete(existingId);
      }
    }

    for (const [fileId, fileData] of Object.entries(nextScene.files)) {
      yMap.set(getFileStorageKey(fileId), JSON.stringify(fileData));
    }

    for (const key of currentKeys) {
      if (!key.startsWith(DRAWING_FILE_PREFIX)) {
        continue;
      }

      const fileId = key.slice(DRAWING_FILE_PREFIX.length);
      if (nextScene.files[fileId] || referencedFileIds.has(fileId)) {
        continue;
      }

      yMap.delete(key);
    }
  });

  return nextScene;
}
