import { jsonHeaders, request, Field, PageHeader, Spinner, Modal } from "../utils.js";

export function CertsScreen() {
    const [groups, setGroups] = Mini.useState({ root: [], server: [], client: [] });
    const [domains, setDomains] = Mini.useState([]);
    const [loading, setLoading] = Mini.useState(true);
    const [error, setError] = Mini.useState("");
    const [activeModal, setActiveModal] = Mini.useState("");
    const [detail, setDetail] = Mini.useState(null);
    const [remoteCertMode, setRemoteCertMode] = Mini.useState(false);
    const [remoteCertURL, setRemoteCertURL] = Mini.useState("");

    const [rootForm, setRootForm] = Mini.useState({ name: "", days: "3650" });
    const [serverForm, setServerForm] = Mini.useState({ domain: "", caBase: "" });
    const [clientForm, setClientForm] = Mini.useState({ name: "", caBase: "", identity: "", p12Pass: "" });

    async function loadCerts() {
        setLoading(true);
        try {
            const [certData, dnsData, sysInfo] = await Promise.all([
                request("/api/certs"),
                request("/api/dns").catch(() => []),
                request("/api/system/info").catch(() => ({}))
            ]);

            setRemoteCertMode(!!sysInfo.remoteCertMode);
            setRemoteCertURL(sysInfo.remoteCertURL || "");

            const next = {
                root: certData.root || [],
                server: certData.server || [],
                client: certData.client || []
            };
            setGroups(next);

            const rootNames = (next.root || []).map(item => item.name);
            if (rootNames.length > 0) {
                setServerForm(old => ({ ...old, caBase: old.caBase || rootNames[0] }));
                setClientForm(old => ({ ...old, caBase: old.caBase || rootNames[0] }));
            }

            const dnsNames = [...new Set((dnsData || []).map(record => record.name).filter(Boolean))].sort();
            setDomains(dnsNames);
            if (dnsNames.length > 0) {
                setServerForm(old => ({ ...old, domain: old.domain || dnsNames[0] }));
            }

            setError("");
        } catch (err) {
            setError(err.error || "Failed to load certificates");
        } finally {
            setLoading(false);
        }
    }

    Mini.useEffect(() => {
        loadCerts();
    }, []);

    async function createRoot(event) {
        event.preventDefault();
        try {
            await request("/api/certs/root", {
                method: "POST",
                headers: jsonHeaders(),
                body: JSON.stringify({
                    name: rootForm.name.trim(),
                    days: parseInt(rootForm.days || "3650", 10)
                })
            });
            setActiveModal("");
            setRootForm({ name: "", days: "3650" });
            loadCerts();
        } catch (err) {
            await showAlert(`Failed to create root CA: ${err.error}`);
        }
    }

    async function createServer(event) {
        event.preventDefault();
        try {
            await request("/api/certs/server", {
                method: "POST",
                headers: jsonHeaders(),
                body: JSON.stringify({
                    domain: serverForm.domain,
                    caBase: serverForm.caBase
                })
            });
            setActiveModal("");
            loadCerts();
        } catch (err) {
            await showAlert(`Failed to create server certificate: ${err.error}`);
        }
    }

    async function createClient(event) {
        event.preventDefault();
        try {
            await request("/api/certs/client", {
                method: "POST",
                headers: jsonHeaders(),
                body: JSON.stringify({
                    name: clientForm.name.trim(),
                    caBase: clientForm.caBase,
                    identity: clientForm.identity,
                    p12Pass: clientForm.p12Pass
                })
            });
            setActiveModal("");
            setClientForm({ name: "", caBase: clientForm.caBase, identity: "", p12Pass: "" });
            loadCerts();
        } catch (err) {
            await showAlert(`Failed to create client certificate: ${err.error}`);
        }
    }

    async function deleteGroup(kind, groupName) {
        if (!(await showConfirm(`Delete and revoke ${groupName}? This cannot be undone.`))) return;
        try {
            await request(`/api/certs?kind=${encodeURIComponent(kind)}&base=${encodeURIComponent(groupName)}`, { method: "DELETE" });
            setDetail(null);
            loadCerts();
        } catch (err) {
            await showAlert(`Failed to delete certificate: ${err.error || err.message || err}`);
        }
    }

    async function saveRemoteCert() {
        if (!detail) return;

        try {
            await request(
                `/api/certs/save?kind=${encodeURIComponent(detail.kind)}&base=${encodeURIComponent(detail.group.name)}`,
                { method: "POST" }
            );
            await showAlert(`Saved ${detail.group.name} to server`, "Success");
            loadCerts();
        } catch (err) {
            await showAlert(`Failed to save ${detail.group.name}: ${err.error || err.message || err}`, "Error");
        }
    }

    function section(kind, title, actionLabel, actionModal) {
        const items = groups[kind] || [];

        return Mini.section({ class: "mui-card" },
            Mini.div({ class: "mui-row spread wrap" },
                Mini.h3({}, title),
                Mini.button({ class: "mui-button", onclick: () => setActiveModal(actionModal) }, actionLabel)
            ),
            items.length === 0
                ? Mini.p({ class: "mui-muted" }, `No ${kind} certificates yet.`)
                : Mini.div({ class: "mui-chip-grid" },
                    items.map(group => Mini.button({
                        class: "mui-chip",
                        key: `${kind}-${group.name}`,
                        onclick: () => setDetail({ kind, group })
                    },
                        Mini.strong({}, group.name),
                        Mini.small({ class: "mui-muted" }, `${(group.files || []).length} files`)
                    ))
                )
        );
    }

    const rootNames = (groups.root || []).map(group => group.name);

    return Mini.div({ class: "mui-screen" },
        PageHeader({
            title: "Certificates",
            subtitle: "Issue root, server, and client certificates"
        }),
        remoteCertMode && Mini.div({ class: "mui-alert warning" },
            "Certificates sourced from ",
            Mini.strong({}, remoteCertURL),
        ),
        error && Mini.div({ class: "mui-alert danger" }, error),
        loading
            ? Spinner({ label: "Loading certificates..." })
            : Mini.div({ class: "mui-stack" },
                section("root", "Certificate Authorities", "Create Root", "root"),
                section("server", "Server Certificates", "Create Server Cert", "server"),
                section("client", "Client Certificates", "Create Client Cert", "client")
            ),

        Mini.vnode(Modal, {
            open: !!detail,
            title: detail ? detail.group.name : "Certificate",
            onClose: () => setDetail(null)
        },
            detail && Mini.div({ class: "mui-stack" },
                Mini.small({ class: "mui-muted" }, `${detail.kind} certificate`),
                Mini.div({ class: "mui-stack" },
                    (detail.group.files || []).map(file => Mini.a({
                        class: "mui-button secondary",
                        key: file,
                        href: `/api/certs/download?kind=${detail.kind}&name=${encodeURIComponent(file)}&base=${encodeURIComponent(detail.group.name)}`
                    }, `Download ${file}`)),
                ),
                Mini.div({ class: "mui-form-actions" },
                    remoteCertMode && Mini.button({
                        class: "mui-button secondary",
                        onclick: () => saveRemoteCert()
                    }, `Save ${detail.group.name} to server`),
                    Mini.button({ class: "mui-button danger", onclick: () => deleteGroup(detail.kind, detail.group.name) }, "Delete"),
                    Mini.button({ class: "mui-button secondary", onclick: () => setDetail(null) }, "Close")
                )
            ),
        ),

        Mini.vnode(Modal, {
            open: activeModal === "root",
            title: "Create Root CA",
            onClose: () => setActiveModal("")
        },
            Mini.vnode("form", { class: "mui-stack", onsubmit: createRoot },
                Field({
                    label: "Name", children: Mini.vnode("input", {
                        value: rootForm.name,
                        required: true,
                        oninput: event => setRootForm({ ...rootForm, name: event.target.value })
                    })
                }),
                Field({
                    label: "Days", children: Mini.vnode("input", {
                        type: "number",
                        value: rootForm.days,
                        oninput: event => setRootForm({ ...rootForm, days: event.target.value })
                    })
                }),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ type: "button", class: "mui-button secondary", onclick: () => setActiveModal("") }, "Cancel"),
                    Mini.button({ type: "submit", class: "mui-button" }, "Create")
                )
            )
        ),

        Mini.vnode(Modal, {
            open: activeModal === "server",
            title: "Create Server Certificate",
            onClose: () => setActiveModal("")
        },
            Mini.vnode("form", { class: "mui-stack", onsubmit: createServer },
                Field({
                    label: "Domain", children: Mini.vnode("select", {
                        value: serverForm.domain,
                        required: true,
                        onchange: event => setServerForm({ ...serverForm, domain: event.target.value })
                    },
                        domains.length === 0
                            ? Mini.vnode("option", { value: "" }, "-- no domains configured --")
                            : domains.map(domain => Mini.vnode("option", { value: domain, key: domain }, domain))
                    )
                }),
                Field({
                    label: "Root CA", children: Mini.vnode("select", {
                        value: serverForm.caBase,
                        onchange: event => setServerForm({ ...serverForm, caBase: event.target.value })
                    },
                        rootNames.length === 0
                            ? Mini.vnode("option", { value: "" }, "-- no root CA available --")
                            : rootNames.map(name => Mini.vnode("option", { value: name, key: name }, name))
                    )
                }),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ type: "button", class: "mui-button secondary", onclick: () => setActiveModal("") }, "Cancel"),
                    Mini.button({ type: "submit", class: "mui-button" }, "Create")
                )
            )
        ),

        Mini.vnode(Modal, {
            open: activeModal === "client",
            title: "Create Client Certificate",
            onClose: () => setActiveModal("")
        },
            Mini.vnode("form", { class: "mui-stack", onsubmit: createClient },
                Field({
                    label: "Name", children: Mini.vnode("input", {
                        value: clientForm.name,
                        required: true,
                        oninput: event => setClientForm({ ...clientForm, name: event.target.value })
                    })
                }),
                Field({
                    label: "Root CA", children: Mini.vnode("select", {
                        value: clientForm.caBase,
                        onchange: event => setClientForm({ ...clientForm, caBase: event.target.value })
                    },
                        rootNames.length === 0
                            ? Mini.vnode("option", { value: "" }, "-- no root CA available --")
                            : rootNames.map(name => Mini.vnode("option", { value: name, key: name }, name))
                    )
                }),
                Field({
                    label: "Client Identity", hint: "Usually an email address", children: Mini.vnode("input", {
                        value: clientForm.identity,
                        oninput: event => setClientForm({ ...clientForm, identity: event.target.value })
                    })
                }),
                Field({
                    label: "P12 Password", children: Mini.vnode("input", {
                        type: "password",
                        value: clientForm.p12Pass,
                        oninput: event => setClientForm({ ...clientForm, p12Pass: event.target.value })
                    })
                }),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ type: "button", class: "mui-button secondary", onclick: () => setActiveModal("") }, "Cancel"),
                    Mini.button({ type: "submit", class: "mui-button" }, "Create")
                )
            )
        )
    );
}