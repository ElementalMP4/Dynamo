import { jsonHeaders, request, formatRate, Field, PageHeader, Spinner, Modal, EmptyState } from "../utils.js";
import Mini from "../mini.js";

export default function VpnScreen() {
    const API = "/api/vpn"; 
    const [interfaces, setInterfaces] = Mini.useState([]);
    const [systemIfaces, setSystemIfaces] = Mini.useState([]);
    const [loading, setLoading] = Mini.useState(true);
    const [error, setError] = Mini.useState("");
    const [expanded, setExpanded] = Mini.useState({});

    const [ifaceModal, setIfaceModal] = Mini.useState({ open: false, editName: "" });
    const [peerModal, setPeerModal] = Mini.useState({ open: false, iface: "", editId: "" });
    const [deleteIface, setDeleteIface] = Mini.useState(null);
    const [deletePeer, setDeletePeer] = Mini.useState(null);
    const [qrModal, setQrModal] = Mini.useState(null);
    const qrRequest = Mini.useState(() => ({ id: 0 }))[0];

    const [ifaceForm, setIfaceForm] = Mini.useState({
        name: "",
        address: "",
        listen_port: "",
        endpoint: "",
        default_dns: "",
        default_allowed_ips: "",
        enable_nat: false,
        out_interface: ""
    });

    const [peerForm, setPeerForm] = Mini.useState({
        name: "",
        dns: "",
        allowed_ips: ""
    });

    const metricsState = Mini.useState(() => ({ samples: {}, times: {} }))[0];

    function trackRates(key, rx, tx) {
        const now = Date.now();
        const prev = metricsState.samples[key];
        let rxRate = 0;
        let txRate = 0;

        if (prev) {
            const elapsed = Math.max((now - metricsState.times[key]) / 1000, 0.001);
            rxRate = ((rx - prev.rx) / elapsed) * 8;
            txRate = ((tx - prev.tx) / elapsed) * 8;

        }

        metricsState.samples[key] = { rx, tx };
        metricsState.times[key] = now;

        return { rxRate, txRate };
    }

    async function loadSystemIfaces(selected = "") {
        try {
            const data = await request("/api/system/interfaces");
            const values = data.interfaces || [];
            setSystemIfaces(values);
            if (selected && values.includes(selected)) {
                setIfaceForm(old => ({ ...old, out_interface: selected }));
            } else if (!selected && values.length && !ifaceForm.out_interface) {
                setIfaceForm(old => ({ ...old, out_interface: values[0] }));
            }
        } catch (err) {
            setError(err.error || "Failed to load system interfaces");
        }
    }

    async function loadInterfaces() {
        setLoading(true);
        try {
            const data = await request(`${API}/interfaces`);
            setInterfaces(data || []);
            setError("");
        } catch (err) {
            setError(err.error || "Failed to load VPN interfaces");
        } finally {
            setLoading(false);
        }
    }

    async function pollMetrics() {
        try {
            const data = await request(`${API}/metrics`);
            if (!data?.interfaces) return;

            setInterfaces(prev => prev.map(iface => {
                const metricsIface = data.interfaces.find(item => item.name === iface.name);
                if (!metricsIface) return iface;

                const ifaceRates = trackRates(`iface-${iface.name}`, Number(metricsIface.rx) || 0, Number(metricsIface.tx) || 0);
                const nextPeers = (iface.peers || []).map(peer => {
                    const metricsPeer = (metricsIface.peers || []).find(item => item.id === peer.id);
                    if (!metricsPeer) return peer;

                    const peerRates = trackRates(`peer-${peer.id}`, Number(metricsPeer.rx) || 0, Number(metricsPeer.tx) || 0);
                    return {
                        ...peer,
                        rxRate: peerRates.rxRate,
                        txRate: peerRates.txRate,
                        active: !!metricsPeer.active
                    };
                });

                return {
                    ...iface,
                    rxRate: ifaceRates.rxRate,
                    txRate: ifaceRates.txRate,
                    peers: nextPeers
                };
            }));
        } catch (_) {
            // Polling errors are transient; keep the last rendered values.
        }
    }

    Mini.useEffect(() => {
        let alive = true;

        async function boot() {
            await Promise.all([loadInterfaces(), loadSystemIfaces()]);
            if (!alive) return;

            async function loop() {
                if (!alive) return;
                await pollMetrics();
                if (alive) setTimeout(loop, 1000);
            }

            loop();
        }

        boot();

        return () => {
            alive = false;
            qrRequest.id++;
        };
    }, []);

    function openCreateInterface() {
        setIfaceModal({ open: true, editName: "" });
        setIfaceForm({
            name: "",
            address: "",
            listen_port: "",
            endpoint: "",
            default_dns: "",
            default_allowed_ips: "",
            enable_nat: false,
            out_interface: systemIfaces[0] || ""
        });
    }

    function openEditInterface(iface) {
        setIfaceModal({ open: true, editName: iface.name });
        setIfaceForm({
            name: iface.name,
            address: iface.address || "",
            listen_port: String(iface.listen_port || ""),
            endpoint: iface.endpoint || "",
            default_dns: iface.default_dns || "",
            default_allowed_ips: iface.default_allowed_ips || "",
            enable_nat: !!iface.enable_nat,
            out_interface: iface.out_interface || ""
        });
        if (iface.enable_nat) loadSystemIfaces(iface.out_interface || "");
    }

    async function saveInterface(event) {
        event.preventDefault();
        const isEdit = !!ifaceModal.editName;

        try {
            if (isEdit) {
                const body = {
                    endpoint: ifaceForm.endpoint || undefined,
                    default_dns: ifaceForm.default_dns || undefined,
                    default_allowed_ips: ifaceForm.default_allowed_ips || undefined,
                    enable_nat: !!ifaceForm.enable_nat
                };
                if (ifaceForm.enable_nat && ifaceForm.out_interface) {
                    body.out_interface = ifaceForm.out_interface;
                }

                await request(`${API}/interfaces/${encodeURIComponent(ifaceModal.editName)}`, {
                    method: "PUT",
                    headers: jsonHeaders(),
                    body: JSON.stringify(body)
                });
            } else {
                const body = {
                    name: ifaceForm.name.trim(),
                    address: ifaceForm.address.trim(),
                    listen_port: parseInt(ifaceForm.listen_port || "0", 10),
                    endpoint: ifaceForm.endpoint.trim(),
                    enable_nat: !!ifaceForm.enable_nat
                };
                if (ifaceForm.default_dns) body.default_dns = ifaceForm.default_dns;
                if (ifaceForm.default_allowed_ips) body.default_allowed_ips = ifaceForm.default_allowed_ips;
                if (ifaceForm.enable_nat && ifaceForm.out_interface) body.out_interface = ifaceForm.out_interface;

                await request(`${API}/interfaces`, {
                    method: "POST",
                    headers: jsonHeaders(),
                    body: JSON.stringify(body)
                });
            }

            setIfaceModal({ open: false, editName: "" });
            loadInterfaces();
        } catch (err) {
            await showAlert(`Failed to save interface: ${err.error}`);
        }
    }

    async function toggleInterface(ifaceName, enabled) {
        try {
            await request(`${API}/interfaces/${encodeURIComponent(ifaceName)}/toggle`, {
                method: "PUT",
                headers: jsonHeaders(),
                body: JSON.stringify({ enabled: !enabled })
            });
            loadInterfaces();
        } catch (err) {
            await showAlert(`Failed to toggle interface: ${err.error}`);
        }
    }

    async function confirmDeleteInterface() {
        if (!deleteIface) return;
        try {
            await request(`${API}/interfaces/${encodeURIComponent(deleteIface.name)}`, { method: "DELETE" });
            setDeleteIface(null);
            loadInterfaces();
        } catch (err) {
            await showAlert(`Failed to delete interface: ${err.error}`);
        }
    }

    function openAddPeer(ifaceName) {
        setPeerModal({ open: true, iface: ifaceName, editId: "" });
        setPeerForm({ name: "", dns: "", allowed_ips: "" });
    }

    function openEditPeer(ifaceName, peer) {
        setPeerModal({ open: true, iface: ifaceName, editId: peer.id });
        setPeerForm({
            name: peer.name || "",
            dns: peer.dns || "",
            allowed_ips: peer.allowed_ips || ""
        });
    }

    async function savePeer(event) {
        event.preventDefault();
        const isEdit = !!peerModal.editId;
        const body = { name: peerForm.name.trim() };
        if (peerForm.dns) body.dns = peerForm.dns;
        if (peerForm.allowed_ips) body.allowed_ips = peerForm.allowed_ips;

        try {
            if (isEdit) {
                await request(`${API}/interfaces/${encodeURIComponent(peerModal.iface)}/peers/${encodeURIComponent(peerModal.editId)}`, {
                    method: "PUT",
                    headers: jsonHeaders(),
                    body: JSON.stringify(body)
                });
            } else {
                await request(`${API}/interfaces/${encodeURIComponent(peerModal.iface)}/peers`, {
                    method: "POST",
                    headers: jsonHeaders(),
                    body: JSON.stringify(body)
                });
            }

            setPeerModal({ open: false, iface: "", editId: "" });
            loadInterfaces();
        } catch (err) {
            await showAlert(`Failed to save peer: ${err.error}`);
        }
    }

    async function togglePeer(ifaceName, peerId, nextEnabled) {
        try {
            await request(`${API}/interfaces/${encodeURIComponent(ifaceName)}/peers/${encodeURIComponent(peerId)}`, {
                method: "PUT",
                headers: jsonHeaders(),
                body: JSON.stringify({ enabled: nextEnabled })
            });
            loadInterfaces();
        } catch (err) {
            await showAlert(`Failed to toggle peer: ${err.error}`);
        }
    }

    async function confirmDeletePeer() {
        if (!deletePeer) return;
        try {
            await request(`${API}/interfaces/${encodeURIComponent(deletePeer.iface)}/peers/${encodeURIComponent(deletePeer.id)}`, {
                method: "DELETE"
            });
            setDeletePeer(null);
            loadInterfaces();
        } catch (err) {
            await showAlert(`Failed to delete peer: ${err.error}`);
        }
    }

    function closeQrModal() {
        qrRequest.id++;
        setQrModal(null);
    }

    async function showConfigQr(ifaceName, peer) {
        const requestId = ++qrRequest.id;
        setQrModal({ name: peer.name, loading: true, image: "", error: "" });
        try {
            const [{ default: qrcode }, text] = await Promise.all([
                import("https://cdn.jsdelivr.net/npm/qrcode-generator@2.0.4/dist/qrcode.mjs"),
                request(`${API}/interfaces/${encodeURIComponent(ifaceName)}/peers/${encodeURIComponent(peer.id)}/config`)
            ]);
            if (qrRequest.id !== requestId) return;
            const code = qrcode(0, "M");
            code.addData(text, "Byte");
            code.make();
            setQrModal({ name: peer.name, loading: false, image: code.createDataURL(6, 24), error: "" });
        } catch (err) {
            if (qrRequest.id !== requestId) return;
            setQrModal({ name: peer.name, loading: false, image: "", error: err.error || err.message || "Failed to generate QR code" });
        }
    }

    async function downloadConfig(ifaceName, peerId, peerName) {
        try {
            const text = await request(`${API}/interfaces/${encodeURIComponent(ifaceName)}/peers/${encodeURIComponent(peerId)}/config`);
            const blob = new Blob([text], { type: "text/plain" });
            const url = URL.createObjectURL(blob);
            const link = document.createElement("a");
            link.href = url;
            link.download = `${peerName}.conf`;
            link.click();
            URL.revokeObjectURL(url);
        } catch (err) {
            await showAlert(`Failed to download config: ${err.error}`);
        }
    }

    return Mini.div({ class: "mui-screen" },
        PageHeader({
            title: "VPN Interfaces",
            subtitle: "Manage WireGuard-style interfaces, peers, and live throughput",
            actions: [Mini.button({ class: "mui-button", onclick: openCreateInterface }, "Add Interface")]
        }),
        error && Mini.div({ class: "mui-alert danger" }, error),
        loading
            ? Spinner({ label: "Loading VPN interfaces..." })
            : interfaces.length === 0
                ? EmptyState({ title: "No interfaces configured", subtitle: "Create your first VPN interface to begin." })
                : Mini.div({ class: "mui-stack" },
                    interfaces.map(iface => {
                        const isExpanded = !!expanded[iface.name];
                        const peers = iface.peers || [];
                        return Mini.div({ class: "mui-card", key: iface.name },
                            Mini.div({ class: "mui-row spread wrap" },
                                Mini.div({ class: "mui-stack tight" },
                                    Mini.div({ class: "mui-row" },
                                        Mini.span({ class: `mui-status-dot ${iface.enabled === false ? "off" : "on"}` }),
                                        Mini.strong({}, iface.name),
                                        Mini.span({ class: "mui-badge" }, `NAT ${iface.enable_nat ? "ON" : "OFF"}`)
                                    ),
                                    Mini.small({ class: "mui-muted" }, `${iface.address || ""}  :${iface.listen_port || ""}  ${iface.endpoint || ""}`),
                                    Mini.small({ class: "mui-muted" }, `TX ${formatRate(iface.txRate || 0)}  RX ${formatRate(iface.rxRate || 0)}`)
                                ),
                                Mini.div({ class: "mui-row wrap" },
                                    Mini.button({ class: "mui-button secondary sm", onclick: () => openEditInterface(iface) }, "Edit"),
                                    Mini.button({
                                        class: `mui-button ${iface.enabled === false ? "success" : "secondary"} sm`,
                                        onclick: () => toggleInterface(iface.name, iface.enabled !== false)
                                    }, iface.enabled === false ? "Enable" : "Disable"),
                                    Mini.button({ class: "mui-button secondary sm", onclick: () => openAddPeer(iface.name) }, "Add Peer"),
                                    Mini.button({ class: "mui-button danger sm", onclick: () => setDeleteIface(iface) }, "Delete")
                                )
                            ),
                            Mini.button({
                                class: "mui-button ghost sm",
                                onclick: () => setExpanded({ ...expanded, [iface.name]: !isExpanded })
                            }, `${isExpanded ? "Hide" : "Show"} Peers (${peers.length})`),
                            isExpanded && Mini.div({ class: "mui-stack" },
                                peers.length === 0
                                    ? Mini.p({ class: "mui-muted" }, "No peers yet")
                                    : peers.map(peer => Mini.div({ class: "mui-sub-card", key: peer.id },
                                        Mini.div({ class: "mui-row spread wrap" },
                                            Mini.div({ class: "mui-stack tight" },
                                                Mini.div({ class: "mui-row" },
                                                    Mini.span({ class: `mui-status-dot ${peer.enabled ? "on" : "off"}` }),
                                                    Mini.span({ class: `mui-status-dot ${peer.active ? "on" : "off"}` }),
                                                    Mini.strong({}, peer.name)
                                                ),
                                                Mini.small({ class: "mui-muted" }, `${peer.ip || ""}${peer.dns ? `  DNS: ${peer.dns}` : ""}`),
                                                Mini.small({ class: "mui-muted" }, `TX ${formatRate(peer.txRate || 0)}  RX ${formatRate(peer.rxRate || 0)}`)
                                            ),
                                            Mini.div({ class: "mui-row wrap" },
                                                Mini.button({ class: "mui-button secondary sm", onclick: () => downloadConfig(iface.name, peer.id, peer.name) }, "Config"),
                                                Mini.button({ class: "mui-button secondary sm", title: "Show WireGuard config QR code", onclick: () => showConfigQr(iface.name, peer) }, "QR Code"),
                                                Mini.button({ class: "mui-button secondary sm", onclick: () => openEditPeer(iface.name, peer) }, "Edit"),
                                                Mini.button({
                                                    class: `mui-button ${peer.enabled ? "secondary" : "success"} sm`,
                                                    onclick: () => togglePeer(iface.name, peer.id, !peer.enabled)
                                                }, peer.enabled ? "Disable" : "Enable"),
                                                Mini.button({ class: "mui-button danger sm", onclick: () => setDeletePeer({ iface: iface.name, id: peer.id, name: peer.name }) }, "Delete")
                                            )
                                        )
                                    ))
                            )
                        );
                    })
                ),

        Mini.vnode(Modal, {
            open: !!qrModal,
            title: qrModal ? `WireGuard QR Code: ${qrModal.name}` : "WireGuard QR Code",
            onClose: closeQrModal
        },
            qrModal && Mini.div({ class: "mui-stack" },
                qrModal.loading && Spinner({ label: "Loading client config..." }),
                qrModal.error && Mini.div({ class: "mui-alert danger", role: "alert" }, qrModal.error),
                qrModal.image && Mini.img({
                    src: qrModal.image,
                    alt: `WireGuard configuration QR code for ${qrModal.name}`,
                    width: 320,
                    height: 320,
                    style: "display:block;width:100%;max-width:320px;height:auto;aspect-ratio:1;margin:0 auto;image-rendering:pixelated;"
                }),
                qrModal.image && Mini.p({ class: "mui-muted" }, "Contains the client's private key. Keep this QR code private.")
            )
        ),

        Mini.vnode(Modal, {
            open: ifaceModal.open,
            title: ifaceModal.editName ? "Edit Interface" : "Create Interface",
            onClose: () => setIfaceModal({ open: false, editName: "" })
        },
            Mini.vnode("form", { class: "mui-stack", onsubmit: saveInterface },
                Field({
                    label: "Name", children: Mini.vnode("input", {
                        value: ifaceForm.name,
                        disabled: !!ifaceModal.editName,
                        required: !ifaceModal.editName,
                        oninput: event => setIfaceForm({ ...ifaceForm, name: event.target.value })
                    })
                }),
                !ifaceModal.editName && Field({
                    label: "Address (CIDR)", children: Mini.vnode("input", {
                        value: ifaceForm.address,
                        required: true,
                        oninput: event => setIfaceForm({ ...ifaceForm, address: event.target.value })
                    })
                }),
                !ifaceModal.editName && Field({
                    label: "Listen Port", children: Mini.vnode("input", {
                        type: "number",
                        value: ifaceForm.listen_port,
                        required: true,
                        oninput: event => setIfaceForm({ ...ifaceForm, listen_port: event.target.value })
                    })
                }),
                Field({
                    label: "Endpoint (host:port)", children: Mini.vnode("input", {
                        value: ifaceForm.endpoint,
                        required: !ifaceModal.editName,
                        oninput: event => setIfaceForm({ ...ifaceForm, endpoint: event.target.value })
                    })
                }),
                Field({
                    label: "Default DNS", children: Mini.vnode("input", {
                        value: ifaceForm.default_dns,
                        oninput: event => setIfaceForm({ ...ifaceForm, default_dns: event.target.value })
                    })
                }),
                Field({
                    label: "Default Allowed IPs", children: Mini.vnode("input", {
                        value: ifaceForm.default_allowed_ips,
                        oninput: event => setIfaceForm({ ...ifaceForm, default_allowed_ips: event.target.value })
                    })
                }),
                Mini.label({ class: "mui-check" },
                    Mini.vnode("input", {
                        type: "checkbox",
                        checked: ifaceForm.enable_nat,
                        onchange: async event => {
                            const checked = event.target.checked;
                            setIfaceForm({ ...ifaceForm, enable_nat: checked });
                            if (checked) await loadSystemIfaces(ifaceForm.out_interface);
                        }
                    }),
                    Mini.span({}, "Enable NAT masquerade")
                ),
                ifaceForm.enable_nat && Field({
                    label: "Outbound Interface", children: Mini.vnode("select", {
                        value: ifaceForm.out_interface,
                        onchange: event => setIfaceForm({ ...ifaceForm, out_interface: event.target.value })
                    },
                        systemIfaces.map(iface => Mini.vnode("option", { value: iface, key: iface }, iface))
                    )
                }),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ type: "button", class: "mui-button secondary", onclick: () => setIfaceModal({ open: false, editName: "" }) }, "Cancel"),
                    Mini.button({ type: "submit", class: "mui-button" }, ifaceModal.editName ? "Save" : "Create")
                )
            )
        ),

        Mini.vnode(Modal, {
            open: peerModal.open,
            title: peerModal.editId ? "Edit Peer" : "Add Peer",
            onClose: () => setPeerModal({ open: false, iface: "", editId: "" })
        },
            Mini.vnode("form", { class: "mui-stack", onsubmit: savePeer },
                Field({
                    label: "Name", children: Mini.vnode("input", {
                        value: peerForm.name,
                        required: true,
                        oninput: event => setPeerForm({ ...peerForm, name: event.target.value })
                    })
                }),
                Field({
                    label: "DNS", hint: "Leave blank for interface default", children: Mini.vnode("input", {
                        value: peerForm.dns,
                        oninput: event => setPeerForm({ ...peerForm, dns: event.target.value })
                    })
                }),
                Field({
                    label: "Allowed IPs", hint: "Leave blank for interface default", children: Mini.vnode("input", {
                        value: peerForm.allowed_ips,
                        oninput: event => setPeerForm({ ...peerForm, allowed_ips: event.target.value })
                    })
                }),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ type: "button", class: "mui-button secondary", onclick: () => setPeerModal({ open: false, iface: "", editId: "" }) }, "Cancel"),
                    Mini.button({ type: "submit", class: "mui-button" }, peerModal.editId ? "Save" : "Add")
                )
            )
        ),

        Mini.vnode(Modal, {
            open: !!deleteIface,
            title: "Delete Interface",
            onClose: () => setDeleteIface(null)
        },
            deleteIface && Mini.div({ class: "mui-stack" },
                Mini.p({ class: "mui-muted" }, `Delete ${deleteIface.name}? This removes all peers on that interface.`),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ class: "mui-button secondary", onclick: () => setDeleteIface(null) }, "Cancel"),
                    Mini.button({ class: "mui-button danger", onclick: confirmDeleteInterface }, "Delete")
                )
            )
        ),

        Mini.vnode(Modal, {
            open: !!deletePeer,
            title: "Delete Peer",
            onClose: () => setDeletePeer(null)
        },
            deletePeer && Mini.div({ class: "mui-stack" },
                Mini.p({ class: "mui-muted" }, `Delete peer ${deletePeer.name}?`),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ class: "mui-button secondary", onclick: () => setDeletePeer(null) }, "Cancel"),
                    Mini.button({ class: "mui-button danger", onclick: confirmDeletePeer }, "Delete")
                )
            )
        )
    );
}