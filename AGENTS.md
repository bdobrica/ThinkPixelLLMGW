# AGENTS.md

This repository is one component of the modular, vendor-neutral **ThinkPixel** platform. Make the smallest coherent change that preserves the repository's ownership boundary and the cross-component security model.

## Read before changing

1. Read the relevant accepted records in `docs/adr/` and the affected contracts/API/security documentation.
2. Use `PLAN.md` and `TODO.md` for current implementation intent when they exist.
3. Treat `README.md` as orientation, not as the normative architecture specification.

When sources conflict, use this order for intended behavior:

**accepted ADRs → versioned contracts/API schemas → normative security/architecture docs → PLAN.md → TODO.md → README.md**

Code and tests are implementation evidence. Do not silently weaken an accepted contract merely to match current code.

## Engineering rules

- Preserve the component boundary described in `ALIGNMENT.md`; do not absorb another ThinkPixel component's responsibilities for convenience.
- Keep integrations replaceable. Put provider-, harness-, storage-, policy-, and ThinkPixel-specific behavior behind explicit ports/adapters.
- Do not create direct cross-repository database access or depend on another repository's `internal` implementation types.
- Cross-component behavior must use a versioned wire/schema contract and stable identifiers.
- Do not let Skills, marketplace metadata, Workspace membership, memory, model output, or guardrail results expand Run authority.
- Keep credentials and long-lived secrets outside untrusted agent/harness state. Never commit secrets or sensitive runtime payloads to documentation/evidence.
- Treat caches, indexes, and projections as non-authoritative unless an accepted ADR explicitly says otherwise.
- Public API/schema changes require corresponding contract, compatibility, documentation, and test updates.
- Accepted ADRs are immutable in meaning. Supersede them with a new ADR instead of rewriting history.
- Avoid speculative abstractions and unrelated refactors. New dependencies need a concrete repository-local justification.

## Repository and documentation hygiene

- Keep the root `README.md` concise: purpose, status, quick start, key concepts, and links to durable docs.
- Do not duplicate `PLAN.md` in the README. Move durable implemented decisions into `docs/adr/` and durable reference material into `docs/`.
- Prefer Mermaid for architecture diagrams and relative links for repository-local documentation.
- Use RFC 2119/8174 normative terms only when a requirement is intentional.
- Keep current implementation sequencing in `PLAN.md`/`TODO.md`; keep release evidence in `docs/evidence/` or the repository's existing evidence area.

## Verification

- Use the repository's documented developer commands. Prefer the root aggregate verification target (for example `make verify`) when one exists.
- Run focused tests first, then the broadest practical repository gate for the change.
- Do not claim a test, migration, generator, live-provider check, or deployment check was run when it was not.
- If a change alters a ThinkPixel integration boundary or shared convention, update `ALIGNMENT.md` and the relevant contract/ADR in the same change.
