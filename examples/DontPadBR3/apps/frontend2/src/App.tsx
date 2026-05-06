import { Suspense, lazy, useEffect } from "react";
import { Route, Routes, useLocation } from "react-router-dom";
import HomePage from "@/pages/HomePage";
import { useFrontendEvents } from "@/stores/events";
import { DocumentLoadingProgress } from "@/components/DocumentLoadingProgress";

const DocumentRouteScreen = lazy(() =>
  import("@/pages/DocumentRouteScreen").then((module) => ({
    default: module.DocumentRouteScreen,
  })),
);

function RouteEvents() {
  const location = useLocation();
  const emit = useFrontendEvents((state) => state.emit);

  useEffect(() => {
    emit("route:navigated", { path: location.pathname });
  }, [emit, location.pathname]);

  return null;
}

export function App() {
  return (
    <>
      <RouteEvents />
      <Suspense
        fallback={
          <DocumentLoadingProgress
            stage="application"
            title="Carregando DontPad"
            description="Preparando a aplicação antes de abrir a rota solicitada."
            detail="Baixando o pacote inicial da interface."
          />
        }
      >
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="*" element={<DocumentRouteScreen />} />
        </Routes>
      </Suspense>
    </>
  );
}
