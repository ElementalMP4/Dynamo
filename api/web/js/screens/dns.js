import { jsonHeaders, request, Field, PageHeader, Spinner, Modal } from "../utils.js";
import { showConfirm, showAlert } from "../alert.js"
import Mini from "../mini.js";

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

export default function DnsScreen() {
    const [records, setRecords] = Mini.useState([]);
    const [loading, setLoading] = Mini.useState(true);
    const [error, setError] = Mini.useState("");
    const [addOpen, setAddOpen] = Mini.useState(false);
    const [editRecord, setEditRecord] = Mini.useState(null);
    const [openGroups, setOpenGroups] = Mini.useState({ A: false, CNAME: false, TXT: false, SRV: false });

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
            setError(err.error || "Failed to load DNS records");
        } finally {
            setLoading(false);
        }
    }

    Mini.useEffect(async () => {
        await loadRecords();
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

    function recordTargetOptions() {
        const options = records
            .filter(record => ["A", "CNAME"].includes(record.type) && record.name)
            .filter(record => !editRecord || record.id !== editRecord.id)
            .map(record => record.name);

        return [...new Set(options)].sort();
    }

    async function createRecord(event) {
        event.preventDefault();
        try {
            await request("/api/dns", {
                method: "POST",
                headers: jsonHeaders(),
                body: JSON.stringify(formToPayload(newRecord))
            });
            await loadRecords();
            setAddOpen(false);
        } catch (err) {
            await showAlert(`Error creating record: ${err.error}`);
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
            await loadRecords();
            setEditRecord(null);
        } catch (err) {
            await showAlert(`Error updating record: ${err.error}`);
        }
    }

    async function removeRecord(id) {
        if (!(await showConfirm("Are you sure you want to delete this record?"))) return;
        try {
            await request(`/api/dns/${id}`, { method: "DELETE" });
            await loadRecords();
        } catch (err) {
            await showAlert(`Error deleting record: ${err.error}`);
        }
    }

    function grouped() {
        const groups = { A: [], CNAME: [], TXT: [], SRV: [] };
        records.forEach(record => {
            if (groups[record.type]) groups[record.type].push(record);
        });
        return groups;
    }

    function toggleGroup(type) {
        setOpenGroups(old => ({ ...old, [type]: !old[type] }));
    }

    function recordContent(record) {
        if (record.type === "A") return record.ipv4;
        if (record.type === "CNAME") return record.cname;
        if (record.type === "TXT") return Array.isArray(record.txt) ? record.txt.join(", ") : "";
        return `Priority: ${record.priority}, Weight: ${record.weight}, Port: ${record.port}, Target: ${record.target}`;
    }

    function renderDynamic(form, setter, editableType = true) {
        const fields = RECORD_FIELDS[form.type] || [];
        const targetOptions = recordTargetOptions();

        return Mini.div({ class: "mui-field-grid" },
            editableType && Field({
                label: "Type", children: Mini.vnode("select", {
                    value: form.type,
                    onchange: event => setter({ ...form, type: event.target.value })
                },
                    Mini.vnode("option", { value: "A" }, "A Record"),
                    Mini.vnode("option", { value: "CNAME" }, "CNAME Record"),
                    Mini.vnode("option", { value: "TXT" }, "TXT Record"),
                    Mini.vnode("option", { value: "SRV" }, "SRV Record")
                )
            }),
            Field({
                label: "Name", children: Mini.vnode("input", {
                    type: "text",
                    value: form.name,
                    required: true,
                    oninput: event => setter({ ...form, name: event.target.value })
                })
            }),
            Field({
                label: "TTL (seconds)", children: Mini.vnode("input", {
                    type: "number",
                    value: form.ttl,
                    oninput: event => setter({ ...form, ttl: event.target.value })
                })
            }),
            fields.map(field => {
                if (form.type === "CNAME" && field.id === "cname" && targetOptions.length > 0) {
                    return Field({
                        key: field.id,
                        label: field.label,
                        children: Mini.vnode("select", {
                            value: form.cname,
                            required: field.required,
                            onchange: event => setter({ ...form, cname: event.target.value })
                        },
                            Mini.vnode("option", { value: "" }, "Select target record"),
                            targetOptions.map(option => Mini.vnode("option", { value: option, key: option }, option))
                        )
                    });
                }

                if (form.type === "SRV" && field.id === "target" && targetOptions.length > 0) {
                    return Field({
                        key: field.id,
                        label: field.label,
                        children: Mini.vnode("select", {
                            value: form.target,
                            required: field.required,
                            onchange: event => setter({ ...form, target: event.target.value })
                        },
                            Mini.vnode("option", { value: "" }, "Select target record"),
                            targetOptions.map(option => Mini.vnode("option", { value: option, key: option }, option))
                        )
                    });
                }

                return Field({
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
                });
            })
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
            : Mini.div({ class: "mui-stack dns-group-list" },
                Object.keys(groups).map(type => Mini.section({ class: "mui-card dns-group-card", key: type },
                    Mini.div({ class: "mui-row spread wrap" },
                        Mini.h3({}, `${type} Records (${groups[type].length})`),
                        Mini.div({ class: "mui-row" },
                            groups[type].length > 0 && Mini.button({ class: "mui-button secondary sm", type: "button", onclick: () => toggleGroup(type) },
                                openGroups[type] ? "Hide" : "Show"
                            )
                        )
                    ),
                    openGroups[type]
                        ? Mini.div({ class: "mui-stack mui-group-details" },
                            groups[type].map(record => Mini.div({ class: "mui-item dns-record-row", key: String(record.id) },
                                Mini.div({ class: "mui-row spread wrap" },
                                    Mini.strong({ class: "mui-truncate" }, record.name),
                                    Mini.div({ class: "mui-row" },
                                        Mini.button({ class: "mui-button secondary sm", onclick: () => openEdit(record), type: "button" }, "Edit"),
                                        Mini.button({ class: "mui-button danger sm", onclick: () => removeRecord(record.id), type: "button" }, "Delete")
                                    )
                                ),
                                Mini.small({ class: "mui-muted" }, `TTL: ${record.ttl}s`),
                                Mini.p({}, recordContent(record))
                            ))
                        ) : null
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