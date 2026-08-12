# vivarium-komodo

Monorepo.

Instances can federate signed public collaborator identities without sharing
accounts or credentials; see `docs/README.md` for the discovery and trust
contract.

```
apps/web    Next.js frontend (TypeScript, Tailwind)
apps/api    Go HTTP API
docs/       notes
```

## Getting started

```sh
bun install

bun dev          # frontend  → http://localhost:3000
bun run dev:api  # api       → http://localhost:8080/health
```
