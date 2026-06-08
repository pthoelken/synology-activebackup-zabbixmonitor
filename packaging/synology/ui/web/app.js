const state = {
  snapshot: null,
  config: null,
  senderLog: null,
  configNotice: "",
};

const apiBase = "../api.cgi";
const appParams = new URLSearchParams(window.location.search);
let synoToken = appParams.get("SynoToken") || appParams.get("synotoken") || "";

function findSynoToken() {
  if (synoToken) {
    return synoToken;
  }
  const roots = [];
  try { roots.push(window.parent); } catch {}
  try { roots.push(window.top); } catch {}
  try { roots.push(window); } catch {}

  for (const root of roots) {
    if (!root) continue;
    try {
      if (typeof root.SynoToken === "string" && root.SynoToken) return root.SynoToken;
      if (typeof root.synotoken === "string" && root.synotoken) return root.synotoken;
      const session = root.SYNO && root.SYNO.SDS && root.SYNO.SDS.Session;
      if (!session) continue;
      if (typeof session.get === "function") {
        const fromGetter = session.get("SynoToken") || session.get("synotoken") || session.get("synoToken");
        if (typeof fromGetter === "string" && fromGetter) return fromGetter;
      }
      for (const key of ["SynoToken", "synotoken", "synoToken"]) {
        if (typeof session[key] === "string" && session[key]) return session[key];
      }
    } catch {
      // Parent access can fail on cross-origin setups; just try the next source.
    }
  }
  return "";
}

function isEmbeddedInDSM() {
  try {
    return window.top !== window;
  } catch {
    return window.top !== window;
  }
}

function renderDSMOnlyMessage() {
  document.body.innerHTML = `<main class="direct-access">
    <div>
      <img src="../images/icon_64.png" alt="">
      <h1>Active Backup Zabbix</h1>
      <p>Diese Ansicht kann nur aus dem DSM-Desktop gestartet werden.</p>
    </div>
  </main>`;
}

const statusNames = {
  1: "OK",
  2: "Warning",
  3: "Running",
  6: "Failed",
  8: "No data",
  9: "DB missing",
  10: "Unknown",
};

function escapeHtml(value) {
  return String(value ?? "").replace(/[&<>"']/g, function(char) {
    return ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;" })[char];
  });
}

function formatTime(value) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString();
}

function statusClass(status) {
  if (status === 1 || status === 3) return "ok";
  if (status === 6 || status === 10) return "bad";
  return "warn";
}

function statusPill(status) {
  const label = statusNames[status] || "Unknown";
  const cls = statusClass(status);
  return `<span class="pill ${cls}">${escapeHtml(label)}</span>`;
}

async function requestJSON(path, options) {
  return requestJSONAttempt(path, options, 0);
}

async function requestJSONAttempt(path, options, attempt) {
  synoToken = findSynoToken();
  const params = new URLSearchParams();
  params.set("t", String(Date.now()));
  if (synoToken) {
    params.set("SynoToken", synoToken);
  }
  const separator = path.includes("?") ? "&" : "?";
  const response = await fetch(`${apiBase}/${path}${separator}${params.toString()}`, Object.assign({
    credentials: "same-origin",
  }, options || {}));
  const body = await response.text();
  if (!response.ok) {
    let message = body.trim();
    try {
      message = JSON.parse(body).error || message;
    } catch {
      // Keep the plain text response.
    }
    if (attempt < 3 && /DSM authentication required/i.test(message)) {
      await delay(400 + attempt * 300);
      return requestJSONAttempt(path, options, attempt + 1);
    }
    throw new Error(message || `${path}: ${response.status}`);
  }
  if (!body.trim()) {
    return null;
  }
  try {
    return JSON.parse(body);
  } catch (err) {
    throw new Error(`${path}: ${err.message}`);
  }
}

function delay(ms) {
  return new Promise(function(resolve) {
    window.setTimeout(resolve, ms);
  });
}

async function readJSON(path) {
  return requestJSON(path);
}

async function writeJSON(path, value) {
  return requestJSON(path, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(value),
  });
}

async function load() {
  try {
    state.snapshot = await readJSON("status");
  } catch (err) {
    state.snapshot = {
      collected_at: null,
      health: { ok: false, job_count: 0, collector_errors: [err.message] },
      jobs: [],
      sources: [],
      errors: [err.message],
    };
  }
  try {
    state.config = await readJSON("config");
  } catch (err) {
    state.config = null;
    state.configNotice = err.message;
  }
  try {
    state.senderLog = await readJSON("sender-log");
  } catch (err) {
    state.senderLog = { entries: [{ at: null, ok: false, error: err.message, values: [] }] };
  }
  renderAll();
}

function renderAll() {
  renderOverview();
  renderJobs();
  renderSources();
  renderSenderLog();
  renderConfig();
}

function renderOverview() {
  const snapshot = state.snapshot || {};
  const health = snapshot.health || {};
  const healthEl = document.getElementById("health");
  healthEl.textContent = health.ok ? "OK" : "Problem";
  healthEl.className = health.ok ? "ok" : "bad";
  document.getElementById("jobCount").textContent = health.job_count || 0;
  document.getElementById("sourceCount").textContent = Array.isArray(snapshot.sources) ? snapshot.sources.length : 0;
  document.getElementById("collected").textContent = formatTime(snapshot.collected_at);

  const errors = [];
  (health.db_missing || []).forEach(function(item) {
    errors.push(`DB missing: ${item}`);
  });
  (health.collector_errors || []).forEach(function(item) {
    errors.push(item);
  });
  (snapshot.errors || []).forEach(function(item) {
    if (!errors.includes(item)) errors.push(item);
  });

  if (!errors.length) {
    document.getElementById("errors").innerHTML = '<div class="empty">No errors</div>';
    return;
  }
  document.getElementById("errors").innerHTML = `<table><tbody>${errors.map(function(err) {
    return `<tr><td class="mono">${escapeHtml(err)}</td></tr>`;
  }).join("")}</tbody></table>`;
}

function renderJobs() {
  const jobs = state.snapshot && Array.isArray(state.snapshot.jobs) ? state.snapshot.jobs : [];
  if (!jobs.length) {
    document.getElementById("jobsTable").innerHTML = '<div class="empty">No jobs collected</div>';
    return;
  }
  document.getElementById("jobsTable").innerHTML = `<table>
    <thead><tr><th>Status</th><th>Product</th><th>ID</th><th>Name</th><th>Last end</th><th>Source</th></tr></thead>
    <tbody>${jobs.map(function(job) {
      return `<tr>
        <td>${statusPill(job.status)}</td>
        <td>${escapeHtml(job.product)}</td>
        <td class="mono">${escapeHtml(job.task_id)}</td>
        <td>${escapeHtml(job.job_name)}</td>
        <td>${escapeHtml(formatTime(job.end_time || job.last_success_time))}</td>
        <td class="mono">${escapeHtml(job.source_db)}</td>
      </tr>`;
    }).join("")}</tbody>
  </table>`;
}

function renderSources() {
  const sources = state.snapshot && Array.isArray(state.snapshot.sources) ? state.snapshot.sources : [];
  if (!sources.length) {
    document.getElementById("sourcesTable").innerHTML = '<div class="empty">No sources scanned</div>';
    return;
  }
  document.getElementById("sourcesTable").innerHTML = `<table>
    <thead><tr><th>State</th><th>Product</th><th>Kind</th><th>Path</th><th>Error</th></tr></thead>
    <tbody>${sources.map(function(source) {
      return `<tr>
        <td><span class="pill ${source.found ? "ok" : "warn"}">${source.found ? "Found" : "Missing"}</span></td>
        <td>${escapeHtml(source.product)}</td>
        <td>${escapeHtml(source.kind)}</td>
        <td class="mono">${escapeHtml(source.path)}</td>
        <td class="mono">${escapeHtml(source.error)}</td>
      </tr>`;
    }).join("")}</tbody>
  </table>`;
}

function renderSenderLog() {
  const target = document.getElementById("senderLogView");
  if (!target) return;
  const log = state.senderLog || {};
  const entries = Array.isArray(log.entries) ? log.entries : [];
  if (!entries.length) {
    target.innerHTML = '<div class="empty">No sender attempts logged yet</div>';
    return;
  }
  target.innerHTML = `<div class="log-list">${entries.map(function(entry, index) {
    const values = Array.isArray(entry.values) ? entry.values : [];
    const shown = values.length;
    const total = Number(entry.values_count || shown);
    const targetName = `${entry.server || "-"}:${entry.port || "-"}`;
    return `<details class="log-entry"${index === 0 ? " open" : ""}>
      <summary>
        <span class="pill ${entry.ok ? "ok" : "bad"}">${entry.ok ? "Sent" : "Failed"}</span>
        <span class="mono">${escapeHtml(formatTime(entry.at))}</span>
        <span>${escapeHtml(entry.host || "-")} -> ${escapeHtml(targetName)}</span>
        <span>${escapeHtml(total)} values</span>
      </summary>
      <div class="log-detail">
        <div class="log-meta">
          <span>TLS <strong>${escapeHtml(entry.tls || "none")}</strong></span>
          <span>Chunks <strong>${escapeHtml(entry.chunks || 0)}</strong></span>
          <span>Values <strong>${escapeHtml(total)}</strong></span>
          <span>Processed <strong>${escapeHtml(entry.processed || 0)}</strong></span>
          <span>Failed <strong>${escapeHtml(entry.failed || 0)}</strong></span>
        </div>
        ${entry.error ? `<div class="notice bad-text">${escapeHtml(entry.error)}</div>` : ""}
        ${Array.isArray(entry.infos) && entry.infos.length ? `<div class="notice">${entry.infos.map(escapeHtml).join("<br>")}</div>` : ""}
        ${renderSenderValues(values)}
        ${total > shown ? `<div class="empty">Showing ${escapeHtml(shown)} of ${escapeHtml(total)} values</div>` : ""}
      </div>
    </details>`;
  }).join("")}</div>`;
}

function renderSenderValues(values) {
  if (!values.length) {
    return '<div class="empty">No value preview available</div>';
  }
  return `<table class="sender-values">
    <thead><tr><th>Host</th><th>Key</th><th>Value</th></tr></thead>
    <tbody>${values.map(function(value) {
      return `<tr>
        <td class="mono">${escapeHtml(value.host)}</td>
        <td class="mono">${escapeHtml(value.key)}</td>
        <td class="mono value-cell">${escapeHtml(truncate(value.value, 800))}</td>
      </tr>`;
    }).join("")}</tbody>
  </table>`;
}

function truncate(value, limit) {
  const text = String(value ?? "");
  if (text.length <= limit) return text;
  return text.slice(0, limit) + "...";
}

function renderConfig() {
  const cfg = state.config;
  if (!cfg) {
    document.getElementById("configView").innerHTML = `<div class="empty">${escapeHtml(state.configNotice || "No config available")}</div>`;
    return;
  }
  const abbPaths = ((cfg.products || {}).active_backup_business || {}).scan_paths || [];
  const m365Paths = ((cfg.products || {}).active_backup_m365 || {}).scan_paths || [];
  const api = cfg.api || {};
  const collector = cfg.collector || {};
  const products = cfg.products || {};
  const abb = products.active_backup_business || {};
  const m365 = products.active_backup_m365 || {};
  const logging = cfg.logging || {};
  const privacy = cfg.privacy || {};
  const zabbix = cfg.zabbix || {};
  const sender = zabbix.sender || {};
  const endpoint = apiEndpoint(api);
  const mode = zabbix.mode || "api";
  const tlsMode = sender.tls || "none";

  document.getElementById("configView").innerHTML = `<form id="configForm" class="config-form">
    <div class="config-stack">
      <section class="config-section">
        <h3>Collector</h3>
        <div class="config-grid">
          ${numberField("collectorInterval", "Collector interval seconds", collector.interval_seconds || 300)}
          ${numberField("collectorMaxAge", "Max age hours", collector.max_age_hours || 30)}
          ${selectField("loggingLevel", "Logging level", logging.level || "info", ["debug", "info", "warn", "error"])}
        </div>
      </section>

      <section class="config-section">
        <h3>Zabbix</h3>
        <div class="config-grid">
          ${selectField("zabbixMode", "Mode", mode, [
            { value: "api", label: "API pull" },
            { value: "sender", label: "Sender push" },
          ])}
        </div>
      </section>

      <section class="config-section" data-mode-section="sender">
        <h3>Sender</h3>
        <div class="config-grid">
          ${textField("zabbixHost", "Zabbix host name", sender.host || "")}
          ${textField("zabbixServer", "Zabbix server or proxy", sender.server || "")}
          ${numberField("zabbixPort", "Zabbix trapper port", sender.port || 10051)}
          ${selectField("zabbixTLS", "TLS", tlsMode, [
            { value: "none", label: "None" },
            { value: "cert", label: "Certificate" },
            { value: "psk", label: "PSK" },
          ])}
          ${numberField("zabbixTimeout", "Timeout seconds", sender.timeout_seconds || 30)}
          ${numberField("zabbixChunkSize", "Chunk size", sender.chunk_size || 250)}
          <div class="config-group" data-tls-section="cert">
            <div class="config-group-title">Certificate TLS</div>
            ${textField("zabbixServerName", "TLS server name", sender.server_name || "")}
            ${textField("zabbixCAFile", "TLS CA file", sender.ca_file || "")}
            ${textField("zabbixCertFile", "TLS cert file", sender.cert_file || "")}
            ${textField("zabbixKeyFile", "TLS key file", sender.key_file || "")}
          </div>
          <div class="config-group" data-tls-section="psk">
            <div class="config-group-title">PSK</div>
            ${textField("zabbixPSKIdentity", "PSK identity", sender.psk_identity || "")}
            ${secretField("zabbixPSK", "PSK value", sender.psk || "")}
          </div>
        </div>
      </section>

      <section class="config-section" data-mode-section="api">
        <h3>API</h3>
        <div class="config-grid">
          ${checkboxField("apiEnabled", "API enabled", api.enabled !== false)}
          ${textField("apiBind", "Bind address", api.bind || "0.0.0.0")}
          ${numberField("apiPort", "Port", api.port || 9876)}
          ${secretField("apiToken", "Token", api.token || "")}
          <div class="api-summary field-wide">
            <span>API URL</span>
            <strong class="mono">${escapeHtml(endpoint)}</strong>
          </div>
        </div>
      </section>

      <section class="config-section">
        <h3>Products</h3>
        <div class="config-grid">
          ${checkboxField("redactNames", "Redact Microsoft 365 names", privacy.redact_names !== false)}
          ${checkboxField("abbEnabled", "Active Backup for Business", abb.enabled !== false)}
          ${checkboxField("m365Enabled", "Active Backup for Microsoft 365", m365.enabled !== false)}
          ${textareaField("abbPaths", "ABB scan paths", abbPaths.join("\n"))}
          ${textareaField("m365Paths", "M365 scan paths", m365Paths.join("\n"))}
        </div>
      </section>
    </div>
    <div class="config-actions">
      <div id="configActionNotice" class="action-notice">${escapeHtml(state.configNotice || "")}</div>
      <div class="action-buttons">
        <button id="saveConfig" class="primary" type="submit">Save</button>
      </div>
    </div>
  </form>`;

  document.getElementById("configForm").addEventListener("submit", saveConfig);
  document.getElementById("zabbixMode").addEventListener("change", updateConfigVisibility);
  document.getElementById("zabbixTLS").addEventListener("change", updateConfigVisibility);
  attachSecretToggles();
  updateConfigVisibility();
}

function apiEndpoint(api) {
  return `http://${window.location.hostname}:${api.port || 9876}/api/v1`;
}

function textField(id, label, value) {
  return `<label class="field"><span>${escapeHtml(label)}</span><input id="${id}" type="text" value="${escapeHtml(value)}"></label>`;
}

function secretField(id, label, value) {
  return `<label class="field secret-field"><span>${escapeHtml(label)}</span><div class="secret-control"><input id="${id}" type="password" autocomplete="new-password" value="${escapeHtml(value)}"><button class="secret-toggle" type="button" data-secret-toggle="${escapeHtml(id)}" aria-label="Show ${escapeHtml(label)}" title="Show ${escapeHtml(label)}">${eyeIcon(false)}</button></div></label>`;
}

function numberField(id, label, value) {
  return `<label class="field"><span>${escapeHtml(label)}</span><input id="${id}" type="number" min="1" value="${escapeHtml(value)}"></label>`;
}

function textareaField(id, label, value) {
  return `<label class="field field-wide"><span>${escapeHtml(label)}</span><textarea id="${id}" spellcheck="false">${escapeHtml(value)}</textarea></label>`;
}

function checkboxField(id, label, checked) {
  return `<label class="check-field"><input id="${id}" type="checkbox"${checked ? " checked" : ""}><span>${escapeHtml(label)}</span></label>`;
}

function selectField(id, label, value, options) {
  return `<label class="field"><span>${escapeHtml(label)}</span><select id="${id}">${options.map(function(option) {
    const optionValue = typeof option === "string" ? option : option.value;
    const optionLabel = typeof option === "string" ? option : option.label;
    return `<option value="${escapeHtml(optionValue)}"${optionValue === value ? " selected" : ""}>${escapeHtml(optionLabel)}</option>`;
  }).join("")}</select></label>`;
}

function eyeIcon(visible) {
  if (visible) {
    return `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"></path><circle cx="12" cy="12" r="3"></circle></svg>`;
  }
  return `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 3l18 18"></path><path d="M10.6 10.6A3 3 0 0 0 14 14"></path><path d="M9.9 5.2A10.8 10.8 0 0 1 12 5c6.5 0 10 7 10 7a18.5 18.5 0 0 1-3.1 4.1"></path><path d="M6.5 6.8C3.6 8.7 2 12 2 12s3.5 7 10 7c1.2 0 2.3-.2 3.3-.6"></path></svg>`;
}

function attachSecretToggles() {
  document.querySelectorAll("[data-secret-toggle]").forEach(function(button) {
    button.addEventListener("click", function() {
      const input = document.getElementById(button.dataset.secretToggle);
      if (!input) return;
      const visible = input.type === "password";
      input.type = visible ? "text" : "password";
      button.innerHTML = eyeIcon(visible);
      const action = visible ? "Hide" : "Show";
      button.setAttribute("aria-label", `${action} ${input.id}`);
      button.setAttribute("title", `${action} ${input.id}`);
    });
  });
}

function updateConfigVisibility() {
  const modeEl = document.getElementById("zabbixMode");
  const tlsEl = document.getElementById("zabbixTLS");
  const mode = modeEl ? modeEl.value : "api";
  const tls = tlsEl ? tlsEl.value : "none";
  const apiBind = document.getElementById("apiBind");
  if (apiBind) {
    const bind = apiBind.value.trim();
    if (mode === "api" && (bind === "" || bind === "127.0.0.1" || bind === "::1" || bind.toLowerCase() === "localhost")) {
      apiBind.value = "0.0.0.0";
    }
    if (mode === "sender" && (bind === "" || bind === "0.0.0.0" || bind === "::")) {
      apiBind.value = "127.0.0.1";
    }
  }
  document.querySelectorAll("[data-mode-section]").forEach(function(section) {
    section.classList.toggle("hidden", section.dataset.modeSection !== mode);
  });
  document.querySelectorAll("[data-tls-section]").forEach(function(section) {
    section.classList.toggle("hidden", section.dataset.tlsSection !== tls);
  });
}

function readPositiveInt(id, label) {
  const value = Number.parseInt(document.getElementById(id).value, 10);
  if (!Number.isFinite(value) || value <= 0) {
    throw new Error(`${label} must be a positive number`);
  }
  return value;
}

function readPaths(id) {
  return document.getElementById(id).value.split(/\r?\n/)
    .map(function(line) { return line.trim(); })
    .filter(Boolean);
}

function formConfig() {
  const next = JSON.parse(JSON.stringify(state.config));
  next.collector = next.collector || {};
  next.api = next.api || {};
  next.zabbix = next.zabbix || {};
  next.zabbix.sender = next.zabbix.sender || {};
  next.logging = next.logging || {};
  next.privacy = next.privacy || {};
  next.products = next.products || {};
  next.products.active_backup_business = next.products.active_backup_business || {};
  next.products.active_backup_m365 = next.products.active_backup_m365 || {};

  next.collector.interval_seconds = readPositiveInt("collectorInterval", "Collector interval seconds");
  next.collector.max_age_hours = readPositiveInt("collectorMaxAge", "Max age hours");
  next.zabbix.mode = document.getElementById("zabbixMode").value;
  next.zabbix.sender.host = document.getElementById("zabbixHost").value.trim();
  next.zabbix.sender.server = document.getElementById("zabbixServer").value.trim();
  next.zabbix.sender.port = readPositiveInt("zabbixPort", "Zabbix trapper port");
  if (next.zabbix.sender.port > 65535) {
    throw new Error("Zabbix trapper port must be between 1 and 65535");
  }
  next.zabbix.sender.tls = document.getElementById("zabbixTLS").value;
  next.zabbix.sender.timeout_seconds = readPositiveInt("zabbixTimeout", "Zabbix timeout seconds");
  next.zabbix.sender.chunk_size = readPositiveInt("zabbixChunkSize", "Zabbix chunk size");
  next.zabbix.sender.server_name = document.getElementById("zabbixServerName").value.trim();
  next.zabbix.sender.ca_file = document.getElementById("zabbixCAFile").value.trim();
  next.zabbix.sender.cert_file = document.getElementById("zabbixCertFile").value.trim();
  next.zabbix.sender.key_file = document.getElementById("zabbixKeyFile").value.trim();
  next.zabbix.sender.psk_identity = document.getElementById("zabbixPSKIdentity").value.trim();
  next.zabbix.sender.psk = document.getElementById("zabbixPSK").value.trim();
  delete next.zabbix.sender.source_ip;
  delete next.zabbix.sender.sender_binary;
  next.api.enabled = document.getElementById("apiEnabled").checked;
  next.api.bind = document.getElementById("apiBind").value.trim() || "0.0.0.0";
  if (next.zabbix.mode === "sender") {
    next.api.enabled = true;
    if (next.api.bind === "0.0.0.0" || next.api.bind === "::") {
      next.api.bind = "127.0.0.1";
    }
  } else if (next.api.bind === "127.0.0.1" || next.api.bind === "::1" || next.api.bind.toLowerCase() === "localhost") {
    next.api.bind = "0.0.0.0";
  }
  next.api.port = readPositiveInt("apiPort", "API port");
  if (next.api.port > 65535) {
    throw new Error("API port must be between 1 and 65535");
  }
  next.api.token = document.getElementById("apiToken").value.trim();
  next.logging.level = document.getElementById("loggingLevel").value;
  next.privacy.redact_names = document.getElementById("redactNames").checked;
  next.products.active_backup_business.enabled = document.getElementById("abbEnabled").checked;
  next.products.active_backup_business.scan_paths = readPaths("abbPaths");
  next.products.active_backup_m365.enabled = document.getElementById("m365Enabled").checked;
  next.products.active_backup_m365.scan_paths = readPaths("m365Paths");
  return next;
}

async function saveConfig(event) {
  event.preventDefault();
  const button = document.getElementById("saveConfig");
  button.disabled = true;
  state.configNotice = "";
  try {
    const result = await writeJSON("config", formConfig());
    state.config = result.config || state.config;
    state.configNotice = result.restart_required ? "Package restart required. Stop and run it again in Package Center." : "";
  } catch (err) {
    state.configNotice = err.message;
  } finally {
    button.disabled = false;
    renderConfig();
  }
}

if (!isEmbeddedInDSM()) {
  renderDSMOnlyMessage();
} else {
  document.querySelectorAll("[data-tab]").forEach(function(button) {
    button.addEventListener("click", function() {
      document.querySelectorAll("[data-tab]").forEach(function(item) {
        item.classList.remove("active");
      });
      button.classList.add("active");
      ["overview", "jobs", "sources", "log", "config"].forEach(function(id) {
        document.getElementById(id).classList.toggle("hidden", id !== button.dataset.tab);
      });
    });
  });

  document.getElementById("refresh").addEventListener("click", load);
  load();
}
