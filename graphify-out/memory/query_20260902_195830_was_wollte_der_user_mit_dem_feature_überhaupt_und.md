---
type: "query"
date: "2026-09-02T19:58:30.220790+00:00"
question: "Was wollte der User mit dem Feature überhaupt und wurde es so umgesetzt, wie er wollte?"
contributor: "graphify"
source_nodes: ["Collector", "structuredABBDeviceQuery()", "readStructuredRuns()", "StatusFromRaw()"]
---

# Q: Was wollte der User mit dem Feature überhaupt und wurde es so umgesetzt, wie er wollte?

## Answer

Expanded from original query via graph vocabulary: [abb, status, warning, zabbix, collector, backup, partial, success, device, result, sqlite, test]. The requested core behavior is that an ABB run shown by DSM as not fully completed must no longer be exported as OK and must activate the existing Zabbix warning trigger. The implementation addresses the likely status-code mapping and cross-device SQL join causes and has automated regression coverage. It is logically implemented but not yet end-to-end accepted because no real affected DSM database or rerun was available. The optional next-run timestamp was not implemented.

## Source Nodes

- Collector
- structuredABBDeviceQuery()
- readStructuredRuns()
- StatusFromRaw()