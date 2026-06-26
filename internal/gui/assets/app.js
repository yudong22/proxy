/* global state */
let allHistory = [];
let currentFilter = '';

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

/* ── /api/status ───────────────────────────────────────────────── */
async function refreshMetrics() {
  try {
    const r = await fetch('/api/metrics');
    if (!r.ok) return;
    const d = await r.json();

    // status badge
    const running = d.proxy_running;
    const dot  = document.getElementById('status-dot');
    const text = document.getElementById('status-text');
    dot.className = 'status-dot ' + (running ? 'running' : 'stopped');
    text.textContent = running ? '运行中' : '已停止';

    // metric cards
    document.getElementById('m-total').textContent   = fmt(d.requests_received);
    document.getElementById('m-success').textContent = fmt(d.requests_success);
    document.getElementById('m-failed').textContent  = fmt(d.requests_failed);
    document.getElementById('m-streamed').textContent = fmt(d.requests_streamed);

    // port info
    if (d.port) document.getElementById('port-info').textContent = '监听端口：' + d.port;

    // model list
    renderModelList(d.model_counts || {});

    // proxy toggle sync
    const proxyToggle = document.getElementById('toggle-proxy');
    if (proxyToggle && !proxyToggle._changing) proxyToggle.checked = running;
  } catch(e) { /* server may not be ready yet */ }
}

function renderModelList(counts) {
  const list = document.getElementById('model-list');
  const entries = Object.entries(counts).sort((a, b) => b[1] - a[1]);
  if (entries.length === 0) {
    list.innerHTML = '<div class="empty-state">暂无数据</div>';
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
    filtered.length + ' 条' + (currentFilter ? '（已筛选）' : '');

  if (filtered.length === 0) {
    tbody.innerHTML = '<tr><td colspan="7" class="empty-state">暂无历史请求</td></tr>';
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
      <td><span class="badge ${h.success ? 'badge-success' : 'badge-error'}">${h.success ? '成功' : '失败'}</span></td>
    </tr>
  `).join('');
}

function updateModelFilter() {
  const sel = document.getElementById('model-filter');
  const current = sel.value;
  const models = [...new Set(allHistory.map(h => h.model).filter(Boolean))].sort();
  sel.innerHTML = '<option value="">全部模型</option>' +
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
    showSaveStatus('未加载当前配置，无法保存', 'error');
    return;
  }
  
  const saveBtn = document.getElementById('btn-save-cfg');
  saveBtn.disabled = true;
  saveBtn.textContent = '保存中…';
  
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
      showSaveStatus('配置保存并应用成功！', 'success');
      await refreshAll();
    } else {
      const txt = await r.text();
      showSaveStatus('保存失败: ' + txt, 'error');
    }
  } catch (e) {
    showSaveStatus('网络错误，保存失败', 'error');
  } finally {
    saveBtn.disabled = false;
    saveBtn.textContent = '保存并应用配置';
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
