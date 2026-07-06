let pendingDeleteId = null;

function buildMeta(rc) {
    const parts = [];
    if (rc.type === 'LOCAL') {
        if (rc.protocol) parts.push(rc.protocol.toUpperCase());
        if (rc.src_port) parts.push(`:${rc.src_port}`);
        if (rc.redirect_port) parts.push(`-> :${rc.redirect_port}`);
        parts.push('(loopback)');
    } else {
        if (rc.in_interface) parts.push(`in: ${rc.in_interface}`);
        if (rc.protocol) parts.push(rc.protocol.toUpperCase());
        if (rc.src_port) parts.push(`:${rc.src_port}`);
        if (rc.type === 'DNAT' && rc.dest_ip && rc.dest_port) {
            parts.push(`-> ${rc.dest_ip}:${rc.dest_port}`);
        } else if (rc.type === 'REDIRECT' && rc.redirect_port) {
            parts.push(`-> :${rc.redirect_port}`);
        }
    }
    return parts.join('  ·  ');
}

function renderCircuits(circuits) {
    const list = document.getElementById('circuits-list');

    if (!circuits || circuits.length === 0) {
        list.innerHTML = `<p style="color:var(--text-secondary);text-align:center;padding:2rem 0;">No circuits yet. Click <strong>Add Circuit</strong> to get started.</p>`;
        return;
    }

    list.innerHTML = circuits.map(rc => `
      <div class="circuit-card ${rc.enabled ? 'enabled' : 'disabled'}" id="card-${rc.id}">
        <div class="status-dot ${rc.enabled ? 'on' : 'off'}"></div>
        <div class="circuit-info">
          <div class="circuit-info-row">
            <span class="circuit-name">${escHtml(rc.name)}</span>
            <span class="circuit-badge">${escHtml(rc.type)}</span>
          </div>
          <div class="circuit-meta">${buildMeta(rc)}</div>
        </div>
        <div class="circuit-actions">
          ${rc.enabled
            ? `<button class="button sm secondary" onclick="toggleCircuit(${rc.id}, false)">Disable</button>`
            : `<button class="button sm success"   onclick="toggleCircuit(${rc.id}, true)">Enable</button>`
        }
          <button class="button sm secondary" onclick="showEditModal(${rc.id})" ${rc.enabled ? 'disabled title="Disable circuit before editing"' : ''}>Edit</button>
          <button class="button sm danger" onclick="showDeleteModal(${rc.id}, this.dataset.name)" data-name="${escHtml(rc.name)}">Delete</button>
        </div>
      </div>
    `).join('');
}

function escHtml(s) {
    return String(s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
function escAttr(s) {
    return String(s ?? '').replace(/'/g, "\\'");
}

async function loadCircuits() {
    try {
        const res = await fetch('/api/circuits');
        const data = await res.json();
        renderCircuits(data);
    } catch (e) {
        document.getElementById('circuits-list').innerHTML =
            `<p style="color:var(--danger)">Failed to load circuits: ${e.message}</p>`;
    }
}

async function toggleCircuit(id, enable) {
    const endpoint = `/api/circuits/${id}/${enable ? 'enable' : 'disable'}`;
    try {
        const res = await fetch(endpoint, { method: 'POST' });
        if (!res.ok) {
            const body = await res.json().catch(() => ({}));
            alert(`Failed: ${body.error || res.statusText}`);
            return;
        }
        await loadCircuits();
    } catch (e) {
        alert(`Error: ${e.message}`);
    }
}

async function confirmDelete() {
    if (!pendingDeleteId) return;
    try {
        const res = await fetch(`/api/circuits/${pendingDeleteId}`, { method: 'DELETE' });
        if (!res.ok) {
            const body = await res.json().catch(() => ({}));
            alert(`Failed: ${body.error || res.statusText}`);
            return;
        }
        hideDeleteModal();
        await loadCircuits();
    } catch (e) {
        alert(`Error: ${e.message}`);
    }
}

function onTypeChange() {
    const type = document.getElementById('circuit-type').value;
    const isDNAT = type === 'DNAT';
    const isRedirect = type === 'REDIRECT';
    const isLocal = type === 'LOCAL';
    document.getElementById('redirect-port-group').style.display = (isRedirect || isLocal) ? '' : 'none';
    document.getElementById('dest-ip-group').style.display = isDNAT ? '' : 'none';
    document.getElementById('dest-port-group').style.display = isDNAT ? '' : 'none';
    document.getElementById('circuit-interface').closest('.form-group').style.display = isLocal ? 'none' : '';
}

function formToPayload() {
    return {
        name: document.getElementById('circuit-name').value,
        type: document.getElementById('circuit-type').value,
        in_interface: document.getElementById('circuit-interface').value,
        protocol: document.getElementById('circuit-protocol').value,
        src_port: parseInt(document.getElementById('circuit-src-port').value) || 0,
        redirect_port: parseInt(document.getElementById('circuit-redirect-port').value) || 0,
        dest_ip: document.getElementById('circuit-dest-ip').value,
        dest_port: parseInt(document.getElementById('circuit-dest-port').value) || 0,
    };
}

function populateForm(rc) {
    document.getElementById('circuit-id').value = rc.id;
    document.getElementById('circuit-name').value = rc.name;
    document.getElementById('circuit-type').value = rc.type;
    document.getElementById('circuit-interface').value = rc.in_interface || '';
    document.getElementById('circuit-protocol').value = rc.protocol || 'tcp';
    document.getElementById('circuit-src-port').value = rc.src_port || '';
    document.getElementById('circuit-redirect-port').value = rc.redirect_port || '';
    document.getElementById('circuit-dest-ip').value = rc.dest_ip || '';
    document.getElementById('circuit-dest-port').value = rc.dest_port || '';
    onTypeChange();
}

document.getElementById('circuit-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const id = document.getElementById('circuit-id').value;
    const payload = formToPayload();
    const isEdit = !!id;

    try {
        const res = await fetch(isEdit ? `/api/circuits/${id}` : '/api/circuits', {
            method: isEdit ? 'PUT' : 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });

        if (!res.ok) {
            const body = await res.json().catch(() => ({}));
            alert(`Failed: ${body.error || res.statusText}`);
            return;
        }

        hideModal();
        await loadCircuits();
    } catch (err) {
        alert(`Error: ${err.message}`);
    }
});

async function loadInterfacesIntoEditor() {
    const interfaces = await fetch(`/api/system/interfaces`);
    if (!interfaces.ok) {
        alert('Failed to load interfaces');
        return;
    }

    const interfacesJson = await interfaces.json();
    const select = document.getElementById('circuit-interface');
    select.innerHTML = "";

    for (const iface of interfacesJson.interfaces) {
        if (iface.startsWith("veth")) continue;
        if (iface.startsWith("docker")) continue;
        if (iface.startsWith("br-")) continue;

        const option = document.createElement('option');
        option.value = iface;
        option.textContent = iface;
        select.appendChild(option);
    }
}

async function showCreateModal() {
    document.getElementById('modal-title').textContent = 'Create Circuit';
    document.getElementById('modal-submit-btn').textContent = 'Create';

    const form = document.getElementById('circuit-form');
    form.reset();

    await loadInterfacesIntoEditor();

    document.getElementById('circuit-id').value = '';
    document.getElementById('circuit-type').value = 'DNAT';
    document.getElementById('circuit-protocol').value = 'tcp';

    onTypeChange();
    document.getElementById('circuit-modal').style.display = 'flex';
}

async function showEditModal(id) {
    try {
        const circuit = await fetch(`/api/circuits/${id}`);
        if (!circuit.ok) {
            alert('Failed to load circuit');
            return;
        }

        const circuitJson = await circuit.json();

        await loadInterfacesIntoEditor();
        document.getElementById('modal-title').textContent = 'Edit Circuit';
        document.getElementById('modal-submit-btn').textContent = 'Save';
        populateForm(circuitJson);
        document.getElementById('circuit-modal').style.display = 'flex';
    } catch (e) {
        alert(`Error: ${e.message}`);
    }
}

function hideModal() {
    document.getElementById('circuit-modal').style.display = 'none';
}

function showDeleteModal(id, name) {
    pendingDeleteId = id;
    document.getElementById('delete-name').textContent = name;
    document.getElementById('delete-modal').style.display = 'flex';
}

function hideDeleteModal() {
    pendingDeleteId = null;
    document.getElementById('delete-modal').style.display = 'none';
}

['circuit-modal', 'delete-modal'].forEach(id => {
    document.getElementById(id).addEventListener('click', function (e) {
        if (e.target === this) this.style.display = 'none';
    });
});

loadCircuits();