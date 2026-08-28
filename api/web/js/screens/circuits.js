import { jsonHeaders, request, Field, PageHeader, Spinner, Modal, EmptyState } from "../utils.js";

export function CircuitsScreen() {
    const [circuits, setCircuits] = Mini.useState([]);
    const [interfaces, setInterfaces] = Mini.useState([]);
    const [loading, setLoading] = Mini.useState(true);
    const [error, setError] = Mini.useState("");

    const [editing, setEditing] = Mini.useState(null);
    const [deleting, setDeleting] = Mini.useState(null);
    const [form, setForm] = Mini.useState({
        id: "",
        name: "",
        type: "DNAT",
        in_interface: "",
        protocol: "tcp",
        src_port: "",
        redirect_port: "",
        dest_ip: "",
        dest_port: ""
    });

    async function loadCircuits() {
        setLoading(true);
        try {
            const data = await request("/api/circuits");
            setCircuits(data || []);
            setError("");
        } catch (err) {
            setError(err.error || "Failed to load circuits");
        } finally {
            setLoading(false);
        }
    }

    async function loadInterfaces() {
        try {
            const data = await request("/api/system/interfaces");
            const list = (data.interfaces || []).filter(iface => (
                !iface.startsWith("veth") && !iface.startsWith("docker") && !iface.startsWith("br-")
            ));
            setInterfaces(list);
        } catch (err) {
            setError(err.error || "Failed to load interfaces");
        }
    }

    Mini.useEffect(() => {
        Promise.all([loadCircuits(), loadInterfaces()]);
    }, []);

    function openCreate() {
        setEditing({ mode: "create" });
        setForm({
            id: "",
            name: "",
            type: "DNAT",
            in_interface: interfaces[0] || "",
            protocol: "tcp",
            src_port: "",
            redirect_port: "",
            dest_ip: "",
            dest_port: ""
        });
    }

    function openEdit(circuit) {
        setEditing({ mode: "edit", id: circuit.id });
        setForm({
            id: String(circuit.id),
            name: circuit.name || "",
            type: circuit.type || "DNAT",
            in_interface: circuit.in_interface || interfaces[0] || "",
            protocol: circuit.protocol || "tcp",
            src_port: String(circuit.src_port || ""),
            redirect_port: String(circuit.redirect_port || ""),
            dest_ip: circuit.dest_ip || "",
            dest_port: String(circuit.dest_port || "")
        });
    }

    function payloadFromForm() {
        return {
            name: form.name,
            type: form.type,
            in_interface: form.in_interface,
            protocol: form.protocol,
            src_port: parseInt(form.src_port || "0", 10) || 0,
            redirect_port: parseInt(form.redirect_port || "0", 10) || 0,
            dest_ip: form.dest_ip,
            dest_port: parseInt(form.dest_port || "0", 10) || 0
        };
    }

    async function saveCircuit(event) {
        event.preventDefault();
        const isEdit = editing?.mode === "edit";
        try {
            await request(isEdit ? `/api/circuits/${editing.id}` : "/api/circuits", {
                method: isEdit ? "PUT" : "POST",
                headers: jsonHeaders(),
                body: JSON.stringify(payloadFromForm())
            });
            setEditing(null);
            loadCircuits();
        } catch (err) {
            await showAlert(`Failed to save circuit: ${err.error}`);
        }
    }

    async function toggleCircuit(id, enable) {
        try {
            await request(`/api/circuits/${id}/${enable ? "enable" : "disable"}`, { method: "POST" });
            loadCircuits();
        } catch (err) {
            await showAlert(`Failed to update circuit: ${err.error}`);
        }
    }

    async function deleteCircuit() {
        if (!deleting) return;
        try {
            await request(`/api/circuits/${deleting.id}`, { method: "DELETE" });
            setDeleting(null);
            loadCircuits();
        } catch (err) {
            await showAlert(`Failed to delete circuit: ${err.error}`);
        }
    }

    function buildMeta(circuit) {
        const parts = [];
        if (circuit.type === "LOCAL") {
            if (circuit.protocol) parts.push(circuit.protocol.toUpperCase());
            if (circuit.src_port) parts.push(`:${circuit.src_port}`);
            if (circuit.redirect_port) parts.push(`-> :${circuit.redirect_port}`);
            parts.push("(loopback)");
        } else {
            if (circuit.in_interface) parts.push(`in: ${circuit.in_interface}`);
            if (circuit.protocol) parts.push(circuit.protocol.toUpperCase());
            if (circuit.src_port) parts.push(`:${circuit.src_port}`);
            if (circuit.type === "DNAT" && circuit.dest_ip && circuit.dest_port) {
                parts.push(`-> ${circuit.dest_ip}:${circuit.dest_port}`);
            }
            if (circuit.type === "REDIRECT" && circuit.redirect_port) {
                parts.push(`-> :${circuit.redirect_port}`);
            }
        }
        return parts.join("  ·  ");
    }

    const isDNAT = form.type === "DNAT";
    const isRedirect = form.type === "REDIRECT";
    const isLocal = form.type === "LOCAL";

    return Mini.div({ class: "mui-screen" },
        PageHeader({
            title: "Routing Circuits",
            subtitle: "Port-forwarding, redirect and local routing rules",
            actions: [Mini.button({ class: "mui-button", onclick: openCreate }, "Add Circuit")]
        }),
        error && Mini.div({ class: "mui-alert danger" }, error),
        loading
            ? Spinner({ label: "Loading circuits..." })
            : circuits.length === 0
                ? EmptyState({ title: "No circuits yet", subtitle: "Create your first circuit to begin." })
                : Mini.div({ class: "mui-stack" },
                    circuits.map(circuit => Mini.div({
                        class: `mui-card circuit ${circuit.enabled ? "is-enabled" : "is-disabled"}`,
                        key: String(circuit.id)
                    },
                        Mini.div({ class: "mui-row spread wrap" },
                            Mini.div({ class: "mui-stack tight" },
                                Mini.div({ class: "mui-row" },
                                    Mini.span({ class: `mui-status-dot ${circuit.enabled ? "on" : "off"}` }),
                                    Mini.strong({ class: "mui-truncate" }, circuit.name),
                                    Mini.span({ class: "mui-badge" }, circuit.type)
                                ),
                                Mini.small({ class: "mui-muted mono" }, buildMeta(circuit))
                            ),
                            Mini.div({ class: "mui-row wrap" },
                                circuit.enabled
                                    ? Mini.button({ class: "mui-button secondary sm", onclick: () => toggleCircuit(circuit.id, false) }, "Disable")
                                    : Mini.button({ class: "mui-button success sm", onclick: () => toggleCircuit(circuit.id, true) }, "Enable"),
                                Mini.button({
                                    class: "mui-button secondary sm",
                                    onclick: () => openEdit(circuit),
                                    disabled: !!circuit.enabled
                                }, "Edit"),
                                Mini.button({ class: "mui-button danger sm", onclick: () => setDeleting(circuit) }, "Delete")
                            )
                        )
                    ))
                ),

        Mini.vnode(Modal, {
            open: !!editing,
            title: editing?.mode === "edit" ? "Edit Circuit" : "Create Circuit",
            onClose: () => setEditing(null)
        },
            Mini.vnode("form", { class: "mui-stack", onsubmit: saveCircuit },
                Field({
                    label: "Name", children: Mini.vnode("input", {
                        required: true,
                        value: form.name,
                        oninput: event => setForm({ ...form, name: event.target.value })
                    })
                }),
                Field({
                    label: "Type", children: Mini.vnode("select", {
                        value: form.type,
                        onchange: event => setForm({ ...form, type: event.target.value })
                    },
                        Mini.vnode("option", { value: "DNAT" }, "Port Forwarding"),
                        Mini.vnode("option", { value: "REDIRECT" }, "Inbound Redirect"),
                        Mini.vnode("option", { value: "LOCAL" }, "Local Redirect")
                    )
                }),
                !isLocal && Field({
                    label: "Inbound Interface", children: Mini.vnode("select", {
                        value: form.in_interface,
                        onchange: event => setForm({ ...form, in_interface: event.target.value })
                    },
                        interfaces.map(iface => Mini.vnode("option", { value: iface, key: iface }, iface))
                    )
                }),
                Field({
                    label: "Protocol", children: Mini.vnode("select", {
                        value: form.protocol,
                        onchange: event => setForm({ ...form, protocol: event.target.value })
                    },
                        Mini.vnode("option", { value: "tcp" }, "TCP"),
                        Mini.vnode("option", { value: "udp" }, "UDP"),
                        Mini.vnode("option", { value: "both" }, "Both")
                    )
                }),
                Field({
                    label: "Source Port", children: Mini.vnode("input", {
                        type: "number",
                        value: form.src_port,
                        oninput: event => setForm({ ...form, src_port: event.target.value })
                    })
                }),
                (isRedirect || isLocal) && Field({
                    label: "Redirect Port", children: Mini.vnode("input", {
                        type: "number",
                        value: form.redirect_port,
                        oninput: event => setForm({ ...form, redirect_port: event.target.value })
                    })
                }),
                isDNAT && Field({
                    label: "Destination IP", children: Mini.vnode("input", {
                        value: form.dest_ip,
                        oninput: event => setForm({ ...form, dest_ip: event.target.value })
                    })
                }),
                isDNAT && Field({
                    label: "Destination Port", children: Mini.vnode("input", {
                        type: "number",
                        value: form.dest_port,
                        oninput: event => setForm({ ...form, dest_port: event.target.value })
                    })
                }),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ type: "button", class: "mui-button secondary", onclick: () => setEditing(null) }, "Cancel"),
                    Mini.button({ type: "submit", class: "mui-button" }, editing?.mode === "edit" ? "Save" : "Create")
                )
            )
        ),

        Mini.vnode(Modal, {
            open: !!deleting,
            title: "Delete Circuit",
            onClose: () => setDeleting(null)
        },
            deleting && Mini.div({ class: "mui-stack" },
                Mini.p({ class: "mui-muted" }, `Delete ${deleting.name}? This will remove all associated iptables rules.`),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ class: "mui-button secondary", onclick: () => setDeleting(null) }, "Cancel"),
                    Mini.button({ class: "mui-button danger", onclick: deleteCircuit }, "Delete")
                )
            )
        )
    );
}