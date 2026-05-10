# WORKFLOW.md
# SQZARR - Current Agent Workflow

This file is the current execution guide for agents working in this repository.
Legacy planning artifacts have been removed from the active docs set.

## Canonical docs

Use these as source of truth in this order:
1. README.md (product behavior, install, operations)
2. docs/architecture.md (current system shape)
3. docs/container-deployment.md (GPU/container runtime specifics)
4. sqzarr.toml.example (config keys/defaults)

If any doc disagrees with the code, treat code + tests as authoritative and update docs.

## Working norms

- Prefer Linux shell workflows and .sh scripts at repo root.
- Do not add Windows-only helper scripts (.bat/.ps1) unless explicitly requested.
- Keep changes small and focused; avoid speculative refactors.
- Preserve no-Docker runtime constraint.

## Build and verify

- Local Linux build: ./build_linux.sh
- Deploy flow: ./deploy.sh (requires DEPLOY_HOST)
- Convenience wrapper: ./build_now.sh
- Standard build/test path:
  - make all
  - make test

## Documentation policy

- Keep docs/ minimal and current.
- Add historical notes only when they are operationally useful.
- Avoid reintroducing large planning or interview transcripts into active docs.

## Change checklist for agents

Before finishing:
1. Confirm scripts/docs references are current.
2. Run targeted verification for touched areas.
3. Update README/docs when behavior or operator workflow changed.
4. Leave the repository cleaner than you found it.
