---
type: "query"
date: "2026-09-02T20:03:31.852847+00:00"
question: "Wie kann ich denn lokal eine bestimmte Version builden vom spk?"
contributor: "graphify"
source_nodes: ["SPK Build Process", "build_one()", "build-spk.sh script"]
---

# Q: Wie kann ich denn lokal eine bestimmte Version builden vom spk?

## Answer

Expanded from original query via graph vocabulary: [build, spk, version, package, release, architecture, script, synology, local]. The Makefile forwards VERSION to packaging/scripts/build-spk.sh. Running make spk VERSION=X builds Linux amd64 and arm64 binaries, injects X into the binary and SPK INFO metadata, and writes versioned x86_64 and aarch64 packages to dist. To build an exact historical source revision without disturbing a dirty worktree, create a separate git worktree at the desired tag or commit and run make spk there.

## Source Nodes

- SPK Build Process
- build_one()
- build-spk.sh script