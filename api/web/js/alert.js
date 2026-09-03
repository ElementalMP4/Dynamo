import Mini from "/js/mini.js";
import { closeModal, openModal } from "/js/utils.js";

export function showAlert(message, title = "Alert") {
    return new Promise(resolve => {
        let modalId = "";
        const handleClose = () => {
            closeModal(modalId);
            resolve();
        };
        modalId = openModal({
            title,
            onClose: handleClose,
            children: Mini.p({}, message)
        });
    });
}

export function showConfirm(message, title = "Confirm") {
    return new Promise(resolve => {
        let modalId = "";
        const finish = confirmed => {
            closeModal(modalId);
            resolve(confirmed);
        };
        modalId = openModal({
            title,
            onClose: () => finish(false),
            children: Mini.div({ class: "mui-stack" },
                Mini.p({}, message),
                Mini.div({ class: "mui-form-actions" },
                    Mini.button({ class: "mui-button secondary", onclick: () => finish(false) }, "Cancel"),
                    Mini.button({ class: "mui-button danger", onclick: () => finish(true) }, "Confirm")
                )
            )
        });
    });
}