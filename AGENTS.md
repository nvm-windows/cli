# Agent Development Guide

A file for [guiding coding agents](https://agents.md/).

## Setup

1. Incorporate the skills in .github/skills.
1. Run `git config core.hooksPath .githooks`.

## Documentation site languages

The public Docusaurus docs (locales, translation mirrors, i18n automation) live in the **`nvm-windows/docs`** repository — in this workspace: `docs_guide/docs/`.

To **add a new language** to the docs site, follow **`AGENTS.md`** in that repo (`docs_guide/docs/AGENTS.md` → section **Adding a new language**). Do not add locale files under `cli/`.

Quick pointer:

| Topic | Location |
|-------|----------|
| Full translation playbook | `docs_guide/docs/AGENTS.md` |
| Locale config (teams, paths) | `docs_guide/docs/.github/i18n-locales.json` |
| Copilot sync rules | `docs_guide/docs/.github/copilot-instructions.md` |
| English source docs | `docs_guide/docs/docs/` |
| Locale mirrors | `docs_guide/docs/i18n/<locale>/` |
