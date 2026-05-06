import * as Y from "yjs";
import { SubdocType } from "./subdocTypes";

export function sanitizeSubdocumentKey(name: string): string {
  return name.toLowerCase().replace(/[^\w\s-]/g, "").replace(/\s+/g, "-");
}

export function getTextStateKey(subdocumentName?: string): string {
  return "text";
}

export function getMarkdownStateKey(subdocumentName: string): string {
  return "markdown";
}

export function getChecklistStateKey(subdocumentName: string): string {
  return "checklist";
}

export function getKanbanColumnsStateKey(subdocumentName: string): string {
  return "kanban-cols";
}

export function getKanbanItemsStateKey(subdocumentName: string): string {
  return "kanban-items";
}

export function getDrawingStateKey(subdocumentName: string): string {
  return "drawing";
}

export function getMetaScopeKey(subdocumentName?: string): string {
  return "document";
}

export function getMetaFieldKey(
  field: "lastModified" | "lastAccessed",
  subdocumentName?: string,
): string {
  return `${getMetaScopeKey(subdocumentName)}:${field}`;
}

function replaceTextContent(target: Y.Text, source: Y.Text) {
  target.delete(0, target.length);

  let index = 0;
  for (const delta of source.toDelta()) {
    if (!delta.insert || typeof delta.insert !== "string") {
      continue;
    }

    if (delta.attributes) {
      target.insert(index, delta.insert, delta.attributes);
    } else {
      target.insert(index, delta.insert);
    }
    index += delta.insert.length;
  }
}

function cloneMapValue<T>(value: T): T {
  if (value === null || value === undefined) {
    return value;
  }

  return JSON.parse(JSON.stringify(value)) as T;
}

function replaceMapContent(target: Y.Map<any>, source: Y.Map<any>) {
  for (const key of Array.from(target.keys())) {
    target.delete(key);
  }

  source.forEach((value, key) => {
    target.set(key, cloneMapValue(value));
  });
}

interface ApplyVersionSnapshotOptions {
  sourceDoc: Y.Doc;
  targetDoc: Y.Doc;
  subdocType: SubdocType;
  subdocumentName?: string;
}

export function applyVersionSnapshot({
  sourceDoc,
  targetDoc,
  subdocType,
  subdocumentName,
}: ApplyVersionSnapshotOptions) {
  targetDoc.transact(() => {
    switch (subdocType) {
      case "texto": {
        replaceTextContent(
          targetDoc.getText(getTextStateKey(subdocumentName)),
          sourceDoc.getText(getTextStateKey(subdocumentName)),
        );
        return;
      }
      case "markdown": {
        if (!subdocumentName) {
          throw new Error("Markdown snapshot requires a subdocument name");
        }
        replaceTextContent(
          targetDoc.getText(getMarkdownStateKey(subdocumentName)),
          sourceDoc.getText(getMarkdownStateKey(subdocumentName)),
        );
        return;
      }
      case "checklist": {
        if (!subdocumentName) {
          throw new Error("Checklist snapshot requires a subdocument name");
        }
        replaceMapContent(
          targetDoc.getMap(getChecklistStateKey(subdocumentName)),
          sourceDoc.getMap(getChecklistStateKey(subdocumentName)),
        );
        return;
      }
      case "kanban": {
        if (!subdocumentName) {
          throw new Error("Kanban snapshot requires a subdocument name");
        }
        replaceMapContent(
          targetDoc.getMap(getKanbanColumnsStateKey(subdocumentName)),
          sourceDoc.getMap(getKanbanColumnsStateKey(subdocumentName)),
        );
        replaceMapContent(
          targetDoc.getMap(getKanbanItemsStateKey(subdocumentName)),
          sourceDoc.getMap(getKanbanItemsStateKey(subdocumentName)),
        );
        return;
      }
      case "desenho": {
        if (!subdocumentName) {
          throw new Error("Drawing snapshot requires a subdocument name");
        }
        replaceMapContent(
          targetDoc.getMap(getDrawingStateKey(subdocumentName)),
          sourceDoc.getMap(getDrawingStateKey(subdocumentName)),
        );
        return;
      }
      default: {
        const unreachable: never = subdocType;
        throw new Error(`Unsupported subdocument type: ${unreachable}`);
      }
    }
  });
}
