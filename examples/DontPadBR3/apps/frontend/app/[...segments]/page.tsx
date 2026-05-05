import { DocumentRouteView } from "@/components/DocumentRouteView";

interface RoutedDocumentPageProps {
    params: Promise<{
        segments?: string[];
    }>;
}

export const dynamic = "force-dynamic";

export default async function RoutedDocumentPage({ params }: RoutedDocumentPageProps) {
    const { segments = [] } = await params;
    return <DocumentRouteView segments={segments} />;
}
