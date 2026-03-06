const netCharts = {};
const lastSamples = {};
const lastTimes = {};

const HISTORY_SECONDS = 60;
const labels = new Array(HISTORY_SECONDS).fill("");
const emptyData = new Array(HISTORY_SECONDS).fill(0);

function formatRate(bytesPerSec) {
    let bitsPerSec = bytesPerSec * 8;
    const units = ["bps", "Kbps", "Mbps", "Gbps"];
    let i = 0;

    while (bitsPerSec > 1000 && i < units.length - 1) {
        bitsPerSec /= 1000;
        i++;
    }

    return bitsPerSec.toFixed(1) + " " + units[i];
}

function updateYAxis(chart) {

    const rxData = chart.data.datasets[0].data;
    const txData = chart.data.datasets[1].data;

    const maxRX = Math.max(...rxData, 0);
    const maxTX = Math.max(...txData, 0);

    const maxValue = Math.max(maxRX, maxTX);

    chart.options.scales.y.suggestedMax = maxValue * 1.3;
}

function createInterfaceCard(iface) {
    const grid = document.getElementById("net-grid");

    const card = document.createElement("div");
    card.className = "stat-card";

    const header = document.createElement("div");
    header.className = "net-header";

    const title = document.createElement("h2");
    title.textContent = iface;

    const stats = document.createElement("div");
    stats.className = "net-stats";
    stats.innerHTML = `
        <span class="rx" id="rx-${iface}">0</span>
        <span class="tx" id="tx-${iface}">0</span>
    `;

    const canvas = document.createElement("canvas");
    canvas.id = "chart-" + iface;

    header.appendChild(title);
    header.appendChild(stats);

    card.appendChild(header);
    card.appendChild(canvas);

    grid.appendChild(card);

    netCharts[iface] = new Chart(canvas, {
        type: "line",
        data: {
            labels: labels,
            datasets: [
                {
                    label: "RX",
                    borderColor: "#22c55e",
                    backgroundColor: "rgba(34,197,94,0.2)",
                    data: [...emptyData],
                    tension: 0.35,
                    pointRadius: 0,
                    spanGaps: true

                },
                {
                    label: "TX",
                    borderColor: "#ef4444",
                    backgroundColor: "rgba(239,68,68,0.2)",
                    data: [...emptyData],
                    tension: 0.35,
                    pointRadius: 0,
                    spanGaps: true

                }
            ]
        },
        options: {
            animation: false,
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                x: {
                    display: false
                },
                y: {
                    beginAtZero: true,
                    ticks: {
                        callback: function (value) {
                            return formatRate(value);
                        }
                    }
                }
            },
            plugins: {
                legend: {
                    display: false
                }
            }
        }
    });
}

async function pollNetwork() {
    const res = await fetch("/stats/network");
    const data = await res.json();

    const now = Date.now();

    for (const iface in data) {
        if (iface.startsWith("veth")) continue;
        if (iface.startsWith("docker")) continue;

        const rx = data[iface].rx;
        const tx = data[iface].tx;

        if (!netCharts[iface]) {
            createInterfaceCard(iface);
        }

        if (lastSamples[iface]) {

            const seconds = (now - lastTimes[iface]) / 1000;

            const rxRate = (rx - lastSamples[iface].rx) / seconds * 8;
            const txRate = (tx - lastSamples[iface].tx) / seconds * 8;

            const chart = netCharts[iface];

            chart.data.labels.push("");
            chart.data.datasets[0].data.push(rxRate || 0);
            chart.data.datasets[1].data.push(txRate || 0);

            if (chart.data.labels.length > HISTORY_SECONDS) {
                chart.data.labels.shift();
                chart.data.datasets.forEach(d => d.data.shift());
            }

            updateYAxis(chart);
            chart.update("none");

            document.getElementById("rx-" + iface).textContent =
                "RX " + formatRate(rxRate);

            document.getElementById("tx-" + iface).textContent =
                "TX " + formatRate(txRate);
        }

        lastSamples[iface] = { rx, tx };
        lastTimes[iface] = now;
    }
}

setInterval(pollNetwork, 1000);

async function loadCounts() {
    const res = await fetch("/stats/dynamo");
    const data = await res.json();

    const dnsValues = Object.values(data.dns);
    const dnsTotal = dnsValues.reduce((a, b) => a + b, 0);
    document.getElementById("dns-total").textContent = dnsTotal;

    new Chart(document.getElementById("dns-chart"), {
        type: "pie",
        data: {
            labels: Object.keys(data.dns),
            datasets: [{
                data: dnsValues
            }]
        }
    });

    const certValues = Object.values(data.certs);
    const certTotal = certValues.reduce((a, b) => a + b, 0);
    document.getElementById("certs-total").textContent = certTotal;

    new Chart(document.getElementById("certs-chart"), {
        type: "pie",
        data: {
            labels: Object.keys(data.certs),
            datasets: [{
                data: certValues
            }]
        }
    });

    const circuitData = {
        "DNAT - ON": data.circuits.DNAT.on,
        "DNAT - OFF": data.circuits.DNAT.off,
        "REDIRECT - ON": data.circuits.REDIRECT.on,
        "REDIRECT - OFF": data.circuits.REDIRECT.off,
        "LOCAL - ON": data.circuits.LOCAL.on,
        "LOCAL - OFF": data.circuits.LOCAL.off
    };

    const circuitValues = Object.values(circuitData);
    const circuitTotal = circuitValues.reduce((a, b) => a + b, 0);
    document.getElementById("circuits-total").textContent = circuitTotal;

    new Chart(document.getElementById("circuits-chart"), {
        type: "pie",
        data: {
            labels: Object.keys(circuitData),
            datasets: [{
                data: circuitValues
            }]
        }
    });
}

loadCounts();