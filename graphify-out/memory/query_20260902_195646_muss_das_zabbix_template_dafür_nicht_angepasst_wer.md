---
type: "query"
date: "2026-09-02T19:56:46.967622+00:00"
question: "Muss das Zabbix Template dafür nicht angepasst werden?"
contributor: "graphify"
source_nodes: ["Job", "StatusFromRaw()", "Zabbix Value Export"]
---

# Q: Muss das Zabbix Template dafür nicht angepasst werden?

## Answer

Expanded from original query via graph vocabulary: [zabbix, template, status, warning, trigger, value, export, job, sender, api]. Nein: Die Anwendung normalisiert ABB-Teilerfolg auf Status 2. JobField exportiert job.Status unverändert, und sowohl Pull- als auch Sender-Template besitzen bereits einen Warning-Trigger auf Status 2 sowie das passende Value Mapping. Eine Template-Anpassung wäre nur nötig, wenn der rohe ABB-Wert 8 direkt an Zabbix übergeben würde; das tut die Anwendung nicht.

## Source Nodes

- Job
- StatusFromRaw()
- Zabbix Value Export