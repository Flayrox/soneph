// Catalog assembly — importing this module registers every plugin with the
// registry. Import it once from main.tsx before the app renders.
import { registerPlugin } from "@/framework/pluginRegistry";
import { corePlugin } from "./core";
import { importPlugin } from "./import";
import { statsPlugin } from "./stats";

registerPlugin(corePlugin);
registerPlugin(importPlugin);
registerPlugin(statsPlugin);
