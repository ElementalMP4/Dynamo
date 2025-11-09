document.addEventListener('DOMContentLoaded', () => {
    setupGeneratorForm();
    loadConfigs();
    // generator SSL toggle behavior
    document.getElementById('enable-ssl').addEventListener('change', (e) => {
        const sslCertGroup = document.getElementById('ssl-cert-group');
        sslCertGroup.style.display = e.target.checked ? 'block' : 'none';
        if (e.target.checked) loadServerCerts();
    });
    // generator form modal toggles inside template
});

async function loadServerCerts() {
    const select = document.getElementById('ssl-cert');
    select.innerHTML = '';
    const res = await fetch('/api/certs/list');
    const data = await res.json();
    const serverCerts = data.server || [];

    if (serverCerts.length === 0) {
        const opt = document.createElement('option');
        opt.value = '';
        opt.textContent = '-- no server certificates available --';
        select.appendChild(opt);
        return;
    }

    serverCerts.forEach(cert => {
        const opt = document.createElement('option');
        opt.value = cert.base;
        opt.textContent = cert.cn || cert.base;
        select.appendChild(opt);
    });
}

// --- NGINX config management ---
async function loadConfigs() {
    const res = await fetch('/api/nginx/list');
    if (!res.ok) return;
    const configs = await res.json();
    const list = document.getElementById('nginx-configs');
    list.innerHTML = '';
    configs.forEach(cfg => {
        const li = document.createElement('li');
        li.className = 'config-item';
        if (cfg.enabled) li.classList.add('enabled');

        const name = document.createElement('span');
        name.textContent = cfg.name;
        name.style.cursor = 'pointer';
        name.onclick = () => loadConfig(cfg.name);

        const badge = document.createElement('small');
        badge.textContent = cfg.enabled ? 'enabled' : 'disabled';
        badge.style.marginLeft = '8px';

        li.appendChild(name);
        li.appendChild(badge);
        list.appendChild(li);
    });
}

let currentConfig = null;
let cmEditor = null;

async function loadConfig(name) {
    currentConfig = name;
    const res = await fetch('/api/nginx/read/' + encodeURIComponent(name));
    if (!res.ok) { alert('Failed to load config'); return; }
    const data = await res.json();
    document.getElementById('editor-filename').textContent = name;
    // open modal editor
    document.getElementById('editor-filename-title').textContent = name;
    showEditorModal();
    // initialize CodeMirror if available
    if (window.CodeMirror && !cmEditor) {
        cmEditor = CodeMirror.fromTextArea(document.getElementById('config-editor'), {
            mode: 'nginx',
            theme: 'default',
            lineNumbers: true,
        });
    }
    if (cmEditor) {
        cmEditor.setValue(data.content);
    } else {
        document.getElementById('config-editor').value = data.content;
    }
    document.getElementById('editor-panel').style.display = 'block';
    document.getElementById('reload-btn').style.display = 'none';
    // update toggle button text based on enabled state
    const list = await (await fetch('/api/nginx/list')).json();
    const cfg = list.find(c => c.name === name);
    const toggleBtn = document.getElementById('toggle-btn');
    toggleBtn.textContent = cfg && cfg.enabled ? 'Disable' : 'Enable';
}

async function saveConfig() {
    if (!currentConfig) return;
    const content = cmEditor ? cmEditor.getValue() : document.getElementById('config-editor').value;
    const res = await fetch('/api/nginx/save/' + encodeURIComponent(currentConfig), {
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({content})
    });
    if (res.ok) {
        document.getElementById('reload-btn').style.display = 'inline-block';
        loadConfigs();
    } else {
        alert('Failed to save config');
    }
}

async function toggleConfig() {
    if (!currentConfig) return;
    const list = await (await fetch('/api/nginx/list')).json();
    const cfg = list.find(c => c.name === currentConfig);
    if (!cfg) return;
    if (cfg.enabled) {
        const res = await fetch('/api/nginx/disable/' + encodeURIComponent(currentConfig), {method: 'DELETE'});
        if (res.ok) {
            loadConfigs();
            document.getElementById('reload-btn').style.display = 'inline-block';
            document.getElementById('toggle-btn').textContent = 'Enable';
        } else {
            alert('Failed to disable');
        }
    } else {
        const res = await fetch('/api/nginx/enable/' + encodeURIComponent(currentConfig), {method: 'POST'});
        if (res.ok) {
            loadConfigs();
            document.getElementById('reload-btn').style.display = 'inline-block';
            document.getElementById('toggle-btn').textContent = 'Disable';
        } else {
            alert('Failed to enable');
        }
    }
}

async function deleteConfig() {
    if (!currentConfig) return;
    if (!confirm('Delete ' + currentConfig + '?')) return;
    const res = await fetch('/api/nginx/delete/' + encodeURIComponent(currentConfig), {method: 'DELETE'});
    if (res.ok) {
        document.getElementById('editor-panel').style.display = 'none';
        currentConfig = null;
        loadConfigs();
    } else {
        alert('Failed to delete');
    }
}

async function reloadNginx() {
    const res = await fetch('/api/nginx/reload', {method: 'POST'});
    if (res.ok) {
        document.getElementById('reload-btn').style.display = 'none';
        alert('Reload triggered');
    } else {
        const txt = await res.text();
        alert('Failed to reload: ' + txt);
    }
}

// Generator modal handling
function showGeneratorModal() { document.getElementById('generator-modal').style.display = 'flex'; }
function hideGeneratorModal() { document.getElementById('generator-modal').style.display = 'none'; }

function showEditorModal() { document.getElementById('editor-modal').style.display = 'flex'; }
function closeEditorModal() { document.getElementById('editor-modal').style.display = 'none'; }

function setupGeneratorForm() {
    document.getElementById('generator-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const serverName = document.getElementById('server-name').value.trim();
        const proxyPass = document.getElementById('proxy-pass').value.trim();
        const enableWebsocket = document.getElementById('enable-websocket').checked;
        const enableSsl = document.getElementById('enable-ssl').checked;
        const sslCertBase = document.getElementById('ssl-cert').value;
        const configName = document.getElementById('config-name').value.trim();
        if (!serverName || !proxyPass || !configName) { alert('Missing fields'); return; }
        if (enableSsl && !sslCertBase) { alert('Select cert'); return; }

        const req = {serverName, proxyPass, enableWebsocket, enableSsl, sslCertBase};
        const res = await fetch('/api/nginx/config', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(req)});
        if (!res.ok) { alert('Failed to generate config'); return; }
        const data = await res.json();
        // save generated to chosen name
        const saveRes = await fetch('/api/nginx/save/' + encodeURIComponent(configName), {method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify({content: data.config})});
        if (!saveRes.ok) { alert('Failed to save generated config'); return; }
        hideGeneratorModal();
        loadConfigs();
        loadConfig(configName);
    });
    // ssl toggle inside modal
    document.getElementById('enable-ssl').addEventListener('change', (e) => {
        document.getElementById('ssl-cert-group').style.display = e.target.checked ? 'block' : 'none';
        if (e.target.checked) loadServerCerts();
    });
}

async function generateConfig() {
    const serverName = document.getElementById('server-name').value.trim();
    const proxyPass = document.getElementById('proxy-pass').value.trim();
    const enableWebsocket = document.getElementById('enable-websocket').checked;
    const enableSsl = document.getElementById('enable-ssl').checked;
    const sslCertBase = document.getElementById('ssl-cert').value;

    if (!serverName) {
        alert('Server name is required');
        return;
    }
    if (!proxyPass) {
        alert('Proxy target is required');
        return;
    }
    if (enableSsl && !sslCertBase) {
        alert('Please select a server certificate');
        return;
    }

    const req = {
        serverName,
        proxyPass,
        enableWebsocket,
        enableSsl,
        sslCertBase
    };

    try {
        const res = await fetch('/api/nginx/config', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(req)
        });

        if (!res.ok) {
            let msg = 'Generation failed';
            try {
                const err = await res.json();
                if (err.error) msg = err.error;
            } catch (_) {}
            alert(msg);
            return;
        }

        const data = await res.json();
        document.getElementById('config-text').textContent = data.config;
        document.getElementById('config-output').style.display = 'block';
    } catch (err) {
        alert('Failed to generate config: ' + err.message);
    }
}

function downloadConfig() {
    const config = document.getElementById('config-text').textContent;
    const serverName = document.getElementById('server-name').value.trim();
    const blob = new Blob([config], {type: 'text/plain'});
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `${serverName || 'nginx'}.conf`;
    a.click();
    URL.revokeObjectURL(a.href);
}