# ADR-0003 — Table settings + politique de migrations

- **Statut** : accepté
- **Date** : 2026-08-17
- **Portée** : M2, et toutes les évolutions de schéma futures

## Contexte

La mission M2 (« SQLite devient la source de vérité ») liste « Runtime config
→ settings table ». Or le schéma de référence (§4 de la constitution) ne
contient **pas** de table `settings` — il a été figé avant que ce besoin
apparaisse. Deux questions se posent :

1. Où mettre les réglages d'exécution (workers, threads, dossier d'export…) ?
2. Comment faire évoluer le schéma sans violer la règle « jamais éditer une
   migration appliquée » ?

## Décision

### Table settings (migration `0002_settings.sql`)

Les réglages d'exécution vivent en base dans une table clé/valeur simple :

```sql
CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
```

- `pkg/store` expose `GetSetting`/`SetSetting` (upsert) ; les handlers et le
  moteur n'écrivent jamais de SQL directement.
- La table est créée par une **nouvelle** migration (`0002`), pas en éditant
  `0001`. C'est l'application stricte de la règle §4 : toute évolution du
  schéma après M2 passe par un nouveau fichier `migrations/NNNN_name.sql`.

### Politique de migrations

- **Emplacement** : `backend/pkg/store/migrations/` — la convention
  `migrations/NNNN_name.sql` est conservée ; le répertoire vit dans le
  package store car `go:embed` ne peut pas référencer de fichiers hors de
  l'arborescence du package qui l'embarque.
- **Format goose** : chaque fichier porte les annotations `-- +goose Up` /
  `-- +goose Down`. Le parseur goose v3.27 rejette les fichiers sans
  annotation (état par défaut) et les `PRAGMA` nus : ils doivent être
  wrappés en blocs `-- +goose StatementBegin`/`StatementEnd`. Les PRAGMA
  critiques (WAL, foreign_keys) sont de toute façon forcées par le DSN à
  l'ouverture du store.
- **Nommage** : `NNNN_snake_case.sql`, numérotation séquentielle ; chaque
  PR ajoutant une migration inclut la migration `Down` correspondante (même
  un no-op documenté).
- **Règle d'or** : une migration appliquée est immuable. Corriger un schéma
  erroné = nouvelle migration, jamais d'édition.
- **Tests** : toute migration est vérifiée par un test qui ouvre une base
  temporaire, applique goose et vérifie la présence des objets (`TestOpenAppliesMigrations`
  en est le garde-fou).

## Conséquences

**Positives** : les réglages survivent au redémarrage et sont lisibles
depuis n'importe quel client ; la politique de migration est explicite et
testée ; `pkg/store/migrations/` rend l'application des migrations
autonome (pas de FS externe à injecter).

**Négatives** : le chemin `pkg/store/migrations/` déroge à l'emplacement
`backend/migrations/` de la constitution — écart documenté ici, motivé par
`go:embed`.

**Risques** : oublier l'annotation goose sur un nouveau fichier → erreur de
parse au démarrage. Mitigation : le test d'ouverture échoue bruyamment.
