---
type: "query"
date: "2026-09-02T19:53:26.575576+00:00"
question: "How does ABB partial status flow to the Zabbix warning trigger?"
contributor: "graphify"
source_nodes: ["Collector", "structuredABBDeviceQuery()", "readStructuredRuns()", "StatusFromRaw()", "StatusName()"]
---

# Q: How does ABB partial status flow to the Zabbix warning trigger?

## Answer

Expanded from original query via graph vocabulary: [abb, status, warning, zabbix, collector, backup, partial, success, device, result, sqlite, query]. The ABB Collector owns collectStructured, structuredABBDeviceQuery, readStructuredRuns, and StatusFromRaw. The structured query reads per-device SQLite results, readStructuredRuns materializes the raw status, and StatusFromRaw normalizes it before StatusName and downstream Zabbix value export consume the Job status. The fix therefore belongs at the device-result join and ABB raw-status normalization boundaries.

## Source Nodes

- Collector
- structuredABBDeviceQuery()
- readStructuredRuns()
- StatusFromRaw()
- StatusName()