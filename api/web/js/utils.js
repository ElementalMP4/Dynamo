import Mini from "/js/mini.js";

const modalStore = Mini.createStore({ activeId: "" });
const modalRegistry = new Map();
let modalIdSeed = 0;

function activateModal(id) {
    const { activeId } = modalStore.getState();
    if (activeId === id) return;

    const previous = activeId ? modalRegistry.get(activeId) : null;
    if (previous && typeof previous.onClose === "function") previous.onClose();

    if (modalStore.getState().activeId !== id) {
        modalStore.setState({ activeId: id });
    }
}

function removeModal(id) {
    modalRegistry.delete(id);
    if (modalStore.getState().activeId === id) {
        modalStore.setState({ activeId: "" });
    }
}

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

export function openModal(config) {
    const id = `modal-${++modalIdSeed}`;
    modalRegistry.set(id, config);
    activateModal(id);
    return id;
}

export function closeModal(id) {
    removeModal(id);
}

export function ModalHost() {
    const activeId = Mini.useStore(modalStore, state => state.activeId);
    const modal = activeId ? modalRegistry.get(activeId) : null;

    Mini.useEffect(() => {
        if (!modal) return undefined;
        const onKeyDown = event => {
            if (event.key === "Escape") modal.onClose();
        };
        window.addEventListener("keydown", onKeyDown);
        return () => window.removeEventListener("keydown", onKeyDown);
    }, [modal]);

    if (!modal) return null;

    return Mini.div({
        key: modal.id,
        class: "mui-modal-backdrop",
        role: "presentation",
        onclick: event => {
            if (event.target.className === "mui-modal-backdrop") modal.onClose();
        }
    },
        Mini.div({
            class: `mui-modal ${modal.wide ? "mui-modal-wide" : ""}`,
            role: "dialog",
            "aria-modal": "true",
            onclick: event => event.stopPropagation()
        },
            Mini.div({ class: "mui-modal-header" },
                Mini.strong({}, modal.title),
                Mini.button({ class: "mui-button secondary", onclick: modal.onClose }, "Close")
            ),
            Mini.div({ class: "mui-modal-body" }, modal.children)
        )
    );
}

export function Modal({ open, title, onClose, children, wide = false }) {
    const [modalId] = Mini.useState(() => `modal-component-${++modalIdSeed}`);

    Mini.useEffect(() => () => removeModal(modalId), []);

    Mini.useEffect(() => {
        if (!open) {
            removeModal(modalId);
            return;
        }
        activateModal(modalId);
    }, [open]);

    if (open) {
        modalRegistry.set(modalId, { title, onClose, children, wide });
    } else {
        modalRegistry.delete(modalId);
    }

    return null;
}

export function EmptyState({ title, subtitle }) {
    return Mini.div({ class: "mui-empty" },
        Mini.strong({}, title),
        subtitle && Mini.p({ class: "mui-muted" }, subtitle)
    );
}