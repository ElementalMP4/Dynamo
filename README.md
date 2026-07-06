# Dynamo

Network Management Platform

![a screenshot showing a demonstration of the Dynamo homepage](demo.png "Dynamo Home")

## Features

- Manages DNS records in dnsmasq
- Creates Root CAs, Service Certificates & Client Certificates
- Creates Nginx mTLS-ready configurations with http -> https redirects & CRLs
- Manages iptables routing rules for a variety of redirects and circuit breakers

## Requires

- dnsmasq
- conntrack
- iptables (or nftables)
- wireguard

Dynamo also generates configuration files that can be included in Nginx, but Nginx is not a mandatory dependency.

Dynamo should not be run as root - however it does require read and write access to `/etc/Dynamo` and `/etc/dnsmasq.d` as well as authorisation to manipulate iptables. 

## Example Nginx config

```nginx
include /etc/Dynamo/ssl/server/<service name>/nginx-redirect.conf;

server {
    include /etc/Dynamo/ssl/server/<service name>/nginx-mtls.conf;
    location / {
        proxy_pass http://127.0.0.1:8080;
    }
}
```

Include looks something like this:

```nginx
listen 443 ssl;
server_name <service name>;

ssl_certificate     /etc/Dynamo/ssl/server/<service name>/<service name>.crt;
ssl_certificate_key /etc/Dynamo/ssl/server/<service name>/<service name>.key;

ssl_client_certificate /etc/Dynamo/ssl/root/elitedesk/elitedesk.crt;
ssl_verify_client on;

ssl_protocols TLSv1.2 TLSv1.3;
ssl_prefer_server_ciphers on;
```

## Example dnsmasq config

`/etc/dnsmasq.conf` must contain a bare minimum of the following:
```ini
# Port 54 doesn't conflict with systemd-resolved
port=54

# Upstream DNS servers for public domains (you can also use your server's existing DNS resolver, or AdGuard for example)
server=1.1.1.1
server=1.0.0.1

# Adds the directory that contains Dynamo's DNS config file
conf-dir=/etc/dnsmasq.d
```

**Hint:** don't use port 53 as this will conflict with your server's built-in DNS. Use a different port, then use a Dynamo circuit to redirect inbound traffic from port 53 to your chosen dnsmasq port.

## Dynamo Config File

```jsonc
{
    "serverBind": "127.0.0.1:6789", // Bind address and port are mandatory. If using nginx (or other proxy) make sure to bind Dynamo to localhost only
    "certsDir": "/etc/Dynamo/ssl", // Where to store SSL certs
    "dnsmasqConfigFile": "/etc/dnsmasq.d/dynamo.conf", // Absolute path to dnsmasq config file (NOT the main config file!)
    "dataDirectory": "/etc/Dynamo/data", // Where to store other data files
    "instance": "Server Name"
}
```
