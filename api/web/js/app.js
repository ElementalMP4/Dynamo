import Mini from "/js/mini.js";
import { IncludeAlertModal, IncludeConfirmModal } from "/js/alert.js";

import HomeScreen from "/js/screens/home.js"
import CircuitsScreen from "/js/screens/circuits.js"
import CertsScreen from "/js/screens/certs.js"
import DnsScreen from "/js/screens/dns.js"
import VpnScreen from "/js/screens/vpn.js"

const appStore = Mini.createStore({ instanceName: "" });

const ROUTE_PATHS = ["/", "/dns", "/certs", "/circuits", "/vpn"];

function normalizeRoute(path) {
    return ROUTE_PATHS.includes(path) ? path : "/";
}

function bootstrapHashFromPath() {
    if (location.hash) return;
    const mapped = normalizeRoute(location.pathname);
    location.hash = mapped;
}

bootstrapHashFromPath();

function NotFoundScreen() {
    return Mini.div({ class: "mui-screen" },
        EmptyState({ title: "Screen not found", subtitle: "Choose a section from the navigation." })
    );
}

function App() {
    const [navOpen, setNavOpen] = Mini.useState(false);
    const { instanceName } = Mini.useStore(appStore);
    const currentPath = normalizeRoute(location.hash.slice(1) || "/");

    const routes = {
        "/": HomeScreen,
        "/dns": DnsScreen,
        "/certs": CertsScreen,
        "/circuits": CircuitsScreen,
        "/vpn": VpnScreen
    };

    const links = [
        { path: "/", label: "Overview" },
        { path: "/dns", label: "DNS" },
        { path: "/certs", label: "Certificates" },
        { path: "/circuits", label: "Circuits" },
        { path: "/vpn", label: "VPN" }
    ];

    return Mini.div({ class: "mui-shell" },
        Mini.vnode(IncludeAlertModal, { key: "alert-modal" }),
        Mini.vnode(IncludeConfirmModal, { key: "confirm-modal" }),
        Mini.header({ class: "mui-topbar" },
            Mini.div({ class: "mui-brand" },
                Mini.span({ class: "mui-brand-mark" }, "D"),
                Mini.div({ class: "mui-brand-text" },
                    Mini.strong({}, "Dynamo"),
                    Mini.small({ class: "mui-muted" }, instanceName)
                )
            ),
            Mini.button({
                class: "mui-button ghost nav-toggle",
                onclick: () => setNavOpen(!navOpen),
                "aria-label": "Toggle navigation"
            }, "Menu")
        ),
        Mini.div({ class: "mui-layout" },
            Mini.aside({ class: `mui-sidebar ${navOpen ? "open" : ""}` },
                links.map(link => Mini.button({
                    class: `mui-nav-link ${currentPath === link.path ? "active" : ""}`,
                    key: link.path,
                    onclick: () => {
                        setNavOpen(false);
                        Mini.navigate(link.path);
                    }
                }, link.label))
            ),
            Mini.main({ class: "mui-main" },
                Mini.vnode(Mini.Router, { routes, fallback: NotFoundScreen })
            )
        )
    );
}

const root = document.getElementById("app");
if (root) {
    Mini.render(App, root);
}
