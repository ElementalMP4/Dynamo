const API = '/api/vpn';
const lastSamples = {};
const lastTimes = {};
const miniCharts = {};

const HISTORY_SIZE = 60;

let pendingDeleteIface = null;
let pendingDeletePeer = { iface: null, id: null };

function createMiniChart(canvasId, key) {
  const canvas = document.getElementById(canvasId);
  if (!canvas || miniCharts[key]) return;

  miniCharts[key] = new Chart(canvas, {
    type: 'line',
    data: {
      labels: new Array(HISTORY_SIZE).fill(''),
      datasets: [
        {
          label: 'Download',
          borderColor: '#22c55e',
          backgroundColor: 'rgba(34,197,94,0.15)',
          data: new Array(HISTORY_SIZE).fill(0),
          tension: 0.35,
          pointRadius: 0,
          spanGaps: true,
          borderWidth: 1.5,
        },
        {
          label: 'Upload',
          borderColor: '#ef4444',
          backgroundColor: 'rgba(239,68,68,0.15)',
          data: new Array(HISTORY_SIZE).fill(0),
          tension: 0.35,
          pointRadius: 0,
          spanGaps: true,
          borderWidth: 1.5,
        },
      ],
    },
    options: {
      animation: false,
      responsive: true,
      maintainAspectRatio: false,
      scales: {
        x: { display: false },
        y: {
          beginAtZero: true,
          ticks: {
            maxTicksLimit: 3,
            font: { size: 9 },
            color: '#6b7280',
            callback: (v) => formatRate(v),
          },
          grid: {
            color: 'rgba(255,255,255,0.05)',
          },
        },
      },
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: (ctx) => `${ctx.dataset.label}: ${formatRate(ctx.raw)}`,
          },
        },
      },
    },
  });
}

function initCharts(interfaces) {
  if (!interfaces || interfaces.length === 0) return;
  for (const iface of interfaces) {
    createMiniChart(`chart-iface-${iface.name}`, `iface-${iface.name}`);
    for (const peer of (iface.peers || [])) {
      createMiniChart(`chart-peer-${peer.id}`, `peer-${peer.id}`);
    }
  }
}

function destroyAllCharts() {
  for (const key in miniCharts) {
    miniCharts[key].destroy();
    delete miniCharts[key];
  }
}

function pushToChart(key, rxRate, txRate) {
  const chart = miniCharts[key];
  if (!chart) return;

  chart.data.labels.push('');
  chart.data.datasets[0].data.push(rxRate || 0);
  chart.data.datasets[1].data.push(txRate || 0);

  if (chart.data.labels.length > HISTORY_SIZE) {
    chart.data.labels.shift();
    chart.data.datasets.forEach((d) => d.data.shift());
  }

  const maxVal = Math.max(
    ...chart.data.datasets[0].data,
    ...chart.data.datasets[1].data,
    0
  );
  chart.options.scales.y.suggestedMax = maxVal * 1.3;
  chart.update('none');
}

async function loadInterfaces() {
  const res = await fetch(`${API}/interfaces`);
  const interfaces = await res.json();
  destroyAllCharts();
  renderInterfaces(interfaces);
  initCharts(interfaces);
}

function renderInterfaces(interfaces) {
  const list = document.getElementById('interfaces-list');
  if (!interfaces || interfaces.length === 0) {
    list.innerHTML =
      '<p style="color: var(--text-secondary); text-align: center; padding: 3rem 0;">No interfaces configured. Click <strong>Add Interface</strong> to get started.</p>';
    return;
  }
  list.innerHTML = interfaces.map(renderInterfaceCard).join('');
}

function renderInterfaceCard(iface) {
  const peers = iface.peers || [];
  const peersHtml =
    peers.length === 0
      ? `<p style="color: var(--text-secondary); font-size: 0.9rem; padding: 0.5rem 0;">No peers yet.</p>`
      : peers.map((p) => renderPeerCard(iface.name, p)).join('');

  const enabled = iface.enabled !== false;
  const dotClass = enabled ? 'on' : 'off';
  const cardClass = enabled ? 'enabled' : 'disabled';
  const toggleLabel = enabled ? 'Disable' : 'Enable';
  const toggleClass = enabled ? 'secondary' : 'success';

  return `
  <div class="record-group" id="iface-group-${esc(iface.name)}">
    <div class="circuit-card circuit-card--with-chart ${cardClass}" style="margin-bottom: 1rem;">
      <div class="circuit-status-dot ${dotClass}"></div>
      <div class="circuit-info">
        <div class="circuit-info-row">
          <span class="circuit-name">${esc(iface.name)}</span>
          <span class="circuit-badge">NAT ${iface.enable_nat ? 'ON' : 'OFF'}</span>
        </div>
        <div class="circuit-meta">${esc(iface.address)} &nbsp; :${iface.listen_port} &nbsp; ${esc(iface.endpoint)}</div>
        <div class="circuit-meta" id="${iface.name}-connectivity-stats">TX: Loading&nbsp; RX: Loading</div>
      </div>
      <div class="circuit-actions">
        <button class="button sm secondary" onclick="showEditInterfaceModal(${jsonAttr(iface)})">Edit</button>
        <button class="button sm ${toggleClass}" onclick="toggleInterface('${esc(iface.name)}', ${enabled})">${toggleLabel}</button>
        <button class="button sm secondary" onclick="event.stopPropagation(); showAddPeerModal('${esc(iface.name)}')">Add Peer</button>
        <button class="button sm danger" onclick="showDeleteIfaceModal('${esc(iface.name)}')">Delete</button>
      </div>
      <div class="circuit-chart">
        <canvas id="chart-iface-${esc(iface.name)}"></canvas>
      </div>
    </div>

    <div class="iface-peers-section">
      <button class="peers-toggle" onclick="togglePeersVisibility('${esc(iface.name)}')" id="peers-toggle-${esc(iface.name)}">
        <span class="peers-toggle-label">
          <svg class="peers-chevron" id="peers-chevron-${esc(iface.name)}" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M5.22 8.22a.75.75 0 0 1 1.06 0L10 11.94l3.72-3.72a.75.75 0 1 1 1.06 1.06l-4.25 4.25a.75.75 0 0 1-1.06 0L5.22 9.28a.75.75 0 0 1 0-1.06z" clip-rule="evenodd"/>
          </svg>
          Peers <span style="font-weight: normal; color: var(--text-secondary);">(${peers.length})</span>
        </span>
      </button>
      <div class="records peers-list" id="peers-list-${esc(iface.name)}" style="display: none;">
        ${peersHtml}
      </div>
    </div>
  </div>`;
}

function renderPeerCard(ifaceName, peer) {
  const dotClass = peer.enabled ? 'on' : 'off';
  const toggleLabel = peer.enabled ? 'Disable' : 'Enable';
  const toggleClass = peer.enabled ? 'secondary' : 'success';

  return `
  <div class="circuit-card circuit-card--with-chart ${peer.enabled ? 'enabled' : 'disabled'}" id="peer-card-${esc(peer.id)}">
    <div class="circuit-status-dot ${dotClass}"></div>
    <div class="circuit-info">
      <div class="circuit-info-row">
        <span class="circuit-name">${esc(peer.name)}</span>
        <span class="circuit-badge" id="${peer.id}-activity">Unknown</span>
      </div>
      <div class="circuit-meta">${esc(peer.ip)}${peer.dns ? ' &nbsp; DNS: ' + esc(peer.dns) : ''}</div>
      <div class="circuit-meta" id="${peer.id}-connectivity-stats">TX: Loading&nbsp; RX: Loading</div>
    </div>
    <div class="circuit-actions">
      <button class="button sm secondary" onclick="downloadConfig('${esc(ifaceName)}', '${esc(peer.id)}', '${esc(peer.name)}')">Config</button>
      <button class="button sm secondary" onclick="showEditPeerModal('${esc(ifaceName)}', ${jsonAttr(peer)})">Edit</button>
      <button class="button sm ${toggleClass}" onclick="togglePeer('${esc(ifaceName)}', '${esc(peer.id)}', ${!peer.enabled})">${toggleLabel}</button>
      <button class="button sm danger" onclick="showDeletePeerModal('${esc(ifaceName)}', '${esc(peer.id)}', '${esc(peer.name)}')">Delete</button>
    </div>
    <div class="circuit-chart circuit-chart--peer">
      <canvas id="chart-peer-${esc(peer.id)}"></canvas>
    </div>
  </div>`;
}

function formatRate(bytesPerSec) {
  let bitsPerSec = bytesPerSec * 8;
  const units = ['bps', 'Kbps', 'Mbps', 'Gbps'];
  let i = 0;
  while (bitsPerSec > 1000 && i < units.length - 1) {
    bitsPerSec /= 1000;
    i++;
  }
  return bitsPerSec.toFixed(1) + ' ' + units[i];
}

function writeStats(element, id, tx, rx, chartKey) {
  const now = Date.now();
  if (lastSamples[id]) {
    const seconds = (now - lastTimes[id]) / 1000;
    const rxRate = (rx - lastSamples[id].rx) / seconds * 8;
    const txRate = (tx - lastSamples[id].tx) / seconds * 8;

    if (element) {
      element.innerHTML = `TX ${formatRate(txRate)}&nbsp; RX ${formatRate(rxRate)}`;
    }
    if (chartKey) {
      pushToChart(chartKey, rxRate || 0, txRate || 0);
    }
  }
  lastSamples[id] = { rx, tx };
  lastTimes[id] = now;
}

async function pollNetwork() {
  const res = await fetch(`${API}/metrics`);
  const data = await res.json();

  if (!data.interfaces) return;

  for (const iface of data.interfaces) {
    const statsField = document.getElementById(`${iface.name}-connectivity-stats`);
    if (!statsField) continue;

    writeStats(statsField, iface.name, iface.tx, iface.rx, `iface-${iface.name}`);

    for (const peer of iface.peers) {
      const peerStatsField = document.getElementById(`${peer.id}-connectivity-stats`);
      const peerActivityField = document.getElementById(`${peer.id}-activity`);

      writeStats(peerStatsField, peer.id, peer.tx, peer.rx, `peer-${peer.id}`);

      if (peerActivityField) {
        peerActivityField.innerHTML = peer.active ? 'Active' : 'Inactive';
      }
    }
  }
}

function jsonAttr(obj) {
  return "'" + JSON.stringify(obj).replace(/'/g, '&#39;').replace(/"/g, '&quot;') + "'";
}

function esc(str) {
  if (str == null) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function showCreateInterfaceModal() {
  document.getElementById('iface-modal-title').textContent = 'Create Interface';
  document.getElementById('iface-submit-btn').textContent = 'Create';
  document.getElementById('iface-edit-name').value = '';
  document.getElementById('iface-name').value = '';
  document.getElementById('iface-name').disabled = false;
  document.getElementById('iface-address').value = '';
  document.getElementById('iface-port').value = '';
  document.getElementById('iface-endpoint').value = '';
  document.getElementById('iface-dns').value = '';
  document.getElementById('iface-allowed-ips').value = '';
  document.getElementById('iface-nat').checked = false;
  document.getElementById('iface-out-iface').innerHTML = '';
  document.getElementById('iface-out-iface-group').style.display = 'none';
  showModal('interface-modal');
}

function showEditInterfaceModal(ifaceJson) {
  const iface =
    typeof ifaceJson === 'string'
      ? JSON.parse(ifaceJson.replace(/&quot;/g, '"'))
      : ifaceJson;
  document.getElementById('iface-modal-title').textContent = 'Edit Interface';
  document.getElementById('iface-submit-btn').textContent = 'Save';
  document.getElementById('iface-edit-name').value = iface.name;
  document.getElementById('iface-name').value = iface.name;
  document.getElementById('iface-name').disabled = true;
  document.getElementById('iface-address').value = iface.address || '';
  document.getElementById('iface-port').value = iface.listen_port || '';
  document.getElementById('iface-endpoint').value = iface.endpoint || '';
  document.getElementById('iface-dns').value = iface.default_dns || '';
  document.getElementById('iface-allowed-ips').value = iface.default_allowed_ips || '';
  document.getElementById('iface-nat').checked = !!iface.enable_nat;
  document.getElementById('iface-out-iface-group').style.display = iface.enable_nat ? 'block' : 'none';
  if (iface.enable_nat) {
    loadInterfacesIntoEditor(iface.out_interface || '');
  } else {
    document.getElementById('iface-out-iface').innerHTML = '';
  }
  showModal('interface-modal');
}

function hideInterfaceModal() {
  hideModal('interface-modal');
}

async function submitInterfaceForm() {
  const editName = document.getElementById('iface-edit-name').value;
  const isEdit = !!editName;
  const nat = document.getElementById('iface-nat').checked;

  if (isEdit) {
    const body = {};
    const dns = document.getElementById('iface-dns').value.trim();
    const allowedIPs = document.getElementById('iface-allowed-ips').value.trim();
    const endpoint = document.getElementById('iface-endpoint').value.trim();
    const outIface = document.getElementById('iface-out-iface').value;
    if (dns) body.default_dns = dns;
    if (allowedIPs) body.default_allowed_ips = allowedIPs;
    if (endpoint) body.endpoint = endpoint;
    body.enable_nat = nat;
    if (nat && outIface) body.out_interface = outIface;

    const res = await fetch(`${API}/interfaces/${encodeURIComponent(editName)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Unknown error' }));
      alert('Error: ' + err.error);
      return;
    }
  } else {
    const name = document.getElementById('iface-name').value.trim();
    const address = document.getElementById('iface-address').value.trim();
    const port = parseInt(document.getElementById('iface-port').value, 10);
    const endpoint = document.getElementById('iface-endpoint').value.trim();
    const dns = document.getElementById('iface-dns').value.trim();
    const allowedIPs = document.getElementById('iface-allowed-ips').value.trim();
    const outIface = document.getElementById('iface-out-iface').value;

    if (!name || !address || !port || !endpoint) {
      alert('Name, Address, Listen Port, and Endpoint are required.');
      return;
    }

    const body = { name, address, listen_port: port, endpoint, enable_nat: nat };
    if (dns) body.default_dns = dns;
    if (allowedIPs) body.default_allowed_ips = allowedIPs;
    if (nat && outIface) body.out_interface = outIface;

    const res = await fetch(`${API}/interfaces`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Unknown error' }));
      alert('Error: ' + err.error);
      return;
    }
  }

  hideInterfaceModal();
  loadInterfaces();
}

async function loadInterfacesIntoEditor(selectedValue) {
  const res = await fetch('/api/system/interfaces');
  if (!res.ok) {
    alert('Failed to load system interfaces');
    return;
  }
  const { interfaces } = await res.json();
  const select = document.getElementById('iface-out-iface');
  select.innerHTML = '';
  interfaces.forEach((iface) => {
    const option = document.createElement('option');
    option.value = iface;
    option.textContent = iface;
    if (iface === selectedValue) option.selected = true;
    select.appendChild(option);
  });
}

function showDeleteIfaceModal(name) {
  pendingDeleteIface = name;
  document.getElementById('delete-iface-name').textContent = name;
  showModal('delete-iface-modal');
}

function hideDeleteIfaceModal() {
  hideModal('delete-iface-modal');
}

async function confirmDeleteIface() {
  if (!pendingDeleteIface) return;
  const res = await fetch(
    `${API}/interfaces/${encodeURIComponent(pendingDeleteIface)}`,
    { method: 'DELETE' }
  );
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Unknown error' }));
    alert('Error: ' + err.error);
    return;
  }
  hideDeleteIfaceModal();
  loadInterfaces();
}

function showAddPeerModal(ifaceName) {
  document.getElementById('peer-modal-title').textContent = 'Add Peer';
  document.getElementById('peer-submit-btn').textContent = 'Add Peer';
  document.getElementById('peer-iface-name').value = ifaceName;
  document.getElementById('peer-edit-id').value = '';
  document.getElementById('peer-name').value = '';
  document.getElementById('peer-dns').value = '';
  document.getElementById('peer-allowed-ips').value = '';
  showModal('peer-modal');
}

function showEditPeerModal(ifaceName, peerJson) {
  const peer =
    typeof peerJson === 'string'
      ? JSON.parse(peerJson.replace(/&quot;/g, '"'))
      : peerJson;
  document.getElementById('peer-modal-title').textContent = 'Edit Peer';
  document.getElementById('peer-submit-btn').textContent = 'Save';
  document.getElementById('peer-iface-name').value = ifaceName;
  document.getElementById('peer-edit-id').value = peer.id;
  document.getElementById('peer-name').value = peer.name || '';
  document.getElementById('peer-dns').value = peer.dns || '';
  document.getElementById('peer-allowed-ips').value = peer.allowed_ips || '';
  showModal('peer-modal');
}

function hidePeerModal() {
  hideModal('peer-modal');
}

async function submitPeerForm() {
  const ifaceName = document.getElementById('peer-iface-name').value;
  const editId = document.getElementById('peer-edit-id').value;
  const isEdit = !!editId;
  const name = document.getElementById('peer-name').value.trim();
  const dns = document.getElementById('peer-dns').value.trim() || null;
  const allowedIPs = document.getElementById('peer-allowed-ips').value.trim() || null;

  if (!name) {
    alert('Name is required.');
    return;
  }

  if (isEdit) {
    const body = { name };
    if (dns !== null) body.dns = dns;
    if (allowedIPs !== null) body.allowed_ips = allowedIPs;

    const res = await fetch(
      `${API}/interfaces/${encodeURIComponent(ifaceName)}/peers/${encodeURIComponent(editId)}`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      }
    );
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Unknown error' }));
      alert('Error: ' + err.error);
      return;
    }
  } else {
    const body = { name };
    if (dns !== null) body.dns = dns;
    if (allowedIPs !== null) body.allowed_ips = allowedIPs;

    const res = await fetch(
      `${API}/interfaces/${encodeURIComponent(ifaceName)}/peers`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      }
    );
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Unknown error' }));
      alert('Error: ' + err.error);
      return;
    }
  }

  hidePeerModal();
  loadInterfaces();
}

async function toggleInterface(name, enabled) {
  const res = await fetch(`${API}/interfaces/${encodeURIComponent(name)}/toggle`, {
    method: 'PUT',
    body: JSON.stringify({ enabled: !enabled }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Unknown error' }));
    alert('Error: ' + err.error);
    return;
  }
  loadInterfaces();
}

async function togglePeer(ifaceName, peerId, enabled) {
  const res = await fetch(
    `${API}/interfaces/${encodeURIComponent(ifaceName)}/peers/${encodeURIComponent(peerId)}`,
    {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled }),
    }
  );
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Unknown error' }));
    alert('Error: ' + err.error);
    return;
  }
  loadInterfaces();
}

function showDeletePeerModal(ifaceName, peerId, peerName) {
  pendingDeletePeer = { iface: ifaceName, id: peerId };
  document.getElementById('delete-peer-name').textContent = peerName;
  showModal('delete-peer-modal');
}

function hideDeletePeerModal() {
  hideModal('delete-peer-modal');
}

async function confirmDeletePeer() {
  const { iface, id } = pendingDeletePeer;
  if (!iface || !id) return;
  const res = await fetch(
    `${API}/interfaces/${encodeURIComponent(iface)}/peers/${encodeURIComponent(id)}`,
    { method: 'DELETE' }
  );
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Unknown error' }));
    alert('Error: ' + err.error);
    return;
  }
  hideDeletePeerModal();
  loadInterfaces();
}

async function downloadConfig(ifaceName, peerId, peerName) {
  const res = await fetch(
    `${API}/interfaces/${encodeURIComponent(ifaceName)}/peers/${encodeURIComponent(peerId)}/config`
  );
  if (!res.ok) {
    alert('Failed to fetch config.');
    return;
  }
  const text = await res.text();
  const blob = new Blob([text], { type: 'text/plain' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${peerName}.conf`;
  a.click();
  URL.revokeObjectURL(url);
}

function togglePeersVisibility(ifaceName) {
  const list = document.getElementById(`peers-list-${ifaceName}`);
  const chevron = document.getElementById(`peers-chevron-${ifaceName}`);
  const open = list.style.display !== 'none';
  list.style.display = open ? 'none' : 'grid';
  chevron.classList.toggle('peers-chevron--open', !open);
}

function showModal(id) {
  document.getElementById(id).style.display = 'flex';
}

function hideModal(id) {
  document.getElementById(id).style.display = 'none';
}

window.addEventListener('click', function (e) {
  ['interface-modal', 'peer-modal', 'delete-iface-modal', 'delete-peer-modal'].forEach((id) => {
    const modal = document.getElementById(id);
    if (e.target === modal) hideModal(id);
  });
});

document.addEventListener('DOMContentLoaded', () => {
  document.getElementById('iface-nat').addEventListener('change', async function () {
    const group = document.getElementById('iface-out-iface-group');
    group.style.display = this.checked ? 'block' : 'none';
    if (this.checked) {
      await loadInterfacesIntoEditor();
    } else {
      document.getElementById('iface-out-iface').innerHTML = '';
    }
  });
  loadInterfaces();
  setInterval(pollNetwork, 1000);
});