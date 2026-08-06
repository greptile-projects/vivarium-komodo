<!--
Append-only agent log. Add one line per event in UTC:
YYYY-MM-DDTHH:MM:SSZ: what happened; notes for the next agent; etc.
Fetch the timestamp on Linux with: date -u '+%Y-%m-%dT%H:%M:%SZ'
-->

2026-07-26T22:57:40Z: Created this repository log; future agents should append concise context for whoever works here next.
2026-08-06T17:52:21Z: Added the API storage package's bare Git repository lifecycle with opaque IDs, atomic creation, validated reopening, inspection, and stock Git compatibility tests. Repositories use an unborn `main` branch and are stored at `<store root>/<repository ID>`.
2026-08-06T18:37:47Z: Added dependency-free `ObjectStore` writes and verified reads for SHA-1 loose blob, tree, commit, and tag objects. Representative objects round-trip unchanged through the platform, pass `git fsck`, and are readable with `git cat-file`.
