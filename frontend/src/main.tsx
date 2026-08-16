import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import { I18nProvider } from "./i18n";
import { PluginsProvider } from "./framework/PluginProvider";
import "./plugins"; // register all plugins before the first render
import "./globals.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <I18nProvider>
      <PluginsProvider>
        <App />
      </PluginsProvider>
    </I18nProvider>
  </React.StrictMode>
);
