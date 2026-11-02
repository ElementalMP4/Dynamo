import { request, PageHeader, Spinner, EmptyState } from "../utils.js";
import { showConfirm } from "../alert.js";
import Mini from "../mini.js";

export default function UpstreamDnsScreen() {
    const [cache, setCache] = Mini.useState(null);
    const [page, setPage] = Mini.useState(1);
    const [revision, setRevision] = Mini.useState(0);
    const [loading, setLoading] = Mini.useState(true);
    const [busy, setBusy] = Mini.useState("");
    const [error, setError] = Mini.useState("");

    Mini.useEffect(() => {
        const controller = new AbortController();
        setLoading(true);
        setError("");
        request(`/api/dns/cache?page=${page}`, { signal: controller.signal })
            .then(data => {
                if (!controller.signal.aborted) setCache(data);
            })
            .catch(err => {
                if (!controller.signal.aborted) {
                    setCache(null);
                    setError(`Failed to load upstream DNS cache: ${err.message}`);
                }
            })
            .finally(() => {
                if (!controller.signal.aborted) setLoading(false);
            });
        return () => controller.abort();
    }, [page, revision]);

    function reload(targetPage = cache?.page || 1) {
        setPage(targetPage);
        setRevision(old => old + 1);
    }

    async function clearCache(record = null) {
        if (loading || busy) return;
        setBusy(record?.id || "all");
        try {
            if (!record && !(await showConfirm("Clear all cached upstream DNS records? New DNS queries will refill the cache."))) return;
            await request(record ? `/api/dns/cache/${encodeURIComponent(record.id)}` : "/api/dns/cache", { method: "DELETE" });
            reload(record ? cache.page : 1);
        } catch (err) {
            setError(`Failed to clear upstream DNS cache: ${err.message}`);
        } finally {
            setBusy("");
        }
    }

    function recordSection(label, records) {
        if (records.length === 0) return null;
        return Mini.div({ class: "mui-stack tight" },
            Mini.small({ class: "mui-muted" }, label),
            Mini.vnode("pre", { class: "upstream-dns-records" }, records.join("\n"))
        );
    }

    const disabled = loading || !!busy;

    return Mini.div({ class: "mui-screen" },
        PageHeader({
            title: "Upstream DNS",
            subtitle: "Inspect and clear cached upstream answers. Each row represents one cached query; NXDOMAIN is not cached.",
            actions: [
                Mini.button({ class: "mui-button secondary", type: "button", disabled, onclick: () => reload() }, "Refresh"),
                Mini.button({
                    class: "mui-button danger", type: "button", disabled: disabled || !cache?.total,
                    onclick: () => clearCache()
                }, busy === "all" ? "Clearing..." : "Clear All")
            ]
        }),
        error && Mini.div({ class: "mui-alert danger", role: "alert" }, error),
        loading
            ? Spinner({ label: "Loading upstream DNS cache..." })
            : cache && Mini.section({ class: "mui-card" },
                Mini.div({ class: "mui-row spread wrap", "aria-live": "polite" },
                    Mini.strong({}, `${cache.total} cached records`),
                    Mini.span({ class: "mui-muted" }, `Page ${cache.total_pages ? cache.page : 0} of ${cache.total_pages} (up to ${cache.page_size} records per page)`)
                ),
                cache.records.length === 0
                    ? EmptyState({ title: "No cached upstream records", subtitle: "Positive upstream DNS answers will appear here after they are resolved." })
                    : Mini.div({ class: "mui-stack" },
                        cache.records.map(record => Mini.div({ class: "mui-item upstream-dns-record", key: record.id },
                            Mini.div({ class: "mui-row spread wrap" },
                                Mini.strong({ class: "upstream-dns-name" }, record.name),
                                Mini.div({ class: "mui-row" },
                                    Mini.span({ class: "mui-badge" }, `${record.class} ${record.type}`),
                                    Mini.button({
                                        class: "mui-button danger sm", type: "button", disabled,
                                        "aria-label": `Clear cached ${record.type} record for ${record.name}`,
                                        onclick: () => clearCache(record)
                                    }, busy === record.id ? "Clearing..." : "Clear")
                                )
                            ),
                            Mini.small({ class: "mui-muted" },
                                `TTL: ${record.ttl}s | Cached: ${new Date(record.cached_at).toLocaleString()} | Expires: ${new Date(record.expires_at).toLocaleString()}`
                            ),
                            recordSection("Answers", record.answers),
                            recordSection("Authority", record.authority),
                            recordSection("Additional", record.additional)
                        ))
                    ),
                Mini.vnode("nav", { class: "mui-row spread wrap", "aria-label": "Upstream DNS pagination" },
                    Mini.button({
                        class: "mui-button secondary", type: "button", disabled: disabled || cache.page <= 1,
                        onclick: () => reload(cache.page - 1)
                    }, "Previous"),
                    Mini.button({
                        class: "mui-button secondary", type: "button", disabled: disabled || cache.page >= cache.total_pages,
                        onclick: () => reload(cache.page + 1)
                    }, "Next")
                )
            )
    );
}
