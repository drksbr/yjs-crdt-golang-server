import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import "./globals.css";
import { App } from "./App";
import { VersionLogger } from "@/components/VersionLogger";
import { syncSystemThemeClass } from "@/lib/theme";

syncSystemThemeClass();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <App />
      <VersionLogger />
    </BrowserRouter>
  </React.StrictMode>,
);
