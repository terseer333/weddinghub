(() => {
    "use strict";

    // The platform admin console. Every request goes through /api/admin/*, which re-checks on the
    // server that the signed-in account is the configured administrator — this script only
    // decides what to draw, never who may see it.
    const API = window.WeddingHubAPI;
    const view = document.getElementById("adminView");
    const nav = document.getElementById("adminNav");
    const sidebar = document.getElementById("sidebar");
    const backdrop = document.getElementById("sidebarBackdrop");
    const status = document.getElementById("apiStatus");

    // The sections the console will grow into. Each is either built or clearly marked as planned;
    // none of them shows invented numbers.
    const SECTIONS = {
        dashboard: {
            title: "Dashboard",
            eyebrow: "Platform administration",
            blurb: "A summary of the whole WeddingHub platform.",
            ready: false,
            planned: "Totals for registered users, active and suspended accounts, weddings created and new signups this month, plus registration and activity trends drawn from real records."
        },
        users: {
            title: "Users", eyebrow: "Accounts", blurb: "Everyone with a WeddingHub account.",
            planned: "List, search and filter accounts, review a user's registration date and associated wedding, and suspend, reactivate or delete an account. Destructive actions will ask for confirmation and be written to the audit log."
        },
        weddings: {
            title: "Weddings", eyebrow: "Content", blurb: "Every wedding created on WeddingHub.",
            planned: "Browse and search weddings by couple, filter by publication status, see the owning account, and hide or remove a wedding page that breaks the platform rules. Private planning data stays out of this view."
        },
        payments: {
            title: "Payments", eyebrow: "Billing", blurb: "Transactions and subscriptions.",
            ready: false,
            missing: "WeddingHub has no payment provider and no transaction records, so there is nothing real to show. This section stays a placeholder until payments actually exist — it will not display invented figures."
        },
        analytics: {
            title: "Analytics", eyebrow: "Reporting", blurb: "How the platform is being used.",
            planned: "Daily, weekly and monthly registrations, weddings created, active users and retention, computed from the accounts and weddings already in the database, with CSV export."
        },
        reports: {
            title: "Reports", eyebrow: "Moderation", blurb: "Reports and complaints from users.",
            missing: "There is no reporting or complaints feature in WeddingHub yet, so there is nothing to review. This section stays a placeholder until the ability to submit a report exists."
        },
        notifications: {
            title: "Notifications", eyebrow: "Communication", blurb: "Announcements to users.",
            missing: "WeddingHub has no platform notification system or stored history, so this section stays a placeholder. It will not claim a message was delivered unless delivery can actually be verified."
        },
        support: {
            title: "Support", eyebrow: "Help desk", blurb: "Requests from users.",
            missing: "There is no support-request system or ticket storage yet, so this section stays a placeholder until that backend exists."
        },
        settings: {
            title: "Website Settings", eyebrow: "Configuration", blurb: "Platform-wide settings.",
            planned: "Site name, official contact address, whether registration is open, maintenance mode, default wedding settings and the legal pages. Changes will be validated on the server, and secrets will never be shown here."
        },
        security: {
            title: "Admin Security", eyebrow: "Safety", blurb: "Administrator activity and the audit trail.",
            planned: "Recent administrator sign-ins, failed attempts, active sessions and a read-only log of administrative actions — the trail the backend is already recording."
        }
    };

    function initials(text) {
        const parts = String(text || "").trim().split(/\s+/).filter(Boolean);
        if (!parts.length) return "WA";
        return parts.map(part => part[0]).join("").slice(0, 2).toUpperCase();
    }

    function escapeHTML(value) {
        return String(value == null ? "" : value).replace(/[&<>"']/g, character => (
            { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[character]
        ));
    }

    function showSection(id) {
        const section = SECTIONS[id];
        if (!section) return;
        for (const button of nav.querySelectorAll("[data-view]")) {
            button.classList.toggle("active", button.dataset.view === id);
        }
        const heading = '<div class="page-heading"><div><p class="eyebrow">' + escapeHTML(section.eyebrow) + "</p><h1>" +
            escapeHTML(section.title) + "</h1><p>" + escapeHTML(section.blurb) + "</p></div></div>";
        if (id === "dashboard") {
            view.innerHTML = heading + dashboardPanel();
            return;
        }
        const body = section.missing
            ? '<span aria-hidden="true">◌</span><h2>Not available yet</h2><p>' + escapeHTML(section.missing) + "</p>"
            : '<span aria-hidden="true">✎</span><h2>Planned</h2><p>This section is not built yet. When it is, it will show: ' + escapeHTML(section.planned) + "</p>";
        view.innerHTML = heading + '<article class="admin-card platform-empty">' + body + "</article>";
    }

    // dashboardPanel shows the administrator's own account — the only real platform data this
    // phase has — and says plainly what is still to come.
    function dashboardPanel() {
        return '<article class="admin-card admin-account-card"><span class="user-avatar" aria-hidden="true">' +
            escapeHTML(initials(adminName.textContent)) + '</span><div><p class="eyebrow">Signed in as</p><h2>' +
            escapeHTML(adminName.textContent) + "</h2><p>" + escapeHTML(adminEmail.textContent) +
            "</p></div></article>" +
            '<div class="admin-facts"><article><small>Access</small><strong>Platform administrator</strong></article>' +
            '<article><small>Scope</small><strong>Every wedding</strong></article></div>' +
            '<article class="admin-card platform-empty" style="margin-top:15px;"><span aria-hidden="true">◔</span>' +
            "<h2>Overview statistics come next</h2><p>" + escapeHTML(SECTIONS.dashboard.planned) +
            " Until that is built, this page shows no platform totals rather than invented ones.</p></article>";
    }

    function closeMenu() {
        sidebar.classList.remove("open");
        backdrop.classList.remove("open");
        document.body.classList.remove("menu-open");
    }

    nav.addEventListener("click", event => {
        const button = event.target.closest("[data-view]");
        if (!button) return;
        showSection(button.dataset.view);
        closeMenu();
    });

    document.getElementById("menuButton").addEventListener("click", () => {
        const open = !sidebar.classList.contains("open");
        sidebar.classList.toggle("open", open);
        backdrop.classList.toggle("open", open);
        document.body.classList.toggle("menu-open", open);
    });
    document.getElementById("sidebarClose").addEventListener("click", closeMenu);
    backdrop.addEventListener("click", closeMenu);

    document.getElementById("adminSignOut").addEventListener("click", async () => {
        try { await API.adminLogout(); } catch (_) {}
        API.clearSession();
        window.location.replace("admin-login.html");
    });

    const adminName = document.getElementById("adminName");
    const adminEmail = document.getElementById("adminEmail");

    // Confirm with the server that this browser holds an administrator session before drawing
    // anything. A missing session returns to the sign-in page; a non-administrator account is
    // sent back with an explanation.
    async function start() {
        if (!API || typeof API.adminMe !== "function") {
            API && API.showFatalError && API.showFatalError("WeddingHub could not load its API client. Reload the page and try again.");
            return;
        }
        let account;
        try {
            account = await API.adminMe();
        } catch (error) {
            if (error && (error.status === 401 || error.status === 403)) {
                window.location.replace("admin-login.html" + (error.status === 403 ? "?denied=1" : ""));
                return;
            }
            API.showFatalError(error && error.message);
            return;
        }
        const name = account.display_name || account.email || "Administrator";
        adminName.textContent = name;
        adminEmail.textContent = account.email || "";
        document.getElementById("adminAvatar").textContent = initials(name);
        if (status) { status.textContent = "● Connected"; status.classList.add("online"); }
        document.title = "Platform Admin · WeddingHub";
        showSection("dashboard");
    }

    start();
})();
