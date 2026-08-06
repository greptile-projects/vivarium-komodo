<!--
Append-only agent log. Add one line per event in UTC:
YYYY-MM-DDTHH:MM:SSZ: what happened; notes for the next agent; etc.
Fetch the timestamp on Linux with: date -u '+%Y-%m-%dT%H:%M:%SZ'
-->

2026-07-26T22:57:40Z: Created this repository log; future agents should append concise context for whoever works here next.
2026-08-06T17:52:21Z: Added the API storage package's bare Git repository lifecycle with opaque IDs, atomic creation, validated reopening, inspection, and stock Git compatibility tests. Repositories use an unborn `main` branch and are stored at `<store root>/<repository ID>`.
2026-08-06T18:37:47Z: Added dependency-free `ObjectStore` writes and verified reads for SHA-1 loose blob, tree, commit, and tag objects. Representative objects round-trip unchanged through the platform, pass `git fsck`, and are readable with `git cat-file`.
2026-08-06T19:27:37Z: Added deterministic enumeration of all verified loose objects with identity, type, size, and exact content. Compatibility coverage proves platform and stock-Git writes produce the same complete set as `git cat-file --batch-all-objects`.
2026-08-06T19:45:54Z: Added `ReferenceStore` CRUD for verified direct and symbolic Git references, default-branch management through symbolic `HEAD`, deterministic enumeration, and packed-ref interoperability. Stock Git compatibility tests cover resolving, packing, updating, and deleting platform-managed references.
2026-08-06T20:28:58Z: Added `GraphStore` readers for typed tree entries and commit snapshot/parent links, enabling recursive snapshot and merge-ancestry traversal. Compatibility tests cover nested executable content, common `git cat-file` operations, and stock `git log` over branched history.
2026-08-06T21:12:23Z: Established `RepositoryStorage` as the complete open-repository contract and documented every lifecycle, object, graph, reference, and default-branch operation. An interface-only compatibility fixture with nested content, merged history, direct and symbolic refs, annotated and lightweight tags, and an unreachable object passes `git fsck --full`.
2026-08-06T21:29:12Z: Added read-only Git smart HTTP at `/repositories/{ID}` using protocol-aware stock `git upload-pack`; `git ls-remote` now succeeds for empty repositories and advertises refs plus the configured default branch for populated repositories. The API uses `$REPOSITORY_ROOT` (default `apps/api/repositories` from the root dev command) and requires `git` on `PATH`.
2026-08-06T22:10:37Z: Proved stock `git clone` over smart HTTP for empty and populated repositories, including unborn default-branch selection, primary-branch checkout, complete reachable history and objects, nested files, and executable modes. Read access continues to use stock `git upload-pack`; write operations remain unsupported.
2026-08-06T22:34:09Z: Proved existing stock Git working copies can incrementally fetch newly reachable objects and updated remote-tracking refs, then fast-forward the primary branch and working tree with `git pull` after repeated server advances. The read-only smart-HTTP implementation continues to rely on stock `git upload-pack` negotiation.
