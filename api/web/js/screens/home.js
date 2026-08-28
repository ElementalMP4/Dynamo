import { request, chartAvailable, formatRate, PageHeader, Spinner, EmptyState } from "../utils.js"

export function HomeScreen() {
    const [summary, setSummary] = Mini.useState({ dns: {}, certs: {}, circuits: null });
    const [netRows, setNetRows] = Mini.useState([]);
    const [error, setError] = Mini.useState("");
    const [isLoading, setIsLoading] = Mini.useState(true);

    const pieRefs = Mini.useState(() => ({ dns: null, certs: null, circuits: null }))[0];
    const pieCharts = Mini.useState(() => ({ dns: null, certs: null, circuits: null }))[0];
    const lineRefs = Mini.useState(() => ({}))[0];
    const lineCharts = Mini.useState(() => ({}))[0];
    const [cpuPct, setCpuPct] = Mini.useState(0);
    const [totalCores, setTotalCores] = Mini.useState(0);
    const [totalThreads, setTotalThreads] = Mini.useState(0);
    const [memUsedGB, setMemUsedGB] = Mini.useState("0");
    const [memTotalGB, setMemTotalGB] = Mini.useState("0");
    const [sysTick, setSysTick] = Mini.useState(0);
    const sysRefs = Mini.useState(() => ({ cpu: null, mem: null }))[0];
    const sysCharts = Mini.useState(() => ({}))[0];
    const sysPrev = Mini.useState(() => ({ cpu: null, cpuHist: new Array(60).fill(0), memHist: new Array(60).fill(0) }))[0];

    function pickPrimaryIPv4(ipList) {
        if (!Array.isArray(ipList)) return "";
        const primary = ipList.find(ip => typeof ip === "string" && ip.includes(":") === false && ip.trim().length > 0);
        return primary || "";
    }

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

            setTotalCores(cpu.cores);
            setTotalThreads(cpu.threads);
            setCpuPct(Math.round(pct * 10) / 10);
            setMemUsedGB((usedKB / 1024 / 1024).toFixed(1));
            setMemTotalGB((mem.total_kb / 1024 / 1024).toFixed(1));

            setSysTick(t => t + 1);

            const now = Date.now();
            const rows = [];
            Object.keys(data.net || {}).forEach(iface => {
                let showVirtualAdapters = localStorage["showVirtualAdapters"] === "yes";
                if ((iface.startsWith("veth") || iface.startsWith("docker") || iface.startsWith("br-")) && !showVirtualAdapters) return;

                const rx = Number(data.net[iface]?.rx) || 0;
                const tx = Number(data.net[iface]?.tx) || 0;
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
                    ip: pickPrimaryIPv4(data.net[iface]?.ip),
                    rxRate,
                    txRate,
                    history: history[iface]
                });
            });

            rows.sort((a, b) => a.iface.localeCompare(b.iface));
            setNetRows(rows);
        }

        async function cycle() {
            try {
                await pollSystem().catch(() => { });
                if (alive) {
                    setError("");
                    setIsLoading(false);
                }
            } catch (err) {
                if (alive) {
                    setError(err.error || "Failed to load dashboard");
                    setIsLoading(false);
                }
            } finally {
                if (alive) setTimeout(cycle, 1000);
            }
        }

        loadSummary();
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
            : Mini.div({ class: "mui-dashboard mui-overview" },
                Mini.div({ class: "mui-grid-2" },
                    Mini.div({ class: "mui-card" },
                        Mini.h3({}, "CPU Usage"),
                        Mini.div({ class: "mui-stat" }, `${cpuPct.toFixed(1)}%`),
                        Mini.small({ class: "mui-muted" }, `${totalThreads} threads across ${totalCores} cores`),
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
                                    Mini.div({ class: "mui-interface-meta" },
                                        Mini.div({ class: "mui-row" },
                                            Mini.span({ class: "mui-pill good" }, `⬇ ${formatRate(row.rxRate)}`),
                                            Mini.span({ class: "mui-pill bad" }, `⬆ ${formatRate(row.txRate)}`),
                                            row.ip && Mini.span({ class: "mui-pill info" }, row.ip)
                                        )
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