# Écrire un plugin

Le framework à plugins (inspiré d'Obsidian / VS Code) permet d'ajouter des
fonctionnalités sans toucher au cœur de l'app. Un plugin contribue des
**vues** (entrées de la sidebar + panneaux) et des **actions** (commandes
de l'hôte).

## Le contrat `{ app }`

Chaque vue reçoit **un seul prop** : `app` (`PluginApp` dans
`frontend/src/framework/plugin.types.ts`). Depuis `app` on peut :

- **Lire** : `app.files` (bibliothèque), `app.playlists`, `app.likes`,
  `app.recent`, `app.top`, `app.artists`, `app.albums`, `app.pinned`,
  `app.nav`, `app.isPlaying`, `app.currentPlayingPath`…
- **Agir** : `app.playTrack(path)`, `app.playList(paths, index)`,
  `app.playNext(paths)`, `app.toggleLike(path)`, `app.setNav(id)`,
  `app.notify(type, title, message)`, `app.openLyricsDrawer(track)`,
  `app.deleteFile(path)`, `app.addToPlaylist(id, path)`,
  `app.createPlaylist(name, path?)`, `app.togglePin(pin)`,
  `app.refreshFiles()`…

## Structure minimale

```tsx
// src/plugins/monplugin.ts
import { Zap } from "lucide-react";
import type { PluginManifest } from "@/framework/plugin.types";

export const monPlugin: PluginManifest = {
  id: "monplugin",
  nameKey: "Mon Plugin",        // clé i18n (nom dans la Marketplace)
  descKey: "Mon Plugin Desc",   // clé i18n (description)
  version: "1.0.0",
  icon: Zap,
  // core: true  → indésactivable. Absent → activable depuis la Marketplace.
  defaultEnabled: true,         // pré-coché à l'onboarding
  contributes: {
    views: [
      {
        id: "monplugin",                       // id de nav
        labelKey: "Mon Plugin",                // libellé sidebar
        section: "library",                    // music | playlists | downloads | library
        icon: Zap,
        component: MonView,                    // React.FC<{ app: PluginApp }>
        badge: (app) => app.files.length,      // compteur optionnel (ou null)
        hidden: true,                          // route interne, pas dans la sidebar
      },
    ],
    actions: [
      {
        id: "monplugin.hello",
        labelKey: "Mon Plugin : hello",
        run: (app) => app.notify("info", "Mon Plugin", "Salut !"),
      },
    ],
  },
};
```

Ensuite, enregistre-le dans `src/plugins/index.ts` :

```ts
import { monPlugin } from "./monplugin";
registerPlugin(monPlugin);
```

## Le plugin d'exemple

`src/plugins/example.ts` + `src/components/ExampleView.tsx` est un plugin
complet et commenté qui montre chaque pièce du contrat en action (lecture
de la bibliothèque, badges, actions, toasts). Copie-le comme point de
départ — il est activable/désactivable depuis la **Marketplace**.

## Vues dynamiques

Une vue avec `hidden: true` n'apparaît pas dans la sidebar mais reste
routable. Le host (`App.tsx`) fait correspondre les préfixes de nav :
`pl:`, `artist:`, `album:` → vues de détail. Un plugin peut déclarer ses
propres préfixes en demandant un routage dans `PluginHostView`.

## i18n

Les clés sont en anglais par défaut ; ajoute la traduction française dans
`frontend/src/i18n.tsx` (dict `fr`). `t()` retombe sur la clé si elle
manque — jamais de trou.
