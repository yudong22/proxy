/* ── i18n ────────────────────────────────────────────────────────── */
const TRANSLATIONS = {
  en: {
    'lang.toggle': '中文',
    'status.checking': 'Checking…',
    'status.running': 'Running',
    'status.stopped': 'Stopped',
    'status.connected': 'Connected',
    'tab.overview': 'Overview',
    'tab.history': 'History',
    'tab.settings': 'Settings',
    'metric.total': 'Total Requests',
    'metric.success': 'Success',
    'metric.failed': 'Failed',
    'metric.streamed': 'Streamed',
    'section.modelDist': 'Model Distribution',
    'empty.noData': 'No data yet',
    'filter.allModels': 'All Models',
    'th.time': 'Time',
    'th.model': 'Model',
    'th.scenario': 'Scenario',
    'th.inputTokens': 'Input Tokens',
    'th.outputTokens': 'Output Tokens',
    'th.duration': 'Duration',
    'th.status': 'Status',
    'empty.noHistory': 'No history yet',
    'setting.proxy': 'Proxy Service',
    'setting.proxyDesc': 'Start or stop the proxy HTTP service',
    'setting.autostart': 'Start on Boot',
    'setting.autostartDesc': 'Auto-start routatic-proxy at login (launchd)',
    'setting.notify': 'Desktop Notifications',
    'setting.notifyDesc': 'Notify on failures or model switches',
    'setting.language': 'Language',
    'setting.languageDesc': 'Switch interface language',
    'section.proxyConfig': 'Proxy Configuration',
    'placeholder.envOrEmpty': 'Use env var or leave empty',
    'placeholder.notSet': 'Not configured',
    'label.globalKey': 'Global API Key (optional)',
    'label.host': 'Listen Address (Host)',
    'label.port': 'Listen Port (Port)',
    'btn.save': 'Save & Apply Config',
    'status.saving': 'Saving…',
    'status.saveOk': 'Config saved successfully!',
    'status.saveFail': 'Save failed: ',
    'status.networkError': 'Network error, save failed',
    'status.count': ' entries',
    'status.filtered': ' (filtered)',
    'badge.success': 'Success',
    'badge.fail': 'Fail',
    'port.info': 'Listening port: —',
    'save.unloaded': 'Config not loaded, cannot save',
  },
  zh: {
    'lang.toggle': 'English',
    'status.checking': '检查中…',
    'status.running': '运行中',
    'status.stopped': '已停止',
    'status.connected': '已连接',
    'tab.overview': '概览',
    'tab.history': '历史请求',
    'tab.settings': '设置',
    'metric.total': '总请求数',
    'metric.success': '成功',
    'metric.failed': '失败',
    'metric.streamed': '流式请求',
    'section.modelDist': '模型调用分布',
    'empty.noData': '暂无数据',
    'filter.allModels': '全部模型',
    'th.time': '时间',
    'th.model': '模型',
    'th.scenario': '场景',
    'th.inputTokens': '输入 Token',
    'th.outputTokens': '输出 Token',
    'th.duration': '耗时',
    'th.status': '状态',
    'empty.noHistory': '暂无历史请求',
    'setting.proxy': '代理服务',
    'setting.proxyDesc': '启动或停止代理 HTTP 服务',
    'setting.autostart': '开机自启',
    'setting.autostartDesc': '登录时自动启动 routatic-proxy（launchd）',
    'setting.notify': '桌面通知',
    'setting.notifyDesc': '请求失败或切换模型时发送系统通知',
    'setting.language': '语言',
    'setting.languageDesc': '切换界面语言',
    'section.proxyConfig': '服务代理配置',
    'placeholder.envOrEmpty': '使用环境变量或留空',
    'placeholder.notSet': '未配置',
    'label.globalKey': 'Global API Key (可选)',
    'label.host': '监听地址 (Host)',
    'label.port': '监听端口 (Port)',
    'btn.save': '保存并应用配置',
    'status.saving': '保存中…',
    'status.saveOk': '配置保存并应用成功！',
    'status.saveFail': '保存失败: ',
    'status.networkError': '网络错误，保存失败',
    'status.count': ' 条',
    'status.filtered': '（已筛选）',
    'badge.success': '成功',
    'badge.fail': '失败',
    'port.info': '监听端口：—',
    'save.unloaded': '未加载当前配置，无法保存',
  }
};

let currentLang = localStorage.getItem('routatic-proxy-lang') || 'en';

function t(key) {
  return (TRANSLATIONS[currentLang] && TRANSLATIONS[currentLang][key]) || key;
}

function applyTranslations() {
  // Update all data-i18n elements
  document.querySelectorAll('[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n');
    el.textContent = t(key);
  });
  // Update placeholder attributes for inputs
  document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
    const key = el.getAttribute('data-i18n-placeholder');
    el.placeholder = t(key);
  });
  // Update the language toggle text
  const langBtn = document.getElementById('btn-lang-toggle');
  if (langBtn) {
    langBtn.innerHTML = '<span data-i18n="lang.toggle">' + t('lang.toggle') + '</span>';
  }
}

function toggleLanguage() {
  currentLang = currentLang === 'en' ? 'zh' : 'en';
  localStorage.setItem('routatic-proxy-lang', currentLang);
  document.documentElement.lang = currentLang;
  applyTranslations();
  // Re-render dynamic content
  renderModelList(lastModelCounts);
  renderHistory();
}

// Apply translations on load
document.addEventListener('DOMContentLoaded', () => {
  document.documentElement.lang = currentLang;
  applyTranslations();
});

/* global state */
let allHistory = [];
let currentFilter = '';
let lastModelCounts = {};

/* ── Tab switching ─────────────────────────────────────────────── */
document.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
    tab.classList.add('active');
    document.getElementById('tab-' + tab.dataset.tab).classList.add('active');
  });
});

/* ── Polling ───────────────────────────────────────────────────── */
function startPolling() {
  refreshAll();
  setInterval(refreshAll, 3000);
}

async function refreshAll() {
  await Promise.all([refreshMetrics(), refreshHistory(), refreshConfig()]);
}

/* ── /api/metrics ──────────────────────────────────────────────── */
async function refreshMetrics() {
  try {
    const r = await fetch('/api/metrics');
    if (!r.ok) return;
    const d = await r.json();

    // status badge
    const running = d.proxy_running;
    const connected = d.connected_to_existing;
    const dot  = document.getElementById('status-dot');
    const text = document.getElementById('status-text');
    dot.className = 'status-dot ' + (running ? 'running' : 'stopped');
    if (running && connected) {
      text.textContent = t('status.connected');
    } else if (running) {
      text.textContent = t('status.running');
    } else {
      text.textContent = t('status.stopped');
    }

    // metric cards
    document.getElementById('m-total').textContent   = fmt(d.requests_received);
    document.getElementById('m-success').textContent = fmt(d.requests_success);
    document.getElementById('m-failed').textContent  = fmt(d.requests_failed);
    document.getElementById('m-streamed').textContent = fmt(d.requests_streamed);

    // port info
    const portEl = document.getElementById('port-info');
    if (d.port) {
      portEl.textContent = (currentLang === 'zh' ? '监听端口：' : 'Listening port: ') + d.port;
    }

    // model list
    lastModelCounts = d.model_counts || {};
    renderModelList(lastModelCounts);

    // proxy toggle sync
    const proxyToggle = document.getElementById('toggle-proxy');
    if (proxyToggle && !proxyToggle._changing) proxyToggle.checked = running;
  } catch(e) { /* server may not be ready yet */ }
}

function renderModelList(counts) {
  lastModelCounts = counts;
  const list = document.getElementById('model-list');
  const entries = Object.entries(counts).sort((a, b) => b[1] - a[1]);
  if (entries.length === 0) {
    list.innerHTML = '<div class="empty-state">' + t('empty.noData') + '</div>';
    return;
  }
  const max = entries[0][1];
  list.innerHTML = entries.slice(0, 10).map(([model, count]) => `
    <div class="model-row">
      <div class="model-name" title="${model}">${model}</div>
      <div class="model-bar-wrap">
        <div class="model-bar" style="width:${Math.round(count/max*100)}%"></div>
      </div>
      <div class="model-count">${count}</div>
    </div>
  `).join('');
}

/* ── /api/history ──────────────────────────────────────────────── */
async function refreshHistory() {
  try {
    const r = await fetch('/api/history');
    if (!r.ok) return;
    allHistory = await r.json() || [];
    renderHistory();
    updateModelFilter();
  } catch(e) {}
}

function renderHistory() {
  const tbody = document.getElementById('history-tbody');
  const filtered = currentFilter
    ? allHistory.filter(h => h.model === currentFilter)
    : allHistory;

  document.getElementById('history-count').textContent =
    filtered.length + t('status.count') + (currentFilter ? t('status.filtered') : '');

  if (filtered.length === 0) {
    tbody.innerHTML = '<tr><td colspan="7" class="empty-state">' + t('empty.noHistory') + '</td></tr>';
    return;
  }

  tbody.innerHTML = filtered.map(h => `
    <tr>
      <td>${fmtTime(h.start_time)}</td>
      <td><span title="${h.provider || ''}">${h.model || '—'}</span></td>
      <td><span class="badge badge-scene">${h.scenario || '—'}</span></td>
      <td>${h.input_tokens > 0 ? h.input_tokens.toLocaleString() : '—'}</td>
      <td>${h.output_tokens > 0 ? h.output_tokens.toLocaleString() : '—'}</td>
      <td>${fmtDuration(h.duration_ms)}</td>
      <td><span class="badge ${h.success ? 'badge-success' : 'badge-error'}">${h.success ? t('badge.success') : t('badge.fail')}</span></td>
    </tr>
  `).join('');
}

function updateModelFilter() {
  const sel = document.getElementById('model-filter');
  const current = sel.value;
  const models = [...new Set(allHistory.map(h => h.model).filter(Boolean))].sort();
  sel.innerHTML = '<option value="">' + t('filter.allModels') + '</option>' +
    models.map(m => `<option value="${m}" ${m===current?'selected':''}>${m}</option>`).join('');
  sel.value = current;
}

document.getElementById('model-filter').addEventListener('change', function() {
  currentFilter = this.value;
  renderHistory();
});

/* ── /api/config ───────────────────────────────────────────────── */
async function refreshConfig() {
  try {
    const r = await fetch('/api/config');
    if (!r.ok) return;
    const d = await r.json();
    const autostartToggle = document.getElementById('toggle-autostart');
    const notifyToggle    = document.getElementById('toggle-notify');
    if (autostartToggle && !autostartToggle._changing) autostartToggle.checked = !!d.autostart;
    if (notifyToggle    && !notifyToggle._changing)    notifyToggle.checked    = !!d.notify;
  } catch(e) {}
}

/* ── Toggle actions ────────────────────────────────────────────── */
async function toggleProxy(el) {
  el._changing = true;
  try {
    const action = el.checked ? 'start' : 'stop';
    const r = await fetch('/api/proxy/' + action, { method: 'POST' });
    if (!r.ok) { el.checked = !el.checked; }
  } catch(e) { el.checked = !el.checked; }
  setTimeout(() => { el._changing = false; }, 1000);
}

async function toggleAutostart(el) {
  el._changing = true;
  try {
    const r = await fetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ autostart: el.checked })
    });
    if (!r.ok) { el.checked = !el.checked; }
  } catch(e) { el.checked = !el.checked; }
  setTimeout(() => { el._changing = false; }, 1000);
}

async function toggleNotify(el) {
  el._changing = true;
  try {
    const r = await fetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ notify: el.checked })
    });
    if (!r.ok) { el.checked = !el.checked; }
  } catch(e) { el.checked = !el.checked; }
  setTimeout(() => { el._changing = false; }, 1000);
}

/* ── Helpers ───────────────────────────────────────────────────── */
function fmt(n) { return n != null ? Number(n).toLocaleString() : '—'; }

function fmtTime(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  const hh = d.getHours().toString().padStart(2,'0');
  const mm = d.getMinutes().toString().padStart(2,'0');
  const ss = d.getSeconds().toString().padStart(2,'0');
  return hh + ':' + mm + ':' + ss;
}

function fmtDuration(ms) {
  if (!ms && ms !== 0) return '—';
  if (ms < 1000) return ms + ' ms';
  return (ms / 1000).toFixed(1) + ' s';
}

/* ── Proxy Config Form ─────────────────────────────────────────── */
let currentProxyConfig = null;

async function loadProxyConfig() {
  try {
    const r = await fetch('/api/proxy/config');
    if (!r.ok) return;
    currentProxyConfig = await r.json();
    if (currentProxyConfig) {
      document.getElementById('cfg-opencode-go-key').value = currentProxyConfig.opencode_go?.api_key || '';
      document.getElementById('cfg-opencode-zen-key').value = currentProxyConfig.opencode_zen?.api_key || '';
      document.getElementById('cfg-global-key').value = currentProxyConfig.api_key || '';
      document.getElementById('cfg-host').value = currentProxyConfig.host || '';
      document.getElementById('cfg-port').value = currentProxyConfig.port || '';
    }
  } catch (e) {
    console.error('Failed to load proxy config:', e);
  }
}

async function saveProxyConfig() {
  if (!currentProxyConfig) {
    showSaveStatus(t('save.unloaded'), 'error');
    return;
  }

  const saveBtn = document.getElementById('btn-save-cfg');
  saveBtn.disabled = true;
  saveBtn.textContent = t('status.saving');

  // Update local copy
  currentProxyConfig.api_key = document.getElementById('cfg-global-key').value.trim();

  if (!currentProxyConfig.opencode_go) currentProxyConfig.opencode_go = {};
  currentProxyConfig.opencode_go.api_key = document.getElementById('cfg-opencode-go-key').value.trim();

  if (!currentProxyConfig.opencode_zen) currentProxyConfig.opencode_zen = {};
  currentProxyConfig.opencode_zen.api_key = document.getElementById('cfg-opencode-zen-key').value.trim();

  currentProxyConfig.host = document.getElementById('cfg-host').value.trim() || '127.0.0.1';
  currentProxyConfig.port = parseInt(document.getElementById('cfg-port').value, 10) || 3456;

  try {
    const r = await fetch('/api/proxy/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(currentProxyConfig)
    });

    if (r.ok) {
      showSaveStatus(t('status.saveOk'), 'success');
      await refreshAll();
    } else {
      const txt = await r.text();
      showSaveStatus(t('status.saveFail') + txt, 'error');
    }
  } catch (e) {
    showSaveStatus(t('status.networkError'), 'error');
  } finally {
    saveBtn.disabled = false;
    saveBtn.textContent = t('btn.save');
  }
}

function showSaveStatus(msg, type) {
  const status = document.getElementById('save-status');
  status.textContent = msg;
  status.className = 'save-status ' + type;
  setTimeout(() => {
    status.textContent = '';
    status.className = 'save-status';
  }, 4000);
}

function togglePasswordVisibility(id) {
  const input = document.getElementById(id);
  if (input.type === 'password') {
    input.type = 'text';
  } else {
    input.type = 'password';
  }
}

/* ── Boot ──────────────────────────────────────────────────────── */
loadProxyConfig();
startPolling();
