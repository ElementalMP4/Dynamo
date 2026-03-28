# Dynamo

Network Management Platform

## Dependencies

- NGINX
- dnsmasq
- Conntrack
- iptables
- iptables-save

## Features

- Manages DNS records in dnsmasq
- Creates Root CAs, Service Certificates & Client Certificates
- Creates NGINX mTLS-ready configurations
- Manages iptables routing rules for a variety of cases 

## Potential Future Features

- WireGuard client management (create/enable/disable/delete clients)
- Better dnsmasq auto config

## Example NGINX config

```nginx
include /opt/Dynamo/ssl/server/<service name>/nginx-redirect.conf;

server {
    include /opt/Dynamo/ssl/server/<service name>/nginx-mtls.conf;
    location / {
        proxy_pass http://127.0.0.1:8080;
    }
}
```

Include looks something like this:

```nginx
listen 443 ssl;
server_name <service name>;

ssl_certificate     /opt/Dynamo/ssl/server/<service name>/<service name>.crt;
ssl_certificate_key /opt/Dynamo/ssl/server/<service name>/<service name>.key;

ssl_client_certificate /opt/Dynamo/ssl/root/elitedesk/elitedesk.crt;
ssl_verify_client on;

ssl_protocols TLSv1.2 TLSv1.3;
ssl_prefer_server_ciphers on;
```

## dnsmasq

`/etc/dnsmasq.conf` should look something like this:
```ini
# Port 54 doesn't conflict with systemd-resolved
port=54

# Accept all traffic
listen-address=0.0.0.0

# Do not listen on wg0 or loopback (optional)
interface=eno1

# MANDATORY
conf-dir=/etc/dnsmasq.d

# Upstream DNS servers for public domains
server=1.1.1.1
server=1.0.0.1

# Your custom domain
domain=elementalmp4.private
local=/elementalmp4.private/
```

## Dynamo Config File

```jsonc
{
    "serverBind": "127.0.0.1:6789", // Bind address and port are mandatory
    "certsDir": "./ssl", // Where to store SSL certs
    "dnsmasqConfigFile": "/etc/dnsmasq.d/dynamo.conf" // Absolute path to dnsmasq config file (NOT the main config file!)
}
```