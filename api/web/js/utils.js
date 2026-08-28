import Mini from "/js/mini.js";

export function jsonHeaders() {
    return { "Content-Type": "application/json" };
}

export function chartAvailable() {
    return typeof window !== "undefined" && typeof window.Chart === "function";
}

export function formatRate(bitsPerSec) {
    let value = Number(bitsPerSec) || 0;
    const units = ["bps", "Kbps", "Mbps", "Gbps"];
    let i = 0;
    while (value > 1000 && i < units.length - 1) {
        value /= 1000;
        i++;
    }
    return `${value.toFixed(1)} ${units[i]}`;
}

export async function request(path, options = {}) {
    const res = await fetch(path, {
        cache: "no-store",
        ...options
    });
    if (!res.ok) {
        const body = await res.text();
        throw new Error(`${res.status} ${res.statusText} ${body || ""}`);
    }
    const contentType = res.headers.get("content-type") || "";
    if (contentType.includes("application/json")) return res.json();
    return res.text();
}

export function Field({ label, children, hint }) {
    return Mini.div({ class: "mui-field" },
        Mini.label({ class: "mui-label" }, label),
        hint && Mini.small({ class: "mui-muted" }, hint),
        children
    );
}

export function PageHeader({ title, subtitle, actions = [] }) {
    return Mini.div({ class: "mui-page-header" },
        Mini.div({ class: "mui-page-heading" },
            Mini.h1({}, title),
            subtitle && Mini.p({ class: "mui-muted" }, subtitle)
        ),
        Mini.div({ class: "mui-page-actions" }, actions)
    );
}

export function Spinner({ label = "Loading..." }) {
    return Mini.div({ class: "mui-loading" },
        Mini.div({ class: "mui-spinner", "aria-hidden": "true" }),
        Mini.span({}, label)
    );
}

export function Modal({ open, title, onClose, children, wide = false }) {
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

export function EmptyState({ title, subtitle }) {
    return Mini.div({ class: "mui-empty" },
        Mini.strong({}, title),
        subtitle && Mini.p({ class: "mui-muted" }, subtitle)
    );
}