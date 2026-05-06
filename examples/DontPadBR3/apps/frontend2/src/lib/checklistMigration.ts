import {
  CHECKLIST_ORDER_STEP,
  ChecklistItem,
  LegacyChecklistItem,
  createChecklistItem,
  isChecklistItem,
  isLegacyChecklistItem,
} from "./checklistModel";

function normalizeEntry(entryKey: string, value: unknown, index: number): ChecklistItem | null {
  if (isChecklistItem(value)) {
    const item = createChecklistItem({
      ...value,
      id: entryKey || value.id,
      parentId: value.parentId,
    });

    return {
      ...item,
      order: Number.isFinite(value.order) ? value.order : (index + 1) * CHECKLIST_ORDER_STEP,
    };
  }

  if (isLegacyChecklistItem(value)) {
    const legacy = value as LegacyChecklistItem;
    return createChecklistItem({
      id: entryKey || legacy.id,
      text: legacy.text,
      checked: legacy.checked,
      parentId: null,
      order: legacy.createdAt || (index + 1) * CHECKLIST_ORDER_STEP,
      collapsed: false,
      createdAt: legacy.createdAt,
      updatedAt: legacy.createdAt,
    });
  }

  return null;
}

export function migrateChecklistEntries(entries: Array<[string, unknown]>): ChecklistItem[] {
  const normalized = entries
    .map(([key, value], index) => normalizeEntry(key, value, index))
    .filter((item): item is ChecklistItem => item !== null);

  if (normalized.length === 0) {
    return [];
  }

  const ids = new Set(normalized.map((item) => item.id));
  const itemsById = new Map(normalized.map((item) => [item.id, item]));
  const childrenByParent = new Map<string | null, ChecklistItem[]>();

  for (const item of normalized) {
    let validParentId =
      item.parentId && item.parentId !== item.id && ids.has(item.parentId)
        ? item.parentId
        : null;

    if (validParentId) {
      const visited = new Set<string>([item.id]);
      let currentParentId: string | null = validParentId;

      while (currentParentId) {
        if (visited.has(currentParentId)) {
          validParentId = null;
          break;
        }

        visited.add(currentParentId);
        currentParentId = itemsById.get(currentParentId)?.parentId ?? null;
      }
    }

    const normalizedItem =
      validParentId === item.parentId ? item : { ...item, parentId: validParentId };

    const siblings = childrenByParent.get(normalizedItem.parentId) ?? [];
    siblings.push(normalizedItem);
    childrenByParent.set(normalizedItem.parentId, siblings);
  }

  const migrated: ChecklistItem[] = [];

  const visit = (parentId: string | null) => {
    const siblings = (childrenByParent.get(parentId) ?? [])
      .sort((a, b) => {
        if (a.order !== b.order) {
          return a.order - b.order;
        }
        if (a.createdAt !== b.createdAt) {
          return a.createdAt - b.createdAt;
        }
        return a.id.localeCompare(b.id);
      })
      .map((item, index) => ({
        ...item,
        order: (index + 1) * CHECKLIST_ORDER_STEP,
      }));

    for (const sibling of siblings) {
      migrated.push(sibling);
      visit(sibling.id);
    }
  };

  visit(null);

  return migrated;
}
