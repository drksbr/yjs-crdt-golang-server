import { useMemo } from "react";
import { useLocation } from "react-router-dom";
import { DocumentRouteView } from "@/components/DocumentRouteView";

function pathnameToSegments(pathname: string): string[] {
  return pathname
    .split("/")
    .filter(Boolean)
    .map((segment) => decodeURIComponent(segment));
}

export function DocumentRouteScreen() {
  const location = useLocation();
  const segments = useMemo(
    () => pathnameToSegments(location.pathname),
    [location.pathname],
  );

  return <DocumentRouteView segments={segments} />;
}
