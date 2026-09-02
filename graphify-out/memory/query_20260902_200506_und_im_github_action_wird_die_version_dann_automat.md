---
type: "query"
date: "2026-09-02T20:05:06.777319+00:00"
question: "und im github action wird die version dann automatisch hochgezählt?"
contributor: "graphify"
source_nodes: ["Automated SPK Release Workflow", "SPK Build Process"]
---

# Q: und im github action wird die version dann automatisch hochgezählt?

## Answer

Expanded from original query via graph vocabulary: [automated, build, package, release, spk, version, workflow]. No. The workflow reads the fixed VERSION value from Makefile and uses it unchanged for the SPK. Only the GitHub release tag is made unique automatically by appending the short commit SHA plus GitHub run number and attempt. To release a higher DSM package version, VERSION in Makefile must be changed explicitly.

## Source Nodes

- Automated SPK Release Workflow
- SPK Build Process