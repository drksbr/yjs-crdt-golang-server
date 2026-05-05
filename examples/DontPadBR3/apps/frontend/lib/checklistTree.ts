import {
  CHECKLIST_ORDER_STEP,
  ChecklistCheckedState,
  ChecklistItem,
  ChecklistStats,
} from "./checklistModel";

export interface ChecklistTreeNode extends ChecklistItem {
  depth: number;
  children: ChecklistTreeNode[];
}

export function sortChecklistItems(items: ChecklistItem[]) {
  return [...items].sort((a, b) => {
    if (a.order !== b.order) {
      return a.order - b.order;
    }
    if (a.createdAt !== b.createdAt) {
      return a.createdAt - b.createdAt;
    }
    return a.id.localeCompare(b.id);
  });
}

export function getChecklistItemMap(items: ChecklistItem[]) {
  return new Map(items.map((item) => [item.id, item]));
}

export function getChecklistChildren(items: ChecklistItem[], parentId: string | null) {
  return sortChecklistItems(items.filter((item) => item.parentId === parentId));
}

export function hasChecklistChildren(items: ChecklistItem[], parentId: string) {
  return items.some((item) => item.parentId === parentId);
}

export function getChecklistAncestors(items: ChecklistItem[], itemId: string) {
  const itemsById = getChecklistItemMap(items);
  const ancestors: ChecklistItem[] = [];
  let current = itemsById.get(itemId);

  while (current?.parentId) {
    const parent = itemsById.get(current.parentId);
    if (!parent) {
      break;
    }
    ancestors.push(parent);
    current = parent;
  }

  return ancestors;
}

export function getChecklistDescendantIds(items: ChecklistItem[], itemId: string): string[] {
  const descendants: string[] = [];
  const queue = getChecklistChildren(items, itemId).map((item) => item.id);

  while (queue.length > 0) {
    const currentId = queue.shift();
    if (!currentId) {
      continue;
    }
    descendants.push(currentId);
    const children = getChecklistChildren(items, currentId);
    for (const child of children) {
      queue.push(child.id);
    }
  }

  return descendants;
}

export function buildChecklistTree(items: ChecklistItem[]) {
  const itemsByParent = new Map<string | null, ChecklistItem[]>();
  for (const item of sortChecklistItems(items)) {
    const siblings = itemsByParent.get(item.parentId) ?? [];
    siblings.push(item);
    itemsByParent.set(item.parentId, siblings);
  }

  const buildLevel = (parentId: string | null, depth: number): ChecklistTreeNode[] => {
    const siblings = itemsByParent.get(parentId) ?? [];
    return siblings.map((item) => ({
      ...item,
      depth,
      children: buildLevel(item.id, depth + 1),
    }));
  };

  return buildLevel(null, 0);
}

export function getChecklistStats(items: ChecklistItem[]): ChecklistStats {
  const total = items.length;
  const completed = items.filter((item) => item.checked).length;

  return {
    total,
    completed,
    percentage: total > 0 ? Math.round((completed / total) * 100) : 0,
  };
}

function getBranchItems(items: ChecklistItem[], itemId: string) {
  const descendants = getChecklistDescendantIds(items, itemId);
  const itemsById = getChecklistItemMap(items);

  return [itemId, ...descendants]
    .map((id) => itemsById.get(id))
    .filter((item): item is ChecklistItem => Boolean(item));
}

export function getChecklistCheckedState(items: ChecklistItem[], itemId: string): ChecklistCheckedState {
  const branchItems = getBranchItems(items, itemId);
  if (branchItems.length === 0) {
    return "unchecked";
  }

  const checkedCount = branchItems.filter((item) => item.checked).length;
  if (checkedCount === 0) {
    return "unchecked";
  }
  if (checkedCount === branchItems.length) {
    return "checked";
  }
  return "mixed";
}

export function resequenceChecklistSiblings(
  items: ChecklistItem[],
  parentId: string | null,
  orderedSiblingIds?: string[],
) {
  const siblings = orderedSiblingIds ?? getChecklistChildren(items, parentId).map((item) => item.id);
  const siblingOrder = new Map(
    siblings.map((id, index) => [id, (index + 1) * CHECKLIST_ORDER_STEP]),
  );

  return items.map((item) =>
    item.parentId === parentId && siblingOrder.has(item.id)
      ? { ...item, order: siblingOrder.get(item.id)! }
      : item,
  );
}

export function syncChecklistParentState(items: ChecklistItem[]) {
  const sorted = sortChecklistItems(items);
  const itemsById = getChecklistItemMap(sorted);

  const computeChecked = (itemId: string): boolean => {
    const item = itemsById.get(itemId);
    if (!item) {
      return false;
    }

    const children = getChecklistChildren(sorted, itemId);
    if (children.length === 0) {
      return item.checked;
    }

    return children.every((child) => computeChecked(child.id));
  };

  return sorted.map((item) => {
    const children = getChecklistChildren(sorted, item.id);
    if (children.length === 0) {
      return item;
    }

    return {
      ...item,
      checked: computeChecked(item.id),
    };
  });
}
