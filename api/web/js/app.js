import Mini from "/js/mini.js";

const appStore = Mini.createStore({ instanceName: "" });

const RECORD_FIELDS = {
    A: [{ id: "ipv4", label: "IPv4 Address", type: "text", required: true }],
    CNAME: [{ id: "cname", label: "Target Domain", type: "text", required: true }],
    TXT: [{ id: "txt", label: "Text Value", type: "textarea", required: true }],
    SRV: [
        { id: "priority", label: "Priority", type: "number", required: true },
        { id: "weight", label: "Weight", type: "number", required: true },
        { id: "port", label: "Port", type: "number", required: true },
        { id: "target", label: "Target", type: "text", required: true }
    ]
};

const ROUTE_PATHS = ["/", "/dns", "/certs", "/circuits", "/vpn"];

function normalizeRoute(path) {
    return ROUTE_PATHS.includes(path) ? path : "/";
}

function bootstrapHashFromPath() {
    if (location.hash) return;
    const mapped = normalizeRoute(location.pathname);
    location.hash = mapped;
}

bootstrapHashFromPath();

function jsonHeaders() {
    return { "Content-Type": "application/json" };
}

function chartAvailable() {
    return typeof window !== "undefined" && typeof window.Chart === "function";
}

function formatRate(bitsPerSec) {
    let value = Number(bitsPerSec) || 0;
    const units = ["bps", "Kbps", "Mbps", "Gbps"];
    let i = 0;
    while (value > 1000 && i < units.length - 1) {
        value /= 1000;
        i++;
    }
    return `${value.toFixed(1)} ${units[i]}`;
}

function formatError(error) {
    if (!error) return "Unknown error";
    if (typeof error === "string") return error;
    if (error.error) return error.error;
    if (error.message) return error.message;
    return "Unknown error";
}

async function parseJsonSafe(response) {
    try {
        return await response.json();
    } catch (_) {
        return null;
    }
}

async function request(path, options = {}) {
    const res = await fetch(path, options);
    if (!res.ok) {
        const body = await parseJsonSafe(res);
        throw new Error(formatError(body) || `${res.status} ${res.statusText}`);
    }
    const contentType = res.headers.get("content-type") || "";
    if (contentType.includes("application/json")) return res.json();
    return res.text();
}

function useAsyncData(loader, deps = []) {
    const [loading, setLoading] = Mini.useState(true);
    const [error, setError] = Mini.useState("");
    const [data, setData] = Mini.useState(null);

    Mini.useEffect(() => {
        let alive = true;
        setLoading(true);
        setError("");

        loader()
            .then(value => {
                if (!alive) return;
                setData(value);
                setLoading(false);
            })
            .catch(err => {
                if (!alive) return;
                setError(err.message || "Failed to load");
                setLoading(false);
            });

        return () => {
            alive = false;
        };
    }, deps);

    return { loading, error, data, setData, setError, setLoading };
}

function Field({ label, children, hint }) {
    return Mini.div({ class: "mui-field" },
        Mini.label({ class: "mui-label" }, label),
        hint && Mini.small({ class: "mui-muted" }, hint),
        children
    );
}

function PageHeader({ title, subtitle, actions = [] }) {
    return Mini.div({ class: "mui-page-header" },
        Mini.div({ class: "mui-page-heading" },
            Mini.h1({}, title),
            subtitle && Mini.p({ class: "mui-muted" }, subtitle)
        ),
        Mini.div({ class: "mui-page-actions" }, actions)
    );
}

function Spinner({ label = "Loading..." }) {
    return Mini.div({ class: "mui-loading" },
        Mini.div({ class: "mui-spinner", "aria-hidden": "true" }),
        Mini.span({}, label)
    );
}

function EmptyState({ title, subtitle }) {
    return Mini.div({ class: "mui-empty" },
        Mini.strong({}, title),
        subtitle && Mini.p({ class: "mui-muted" }, subtitle)
    );
}

function Modal({ open, title, onClose, children, wide = false }) {
    Mini.useEffect(() => {
        if (!open) return undefined;
        const onKeyDown = event => {
            if (event.key === "Escape") onClose();
        };
        window.addEventListener("keydown", onKeyDown);
        return () => window.removeEventListener("keydown", onKeyDown);
    }, [open]);

    if (!open) return null;

    return Mini.div({
        class: "mui-modal-backdrop",
        role: "presentation",
        onclick: event => {
            if (event.target.className === "mui-modal-backdrop") onClose();
        }
    },
        Mini.div({
            class: `mui-modal ${wide ? "mui-modal-wide" : ""}`,
            role: "dialog",
            "aria-modal": "true",
            onclick: event => event.stopPropagation()
        },
            Mini.div({ class: "mui-modal-header" },
                Mini.strong({}, title),
                Mini.button({ class: "mui-button secondary", onclick: onClose }, "Close")
            ),
            Mini.div({ class: "mui-modal-body" }, children)
        )
    );
}

function HomeScreen() {
    const [summary, setSummary] = Mini.useState({ dns: {}, certs: {}, circuits: null });
    const [netRows, setNetRows] = Mini.useState([]);
    const [error, setError] = Mini.useState("");
    const [isLoading, setIsLoading] = Mini.useState(true);

    const pieRefs = Mini.useState(() => ({ dns: null, certs: null, circuits: null }))[0];
    const pieCharts = Mini.useState(() => ({ dns: null, certs: null, circuits: null }))[0];
    const lineRefs = Mini.useState(() => ({}))[0];
    const lineCharts = Mini.useState(() => ({}))[0];
    const [cpuPct, setCpuPct] = Mini.useState(0);
    const [memUsedGB, setMemUsedGB] = Mini.useState("0");
    const [memTotalGB, setMemTotalGB] = Mini.useState("0");
    const [sysTick, setSysTick] = Mini.useState(0);
    const sysRefs = Mini.useState(() => ({ cpu: null, mem: null }))[0];
    const sysCharts = Mini.useState(() => ({}))[0];
    const sysPrev = Mini.useState(() => ({ cpu: null, cpuHist: new Array(60).fill(0), memHist: new Array(60).fill(0) }))[0];

    Mini.useEffect(() => {
        let alive = true;
        const samples = {};
        const times = {};
        const history = {};

        async function loadSummary() {
            const data = await request("/api/stats/dynamo");
            if (!alive) return;
            setSummary(data || { dns: {}, certs: {}, circuits: null });
            if (data?.instance) appStore.setState({ instanceName: data.instance });
        }

        async function pollNetwork() {
            const data = await request("/api/stats/network");
            if (!alive) return;

            const now = Date.now();
            const rows = [];
            Object.keys(data || {}).forEach(iface => {
                if (iface.startsWith("veth") || iface.startsWith("docker") || iface.startsWith("br-")) return;

                const rx = Number(data[iface].rx) || 0;
                const tx = Number(data[iface].tx) || 0;
                const previous = samples[iface];

                if (!history[iface]) {
                    history[iface] = {
                        labels: new Array(60).fill(""),
                        rx: new Array(60).fill(0),
                        tx: new Array(60).fill(0)
                    };
                }

                let rxRate = 0;
                let txRate = 0;

                if (previous) {
                    const elapsed = Math.max((now - times[iface]) / 1000, 0.001);
                    rxRate = ((rx - previous.rx) / elapsed) * 8;
                    txRate = ((tx - previous.tx) / elapsed) * 8;

                    history[iface].labels.push("");
                    history[iface].rx.push(rxRate || 0);
                    history[iface].tx.push(txRate || 0);
                    if (history[iface].labels.length > 60) {
                        history[iface].labels.shift();
                        history[iface].rx.shift();
                        history[iface].tx.shift();
                    }
                }

                samples[iface] = { rx, tx };
                times[iface] = now;

                rows.push({
                    iface,
                    rxRate,
                    txRate,
                    history: history[iface]
                });
            });

            rows.sort((a, b) => a.iface.localeCompare(b.iface));
            setNetRows(rows);
        }

        async function pollSystem() {
            const data = await request("/api/stats/system");
            if (!alive) return;

            const cpu = data.cpu;
            const mem = data.mem;
            const total = cpu.user + cpu.nice + cpu.system + cpu.idle + cpu.iowait + cpu.irq + cpu.softirq + cpu.steal;
            const idle = cpu.idle + cpu.iowait;

            let pct = 0;
            if (sysPrev.cpu) {
                const totalDelta = total - sysPrev.cpu.total;
                const idleDelta = idle - sysPrev.cpu.idle;
                pct = totalDelta > 0 ? Math.max(0, Math.min(100, ((totalDelta - idleDelta) / totalDelta) * 100)) : 0;
            }
            sysPrev.cpu = { total, idle };
            sysPrev.cpuHist.push(pct);
            if (sysPrev.cpuHist.length > 60) sysPrev.cpuHist.shift();

            const usedKB = mem.total_kb - mem.available_kb;
            const memPct = mem.total_kb > 0 ? (usedKB / mem.total_kb) * 100 : 0;
            sysPrev.memHist.push(memPct);
            if (sysPrev.memHist.length > 60) sysPrev.memHist.shift();

            setCpuPct(Math.round(pct * 10) / 10);
            setMemUsedGB((usedKB / 1024 / 1024).toFixed(1));
            setMemTotalGB((mem.total_kb / 1024 / 1024).toFixed(1));
            setSysTick(t => t + 1);
        }

        async function cycle() {
            try {
                await Promise.all([loadSummary(), pollNetwork(), pollSystem().catch(() => {})]);
                if (alive) {
                    setError("");
                    setIsLoading(false);
                }
            } catch (err) {
                if (alive) {
                    setError(err.message || "Failed to load dashboard");
                    setIsLoading(false);
                }
            } finally {
                if (alive) setTimeout(cycle, 1000);
            }
        }

        cycle();

        return () => {
            alive = false;
            Object.keys(lineCharts).forEach(key => {
                lineCharts[key]?.destroy?.();
                delete lineCharts[key];
            });
            Object.keys(sysCharts).forEach(key => {
                sysCharts[key]?.destroy?.();
                delete sysCharts[key];
            });
            Object.values(pieCharts).forEach(chart => chart?.destroy?.());
        };
    }, []);

    Mini.useEffect(() => {
        if (!chartAvailable()) return;

        const dnsData = summary?.dns || {};
        const certData = summary?.certs || {};
        const circuitData = summary?.circuits
            ? {
                "DNAT - ON": summary.circuits.DNAT.on,
                "DNAT - OFF": summary.circuits.DNAT.off,
                "REDIRECT - ON": summary.circuits.REDIRECT.on,
                "REDIRECT - OFF": summary.circuits.REDIRECT.off,
                "LOCAL - ON": summary.circuits.LOCAL.on,
                "LOCAL - OFF": summary.circuits.LOCAL.off
            }
            : {};

        const specs = [
            { key: "dns", data: dnsData, palette: ["#f97316", "#fb923c", "#fdba74", "#fed7aa"] },
            { key: "certs", data: certData, palette: ["#3b82f6", "#60a5fa", "#93c5fd", "#bfdbfe"] },
            { key: "circuits", data: circuitData, palette: ["#22c55e", "#86efac", "#4ade80", "#16a34a", "#f97316", "#fb923c"] }
        ];

        specs.forEach(spec => {
            const canvas = pieRefs[spec.key];
            if (!canvas) return;

            const labels = Object.keys(spec.data || {});
            const values = Object.values(spec.data || {});

            pieCharts[spec.key]?.destroy?.();
            pieCharts[spec.key] = new window.Chart(canvas, {
                type: "doughnut",
                data: {
                    labels,
                    datasets: [{ data: values, backgroundColor: spec.palette, borderWidth: 0 }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { display: false }
                    }
                }
            });
        });
    }, [summary]);

    Mini.useEffect(() => {
        if (!chartAvailable()) return;

        const currentKeys = new Set(netRows.map(row => row.iface));
        Object.keys(lineCharts).forEach(key => {
            if (!currentKeys.has(key)) {
                lineCharts[key]?.destroy?.();
                delete lineCharts[key];
            }
        });

        netRows.forEach(row => {
            const canvas = lineRefs[row.iface];
            if (!canvas) return;

            if (!lineCharts[row.iface]) {
                lineCharts[row.iface] = new window.Chart(canvas, {
                    type: "line",
                    data: {
                        labels: row.history.labels,
                        datasets: [
                            {
                                label: "Download",
                                borderColor: "#22c55e",
                                data: row.history.rx,
                                pointRadius: 0,
                                tension: 0.35
                            },
                            {
                                label: "Upload",
                                borderColor: "#ef4444",
                                data: row.history.tx,
                                pointRadius: 0,
                                tension: 0.35
                            }
                        ]
                    },
                    options: {
                        animation: false,
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: { legend: { display: false } },
                        scales: {
                            x: { display: false },
                            y: {
                                beginAtZero: true,
                                ticks: { callback: value => formatRate(value), maxTicksLimit: 3 }
                            }
                        }
                    }
                });
                return;
            }

            const chart = lineCharts[row.iface];
            chart.data.labels = row.history.labels;
            chart.data.datasets[0].data = row.history.rx;
            chart.data.datasets[1].data = row.history.tx;
            const maxValue = Math.max(...row.history.rx, ...row.history.tx, 0);
            chart.options.scales.y.suggestedMax = maxValue * 1.3;
            chart.update("none");
        });
    }, [netRows]);

    Mini.useEffect(() => {
        if (!chartAvailable()) return;

        const specs = [
            { key: "cpu", label: "CPU %", color: "#a855f7", bg: "rgba(168,85,247,0.12)", data: sysPrev.cpuHist },
            { key: "mem", label: "Memory %", color: "#3b82f6", bg: "rgba(59,130,246,0.12)", data: sysPrev.memHist }
        ];

        specs.forEach(spec => {
            const canvas = sysRefs[spec.key];
            if (!canvas) return;

            if (!sysCharts[spec.key]) {
                sysCharts[spec.key] = new window.Chart(canvas, {
                    type: "line",
                    data: {
                        labels: new Array(60).fill(""),
                        datasets: [{
                            label: spec.label,
                            borderColor: spec.color,
                            backgroundColor: spec.bg,
                            data: [...spec.data],
                            pointRadius: 0,
                            tension: 0.35,
                            fill: true
                        }]
                    },
                    options: {
                        animation: false,
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: { legend: { display: false } },
                        scales: {
                            x: { display: false },
                            y: {
                                beginAtZero: true,
                                max: 100,
                                ticks: { callback: v => `${v}%`, maxTicksLimit: 3 }
                            }
                        }
                    }
                });
                return;
            }

            const chart = sysCharts[spec.key];
            chart.data.datasets[0].data = [...spec.data];
            chart.update("none");
        });
    }, [sysTick]);

    const dnsValues = Object.values(summary?.dns || {});
    const certValues = Object.values(summary?.certs || {});
    const circuits = summary?.circuits;
    const circuitTotal = circuits
        ? circuits.DNAT.on + circuits.DNAT.off + circuits.REDIRECT.on + circuits.REDIRECT.off + circuits.LOCAL.on + circuits.LOCAL.off
        : 0;

    return Mini.div({ class: "mui-screen" },
        PageHeader({
            title: "Overview",
            subtitle: "Traffic, service totals, and live interface rates"
        }),
        error && Mini.div({ class: "mui-alert danger" }, error),
        isLoading
            ? Spinner({ label: "Loading dashboard..." })
            : Mini.div({ class: "mui-dashboard" },
                Mini.div({ class: "mui-grid-2" },
                    Mini.div({ class: "mui-card" },
                        Mini.h3({}, "CPU Usage"),
                        Mini.div({ class: "mui-stat" }, `${cpuPct.toFixed(1)}%`),
                        Mini.div({ class: "mui-chart-line" }, Mini.vnode("canvas", { ref: el => sysRefs.cpu = el }))
                    ),
                    Mini.div({ class: "mui-card" },
                        Mini.h3({}, "Memory Usage"),
                        Mini.div({ class: "mui-stat" }, `${memUsedGB} GB`),
                        Mini.small({ class: "mui-muted" }, `of ${memTotalGB} GB total`),
                        Mini.div({ class: "mui-chart-line" }, Mini.vnode("canvas", { ref: el => sysRefs.mem = el }))
                    )
                ),
                Mini.section({ class: "mui-card" },
                    Mini.h3({}, "Network Interfaces"),
                    netRows.length === 0
                        ? EmptyState({ title: "No active interfaces yet", subtitle: "Traffic charts will appear when counters are available." })
                        : Mini.div({ class: "mui-interface-grid" },
                            netRows.map(row => Mini.div({ class: "mui-interface-card", key: row.iface },
                                Mini.div({ class: "mui-row spread" },
                                    Mini.strong({}, row.iface),
                                    Mini.div({ class: "mui-row" },
                                        Mini.span({ class: "mui-pill good" }, `Down ${formatRate(row.rxRate)}`),
                                        Mini.span({ class: "mui-pill bad" }, `Up ${formatRate(row.txRate)}`)
                                    )
                                ),
                                Mini.div({ class: "mui-chart-line" }, Mini.vnode("canvas", { ref: el => lineRefs[row.iface] = el }))
                            ))
                        )
                ),
                Mini.div({ class: "mui-summary-grid" },
                    Mini.div({ class: "mui-card" },
                        Mini.h3({}, "DNS Records"),
                        Mini.div({ class: "mui-stat" }, String(dnsValues.reduce((a, b) => a + b, 0))),
                        Mini.div({ class: "mui-chart-small" }, Mini.vnode("canvas", { ref: el => pieRefs.dns = el }))
                    ),
                    Mini.div({ class: "mui-card" },
                        Mini.h3({}, "Certificates"),
                        Mini.div({ class: "mui-stat" }, String(certValues.reduce((a, b) => a + b, 0))),
                        Mini.div({ class: "mui-chart-small" }, Mini.vnode("canvas", { ref: el => pieRefs.certs = el }))
                    ),
                    Mini.div({ class: "mui-card" },
                        Mini.h3({}, "Circuits"),
                        Mini.div({ class: "mui-stat" }, String(circuitTotal)),
                        Mini.div({ class: "mui-chart-small" }, Mini.vnode("canvas", { ref: el => pieRefs.circuits = el }))
                    )
                )
            )
    );
}

function DnsScreen() {
    const [records, setRecords] = Mini.useState([]);
    const [loading, setLoading] = Mini.useState(true);
    const [error, setError] = Mini.useState("");
    const [addOpen, setAddOpen] = Mini.useState(false);
    const [editRecord, setEditRecord] = Mini.useState(null);

    const [newRecord, setNewRecord] = Mini.useState({
        type: "A",
        name: "",
        ttl: "3600",
        ipv4: "",
        cname: "",
        txt: "",
        priority: "",
        weight: "",
        port: "",
        target: ""
    });

    const [editForm, setEditForm] = Mini.useState({
        type: "A",
        name: "",
        ttl: "3600",
        ipv4: "",
        cname: "",
        txt: "",
        priority: "",
        weight: "",
        port: "",
        target: ""
    });

    async function loadRecords() {
        setLoading(true);
        try {
            const data = await request("/api/dns");
            setRecords(data || []);
            setError("");
        } catch (err) {
            setError(err.message || "Failed to load DNS records");
        } finally {
            setLoading(false);
        }
    }

    Mini.useEffect(() => {
        loadRecords();
    }, []);

    function formToPayload(form) {
        const payload = {
            name: form.name.trim(),
            type: form.type,
            ttl: parseInt(form.ttl || "0", 10),
            class: "IN"
        };

        RECORD_FIELDS[form.type].forEach(field => {
            const value = form[field.id];
            if (field.type === "number") payload[field.id] = parseInt(value || "0", 10);
            else if (field.id === "txt") payload[field.id] = [value || ""];
            else payload[field.id] = value;
        });

        return payload;
    }

    function openAddModal() {
        setNewRecord({
            type: "A",
            name: "",
            ttl: "3600",
            ipv4: "",
            cname: "",
            txt: "",
            priority: "",
            weight: "",
            port: "",
            target: ""
        });
        setAddOpen(true);
    }

    function openEdit(record) {
        setEditRecord(record);
        setEditForm({
            type: record.type,
            name: record.name || "",
            ttl: String(record.ttl || 3600),
            ipv4: record.ipv4 || "",
            cname: record.cname || "",
            txt: Array.isArray(record.txt) ? (record.txt[0] || "") : "",
            priority: String(record.priority || ""),
            weight: String(record.weight || ""),
            port: String(record.port || ""),
            target: record.target || ""
        });
    }

    async function createRecord(event) {
        event.preventDefault();
        try {
            await request("/api/dns", {
                method: "POST",
                headers: jsonHeaders(),
                body: JSON.stringify(formToPayload(newRecord))
            });
            setAddOpen(false);
            loadRecords();
        } catch (err) {
            alert(`Error creating record: ${err.message}`);
        }
    }

    async function saveRecord(event) {
        event.preventDefault();
        if (!editRecord) return;

        const payload = formToPayload(editForm);
        delete payload.class;
        delete payload.type;

        try {
            await request(`/api/dns/${editRecord.id}`, {
                method: "PUT",
                headers: jsonHeaders(),
                body: JSON.stringify(payload)
            });
            setEditRecord(null);
            loadRecords();
        } catch (err) {
            alert(`Error updating record: ${err.message}`);
        }
    }

    async function removeRecord(id) {
        if (!confirm("Are you sure you want to delete this record?")) return;
        try {
            await request(`/api/dns/${id}`, { method: "DELETE" });
            loadRecords();
        } catch (err) {
            alert(`Error deleting record: ${err.message}`);
        }
    }

    function grouped() {
        const groups = { A: [], CNAME: [], TXT: [], SRV: [] };
        records.forEach(record => {
            if (groups[record.type]) groups[record.type].push(record);
        });
        return groups;
    }

    function recordContent(record) {
        if (record.type === "A") return record.ipv4;
        if (record.type === "CNAME") return record.cname;
        if (record.type === "TXT") return Array.isArray(record.txt) ? record.txt.join(", ") : "";
        return `Priority: ${record.priority}, Weight: ${record.weight}, Port: ${record.port}, Target: ${record.target}`;
    }

    function renderDynamic(form, setter, editableType = true) {
        const fields = RECORD_FIELDS[form.type] || [];

        return Mini.div({ class: "mui-field-grid" },
            editableType && Field({ label: "Type", children: Mini.vnode("select", {
                value: form.type,
                onchange: event => setter({ ...form, type: event.target.value })
            },
                Mini.vnode("option", { value: "A" }, "A Record"),
                Mini.vnode("option", { value: "CNAME" }, "CNAME Record"),
                Mini.vnode("option", { value: "TXT" }, "TXT Record"),
                Mini.vnode("option", { value: "SRV" }, "SRV Record")
            ) }),
            Field({ label: "Name", children: Mini.vnode("input", {
                type: "text",
                value: form.name,
                required: true,
                oninput: event => setter({ ...form, name: event.target.value })
            }) }),
            Field({ label: "TTL (seconds)", children: Mini.vnode("input", {
                type: "number",
                value: form.ttl,
                oninput: event => setter({ ...form, ttl: event.target.value })
            }) }),
            fields.map(field => Field({
                key: field.id,
                label: field.label,
                children: field.type === "textarea"
                    ? Mini.vnode("textarea", {
                        value: form[field.id] || "",
                        required: field.required,
                        oninput: event => setter({ ...form, [field.id]: event.target.value })
                    })
                    : Mini.vnode("input", {
                        type: field.type,
                        value: form[field.id] || "",
                        required: field.required,
                        oninput: event => setter({ ...form, [field.id]: event.target.value })
                    })
            }))
        );
    }

    const groups = grouped();

    return Mini.div({ class: "mui-screen" },
        PageHeader({
            title: "DNS Records",
            subtitle: "Manage A, CNAME, TXT and SRV entries",
            actions: [Mini.button({ class: "mui-button", onclick: openAddModal }, "Add Record")]
        }),
        error && Mini.div({ class: "mui-alert danger" }, error),
        loading
            ? Spinner({ label: "Loading DNS records..." })
            : Mini.div({ class: "mui-grid-2" },
                Object.keys(groups).map(type => Mini.section({ class: "mui-card", key: type },
                    Mini.h3({}, `${type} Records`),
                    groups[type].length === 0
                        ? Mini.p({ class: "mui-muted" }, "No records yet")
                        : Mini.div({ class: "mui-stack" },
                            groups[type].map(record => Mini.div({ class: "mui-item", key: String(record.id) },
                                Mini.div({ class: "mui-row spread wrap" },
                                    Mini.strong({ class: "mui-truncate" }, record.name),
                                    Mini.div({ class: "mui-row" },
                                        Mini.button({ class: "mui-button secondary sm", onclick: () => openEdit(record) }, "Edit"),
                                        Mini.button({ class: "mui-button danger sm", onclick: () => removeRecord(record.id) }, "Delete")
                                    )
                                ),
                                Mini.small({ class: "mui-muted" }, `TTL: ${record.ttl}s`),
                                Mini.p({}, recordContent(record))
                            ))
                        )
                ))
            ),
        Mini.vnode(Modal, {
            open: addOpen,
            title: "Add DNS Record",
            onClose: () => setAddOpen(false)
        },
            Mini.vnode("form", { onsubmit: createRecord, class: "mui-stack" },
                renderDynamic(newRecord, setNewRecord, true),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ type: "button", class: "mui-button secondary", onclick: () => setAddOpen(false) }, "Cancel"),
                    Mini.button({ type: "submit", class: "mui-button" }, "Create")
                )
            )
        ),
        Mini.vnode(Modal, {
            open: !!editRecord,
            title: "Edit DNS Record",
            onClose: () => setEditRecord(null)
        },
            Mini.vnode("form", { onsubmit: saveRecord, class: "mui-stack" },
                renderDynamic(editForm, setEditForm, false),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ type: "button", class: "mui-button secondary", onclick: () => setEditRecord(null) }, "Cancel"),
                    Mini.button({ type: "submit", class: "mui-button" }, "Save")
                )
            )
        )
    );
}

function CertsScreen() {
    const [groups, setGroups] = Mini.useState({ root: [], server: [], client: [] });
    const [domains, setDomains] = Mini.useState([]);
    const [loading, setLoading] = Mini.useState(true);
    const [error, setError] = Mini.useState("");
    const [activeModal, setActiveModal] = Mini.useState("");
    const [detail, setDetail] = Mini.useState(null);

    const [rootForm, setRootForm] = Mini.useState({ name: "", days: "3650" });
    const [serverForm, setServerForm] = Mini.useState({ domain: "", caBase: "" });
    const [clientForm, setClientForm] = Mini.useState({ name: "", caBase: "", identity: "", p12Pass: "" });

    async function loadCerts() {
        setLoading(true);
        try {
            const [certData, dnsData] = await Promise.all([
                request("/api/certs"),
                request("/api/dns").catch(() => [])
            ]);

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
            setError(err.message || "Failed to load certificates");
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
            alert(`Failed to create root CA: ${err.message}`);
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
            alert(`Failed to create server certificate: ${err.message}`);
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
            alert(`Failed to create client certificate: ${err.message}`);
        }
    }

    async function deleteGroup(kind, groupName) {
        if (!confirm(`Delete and revoke ${groupName}? This cannot be undone.`)) return;
        try {
            await request(`/api/certs?kind=${encodeURIComponent(kind)}&base=${encodeURIComponent(groupName)}`, { method: "DELETE" });
            setDetail(null);
            loadCerts();
        } catch (err) {
            alert(`Failed to delete certificate: ${err.message}`);
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
        error && Mini.div({ class: "mui-alert danger" }, error),
        loading
            ? Spinner({ label: "Loading certificates..." })
            : Mini.div({ class: "mui-stack" },
                section("root", "Root CAs", "Create Root", "root"),
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
                    (detail.group.files || []).map(file => Mini.vnode("a", {
                        class: "mui-button secondary",
                        key: file,
                        href: `/api/certs/download?kind=${detail.kind}&name=${encodeURIComponent(file)}&base=${encodeURIComponent(detail.group.name)}`
                    }, `Download ${file}`))
                ),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ class: "mui-button danger", onclick: () => deleteGroup(detail.kind, detail.group.name) }, "Delete"),
                    Mini.button({ class: "mui-button secondary", onclick: () => setDetail(null) }, "Close")
                )
            )
        ),

        Mini.vnode(Modal, {
            open: activeModal === "root",
            title: "Create Root CA",
            onClose: () => setActiveModal("")
        },
            Mini.vnode("form", { class: "mui-stack", onsubmit: createRoot },
                Field({ label: "Name", children: Mini.vnode("input", {
                    value: rootForm.name,
                    required: true,
                    oninput: event => setRootForm({ ...rootForm, name: event.target.value })
                }) }),
                Field({ label: "Days", children: Mini.vnode("input", {
                    type: "number",
                    value: rootForm.days,
                    oninput: event => setRootForm({ ...rootForm, days: event.target.value })
                }) }),
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
                Field({ label: "Domain", children: Mini.vnode("select", {
                    value: serverForm.domain,
                    required: true,
                    onchange: event => setServerForm({ ...serverForm, domain: event.target.value })
                },
                    domains.length === 0
                        ? Mini.vnode("option", { value: "" }, "-- no domains configured --")
                        : domains.map(domain => Mini.vnode("option", { value: domain, key: domain }, domain))
                ) }),
                Field({ label: "Root CA", children: Mini.vnode("select", {
                    value: serverForm.caBase,
                    onchange: event => setServerForm({ ...serverForm, caBase: event.target.value })
                },
                    rootNames.length === 0
                        ? Mini.vnode("option", { value: "" }, "-- no root CA available --")
                        : rootNames.map(name => Mini.vnode("option", { value: name, key: name }, name))
                ) }),
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
                Field({ label: "Name", children: Mini.vnode("input", {
                    value: clientForm.name,
                    required: true,
                    oninput: event => setClientForm({ ...clientForm, name: event.target.value })
                }) }),
                Field({ label: "Root CA", children: Mini.vnode("select", {
                    value: clientForm.caBase,
                    onchange: event => setClientForm({ ...clientForm, caBase: event.target.value })
                },
                    rootNames.length === 0
                        ? Mini.vnode("option", { value: "" }, "-- no root CA available --")
                        : rootNames.map(name => Mini.vnode("option", { value: name, key: name }, name))
                ) }),
                Field({ label: "Client Identity", hint: "Usually an email address", children: Mini.vnode("input", {
                    value: clientForm.identity,
                    oninput: event => setClientForm({ ...clientForm, identity: event.target.value })
                }) }),
                Field({ label: "P12 Password", children: Mini.vnode("input", {
                    type: "password",
                    value: clientForm.p12Pass,
                    oninput: event => setClientForm({ ...clientForm, p12Pass: event.target.value })
                }) }),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ type: "button", class: "mui-button secondary", onclick: () => setActiveModal("") }, "Cancel"),
                    Mini.button({ type: "submit", class: "mui-button" }, "Create")
                )
            )
        )
    );
}

function CircuitsScreen() {
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
            setError(err.message || "Failed to load circuits");
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
            setError(err.message || "Failed to load interfaces");
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
            alert(`Failed to save circuit: ${err.message}`);
        }
    }

    async function toggleCircuit(id, enable) {
        try {
            await request(`/api/circuits/${id}/${enable ? "enable" : "disable"}`, { method: "POST" });
            loadCircuits();
        } catch (err) {
            alert(`Failed to update circuit: ${err.message}`);
        }
    }

    async function deleteCircuit() {
        if (!deleting) return;
        try {
            await request(`/api/circuits/${deleting.id}`, { method: "DELETE" });
            setDeleting(null);
            loadCircuits();
        } catch (err) {
            alert(`Failed to delete circuit: ${err.message}`);
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
                                    Mini.strong({class: "mui-truncate"}, circuit.name),
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
                Field({ label: "Name", children: Mini.vnode("input", {
                    required: true,
                    value: form.name,
                    oninput: event => setForm({ ...form, name: event.target.value })
                }) }),
                Field({ label: "Type", children: Mini.vnode("select", {
                    value: form.type,
                    onchange: event => setForm({ ...form, type: event.target.value })
                },
                    Mini.vnode("option", { value: "DNAT" }, "Port Forwarding"),
                    Mini.vnode("option", { value: "REDIRECT" }, "Inbound Redirect"),
                    Mini.vnode("option", { value: "LOCAL" }, "Local Redirect")
                ) }),
                !isLocal && Field({ label: "Inbound Interface", children: Mini.vnode("select", {
                    value: form.in_interface,
                    onchange: event => setForm({ ...form, in_interface: event.target.value })
                },
                    interfaces.map(iface => Mini.vnode("option", { value: iface, key: iface }, iface))
                ) }),
                Field({ label: "Protocol", children: Mini.vnode("select", {
                    value: form.protocol,
                    onchange: event => setForm({ ...form, protocol: event.target.value })
                },
                    Mini.vnode("option", { value: "tcp" }, "TCP"),
                    Mini.vnode("option", { value: "udp" }, "UDP"),
                    Mini.vnode("option", { value: "both" }, "Both")
                ) }),
                Field({ label: "Source Port", children: Mini.vnode("input", {
                    type: "number",
                    value: form.src_port,
                    oninput: event => setForm({ ...form, src_port: event.target.value })
                }) }),
                (isRedirect || isLocal) && Field({ label: "Redirect Port", children: Mini.vnode("input", {
                    type: "number",
                    value: form.redirect_port,
                    oninput: event => setForm({ ...form, redirect_port: event.target.value })
                }) }),
                isDNAT && Field({ label: "Destination IP", children: Mini.vnode("input", {
                    value: form.dest_ip,
                    oninput: event => setForm({ ...form, dest_ip: event.target.value })
                }) }),
                isDNAT && Field({ label: "Destination Port", children: Mini.vnode("input", {
                    type: "number",
                    value: form.dest_port,
                    oninput: event => setForm({ ...form, dest_port: event.target.value })
                }) }),
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

function VpnScreen() {
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

    const lineRefs = Mini.useState(() => ({}))[0];
    const lineCharts = Mini.useState(() => ({}))[0];
    const metricsState = Mini.useState(() => ({ samples: {}, times: {}, history: {} }))[0];

    function trackHistory(key, rx, tx) {
        if (!metricsState.history[key]) {
            metricsState.history[key] = {
                labels: new Array(60).fill(""),
                rx: new Array(60).fill(0),
                tx: new Array(60).fill(0)
            };
        }

        const now = Date.now();
        const prev = metricsState.samples[key];
        let rxRate = 0;
        let txRate = 0;

        if (prev) {
            const elapsed = Math.max((now - metricsState.times[key]) / 1000, 0.001);
            rxRate = ((rx - prev.rx) / elapsed) * 8;
            txRate = ((tx - prev.tx) / elapsed) * 8;

            metricsState.history[key].labels.push("");
            metricsState.history[key].rx.push(rxRate || 0);
            metricsState.history[key].tx.push(txRate || 0);
            if (metricsState.history[key].labels.length > 60) {
                metricsState.history[key].labels.shift();
                metricsState.history[key].rx.shift();
                metricsState.history[key].tx.shift();
            }
        }

        metricsState.samples[key] = { rx, tx };
        metricsState.times[key] = now;

        return { rxRate, txRate, history: metricsState.history[key] };
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
            setError(err.message || "Failed to load system interfaces");
        }
    }

    async function loadInterfaces() {
        setLoading(true);
        try {
            const data = await request(`${API}/interfaces`);
            setInterfaces(data || []);
            setError("");
        } catch (err) {
            setError(err.message || "Failed to load VPN interfaces");
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

                const ifaceRates = trackHistory(`iface-${iface.name}`, Number(metricsIface.rx) || 0, Number(metricsIface.tx) || 0);
                const nextPeers = (iface.peers || []).map(peer => {
                    const metricsPeer = (metricsIface.peers || []).find(item => item.id === peer.id);
                    if (!metricsPeer) return peer;

                    const peerRates = trackHistory(`peer-${peer.id}`, Number(metricsPeer.rx) || 0, Number(metricsPeer.tx) || 0);
                    return {
                        ...peer,
                        rxRate: peerRates.rxRate,
                        txRate: peerRates.txRate,
                        active: !!metricsPeer.active,
                        _history: peerRates.history
                    };
                });

                return {
                    ...iface,
                    rxRate: ifaceRates.rxRate,
                    txRate: ifaceRates.txRate,
                    peers: nextPeers,
                    _history: ifaceRates.history
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
            Object.keys(lineCharts).forEach(key => {
                lineCharts[key]?.destroy?.();
                delete lineCharts[key];
            });
        };
    }, []);

    Mini.useEffect(() => {
        if (!chartAvailable()) return;

        const allKeys = new Set();
        interfaces.forEach(iface => {
            allKeys.add(`iface-${iface.name}`);
            (iface.peers || []).forEach(peer => allKeys.add(`peer-${peer.id}`));
        });

        Object.keys(lineCharts).forEach(key => {
            if (!allKeys.has(key)) {
                lineCharts[key]?.destroy?.();
                delete lineCharts[key];
            }
        });

        allKeys.forEach(key => {
            const canvas = lineRefs[key];
            const history = metricsState.history[key] || { labels: new Array(60).fill(""), rx: new Array(60).fill(0), tx: new Array(60).fill(0) };
            if (!canvas) return;

            if (!lineCharts[key]) {
                lineCharts[key] = new window.Chart(canvas, {
                    type: "line",
                    data: {
                        labels: history.labels,
                        datasets: [
                            { label: "Download", borderColor: "#22c55e", data: history.rx, pointRadius: 0, tension: 0.35 },
                            { label: "Upload", borderColor: "#ef4444", data: history.tx, pointRadius: 0, tension: 0.35 }
                        ]
                    },
                    options: {
                        animation: false,
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: { legend: { display: false } },
                        scales: {
                            x: { display: false },
                            y: { display: false }
                        }
                    }
                });
                return;
            }

            const chart = lineCharts[key];
            chart.data.labels = history.labels;
            chart.data.datasets[0].data = history.rx;
            chart.data.datasets[1].data = history.tx;
            chart.update("none");
        });
    }, [interfaces]);

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
            alert(`Failed to save interface: ${err.message}`);
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
            alert(`Failed to toggle interface: ${err.message}`);
        }
    }

    async function confirmDeleteInterface() {
        if (!deleteIface) return;
        try {
            await request(`${API}/interfaces/${encodeURIComponent(deleteIface.name)}`, { method: "DELETE" });
            setDeleteIface(null);
            loadInterfaces();
        } catch (err) {
            alert(`Failed to delete interface: ${err.message}`);
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
            alert(`Failed to save peer: ${err.message}`);
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
            alert(`Failed to toggle peer: ${err.message}`);
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
            alert(`Failed to delete peer: ${err.message}`);
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
            alert(`Failed to download config: ${err.message}`);
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
                            Mini.div({ class: "mui-chart-line compact" },
                                Mini.vnode("canvas", { ref: el => lineRefs[`iface-${iface.name}`] = el })
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
                                                    Mini.span({ class: `mui-status-dot ${peer.active ? "on" : "idle"}` }),
                                                    Mini.strong({}, peer.name)
                                                ),
                                                Mini.small({ class: "mui-muted" }, `${peer.ip || ""}${peer.dns ? `  DNS: ${peer.dns}` : ""}`),
                                                Mini.small({ class: "mui-muted" }, `TX ${formatRate(peer.txRate || 0)}  RX ${formatRate(peer.rxRate || 0)}`)
                                            ),
                                            Mini.div({ class: "mui-row wrap" },
                                                Mini.button({ class: "mui-button secondary sm", onclick: () => downloadConfig(iface.name, peer.id, peer.name) }, "Config"),
                                                Mini.button({ class: "mui-button secondary sm", onclick: () => openEditPeer(iface.name, peer) }, "Edit"),
                                                Mini.button({
                                                    class: `mui-button ${peer.enabled ? "secondary" : "success"} sm`,
                                                    onclick: () => togglePeer(iface.name, peer.id, !peer.enabled)
                                                }, peer.enabled ? "Disable" : "Enable"),
                                                Mini.button({ class: "mui-button danger sm", onclick: () => setDeletePeer({ iface: iface.name, id: peer.id, name: peer.name }) }, "Delete")
                                            )
                                        ),
                                        Mini.div({ class: "mui-chart-line compact" },
                                            Mini.vnode("canvas", { ref: el => lineRefs[`peer-${peer.id}`] = el })
                                        )
                                    ))
                            )
                        );
                    })
                ),

        Mini.vnode(Modal, {
            open: ifaceModal.open,
            title: ifaceModal.editName ? "Edit Interface" : "Create Interface",
            onClose: () => setIfaceModal({ open: false, editName: "" })
        },
            Mini.vnode("form", { class: "mui-stack", onsubmit: saveInterface },
                Field({ label: "Name", children: Mini.vnode("input", {
                    value: ifaceForm.name,
                    disabled: !!ifaceModal.editName,
                    required: !ifaceModal.editName,
                    oninput: event => setIfaceForm({ ...ifaceForm, name: event.target.value })
                }) }),
                !ifaceModal.editName && Field({ label: "Address (CIDR)", children: Mini.vnode("input", {
                    value: ifaceForm.address,
                    required: true,
                    oninput: event => setIfaceForm({ ...ifaceForm, address: event.target.value })
                }) }),
                !ifaceModal.editName && Field({ label: "Listen Port", children: Mini.vnode("input", {
                    type: "number",
                    value: ifaceForm.listen_port,
                    required: true,
                    oninput: event => setIfaceForm({ ...ifaceForm, listen_port: event.target.value })
                }) }),
                Field({ label: "Endpoint (host:port)", children: Mini.vnode("input", {
                    value: ifaceForm.endpoint,
                    required: !ifaceModal.editName,
                    oninput: event => setIfaceForm({ ...ifaceForm, endpoint: event.target.value })
                }) }),
                Field({ label: "Default DNS", children: Mini.vnode("input", {
                    value: ifaceForm.default_dns,
                    oninput: event => setIfaceForm({ ...ifaceForm, default_dns: event.target.value })
                }) }),
                Field({ label: "Default Allowed IPs", children: Mini.vnode("input", {
                    value: ifaceForm.default_allowed_ips,
                    oninput: event => setIfaceForm({ ...ifaceForm, default_allowed_ips: event.target.value })
                }) }),
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
                ifaceForm.enable_nat && Field({ label: "Outbound Interface", children: Mini.vnode("select", {
                    value: ifaceForm.out_interface,
                    onchange: event => setIfaceForm({ ...ifaceForm, out_interface: event.target.value })
                },
                    systemIfaces.map(iface => Mini.vnode("option", { value: iface, key: iface }, iface))
                ) }),
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
                Field({ label: "Name", children: Mini.vnode("input", {
                    value: peerForm.name,
                    required: true,
                    oninput: event => setPeerForm({ ...peerForm, name: event.target.value })
                }) }),
                Field({ label: "DNS", hint: "Leave blank for interface default", children: Mini.vnode("input", {
                    value: peerForm.dns,
                    oninput: event => setPeerForm({ ...peerForm, dns: event.target.value })
                }) }),
                Field({ label: "Allowed IPs", hint: "Leave blank for interface default", children: Mini.vnode("input", {
                    value: peerForm.allowed_ips,
                    oninput: event => setPeerForm({ ...peerForm, allowed_ips: event.target.value })
                }) }),
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

function NotFoundScreen() {
    return Mini.div({ class: "mui-screen" },
        EmptyState({ title: "Screen not found", subtitle: "Choose a section from the navigation." })
    );
}

function App() {
    const [navOpen, setNavOpen] = Mini.useState(false);
    const { instanceName } = Mini.useStore(appStore);
    const currentPath = normalizeRoute(location.hash.slice(1) || "/");

    const routes = {
        "/": HomeScreen,
        "/dns": DnsScreen,
        "/certs": CertsScreen,
        "/circuits": CircuitsScreen,
        "/vpn": VpnScreen
    };

    const links = [
        { path: "/", label: "Overview" },
        { path: "/dns", label: "DNS" },
        { path: "/certs", label: "Certificates" },
        { path: "/circuits", label: "Circuits" },
        { path: "/vpn", label: "VPN" }
    ];

    return Mini.div({ class: "mui-shell" },
        Mini.header({ class: "mui-topbar" },
            Mini.div({ class: "mui-brand" },
                Mini.span({ class: "mui-brand-mark" }, "D"),
                Mini.div({ class: "mui-brand-text" },
                    Mini.strong({}, "Dynamo"),
                    Mini.small({ class: "mui-muted" }, instanceName)
                )
            ),
            Mini.button({
                class: "mui-button ghost nav-toggle",
                onclick: () => setNavOpen(!navOpen),
                "aria-label": "Toggle navigation"
            }, "Menu")
        ),
        Mini.div({ class: "mui-layout" },
            Mini.aside({ class: `mui-sidebar ${navOpen ? "open" : ""}` },
                links.map(link => Mini.button({
                    class: `mui-nav-link ${currentPath === link.path ? "active" : ""}`,
                    key: link.path,
                    onclick: () => {
                        setNavOpen(false);
                        Mini.navigate(link.path);
                    }
                }, link.label))
            ),
            Mini.main({ class: "mui-main" },
                Mini.vnode(Mini.Router, { routes, fallback: NotFoundScreen })
            )
        )
    );
}

const root = document.getElementById("app");
if (root) {
    Mini.render(App, root);
}
