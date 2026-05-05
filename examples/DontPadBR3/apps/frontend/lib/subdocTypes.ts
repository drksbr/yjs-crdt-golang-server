export type SubdocType =
  | "texto"
  | "markdown"
  | "checklist"
  | "kanban"
  | "desenho";

export interface SubdocTypeConfig {
  type: SubdocType;
  label: string;
  description: string;
}

export const SUBDOC_TYPE_CONFIGS: SubdocTypeConfig[] = [
  { type: "texto", label: "Texto", description: "Editor de texto rico" },
  {
    type: "markdown",
    label: "Markdown",
    description: "Editor Markdown com preview",
  },
  { type: "checklist", label: "Checklist", description: "Lista de tarefas" },
  { type: "kanban", label: "Kanban", description: "Quadro com colunas" },
  {
    type: "desenho",
    label: "Desenho",
    description: "Quadro colaborativo de desenho",
  },
];

export function getSubdocTypeLabel(type?: SubdocType): string {
  return SUBDOC_TYPE_CONFIGS.find((c) => c.type === type)?.label ?? "Texto";
}
