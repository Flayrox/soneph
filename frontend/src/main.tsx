import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import { I18nProvider } from "./i18n";
import { ModulesProvider } from "./modules";
import "./globals.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <I18nProvider>
      <ModulesProvider>
        <App />
      </ModulesProvider>
    </I18nProvider>
  </React.StrictMode>
);
