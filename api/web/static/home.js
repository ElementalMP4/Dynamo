async function loadCounts() {
    const res = await fetch("/counts");
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