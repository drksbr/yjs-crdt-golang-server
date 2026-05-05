export interface LegacyChecklistItem {
  id: string;
  text: string;
  checked: boolean;
  createdAt: number;
}

export interface ChecklistItem {
  id: string;
  text: string;
  checked: boolean;
  parentId: string | null;
  order: number;
  collapsed: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface ChecklistStats {
  total: number;
  completed: number;
  percentage: number;
}

export type ChecklistCheckedState = "checked" | "unchecked" | "mixed";

export const CHECKLIST_ORDER_STEP = 1000;

export function generateChecklistItemId() {
  return `item-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

export function isLegacyChecklistItem(value: unknown): value is LegacyChecklistItem {
  if (!value || typeof value !== "object") {
    return false;
  }

  const item = value as Partial<LegacyChecklistItem>;
  return (
    typeof item.id === "string" &&
    typeof item.text === "string" &&
    typeof item.checked === "boolean" &&
    typeof item.createdAt === "number"
  );
}

export function isChecklistItem(value: unknown): value is ChecklistItem {
  if (!value || typeof value !== "object") {
    return false;
  }

  const item = value as Partial<ChecklistItem>;
  return (
    typeof item.id === "string" &&
    typeof item.text === "string" &&
    typeof item.checked === "boolean" &&
    (typeof item.parentId === "string" || item.parentId === null) &&
    typeof item.order === "number" &&
    typeof item.collapsed === "boolean" &&
    typeof item.createdAt === "number" &&
    typeof item.updatedAt === "number"
  );
}

export function createChecklistItem(
  input: Partial<ChecklistItem> & Pick<ChecklistItem, "id">,
): ChecklistItem {
  const now = Date.now();

  return {
    id: input.id,
    text: typeof input.text === "string" ? input.text : "",
    checked: input.checked ?? false,
    parentId:
      typeof input.parentId === "string" && input.parentId.length > 0
        ? input.parentId
        : null,
    order: Number.isFinite(input.order) ? Number(input.order) : CHECKLIST_ORDER_STEP,
    collapsed: input.collapsed ?? false,
    createdAt: Number.isFinite(input.createdAt) ? Number(input.createdAt) : now,
    updatedAt: Number.isFinite(input.updatedAt)
      ? Number(input.updatedAt)
      : Number.isFinite(input.createdAt)
        ? Number(input.createdAt)
        : now,
  };
}
