"use client";

import { useMap } from "@/lib/collab/react";
import { DocumentView } from "@/components/DocumentView";
import { SecureDocumentProvider } from "@/components/SecureDocumentProvider";
import {
    getSubdocumentDocumentId,
    resolveDocumentRoute,
    resolveSubdocumentEntry,
} from "@/lib/documentRouting";

interface DocumentRouteViewProps {
    segments: string[];
}

function SubdocumentRouteContent({
    parentDocumentId,
    displayDocumentId,
    parentHref,
    subdocumentSlug,
    subdocumentName,
}: {
    parentDocumentId: string;
    displayDocumentId: string;
    parentHref: string;
    subdocumentSlug: string;
    subdocumentName: string;
}) {
    const parentSubdocumentsMap = useMap("subdocuments");
    const subdocument = resolveSubdocumentEntry(parentSubdocumentsMap, subdocumentSlug, subdocumentName);
    const childDocumentId = subdocument.documentId ?? getSubdocumentDocumentId(parentDocumentId, subdocument.id);
    const visibleDocumentId = `${displayDocumentId}/${subdocument.name}`;

    return (
        <SecureDocumentProvider
            documentId={childDocumentId}
            displayDocumentId={visibleDocumentId}
        >
            <DocumentView
                documentId={parentDocumentId}
                activeDocumentId={childDocumentId}
                displayDocumentId={displayDocumentId}
                parentHref={parentHref}
                subdocumentName={subdocument.name}
                subdocumentSlug={subdocument.id}
                subdocumentType={subdocument.type}
                parentSubdocumentsMap={parentSubdocumentsMap}
            />
        </SecureDocumentProvider>
    );
}

export function DocumentRouteView({ segments }: DocumentRouteViewProps) {
    const route = resolveDocumentRoute(segments);

    if (route.kind === "document") {
        return (
            <SecureDocumentProvider documentId={route.documentId} displayDocumentId={route.displayDocumentId}>
                <DocumentView
                    documentId={route.documentId}
                    displayDocumentId={route.displayDocumentId}
                    parentHref={route.parentHref}
                />
            </SecureDocumentProvider>
        );
    }

    return (
        <SecureDocumentProvider documentId={route.parentDocumentId} displayDocumentId={route.displayDocumentId}>
            <SubdocumentRouteContent
                parentDocumentId={route.parentDocumentId}
                displayDocumentId={route.displayDocumentId}
                parentHref={route.parentHref}
                subdocumentSlug={route.subdocumentSlug}
                subdocumentName={route.subdocumentName}
            />
        </SecureDocumentProvider>
    );
}
