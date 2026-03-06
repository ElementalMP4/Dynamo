const netCharts = {};
const lastSamples = {};
const lastTimes = {};

function createInterfaceCard(iface) {
    const grid = document.getElementById("net-grid");

    const card = document.createElement("div");
    card.className = "stat-card";

    const title = document.createElement("h2");
    title.textContent = iface;

    const canvas = document.createElement("canvas");
    canvas.id = "chart-" + iface;

    card.appendChild(title);
    card.appendChild(canvas);
    grid.appendChild(card);

    netCharts[iface] = new Chart(canvas, {
        type: "line",
        data: {
            labels: [],
            datasets: [
                {
                    label: "RX KB/s",
                    data: [],
                    tension: 0.25
                },
                {
                    label: "TX KB/s",
                    data: [],
                    tension: 0.25
                }
            ]
        },
        options: {
            animation: false,
            responsive: true,
            scales: {
                y: {
                    beginAtZero: true
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

            const rxRate = (rx - lastSamples[iface].rx) / seconds / 1024;
            const txRate = (tx - lastSamples[iface].tx) / seconds / 1024;

            const chart = netCharts[iface];

            const label = new Date().toLocaleTimeString();

            chart.data.labels.push(label);
            chart.data.datasets[0].data.push(rxRate);
            chart.data.datasets[1].data.push(txRate);

            if (chart.data.labels.length > 60) {
                chart.data.labels.shift();
                chart.data.datasets.forEach(d => d.data.shift());
            }

            chart.update();
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