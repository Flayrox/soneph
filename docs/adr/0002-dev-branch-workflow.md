# ADR-0002 — Workflow branches : `dev` intègre, `main` publie

- **Statut** : accepté
- **Date** : 2026-08-17
- **Portée** : organisation du dépôt, missions M1 → M15

## Contexte

Le dépôt n'a longtemps eu qu'une branche de travail. Avec la roadmap en
missions successives (M1…M15), plusieurs chantiers coexistent : il faut
isoler le travail en cours, garantir que chaque mission soit révisable
indépendamment, et que `main` ne reçoive que des états stables et publiables.

## Décision

- `dev` est la branche d'intégration : toute mission part de `dev` à jour
  sur une branche `m<N>-<slug>` (ex. `m2-db-layer`), une mission = une
  branche.
- Chaque PR est ouverte en DRAFT avec la Definition of Done de sa mission
  en checklist ; elle passe sur `dev` par squash-merge uniquement après
  validation du CTO.
- `main` ne reçoit que les merges de publication (au moment du release),
  effectués exclusivement par le CTO. Un tag `v*` déclenche le pipeline
  de release (DMG + GitHub Release).
- Protection des branches activée : pas de push direct sur `dev` ni
  `main`, revues requises.
- Règle de taille : une PR de plus de ~1500 lignes modifiées est découpée
  en PR empilées, sauf mention contraire de la mission.

## Conséquences

**Positives** : missions isolées et révisables en ~10 minutes, `dev`
toujours intégrable, `main` toujours publiable, historique lisible (une
mission = un squash sur `dev`).

**Négatives** : coût de gestion de branches (rebase fréquent sur `dev`) ;
une mission ne peut pas être « presque finie » sur `dev`.

**Risques** : des branches qui vivent trop longtemps divergent de `dev`.
Mitigation : petits commits verts, push régulier, PR en draft dès
l'ouverture.
