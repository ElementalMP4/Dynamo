document.addEventListener('DOMContentLoaded', () => {
    loadCerts();
    setupModals();
});

async function loadCerts() {
    const res = await fetch('/api/certs/list');
    const data = await res.json();
    ['root','server','client'].forEach(kind => {
        const container = document.getElementById(kind + '-list');
        container.innerHTML = '';
        (data[kind] || []).forEach(group => {
            const li = document.createElement('li');
            li.className = 'cert-bubble';
            const title = document.createElement('div');
            title.className = 'cert-title';
            title.textContent = group.cn || group.base;
            const meta = document.createElement('div');
            meta.className = 'cert-meta';
            const lines = [];
            if (group.cn) lines.push('CN: ' + group.cn);
            if (group.notBefore) lines.push('Valid from: ' + new Date(group.notBefore).toLocaleString());
            if (group.notAfter) lines.push('Expires: ' + new Date(group.notAfter).toLocaleString());

            meta.innerHTML = lines.join('<br>');
            li.appendChild(title);
            li.appendChild(meta);

            // file buttons
            const filesDiv = document.createElement('div');
            filesDiv.className = 'cert-files';
            // tooltip on meta with file list
            meta.title = (group.files || []).map(f => f.name).join('\n');
            (group.files || []).forEach(f => {
                const btn = document.createElement('a');
                btn.className = 'button secondary';
                // friendly label
                let label = (f.ext || '').replace('.', '').toUpperCase();
                if (label === '') label = 'FILE';
                btn.textContent = label;
                btn.href = `/api/certs/download?kind=${kind}&name=${encodeURIComponent(f.name)}`;
                btn.title = `Download ${f.name}`;
                filesDiv.appendChild(btn);
            });

            // delete / revoke button for the group
            const delBtn = document.createElement('button');
            delBtn.type = 'button';
            delBtn.className = 'button danger';
            delBtn.textContent = 'Delete';
            delBtn.title = 'Delete & revoke this certificate (moves files to revoked)';
            delBtn.addEventListener('click', async (e) => {
                if (!confirm(`Delete and revoke ${group.base || group.cn || 'certificate'}? This action cannot be undone.`)) return;
                const res = await fetch(`/api/certs?kind=${kind}&base=${encodeURIComponent(group.base)}`, {method: 'DELETE'});
                if (res.ok) {
                    loadCerts();
                } else {
                    let msg = 'failed';
                    try { msg = await res.text(); } catch (_) {}
                    alert('Failed to delete: ' + msg);
                }
            });

            li.appendChild(filesDiv);
            li.appendChild(delBtn);
            container.appendChild(li);
        });
    });

    // populate CA dropdowns for server/client using root cert filenames (.crt only)
    const rootGroups = (data['root'] || []);
    const serverSelect = document.getElementById('server-cabase');
    const clientSelect = document.getElementById('client-cabase');
    if (serverSelect && clientSelect) {
        serverSelect.innerHTML = '';
        clientSelect.innerHTML = '';
        // populate options from root groups (use base)
        rootGroups.forEach(g => {
            // only include groups that have a .crt file
            const hasCrt = (g.files || []).some(f => f.ext === '.crt' || f.ext === '.pem');
            if (!hasCrt) return;
            const opt1 = document.createElement('option');
            opt1.value = g.base;
            opt1.textContent = g.base;
            serverSelect.appendChild(opt1);

            const opt2 = document.createElement('option');
            opt2.value = g.base;
            opt2.textContent = g.base;
            clientSelect.appendChild(opt2);
        });
        // if no roots exist, add a placeholder
        if (serverSelect.options.length === 0) {
            const opt = document.createElement('option');
            opt.value = '';
            opt.textContent = '-- no root CA available --';
            serverSelect.appendChild(opt);
            clientSelect.appendChild(opt.cloneNode(true));
        }
    }

    // Populate domain dropdown for server certs from configured DNS records
    try {
        const dnsRes = await fetch('/api/dns');
        if (dnsRes.ok) {
            const dnsList = await dnsRes.json();
            const domainSet = new Set();
            dnsList.forEach(r => {
                if (r.name && r.name !== '') domainSet.add(r.name);
            });
            const domains = Array.from(domainSet).sort();
            const domainSelect = document.getElementById('server-domain');
            if (domainSelect) {
                domainSelect.innerHTML = '';
                if (domains.length === 0) {
                    const opt = document.createElement('option');
                    opt.value = '';
                    opt.textContent = '-- no domains configured --';
                    domainSelect.appendChild(opt);
                } else {
                    domains.forEach(d => {
                        const opt = document.createElement('option');
                        opt.value = d;
                        opt.textContent = d;
                        domainSelect.appendChild(opt);
                    });
                }
            }
        }
    } catch (e) {
        console.error('Failed to load DNS domains for cert dropdown:', e);
    }
}

function setupModals() {
    document.getElementById('root-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const body = {name: document.getElementById('root-name').value, days: parseInt(document.getElementById('root-days').value || '3650')};
        const res = await fetch('/api/certs/root', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body)});
        if (res.ok) { hideRootModal(); loadCerts(); } else { alert('Failed to create root'); }
    });

    document.getElementById('server-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const body = {domain: document.getElementById('server-domain').value, caBase: document.getElementById('server-cabase').value};
        const res = await fetch('/api/certs/server', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body)});
        if (res.ok) { hideServerModal(); loadCerts(); } else { alert('Failed to create server cert'); }
    });

    document.getElementById('client-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const body = {name: document.getElementById('client-name').value, caBase: document.getElementById('client-cabase').value, p12Pass: document.getElementById('client-pass').value};
        const res = await fetch('/api/certs/client', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body)});
        if (res.ok) { hideClientModal(); loadCerts(); } else { alert('Failed to create client cert'); }
    });
}

function showRootModal(){ document.getElementById('root-modal').style.display='flex'; }
function hideRootModal(){ document.getElementById('root-modal').style.display='none'; }
function showServerModal(){ document.getElementById('server-modal').style.display='flex'; }
function hideServerModal(){ document.getElementById('server-modal').style.display='none'; }
function showClientModal(){ document.getElementById('client-modal').style.display='flex'; }
function hideClientModal(){ document.getElementById('client-modal').style.display='none'; }
