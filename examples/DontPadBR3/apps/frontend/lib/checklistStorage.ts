import * as Y from "yjs";
import {
  ChecklistItem,
  createChecklistItem,
  generateChecklistItemId,
  isChecklistItem,
  isLegacyChecklistItem,
} from "./checklistModel";
import { migrateChecklistEntries } from "./checklistMigration";
import {
  getChecklistAncestors,
  getChecklistChildren,
  getChecklistDescendantIds,
  getChecklistItemMap,
  resequenceChecklistSiblings,
  syncChecklistParentState,
} from "./checklistTree";

function writeChecklistSnapshot(itemsMap: Y.Map<unknown>, items: ChecklistItem[]) {
  itemsMap.doc?.transact(() => {
    for (const key of Array.from(itemsMap.keys())) {
      itemsMap.delete(key);
    }

    for (const item of items) {
      itemsMap.set(item.id, item);
    }
  });
}

function normalizeChecklistSnapshot(items: ChecklistItem[]) {
  return JSON.stringify(
    items.map((item) => ({
      id: item.id,
      text: item.text,
      checked: item.checked,
      parentId: item.parentId,
      order: item.order,
      collapsed: item.collapsed,
      createdAt: item.createdAt,
      updatedAt: item.updatedAt,
    })),
  );
}

function commitChecklistItems(itemsMap: Y.Map<unknown>, nextItems: ChecklistItem[]) {
  writeChecklistSnapshot(itemsMap, syncChecklistParentState(nextItems));
}

export function readChecklistItems(itemsMap: Y.Map<unknown> | null | undefined) {
  if (!itemsMap) {
    return [] as ChecklistItem[];
  }

  return migrateChecklistEntries(Array.from(itemsMap.entries()));
}

export function ensureChecklistV2(itemsMap: Y.Map<unknown> | null | undefined) {
  if (!itemsMap) {
    return [] as ChecklistItem[];
  }

  const rawEntries = Array.from(itemsMap.entries());
  const migrated = migrateChecklistEntries(rawEntries);
  const rawValues = rawEntries.map(([, value]) => value);
  const needsMigration =
    rawValues.some((value) => !isChecklistItem(value)) ||
    migrated.length !== rawEntries.length ||
    normalizeChecklistSnapshot(migrated) !==
      normalizeChecklistSnapshot(
        rawValues.filter((value): value is ChecklistItem => isChecklistItem(value)),
      );

  if (needsMigration) {
    writeChecklistSnapshot(itemsMap, migrated);
  }

  return migrated;
}

function updateBranchCheckedState(items: ChecklistItem[], itemId: string, checked: boolean) {
  const branchIds = new Set([itemId, ...getChecklistDescendantIds(items, itemId)]);
  return items.map((item) =>
    branchIds.has(item.id)
      ? {
          ...item,
          checked,
          updatedAt: Date.now(),
        }
      : item,
  );
}

export function addChecklistRootItem(itemsMap: Y.Map<unknown>, text = "") {
  const items = ensureChecklistV2(itemsMap);
  const rootSiblings = getChecklistChildren(items, null);
  const newItem = createChecklistItem({
    id: generateChecklistItemId(),
    text,
    parentId: null,
    order: (rootSiblings.length + 1) * 1000,
    checked: false,
    collapsed: false,
  });

  commitChecklistItems(itemsMap, [...items, newItem]);
  return newItem;
}

export function addChecklistChildItem(itemsMap: Y.Map<unknown>, parentId: string, text = "") {
  const items = ensureChecklistV2(itemsMap);
  const itemsById = getChecklistItemMap(items);
  const parent = itemsById.get(parentId);

  if (!parent) {
    return null;
  }

  const children = getChecklistChildren(items, parentId);
  const newItem = createChecklistItem({
    id: generateChecklistItemId(),
    text,
    parentId,
    order: (children.length + 1) * 1000,
    checked: false,
    collapsed: false,
  });

  const nextItems = items.map((item) =>
    item.id === parentId ? { ...item, collapsed: false, updatedAt: Date.now() } : item,
  );

  commitChecklistItems(itemsMap, [...nextItems, newItem]);
  return newItem;
}

export function addChecklistSiblingItem(itemsMap: Y.Map<unknown>, targetId: string, text = "") {
  const items = ensureChecklistV2(itemsMap);
  const itemsById = getChecklistItemMap(items);
  const target = itemsById.get(targetId);

  if (!target) {
    return null;
  }

  const siblings = getChecklistChildren(items, target.parentId);
  const orderedIds = siblings.map((item) => item.id);
  const targetIndex = orderedIds.indexOf(targetId);
  const newItem = createChecklistItem({
    id: generateChecklistItemId(),
    text,
    parentId: target.parentId,
    order: target.order + 1,
    checked: false,
    collapsed: false,
  });

  orderedIds.splice(targetIndex + 1, 0, newItem.id);
  const nextItems = resequenceChecklistSiblings([...items, newItem], target.parentId, orderedIds);

  commitChecklistItems(itemsMap, nextItems);
  return newItem;
}

export function updateChecklistItemText(itemsMap: Y.Map<unknown>, itemId: string, text: string) {
  const items = ensureChecklistV2(itemsMap);
  const nextItems = items.map((item) =>
    item.id === itemId ? { ...item, text, updatedAt: Date.now() } : item,
  );

  commitChecklistItems(itemsMap, nextItems);
}

export function toggleChecklistItem(itemsMap: Y.Map<unknown>, itemId: string, checked?: boolean) {
  const items = ensureChecklistV2(itemsMap);
  const itemsById = getChecklistItemMap(items);
  const target = itemsById.get(itemId);

  if (!target) {
    return;
  }

  const nextItems = updateBranchCheckedState(items, itemId, checked ?? !target.checked);
  commitChecklistItems(itemsMap, nextItems);
}

export function toggleChecklistCollapsed(itemsMap: Y.Map<unknown>, itemId: string) {
  const items = ensureChecklistV2(itemsMap);
  const nextItems = items.map((item) =>
    item.id === itemId
      ? {
          ...item,
          collapsed: !item.collapsed,
          updatedAt: Date.now(),
        }
      : item,
  );

  commitChecklistItems(itemsMap, nextItems);
}

export function deleteChecklistItem(itemsMap: Y.Map<unknown>, itemId: string) {
  const items = ensureChecklistV2(itemsMap);
  const descendants = new Set([itemId, ...getChecklistDescendantIds(items, itemId)]);
  const remaining = items.filter((item) => !descendants.has(item.id));
  const itemsById = getChecklistItemMap(items);
  const target = itemsById.get(itemId);

  if (!target) {
    return;
  }

  const resequenced = resequenceChecklistSiblings(remaining, target.parentId);
  commitChecklistItems(itemsMap, resequenced);
}

export function indentChecklistItem(itemsMap: Y.Map<unknown>, itemId: string) {
  const items = ensureChecklistV2(itemsMap);
  const itemsById = getChecklistItemMap(items);
  const target = itemsById.get(itemId);

  if (!target) {
    return false;
  }

  const siblings = getChecklistChildren(items, target.parentId);
  const targetIndex = siblings.findIndex((item) => item.id === itemId);
  if (targetIndex <= 0) {
    return false;
  }

  const previousSibling = siblings[targetIndex - 1];
  const nextItems = items.map((item) =>
    item.id === itemId
      ? {
          ...item,
          parentId: previousSibling.id,
          updatedAt: Date.now(),
        }
      : item.id === previousSibling.id
        ? { ...item, collapsed: false, updatedAt: Date.now() }
        : item,
  );

  const oldSiblingIds = siblings.filter((item) => item.id !== itemId).map((item) => item.id);
  const newChildIds = getChecklistChildren(items, previousSibling.id)
    .map((item) => item.id)
    .concat(itemId);

  const resequencedOldParent = resequenceChecklistSiblings(nextItems, target.parentId, oldSiblingIds);
  const resequencedNewParent = resequenceChecklistSiblings(
    resequencedOldParent,
    previousSibling.id,
    newChildIds,
  );

  commitChecklistItems(itemsMap, resequencedNewParent);
  return true;
}

export function outdentChecklistItem(itemsMap: Y.Map<unknown>, itemId: string) {
  const items = ensureChecklistV2(itemsMap);
  const itemsById = getChecklistItemMap(items);
  const target = itemsById.get(itemId);

  if (!target?.parentId) {
    return false;
  }

  const parent = itemsById.get(target.parentId);
  if (!parent) {
    return false;
  }

  const oldParentChildren = getChecklistChildren(items, parent.id)
    .filter((item) => item.id !== itemId)
    .map((item) => item.id);
  const newParentSiblings = getChecklistChildren(items, parent.parentId).map((item) => item.id);
  const parentIndex = newParentSiblings.indexOf(parent.id);
  newParentSiblings.splice(parentIndex + 1, 0, itemId);

  const nextItems = items.map((item) =>
    item.id === itemId
      ? {
          ...item,
          parentId: parent.parentId,
          updatedAt: Date.now(),
        }
      : item,
  );

  const resequencedOldParent = resequenceChecklistSiblings(nextItems, parent.id, oldParentChildren);
  const resequencedNewParent = resequenceChecklistSiblings(
    resequencedOldParent,
    parent.parentId,
    newParentSiblings,
  );

  commitChecklistItems(itemsMap, resequencedNewParent);
  return true;
}

export function removeChecklistItemIfBlank(itemsMap: Y.Map<unknown>, itemId: string) {
  const items = ensureChecklistV2(itemsMap);
  const target = items.find((item) => item.id === itemId);
  if (!target || target.text.trim().length > 0) {
    return false;
  }

  deleteChecklistItem(itemsMap, itemId);
  return true;
}

export function getChecklistEditingContext(items: ChecklistItem[], itemId: string) {
  const itemsById = getChecklistItemMap(items);
  const target = itemsById.get(itemId);
  const ancestors = getChecklistAncestors(items, itemId);

  return {
    item: target ?? null,
    ancestors,
  };
}
