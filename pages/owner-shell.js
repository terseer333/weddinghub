(() => {
  const nav = [
    { group: "Overview", items: [["dashboard", "Dashboard", "grid", "dashboard.html"]] },
    { group: "Invitations", items: [["wedding-info", "Wedding information", "heart", "wedding-info.html"], ["card-studio", "Card Studio & Templates", "card", "card-studio.html"], ["preview-card", "Preview Card", "eye", "preview-card.html"]] },
    { group: "Guests", items: [["guests", "Guests & RSVP", "users", "guests.html"], ["messages", "Guest messages", "message", "messages.html"]] },
    { group: "Planning", items: [["events", "Events", "calendar", "events.html"], ["committee", "Planning committee", "committee", "committee.html"]] },
    { group: "Content", items: [["story", "Photos & story", "image", "story.html"], ["announcements", "Announcements", "megaphone", "announcements.html"]] }
  ];
  const symbols = `<svg class="owner-symbols" aria-hidden="true"><symbol id="i-grid" viewBox="0 0 24 24"><rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/></symbol><symbol id="i-heart" viewBox="0 0 24 24"><path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8l1.1 1.1L12 21l7.8-7.5 1.1-1.1a5.5 5.5 0 0 0-.1-7.8Z"/></symbol><symbol id="i-card" viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3 10h18M7 15h4"/></symbol><symbol id="i-calendar" viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="16" rx="2"/><path d="M16 3v4M8 3v4M3 10h18"/></symbol><symbol id="i-users" viewBox="0 0 24 24"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8ZM22 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8"/></symbol><symbol id="i-committee" viewBox="0 0 24 24"><circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0M5 8H2M22 8h-3"/></symbol><symbol id="i-image" viewBox="0 0 24 24"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="m21 15-5-5L5 21"/></symbol><symbol id="i-megaphone" viewBox="0 0 24 24"><path d="m3 11 18-5v12L3 14v-3ZM11.6 16.1 13 21H7l-1-6"/></symbol><symbol id="i-message" viewBox="0 0 24 24"><path d="M21 15a4 4 0 0 1-4 4H8l-5 3V7a4 4 0 0 1 4-4h10a4 4 0 0 1 4 4v8Z"/></symbol><symbol id="i-eye" viewBox="0 0 24 24"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/></symbol><symbol id="i-search" viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/></symbol><symbol id="i-bell" viewBox="0 0 24 24"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM14 21h-4"/></symbol><symbol id="i-moon" viewBox="0 0 24 24"><path d="M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8Z"/></symbol><symbol id="i-sun" viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></symbol><symbol id="i-menu" viewBox="0 0 24 24"><path d="M4 7h16M4 12h16M4 17h16"/></symbol><symbol id="i-close" viewBox="0 0 24 24"><path d="m6 6 12 12M18 6 6 18"/></symbol><symbol id="i-chevron" viewBox="0 0 24 24"><path d="m9 18 6-6-6-6"/></symbol><symbol id="i-arrow" viewBox="0 0 24 24"><path d="M5 12h14M13 6l6 6-6 6"/></symbol><symbol id="i-check" viewBox="0 0 24 24"><path d="m5 12 4 4L19 6"/></symbol><symbol id="i-clock" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></symbol><symbol id="i-map" viewBox="0 0 24 24"><path d="M20 10c0 5-8 11-8 11S4 15 4 10a8 8 0 1 1 16 0Z"/><circle cx="12" cy="10" r="2"/></symbol><symbol id="i-plus" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14"/></symbol></svg>`;
  const page = document.body.dataset.page || "dashboard";
  const itemMarkup = nav.map(group => `<div class="nav-group"><div class="nav-label">${group.group}</div>${group.items.map(([key, title, icon, href]) => `<a class="nav-item${key === page ? " active" : ""}" ${key === page ? 'aria-current="page"' : ""} href="${href}"><svg class="icon" width="17" height="17" viewBox="0 0 24 24" aria-hidden="true"><use href="#i-${icon}"/></svg><span>${title}</span>${key === "messages" ? '<b id="unread-badge" hidden></b>' : ""}</a>`).join("")}</div>`).join("");
  const app = document.createElement("div");
  app.className = "app";
  app.innerHTML = `${symbols}<button class="sidebar-scrim" id="sidebar-scrim" aria-label="Close menu" hidden></button><aside class="sidebar" id="sidebar"><div class="brand-row"><div class="brand-mark">W</div><div class="brand-name">WeddingHub</div><button class="icon-button sidebar-close" id="sidebar-close" aria-label="Close navigation"><svg class="icon" width="18" height="18" viewBox="0 0 24 24"><use href="#i-close"/></svg></button></div><div class="couple-card"><div class="couple-monogram" id="couple-monogram">W</div><div><strong id="couple-names">Your wedding</strong><small id="couple-date">Date to be set</small></div></div><nav class="nav" aria-label="Main">${itemMarkup}</nav><a class="sidebar-help" href="help.html" style="text-decoration:none;color:inherit"><div class="help-icon">♡</div><div><strong>Need a hand?</strong><small>Visit the help centre</small></div><svg class="icon" width="15" height="15" viewBox="0 0 24 24"><use href="#i-chevron"/></svg></a></aside><main class="main"><header class="topbar"><button class="icon-button mobile-menu" id="sidebar-open" aria-label="Open navigation"><svg class="icon" width="18" height="18" viewBox="0 0 24 24"><use href="#i-menu"/></svg></button><div class="mobile-brand"><div class="brand-mark">W</div><span>WeddingHub</span></div><label class="search"><svg class="icon" width="17" height="17" viewBox="0 0 24 24"><use href="#i-search"/></svg><input id="search-input" aria-label="Search this page" placeholder="Search this page…"><kbd>⌘ K</kbd></label><div class="top-actions"><button class="icon-button theme-toggle" id="theme-toggle" aria-label="Use dark mode"><svg class="icon i-moon" width="18" height="18" viewBox="0 0 24 24"><use href="#i-moon"/></svg><svg class="icon i-sun" width="18" height="18" viewBox="0 0 24 24"><use href="#i-sun"/></svg></button><button class="icon-button notification" aria-label="Notifications"><svg class="icon" width="18" height="18" viewBox="0 0 24 24"><use href="#i-bell"/></svg></button><div class="profile" id="profile-trigger" tabindex="0" role="button" aria-haspopup="true"><div class="avatar rose" id="user-initials">WH</div><div><strong id="user-name">Wedding owner</strong><small id="user-role">Wedding owner</small></div><svg class="icon" width="14" height="14" viewBox="0 0 24 24"><use href="#i-chevron"/></svg></div><div class="profile-menu" id="profile-menu" hidden><button id="logout-button" type="button">Log out</button></div></div></header><div class="content" id="owner-content"></div></main>`;
  const mount = document.getElementById("app-shell");
  const template = document.getElementById("page-template");
  if (!mount) return;
  mount.replaceWith(app);
  if (template) app.querySelector("#owner-content").append(template.content.cloneNode(true));
  const login = () => location.replace("login.html");
  const api = window.WeddingHubAPI;
  const esc = value => String(value ?? "").replace(/[&<>"']/g, char => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);
  const toast = (message, type = "success") => {
    let node = document.getElementById("owner-toast");
    if (!node) { node = document.createElement("div"); node.id = "owner-toast"; node.className = "owner-toast"; node.setAttribute("role", "status"); document.body.append(node); }
    node.textContent = message; node.dataset.type = type; node.classList.add("is-visible");
    clearTimeout(node.timer); node.timer = setTimeout(() => node.classList.remove("is-visible"), 3200);
  };
  const loading = (loadingID = "page-loading") => { const state = document.getElementById(loadingID); if (state) { state.hidden = false; state.innerHTML = '<span class="loading-dots">Loading your wedding details…</span>'; } };
  const fail = (error, id = "page-error") => { const state = document.getElementById(id); if (state) { state.hidden = false; state.innerHTML = `<div class="empty-state"><div><strong>We could not load this page.</strong><p>${esc(error?.message || "Please try again.")}</p><button class="button button-secondary" type="button" data-retry>Try again</button></div></div>`; state.querySelector("[data-retry]")?.addEventListener("click", () => location.reload()); } };
  const loadWorkspace = async () => {
    try { return await api.ownerWorkspace(); }
    catch (error) { if (error.status === 401) login(); throw error; }
  };
  const saveWorkspace = async data => {
    try { return await api.saveOwnerWorkspace(data); }
    catch (error) { if (error.status === 401) login(); throw error; }
  };
  window.WeddingHubOwner = { api, esc, toast, loading, fail, loadWorkspace, saveWorkspace };
  const closeSidebar = () => { app.querySelector("#sidebar").classList.remove("is-open"); app.querySelector("#sidebar-scrim").hidden = true; document.body.classList.remove("menu-open"); };
  app.querySelector("#sidebar-open").addEventListener("click", () => { app.querySelector("#sidebar").classList.add("is-open"); app.querySelector("#sidebar-scrim").hidden = false; document.body.classList.add("menu-open"); });
  app.querySelector("#sidebar-close").addEventListener("click", closeSidebar);
  app.querySelector("#sidebar-scrim").addEventListener("click", closeSidebar);
  document.addEventListener("keydown", event => { if (event.key === "Escape") closeSidebar(); });
  app.querySelector("#theme-toggle").addEventListener("click", () => {
    const theme = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = theme;
    try { localStorage.setItem("weddinghub-theme", theme); } catch (_) {}
  });
  app.querySelector("#profile-trigger").addEventListener("click", () => { const menu = app.querySelector("#profile-menu"); menu.hidden = !menu.hidden; });
  app.querySelector("#logout-button").addEventListener("click", async () => { try { await api.logout(); } finally { login(); } });
  const search = app.querySelector("#search-input");
  search.addEventListener("input", () => {
    const q = search.value.trim().toLocaleLowerCase();
    app.querySelectorAll("[data-searchable]").forEach(row => { row.hidden = q && !row.textContent.toLocaleLowerCase().includes(q); });
  });
  (async () => {
    const token = api.sessionToken();
    if (!token) { login(); return; }
    const user = await api.currentUser();
    if (!user) { login(); return; }
    app.querySelector("#user-name").textContent = user.display_name || user.email;
    app.querySelector("#user-role").textContent = "Wedding owner";
    app.querySelector("#user-initials").textContent = (user.display_name || user.email).split(/\s+/).map(part => part[0]).join("").slice(0, 2).toUpperCase();
    try {
      const dashboard = await fetch(`${api.baseURL()}/api/dashboard`, { headers: { Authorization: `Bearer ${token}` } });
      if (dashboard.status === 401) { login(); return; }
      if (dashboard.ok) {
        const summary = await dashboard.json();
        const names = summary.couple?.names || "Your wedding";
        app.querySelector("#couple-names").textContent = names;
        app.querySelector("#couple-date").textContent = summary.couple?.date || "Date to be set";
        app.querySelector("#couple-monogram").textContent = names.split(/\s+/).filter(word => word !== "&").map(part => part[0]).join("").slice(0, 2).toUpperCase();
        const badge = app.querySelector("#unread-badge");
        if (summary.unreadMessages > 0 && badge) { badge.hidden = false; badge.textContent = summary.unreadMessages > 99 ? "99+" : summary.unreadMessages; }
      }
    } catch (_) { /* The page-level loader shows feature request failures. */ }
    try {
      const workspace = await api.ownerWorkspace();
      const local = window.WeddingHub.getData() || {};
      window.WeddingHub.saveData(api.mergeAPI(local, workspace));
    } catch (error) { if (error.status === 401) login(); }
  })();
})();
