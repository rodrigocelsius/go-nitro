# CLAUDE.md

Read [`AGENTS.md`](AGENTS.md) (the contract) first, then [`AI.md`](AI.md) (checklist) and [`docs/ai/PLAYBOOK.en.md`](docs/ai/PLAYBOOK.en.md) (recipes).

- Create systems with the CLI (`nitrogen new` → `install` → `make:scaffold … --resource`), not by hand.
- Keep diffs minimal; don't refactor the framework (`pkg/`, `domain/`, `usecases/`) when building an app.
- Code, identifiers and commits in English; commit only when asked.
- Living references: `examples/v1/cleanarch-blinq` (SQL + BLinq), `examples/v1/cleanarch` (memory). CLI docs: `docs/guide/cli.md`.
