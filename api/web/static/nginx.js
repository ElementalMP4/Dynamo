document.addEventListener('DOMContentLoaded', () => {
    const sslToggle = document.getElementById('enable-ssl');
    const sslCertGroup = document.getElementById('ssl-cert-group');
    sslToggle.addEventListener('change', () => {
        sslCertGroup.style.display = sslToggle.checked ? 'block' : 'none';
        if (sslToggle.checked) {
            loadServerCerts();
        }
    });
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