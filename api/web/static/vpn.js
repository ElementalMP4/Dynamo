const API = '/api/wg';

let pendingDeleteIface = null;
let pendingDeletePeer = { iface: null, id: null };

async function loadInterfaces() {
  const res = await fetch(`${API}/interfaces`);
  const interfaces = await res.json();
  renderInterfaces(interfaces);
}

function renderInterfaces(interfaces) {
  const list = document.getElementById('interfaces-list');
  if (!interfaces || interfaces.length === 0) {
    list.innerHTML = '<p style="color: var(--text-secondary); text-align: center; padding: 3rem 0;">No interfaces configured. Click <strong>Add Interface</strong> to get started.</p>';
    return;
  }
  list.innerHTML = interfaces.map(renderInterfaceCard).join('');
}

function renderInterfaceCard(iface) {
  const peers = iface.peers || [];
  const peersHtml = peers.length === 0
    ? `<p style="color: var(--text-secondary); font-size: 0.9rem; padding: 0.5rem 0;">No peers yet.</p>`
    : peers.map(p => renderPeerCard(iface.name, p)).join('');

  return `
  <div class="record-group" id="iface-group-${esc(iface.name)}">
    <div class="circuit-card enabled" style="margin-bottom: 1rem;">
      <div class="circuit-status-dot on"></div>
      <div class="circuit-info">
        <div class="circuit-info-row">
          <span class="circuit-name">${esc(iface.name)}</span>
          ${iface.enable_nat ? `<span class="circuit-badge">NAT</span>` : ''}
        </div>
        <div class="circuit-meta">${esc(iface.address)} &nbsp;·&nbsp; :${iface.listen_port} &nbsp;·&nbsp; ${esc(iface.endpoint)}</div>
      </div>
      <div class="circuit-actions">
        <button class="button sm secondary" onclick="showEditInterfaceModal(${jsonAttr(iface)})">Edit</button>
        <button class="button sm danger" onclick="showDeleteIfaceModal('${esc(iface.name)}')">Delete</button>
      </div>
    </div>

    <div style="margin-left: 1.5rem;">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem;">
        <h2 class="record-group" style="font-size: 1rem; color: var(--text-secondary); margin: 0;">
          Peers <span style="font-weight: normal;">(${peers.length})</span>
        </h2>
        <button class="button sm" onclick="showAddPeerModal('${esc(iface.name)}')">Add Peer</button>
      </div>
      <div class="records" id="peers-list-${esc(iface.name)}">
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
  <div class="circuit-card ${peer.enabled ? 'enabled' : 'disabled'}" id="peer-card-${esc(peer.id)}">
    <div class="circuit-status-dot ${dotClass}"></div>
    <div class="circuit-info">
      <div class="circuit-info-row">
        <span class="circuit-name">${esc(peer.name)}</span>
      </div>
      <div class="circuit-meta">${esc(peer.ip)}${peer.dns ? ' &nbsp;·&nbsp; DNS: ' + esc(peer.dns) : ''}</div>
    </div>
    <div class="circuit-actions">
      <button class="button sm secondary" onclick="downloadConfig('${esc(ifaceName)}', '${esc(peer.id)}', '${esc(peer.name)}')">Config</button>
      <button class="button sm secondary" onclick="showEditPeerModal('${esc(ifaceName)}', ${jsonAttr(peer)})">Edit</button>
      <button class="button sm ${toggleClass}" onclick="togglePeer('${esc(ifaceName)}', '${esc(peer.id)}', ${!peer.enabled})">${toggleLabel}</button>
      <button class="button sm danger" onclick="showDeletePeerModal('${esc(ifaceName)}', '${esc(peer.id)}', '${esc(peer.name)}')">Delete</button>
    </div>
  </div>`;
}

function jsonAttr(obj) {
  return "'" + JSON.stringify(obj).replace(/'/g, '&#39;').replace(/"/g, '&quot;') + "'";
}

function esc(str) {
  if (str == null) return '';
  return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
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
  document.getElementById('iface-out-iface').value = '';
  document.getElementById('iface-out-iface-group').style.display = 'none';
  showModal('interface-modal');
}

function showEditInterfaceModal(ifaceJson) {
  const iface = typeof ifaceJson === 'string' ? JSON.parse(ifaceJson.replace(/&quot;/g, '"')) : ifaceJson;
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
  document.getElementById('iface-out-iface').value = iface.out_interface || '';
  document.getElementById('iface-out-iface-group').style.display = iface.enable_nat ? 'block' : 'none';
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
    const outIface = document.getElementById('iface-out-iface').value.trim();
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
    const outIface = document.getElementById('iface-out-iface').value.trim();

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

document.addEventListener('DOMContentLoaded', () => {
  document.getElementById('iface-nat').addEventListener('change', function () {
    document.getElementById('iface-out-iface-group').style.display = this.checked ? 'block' : 'none';
  });
  loadInterfaces();
});

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
  const res = await fetch(`${API}/interfaces/${encodeURIComponent(pendingDeleteIface)}`, { method: 'DELETE' });
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
  const peer = typeof peerJson === 'string' ? JSON.parse(peerJson.replace(/&quot;/g, '"')) : peerJson;
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

    const res = await fetch(`${API}/interfaces/${encodeURIComponent(ifaceName)}/peers/${encodeURIComponent(editId)}`, {
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
    const body = { name };
    if (dns !== null) body.dns = dns;
    if (allowedIPs !== null) body.allowed_ips = allowedIPs;

    const res = await fetch(`${API}/interfaces/${encodeURIComponent(ifaceName)}/peers`, {
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

  hidePeerModal();
  loadInterfaces();
}

async function togglePeer(ifaceName, peerId, enabled) {
  const res = await fetch(`${API}/interfaces/${encodeURIComponent(ifaceName)}/peers/${encodeURIComponent(peerId)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  });
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
  const res = await fetch(`${API}/interfaces/${encodeURIComponent(iface)}/peers/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Unknown error' }));
    alert('Error: ' + err.error);
    return;
  }
  hideDeletePeerModal();
  loadInterfaces();
}

async function downloadConfig(ifaceName, peerId, peerName) {
  const res = await fetch(`${API}/interfaces/${encodeURIComponent(ifaceName)}/peers/${encodeURIComponent(peerId)}/config`);
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

function showModal(id) {
  const el = document.getElementById(id);
  el.style.display = 'flex';
}

function hideModal(id) {
  const el = document.getElementById(id);
  el.style.display = 'none';
}

window.addEventListener('click', function (e) {
  ['interface-modal', 'peer-modal', 'delete-iface-modal', 'delete-peer-modal'].forEach(id => {
    const modal = document.getElementById(id);
    if (e.target === modal) hideModal(id);
  });
});