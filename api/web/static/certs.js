"use strict";

document.addEventListener('DOMContentLoaded', () => {
    loadCerts();
    setupModals();
});

async function loadCerts() {
    const res = await fetch('/api/certs');
    const data = await res.json();

    ['root', 'server', 'client'].forEach(kind => {
        const container = document.getElementById(kind + '-list');
        container.innerHTML = '';

        const groups = data[kind] || [];
        if (groups.length === 0) {
            container.innerHTML = `<p style="color: var(--text-secondary); font-size: 0.9rem;">No ${kind} certificates yet.</p>`;
            return;
        }

        groups.forEach(group => {
            const bubble = document.createElement('div');
            bubble.className = 'cert-bubble';

            const title = document.createElement('div');
            title.className = 'cert-title';
            title.textContent = group.name;
            title.title = group.name;   // full name on hover

            const meta = document.createElement('div');
            meta.className = 'cert-meta';
            meta.textContent = `${(group.files || []).length} file${group.files?.length === 1 ? '' : 's'}`;

            bubble.appendChild(title);
            bubble.appendChild(meta);
            bubble.addEventListener('click', () => openCertDetail(kind, group));
            container.appendChild(bubble);
        });
    });

    // Populate CA dropdowns
    const rootGroups = data['root'] || [];
    const serverSelect = document.getElementById('server-cabase');
    const clientSelect = document.getElementById('client-cabase');
    if (serverSelect && clientSelect) {
        serverSelect.innerHTML = '';
        clientSelect.innerHTML = '';
        rootGroups.forEach(g => {
            const makeOpt = () => { const o = document.createElement('option'); o.value = g.name; o.textContent = g.name; return o; };
            serverSelect.appendChild(makeOpt());
            clientSelect.appendChild(makeOpt());
        });
        if (serverSelect.options.length === 0) {
            const placeholder = () => { const o = document.createElement('option'); o.value = ''; o.textContent = '-- no root CA available --'; return o; };
            serverSelect.appendChild(placeholder());
            clientSelect.appendChild(placeholder());
        }
    }

    // Populate domain dropdown
    try {
        const dnsRes = await fetch('/api/dns');
        if (dnsRes.ok) {
            const dnsList = await dnsRes.json();
            const domains = [...new Set(dnsList.map(r => r.name).filter(Boolean))].sort();
            const domainSelect = document.getElementById('server-domain');
            if (domainSelect) {
                domainSelect.innerHTML = '';
                if (domains.length === 0) {
                    const opt = document.createElement('option'); opt.value = ''; opt.textContent = '-- no domains configured --';
                    domainSelect.appendChild(opt);
                } else {
                    domains.forEach(d => { const opt = document.createElement('option'); opt.value = d; opt.textContent = d; domainSelect.appendChild(opt); });
                }
            }
        }
    } catch (e) {
        console.error('Failed to load DNS domains:', e);
    }
}

function openCertDetail(kind, group) {
    document.getElementById('cert-detail-title').textContent = group.name;
    document.getElementById('cert-detail-kind').textContent = kind.charAt(0).toUpperCase() + kind.slice(1) + ' Certificate';

    const filesContainer = document.getElementById('cert-detail-files');
    filesContainer.innerHTML = '';
    (group.files || []).forEach(f => {
        const a = document.createElement('a');
        a.className = 'button secondary';
        a.textContent = `Download ${f}`;
        a.href = `/api/certs/download?kind=${kind}&name=${encodeURIComponent(f)}&base=${encodeURIComponent(group.name)}`;
        a.title = `Download ${f}`;
        filesContainer.appendChild(a);
    });

    const deleteBtn = document.getElementById('cert-detail-delete');
    // Replace to clear any old listener
    const freshDelete = deleteBtn.cloneNode(true);
    deleteBtn.parentNode.replaceChild(freshDelete, deleteBtn);
    freshDelete.addEventListener('click', async () => {
        if (!confirm(`Delete and revoke ${group.name}? This cannot be undone.`)) return;
        const res = await fetch(`/api/certs?kind=${kind}&base=${encodeURIComponent(group.name)}`, { method: 'DELETE' });
        if (res.ok) {
            hideCertDetail();
            loadCerts();
        } else {
            let msg = 'failed';
            try { msg = await res.text(); } catch (_) {}
            alert('Failed to delete: ' + msg);
        }
    });

    document.getElementById('cert-detail-modal').style.display = 'flex';
}

function hideCertDetail() {
    document.getElementById('cert-detail-modal').style.display = 'none';
}

function setupModals() {
    document.getElementById('root-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const body = { name: document.getElementById('root-name').value, days: parseInt(document.getElementById('root-days').value || '3650') };
        const res = await fetch('/api/certs/root', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
        if (res.ok) { hideRootModal(); loadCerts(); } else { alert('Failed to create root CA'); }
    });

    document.getElementById('server-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const body = { domain: document.getElementById('server-domain').value, caBase: document.getElementById('server-cabase').value };
        const res = await fetch('/api/certs/server', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
        if (res.ok) { hideServerModal(); loadCerts(); } else { alert('Failed to create server cert'); }
    });

    document.getElementById('client-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const body = { name: document.getElementById('client-name').value, caBase: document.getElementById('client-cabase').value, p12Pass: document.getElementById('client-pass').value, identity: document.getElementById('client-identity').value };
        const res = await fetch('/api/certs/client', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
        if (res.ok) { hideClientModal(); loadCerts(); } else { alert('Failed to create client cert'); }
    });
}

function showRootModal() { document.getElementById('root-modal').style.display = 'flex'; }
function hideRootModal() { document.getElementById('root-modal').style.display = 'none'; }
function showServerModal() { document.getElementById('server-modal').style.display = 'flex'; }
function hideServerModal() { document.getElementById('server-modal').style.display = 'none'; }
function showClientModal() { document.getElementById('client-modal').style.display = 'flex'; }
function hideClientModal() { document.getElementById('client-modal').style.display = 'none'; }