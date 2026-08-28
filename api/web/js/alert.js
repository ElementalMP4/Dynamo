import Mini from "/js/mini.js";

const alertStore = Mini.createStore({ open: false, title: "", message: "", resolve: null });
const confirmStore = Mini.createStore({ open: false, title: "", message: "", resolve: null });

export function showAlert(message, title = "Alert") {
    return new Promise(resolve => {
        alertStore.setState({ open: true, title, message, resolve });
    });
}

function closeAlert() {
    const { resolve } = alertStore.getState();
    alertStore.setState({ open: false, title: "", message: "", resolve: null });
    if (resolve) resolve();
}

export function showConfirm(message, title = "Confirm") {
    return new Promise(resolve => {
        confirmStore.setState({ open: true, title, message, resolve });
    });
}

function closeConfirm(confirmed) {
    const { resolve } = confirmStore.getState();
    confirmStore.setState({ open: false, title: "", message: "", resolve: null });
    if (resolve) resolve(confirmed);
}

export function IncludeAlertModal() {
    const { open, title, message } = Mini.useStore(alertStore);

    if (!open) return null;

    return Mini.div({
        class: "mui-modal-backdrop",
        role: "presentation",
        onclick: event => {
            if (event.target.className === "mui-modal-backdrop") closeAlert();
        }
    },
        Mini.div({
            class: "mui-modal",
            role: "dialog",
            "aria-modal": "true",
            onclick: event => event.stopPropagation()
        },
            Mini.div({ class: "mui-modal-header" },
                Mini.strong({}, title || "Alert"),
                Mini.button({ class: "mui-button secondary", onclick: closeAlert }, "Close")
            ),
            Mini.div({ class: "mui-modal-body" },
                Mini.p({}, message)
            )
        )
    );
}

export function IncludeConfirmModal() {
    const { open, title, message } = Mini.useStore(confirmStore);

    if (!open) return null;

    return Mini.div({
        class: "mui-modal-backdrop",
        role: "presentation",
        onclick: event => {
            if (event.target.className === "mui-modal-backdrop") closeConfirm(false);
        }
    },
        Mini.div({
            class: "mui-modal",
            role: "dialog",
            "aria-modal": "true",
            onclick: event => event.stopPropagation()
        },
            Mini.div({ class: "mui-modal-header" },
                Mini.strong({}, title || "Confirm"),
                Mini.button({ class: "mui-button secondary", onclick: () => closeConfirm(false) }, "Close")
            ),
            Mini.div({ class: "mui-modal-body" },
                Mini.p({}, message),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ class: "mui-button secondary", onclick: () => closeConfirm(false) }, "Cancel"),
                    Mini.button({ class: "mui-button danger", onclick: () => closeConfirm(true) }, "Confirm")
                )
            )
        )
    );
}