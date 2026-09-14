# Graph Report - .  (2026-09-02)

## Corpus Check
- 79 files · ~98,159 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1272 nodes · 2831 edges · 65 communities (50 shown, 15 thin omitted)
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 362 edges (avg confidence: 0.81)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_TLS Connection Tests|TLS Connection Tests]]
- [[_COMMUNITY_TLS Server Test Suite|TLS Server Test Suite]]
- [[_COMMUNITY_TLS Record Primitives|TLS Record Primitives]]
- [[_COMMUNITY_Certificate Authentication|Certificate Authentication]]
- [[_COMMUNITY_TLS Server Handshake|TLS Server Handshake]]
- [[_COMMUNITY_Shared Runtime Types|Shared Runtime Types]]
- [[_COMMUNITY_Cipher Implementations|Cipher Implementations]]
- [[_COMMUNITY_Key Exchange|Key Exchange]]
- [[_COMMUNITY_Connection Error Handling|Connection Error Handling]]
- [[_COMMUNITY_Code Generation Helpers|Code Generation Helpers]]
- [[_COMMUNITY_HTTP API Server|HTTP API Server]]
- [[_COMMUNITY_Packaging and Configuration|Packaging and Configuration]]
- [[_COMMUNITY_DSM Web Interface|DSM Web Interface]]
- [[_COMMUNITY_ABB Data Collection|ABB Data Collection]]
- [[_COMMUNITY_Collector State Store|Collector State Store]]
- [[_COMMUNITY_TLS PSK Dialing|TLS PSK Dialing]]
- [[_COMMUNITY_TLS Dynamic Record Tests|TLS Dynamic Record Tests]]
- [[_COMMUNITY_Application Configuration|Application Configuration]]
- [[_COMMUNITY_M365 Data Collection|M365 Data Collection]]
- [[_COMMUNITY_TLS 1.3 Client|TLS 1.3 Client]]
- [[_COMMUNITY_TLS 1.3 Server|TLS 1.3 Server]]
- [[_COMMUNITY_TLS Client Authentication|TLS Client Authentication]]
- [[_COMMUNITY_Signature Validation Tests|Signature Validation Tests]]
- [[_COMMUNITY_Handshake Message Encoding|Handshake Message Encoding]]
- [[_COMMUNITY_TLS Client Handshake|TLS Client Handshake]]
- [[_COMMUNITY_Zabbix Value Export|Zabbix Value Export]]
- [[_COMMUNITY_ABB SQLite Queries|ABB SQLite Queries]]
- [[_COMMUNITY_TLS Message Builder|TLS Message Builder]]
- [[_COMMUNITY_SQLite Schema Utilities|SQLite Schema Utilities]]
- [[_COMMUNITY_Certificate Parsing|Certificate Parsing]]
- [[_COMMUNITY_DSM CGI Proxy|DSM CGI Proxy]]
- [[_COMMUNITY_Handshake Message Codec|Handshake Message Codec]]
- [[_COMMUNITY_TLS Key Schedule|TLS Key Schedule]]
- [[_COMMUNITY_Collector Domain Model|Collector Domain Model]]
- [[_COMMUNITY_Status Mapping Tests|Status Mapping Tests]]
- [[_COMMUNITY_TLS Session Tickets|TLS Session Tickets]]
- [[_COMMUNITY_Issue Governance|Issue Governance]]
- [[_COMMUNITY_Key Schedule Tests|Key Schedule Tests]]
- [[_COMMUNITY_Synology Database Paths|Synology Database Paths]]
- [[_COMMUNITY_Certificate Message Codec|Certificate Message Codec]]
- [[_COMMUNITY_Package Icon Semantics|Package Icon Semantics]]
- [[_COMMUNITY_Package Icon Semantics|Package Icon Semantics]]
- [[_COMMUNITY_Package Icon Semantics|Package Icon Semantics]]
- [[_COMMUNITY_Package Icon Semantics|Package Icon Semantics]]
- [[_COMMUNITY_Package Icon Semantics|Package Icon Semantics]]
- [[_COMMUNITY_Package Icon Semantics|Package Icon Semantics]]
- [[_COMMUNITY_Package Icon Semantics|Package Icon Semantics]]
- [[_COMMUNITY_Package Icon Semantics|Package Icon Semantics]]
- [[_COMMUNITY_Package Icon Semantics|Package Icon Semantics]]
- [[_COMMUNITY_TLS Alert Formatting|TLS Alert Formatting]]
- [[_COMMUNITY_Ticket Encryption|Ticket Encryption]]
- [[_COMMUNITY_Encrypted Extensions Codec|Encrypted Extensions Codec]]
- [[_COMMUNITY_Early Data Codec|Early Data Codec]]
- [[_COMMUNITY_Finished Message Codec|Finished Message Codec]]
- [[_COMMUNITY_Key Update Codec|Key Update Codec]]
- [[_COMMUNITY_Session Ticket Codec|Session Ticket Codec]]
- [[_COMMUNITY_Server Hello Codec|Server Hello Codec]]
- [[_COMMUNITY_SPK Build Script|SPK Build Script]]
- [[_COMMUNITY_Client Authentication Types|Client Authentication Types]]
- [[_COMMUNITY_Elliptic Curve IDs|Elliptic Curve IDs]]
- [[_COMMUNITY_PSK Configuration|PSK Configuration]]
- [[_COMMUNITY_Signature Scheme IDs|Signature Scheme IDs]]
- [[_COMMUNITY_Generated Placeholder|Generated Placeholder]]

## God Nodes (most connected - your core abstractions)
1. `T` - 65 edges
2. `T` - 57 edges
3. `Conn` - 52 edges
4. `Collector` - 38 edges
5. `localPipe()` - 34 edges
6. `Client()` - 33 edges
7. `Config` - 32 edges
8. `Collector` - 30 edges
9. `runClientTestTLS12()` - 30 edges
10. `Server` - 28 edges

## Surprising Connections (you probably didn't know these)
- `run()` --calls--> `WriteDefault()`  [INFERRED]
  cmd/synology-activebackup-zabbix/main.go → internal/config/config.go
- `runService()` --calls--> `NewStore()`  [INFERRED]
  cmd/synology-activebackup-zabbix/main.go → internal/collector/store.go
- `runService()` --calls--> `Duration`  [INFERRED]
  cmd/synology-activebackup-zabbix/main.go → third_party/go-tls-psk/tls_test.go
- `collectAndStore()` --calls--> `SendSnapshot()`  [INFERRED]
  cmd/synology-activebackup-zabbix/main.go → internal/zabbix/sender.go
- `cmdDiscovery()` --calls--> `DiscoveryJSON()`  [INFERRED]
  cmd/synology-activebackup-zabbix/main.go → internal/zabbix/zabbix.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Dual Zabbix Monitoring Modes** — readme_api_pull_mode, readme_sender_push_mode, readme_token_protected_http_api, zabbix_template_synology_activebackup_zabbix_7_4_api_template, zabbix_template_synology_activebackup_zabbix_sender_7_4_sender_template [EXTRACTED 1.00]
- **Validated Cross-Architecture Release Pipeline** — workflows_release_automated_spk_release, workflows_release_release_validation, workflows_release_cross_architecture_spk_build, docs_build_build_process [EXTRACTED 1.00]
- **DSM Secure Local UI Flow** — readme_dsm_desktop_app, docs_security_dsm_authenticated_cgi_proxy, docs_security_local_only_sender_api, web_index_desktop_status_interface [EXTRACTED 1.00]
- **Backup Monitoring Symbolism** — synology_package_icon_storage_device, synology_package_icon_backup_cycle, synology_package_icon_success_status [EXTRACTED 1.00]
- **NAS Backup Monitoring Workflow** — synology_package_icon_256_nas_device, synology_package_icon_256_backup_cycle, synology_package_icon_256_monitoring_signal, synology_package_icon_256_success_status [INFERRED 0.95]
- **Backup Monitoring Symbolism** — images_icon_16_storage_device, images_icon_16_backup_cycle, images_icon_16_success_status [EXTRACTED 1.00]
- **NAS Backup Monitoring Workflow** — images_icon_24_nas_device, images_icon_24_backup_cycle, images_icon_24_monitoring_signal, images_icon_24_success_status [INFERRED 0.95]
- **Healthy Backup Monitoring Symbolism** — images_icon_256_nas_device, images_icon_256_backup_cycle, images_icon_256_monitoring_pulse, images_icon_256_success_status [EXTRACTED 1.00]
- **NAS Backup Monitoring Workflow** — images_icon_32_nas_device, images_icon_32_backup_cycle, images_icon_32_monitoring_signal, images_icon_32_success_status [INFERRED 0.95]
- **Healthy Backup Monitoring Symbolism** — images_icon_48_nas_device, images_icon_48_backup_cycle, images_icon_48_monitoring_pulse, images_icon_48_success_status [EXTRACTED 1.00]
- **NAS Backup Monitoring Workflow** — images_icon_64_nas_device, images_icon_64_backup_cycle, images_icon_64_monitoring_signal, images_icon_64_success_status [INFERRED 0.95]
- **Healthy Backup Monitoring Symbolism** — images_icon_72_nas_device, images_icon_72_backup_cycle, images_icon_72_monitoring_pulse, images_icon_72_success_status [EXTRACTED 1.00]

## Communities (65 total, 15 thin omitted)

### Community 0 - "TLS Connection Tests"
Cohesion: 0.06
Nodes (84): brokenConn, clientTest, NewLRUClientSessionCache(), newOpensslOutputSink(), peekError(), runClientTestForVersion(), runClientTestTLS10(), runClientTestTLS11() (+76 more)

### Community 1 - "TLS Server Test Suite"
Cohesion: 0.08
Nodes (72): expectError(), runServerTestForVersion(), runServerTestTLS10(), runServerTestTLS11(), runServerTestTLS12(), runServerTestTLS13(), TestAESCipherReordering(), TestAESCipherReorderingTLS13() (+64 more)

### Community 2 - "TLS Record Primitives"
Cohesion: 0.06
Nodes (27): alert, BlockMode, Buffer, Error, atLeastReader, cbcMode, extractPadding(), roundUp() (+19 more)

### Community 3 - "Certificate Authentication"
Cohesion: 0.06
Nodes (39): CertPool, Element, selectSignatureScheme(), signatureSchemesForCertificate(), unsupportedCertificateError(), Certificate, CertificateRequestInfo, ClientHelloInfo (+31 more)

### Community 4 - "TLS Server Handshake"
Cohesion: 0.06
Nodes (37): ClientHelloInfo, supportedVersionsFromMax(), finishedHash, clientHelloInfo(), negotiateALPN(), supportsECDHE(), ekmFromMasterSecret(), keysFromMasterSecret() (+29 more)

### Community 5 - "Shared Runtime Types"
Cohesion: 0.10
Nodes (49): Duration, Config, Conn, Context, Reader, Snapshot, Time, ZabbixSenderConfig (+41 more)

### Community 6 - "Cipher Implementations"
Cohesion: 0.07
Nodes (30): aead, aead, aeadAESGCM(), aeadAESGCMTLS13(), aeadChaCha20Poly1305(), cipherSuiteByID(), CipherSuiteName(), CipherSuites() (+22 more)

### Community 7 - "Key Exchange"
Cohesion: 0.10
Nodes (26): clientKeyExchangeMsg, Curve, ecdheKeyAgreement, ecdheParameters, ecdhePskKeyAgreement, main(), publicKey(), hashForServerKeyExchange() (+18 more)

### Community 8 - "Connection Error Handling"
Cohesion: 0.11
Nodes (38): brokenSigner, changeImplConn, readerFunc, slowConn, BenchmarkLatency(), BenchmarkThroughput(), isTimeoutError(), latency() (+30 more)

### Community 9 - "Code Generation Helpers"
Cohesion: 0.09
Nodes (26): randomBytes(), randomString(), TestFuzz(), TestMarshalUnmarshal(), TestRejectEmptySCT(), TestRejectEmptySCTList(), Rand, certificateMsg (+18 more)

### Community 10 - "HTTP API Server"
Cohesion: 0.13
Nodes (26): Server, configsEqual(), New(), writeError(), writeJSON(), writeRaw(), writeText(), DiscoveryEntry (+18 more)

### Community 11 - "Packaging and Configuration"
Cohesion: 0.06
Nodes (41): SPK Build Process, DSM Package Update Identity, Microsoft 365 Name Redaction, Restart-Required Configuration, Runtime Configuration, Sender TLS Modes, DSM-Authenticated CGI Proxy, Local-Only API in Sender Mode (+33 more)

### Community 12 - "DSM Web Interface"
Cohesion: 0.11
Nodes (38): apiEndpoint(), appParams, attachSecretToggles(), checkboxField(), delay(), escapeHtml(), eyeIcon(), findSynoToken() (+30 more)

### Community 13 - "ABB Data Collection"
Cohesion: 0.13
Nodes (35): abbDBSet, Collector, choose(), chooseSizeColumn(), findABBDBSets(), findTableInfo(), genericRunTime(), jobName() (+27 more)

### Community 14 - "Collector State Store"
Cohesion: 0.16
Nodes (32): Config, Context, Logger, Snapshot, Store, ReadCache(), WriteCache(), Store (+24 more)

### Community 15 - "TLS PSK Dialing"
Cohesion: 0.10
Nodes (23): Dialer, defaultConfig(), Dialer, ExampleConfig_keyLogWriter(), ExampleLoadX509KeyPair(), ExampleX509KeyPair(), ExampleX509KeyPair_httpServer(), listener (+15 more)

### Community 16 - "TLS Dynamic Record Tests"
Cohesion: 0.09
Nodes (28): runDynamicRecordSizingTest(), TestCertificateSelection(), TestDynamicRecordSizingWithAEAD(), TestDynamicRecordSizingWithCBC(), TestDynamicRecordSizingWithStreamCipher(), TestDynamicRecordSizingWithTLSv13(), TestHairpinInClose(), TestRemovePadding() (+20 more)

### Community 17 - "Application Configuration"
Cohesion: 0.10
Nodes (23): APIConfig, CollectorConfig, APIConfig, CollectorConfig, Config, Default(), Load(), Write() (+15 more)

### Community 18 - "M365 Data Collection"
Cohesion: 0.19
Nodes (24): ColumnInfo, DB, Logger, TableInfo, Collector, chooseM365JSONSizeColumns(), chooseM365SizeColumns(), chooseM365TaskIDColumn() (+16 more)

### Community 19 - "TLS 1.3 Client"
Cohesion: 0.12
Nodes (14): certificateRequestMsgTLS13, aesgcmPreferred(), cipherSuiteTLS13ByID(), mutualCipherSuiteTLS13(), clientHandshakeStateTLS13, checkALPN(), cipherSuiteTLS13, clientHelloMsg (+6 more)

### Community 20 - "TLS 1.3 Server"
Cohesion: 0.14
Nodes (12): cloneHash(), illegalClientHelloChange(), serverHandshakeStateTLS13, Certificate, cipherSuiteTLS13, clientHelloMsg, Conn, Context (+4 more)

### Community 21 - "TLS Client Authentication"
Cohesion: 0.11
Nodes (16): CertificateRequestInfo, certificateRequestMsg, certificateRequestInfoFromMsg(), clientSessionCacheKey(), hostnameInSNI(), peerCertificate(), newSessionTicketMsgTLS13, Addr (+8 more)

### Community 22 - "Signature Validation Tests"
Cohesion: 0.22
Nodes (11): legacyTypeAndHashFromPublicKey(), signedMessage(), TestLegacyTypeAndHash(), TestSignatureSelection(), TestSupportedSignatureAlgorithms(), typeAndHashFromSignatureScheme(), verifyHandshakeSignature(), PublicKey (+3 more)

### Community 23 - "Handshake Message Encoding"
Cohesion: 0.12
Nodes (6): helloRequestMsg, serverKeyExchangeMsg, certificateMsg, certificateStatusMsg, clientKeyExchangeMsg, newSessionTicketMsgTLS13

### Community 24 - "TLS Client Handshake"
Cohesion: 0.21
Nodes (6): clientHandshakeState, unexpectedMessageError(), cipherSuite, Conn, Context, serverHelloMsg

### Community 25 - "Zabbix Value Export"
Cohesion: 0.18
Nodes (12): Logger, Context, Job, Result, Time, New(), Redact(), latestRun() (+4 more)

### Community 26 - "ABB SQLite Queries"
Cohesion: 0.27
Nodes (12): attachReadOnly(), readRunningDeviceIDs(), readStructuredRuns(), Context, DB, Time, AgeSeconds(), Int64Value() (+4 more)

### Community 27 - "TLS Message Builder"
Cohesion: 0.19
Nodes (8): Builder, addBytesWithLength(), marshalingFunction, keyShare, pskIdentity, CurveID, clientHelloMsg, serverHelloMsg

### Community 28 - "SQLite Schema Utilities"
Cohesion: 0.33
Nodes (13): ColumnInfo, DB, ColumnInfo, HasTable(), ListColumns(), ListColumnsInSchema(), ListTables(), ListTablesInSchema() (+5 more)

### Community 29 - "Certificate Parsing"
Cohesion: 0.35
Nodes (6): readUint16LengthPrefixed(), readUint24LengthPrefixed(), readUint64(), readUint8LengthPrefixed(), unmarshalCertificate(), String

### Community 30 - "DSM CGI Proxy"
Cohesion: 0.36
Nodes (9): cgiPath(), isDSMUserAuthenticated(), isDSMWebAPIAuthenticated(), proxyRequest(), Run(), synoTokenFromQuery(), writeCGI(), Config (+1 more)

### Community 31 - "Handshake Message Codec"
Cohesion: 0.24
Nodes (4): certificateRequestMsg, certificateRequestMsgTLS13, certificateVerifyMsg, SignatureScheme

### Community 33 - "Collector Domain Model"
Cohesion: 0.57
Nodes (6): Health, Job, Result, Snapshot, Source, Time

### Community 34 - "Status Mapping Tests"
Cohesion: 0.57
Nodes (6): StatusFromRaw(), TestStatusFromRawABBPartialSuccessIsWarning(), TestStatusFromRawABBSuccessfulIsOK(), TestStatusFromRawM365Failed(), TestStatusFromRawM365SkippedItemsAreWarning(), T

### Community 35 - "TLS Session Tickets"
Cohesion: 0.33
Nodes (4): addUint64(), Certificate, sessionState, sessionStateTLS13

### Community 36 - "Issue Governance"
Cohesion: 0.33
Nodes (6): Structured Secret-Safe Logging, Bug Report Form, Secret Sanitization Checklist, GitHub Issue Intake Policy, Feature Request Form, Synology Data and Maintainer-Time Constraints

### Community 37 - "Key Schedule Tests"
Cohesion: 0.67
Nodes (5): parseVector(), TestDeriveSecret(), TestExtract(), TestTrafficKey(), T

### Community 38 - "Synology Database Paths"
Cohesion: 0.67
Nodes (5): ExistingPaths(), existingUnique(), ExpandScanPaths(), FindM365LogDBs(), FindSQLiteDBs()

### Community 39 - "Certificate Message Codec"
Cohesion: 0.50
Nodes (3): marshalCertificate(), Certificate, certificateMsgTLS13

### Community 40 - "Package Icon Semantics"
Cohesion: 0.70
Nodes (5): Backup Cycle, Monitoring Signal, NAS Device, Synology Active Backup Zabbix Monitor 24-Pixel Icon, Successful Status

### Community 41 - "Package Icon Semantics"
Cohesion: 0.50
Nodes (5): Synology Active Backup Zabbix Monitor 256-Pixel Icon, Backup or Synchronization Cycle, Monitoring Pulse, NAS Storage Device, Successful Health Status

### Community 42 - "Package Icon Semantics"
Cohesion: 0.70
Nodes (5): Backup Cycle, Monitoring Signal, NAS Device, Synology Active Backup Zabbix Monitor 32-Pixel Icon, Successful Status

### Community 43 - "Package Icon Semantics"
Cohesion: 0.50
Nodes (5): Synology Active Backup Zabbix Monitor 48-Pixel Icon, Backup or Synchronization Cycle, Monitoring Pulse, NAS Storage Device, Successful Health Status

### Community 44 - "Package Icon Semantics"
Cohesion: 0.70
Nodes (5): Backup Cycle, Monitoring Signal, NAS Device, Synology Active Backup Zabbix Monitor 64-Pixel Icon, Successful Status

### Community 45 - "Package Icon Semantics"
Cohesion: 0.50
Nodes (5): Synology Active Backup Zabbix Monitor 72-Pixel Icon, Backup or Synchronization Cycle, Monitoring Pulse, NAS Storage Device, Successful Health Status

### Community 46 - "Package Icon Semantics"
Cohesion: 0.70
Nodes (5): Backup Cycle, Monitoring Signal, NAS Device, Synology Active Backup Zabbix Monitor Package Icon, Successful Status

### Community 47 - "Package Icon Semantics"
Cohesion: 0.50
Nodes (4): Synology Active Backup Zabbix Monitor 16-Pixel Icon, Backup or Synchronization Cycle, Storage Device, Successful Health Status

### Community 48 - "Package Icon Semantics"
Cohesion: 0.50
Nodes (4): Synology Active Backup Zabbix Monitor Package Icon, Backup or Synchronization Cycle, Storage Device, Successful Health Status

## Knowledge Gaps
- **172 isolated node(s):** `Store`, `Source`, `Logger`, `Result`, `RWMutex` (+167 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **15 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `unexpectedMessageError()` connect `TLS Client Handshake` to `TLS Record Primitives`, `Certificate Authentication`, `TLS Server Handshake`, `Key Exchange`, `TLS 1.3 Client`, `TLS 1.3 Server`, `Signature Validation Tests`?**
  _High betweenness centrality (0.066) - this node is a cross-community bridge._
- **Why does `cipherSuiteTLS13ByID()` connect `TLS 1.3 Client` to `TLS Record Primitives`, `Cipher Implementations`, `Key Exchange`, `TLS 1.3 Server`, `TLS Client Authentication`?**
  _High betweenness centrality (0.062) - this node is a cross-community bridge._
- **Are the 30 inferred relationships involving `localPipe()` (e.g. with `.run()` and `runDynamicRecordSizingTest()`) actually correct?**
  _`localPipe()` has 30 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Store`, `Source`, `Logger` to the rest of the system?**
  _173 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `TLS Connection Tests` be split into smaller, more focused modules?**
  _Cohesion score 0.06131320064058568 - nodes in this community are weakly interconnected._
- **Should `TLS Server Test Suite` be split into smaller, more focused modules?**
  _Cohesion score 0.07518796992481203 - nodes in this community are weakly interconnected._
- **Should `TLS Record Primitives` be split into smaller, more focused modules?**
  _Cohesion score 0.05693693693693694 - nodes in this community are weakly interconnected._