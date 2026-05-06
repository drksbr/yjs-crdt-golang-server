"use client";

import { ChecklistCheckedState } from "@/lib/checklistModel";
import { ChecklistTreeNode } from "@/lib/checklistTree";
import { ChecklistRow } from "./ChecklistRow";

interface ChecklistTreeProps {
    nodes: ChecklistTreeNode[];
    checkedStates: Record<string, ChecklistCheckedState>;
    editingId: string | null;
    editText: string;
    isReadOnly: boolean;
    onStartEdit: (node: ChecklistTreeNode) => void;
    onEditChange: (value: string) => void;
    onCommitEdit: (node: ChecklistTreeNode, options?: { createSibling?: boolean; createChild?: boolean }) => void;
    onCancelEdit: () => void;
    onToggleCheck: (node: ChecklistTreeNode) => void;
    onToggleCollapsed: (node: ChecklistTreeNode) => void;
    onAddChild: (node: ChecklistTreeNode) => void;
    onAddSibling: (node: ChecklistTreeNode) => void;
    onIndent: (node: ChecklistTreeNode) => void;
    onOutdent: (node: ChecklistTreeNode) => void;
    onDelete: (node: ChecklistTreeNode) => void;
}

export function ChecklistTree({
    nodes,
    checkedStates,
    editingId,
    editText,
    isReadOnly,
    onStartEdit,
    onEditChange,
    onCommitEdit,
    onCancelEdit,
    onToggleCheck,
    onToggleCollapsed,
    onAddChild,
    onAddSibling,
    onIndent,
    onOutdent,
    onDelete,
}: ChecklistTreeProps) {
    return (
        <div className="space-y-2">
            {nodes.map((node) => (
                <div key={node.id} className="space-y-2">
                    <ChecklistRow
                        node={node}
                        checkedState={checkedStates[node.id] ?? "unchecked"}
                        isEditing={editingId === node.id}
                        editText={editingId === node.id ? editText : node.text}
                        isReadOnly={isReadOnly}
                        onStartEdit={onStartEdit}
                        onEditChange={onEditChange}
                        onCommitEdit={onCommitEdit}
                        onCancelEdit={onCancelEdit}
                        onToggleCheck={onToggleCheck}
                        onToggleCollapsed={onToggleCollapsed}
                        onAddChild={onAddChild}
                        onAddSibling={onAddSibling}
                        onIndent={onIndent}
                        onOutdent={onOutdent}
                        onDelete={onDelete}
                    />
                    {!node.collapsed && node.children.length > 0 ? (
                        <ChecklistTree
                            nodes={node.children}
                            checkedStates={checkedStates}
                            editingId={editingId}
                            editText={editText}
                            isReadOnly={isReadOnly}
                            onStartEdit={onStartEdit}
                            onEditChange={onEditChange}
                            onCommitEdit={onCommitEdit}
                            onCancelEdit={onCancelEdit}
                            onToggleCheck={onToggleCheck}
                            onToggleCollapsed={onToggleCollapsed}
                            onAddChild={onAddChild}
                            onAddSibling={onAddSibling}
                            onIndent={onIndent}
                            onOutdent={onOutdent}
                            onDelete={onDelete}
                        />
                    ) : null}
                </div>
            ))}
        </div>
    );
}
