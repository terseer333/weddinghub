/**
 * First-time guided tour of the WeddingHub admin dashboard.
 *
 * The tour runs once per browser after the owner's first sign-in, walking through every
 * feature by highlighting its sidebar entry and switching the dashboard to that section.
 * It is skippable and can be replayed from Settings → Getting started.
 */
(() => {
  "use strict";

  const DONE_KEY = "weddinghub_tour_done";
  const PENDING_KEY = "weddinghub_tour_pending";
  const MOBILE_BREAKPOINT = 1000;

  const STEPS = [
    {
      title: "Welcome to WeddingHub",
      body: "This is your wedding command centre. Let's walk through everything you can do here — tap Next to move on, or skip straight to exploring."
    },
    {
      target: '[data-view="overview"]',
      view: "overview",
      title: "Dashboard",
      body: "Your daily snapshot: total guests, who is attending, replies still pending, invitation views and committee size, plus a live countdown and recent activity."
    },
    {
      target: '[data-view="information"]',
      view: "information",
      title: "Wedding information",
      body: "The single source of truth. Set the couple, date, venues, invitation message, verse and dress code — changes here update the invitation and every guest dashboard."
    },
    {
      target: '[data-view="designs"]',
      view: "designs",
      title: "Card Studio & Templates",
      body: "Browse 107+ luxury stationery templates in a three-panel studio, then tune fonts, colours and decorations with a live preview."
    },
    {
      target: '[data-view="events"]',
      view: "events",
      title: "Events",
      body: "Build the order of celebration — ceremony, reception, after-party. Only events you publish appear for guests."
    },
    {
      target: '[data-view="guests"]',
      view: "guests",
      title: "Guests & RSVP",
      body: "Add guests, personalise each invitation link, send it by email or WhatsApp, and track who has accepted, declined or not yet replied."
    },
    {
      target: '[data-view="committee"]',
      view: "committee",
      title: "Planning committee",
      body: "Invite the people helping you plan. Members get their own private workspace with shared tasks, committee chat and assigned roles."
    },
    {
      target: '[data-view="content"]',
      view: "content",
      title: "Photos & story",
      body: "Publish a photo slideshow and a chapter-by-chapter love story that accepted guests see on their dashboard."
    },
    {
      target: '[data-view="announcements"]',
      view: "announcements",
      title: "Announcements",
      body: "Post timings, hotel notes or any update. Published announcements appear immediately in guest dashboards."
    },
    {
      target: '[data-view="messages"]',
      view: "messages",
      title: "Guest messages",
      body: "Read the wishes and blessings guests leave for you, and approve or hide them from the guestbook."
    },
    {
      target: ".preview-link",
      title: "Preview Card",
      body: "Open the invitation exactly as a guest receives it, whenever you want, without leaving your workspace."
    },
    {
      target: '[data-view="settings"]',
      view: "settings",
      title: "Settings",
      body: "Connect the workspace to your API, reset local data, sign out — and replay this tour any time from Getting started."
    }
  ];

  const dashboard = document.querySelector(".admin-layout");
  if (!dashboard || !document.getElementById("view-overview")) return;

  let overlay = null;
  let spotlight = null;
  let popover = null;
  let index = 0;
  let active = false;
  let target = null;
  const saved = { collapsed: false, sidebarOpen: false };

  function sidebar() { return document.getElementById("sidebar"); }

  function setCollapsed(collapsed) {
    dashboard.classList.toggle("sidebar-collapsed", collapsed);
    const toggle = document.getElementById("desktopMenuToggle");
    if (toggle) toggle.setAttribute("aria-expanded", String(!collapsed));
  }

  function setSidebarOpen(open) {
    const aside = sidebar();
    const backdrop = document.getElementById("sidebarBackdrop");
    if (aside) aside.classList.toggle("open", open);
    if (backdrop) backdrop.classList.toggle("open", open);
    document.body.classList.toggle("menu-open", open);
  }

  // The tour highlights sidebar entries, so the sidebar has to be on screen even on the
  // off-canvas layout. Switching sections closes it again, hence the call on every step.
  function revealSidebar() {
    setCollapsed(false);
    if (window.innerWidth <= MOBILE_BREAKPOINT) setSidebarOpen(true);
  }

  function visible(node) {
    const rect = node.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0;
  }

  function clamp(value, min, max) {
    return Math.min(Math.max(value, min), max);
  }

  function position() {
    if (!active || !popover) return;

    if (!target || !target.isConnected || !visible(target)) {
      const size = popover.getBoundingClientRect();
      popover.style.left = `${Math.max(12, (window.innerWidth - size.width) / 2)}px`;
      popover.style.top = `${Math.max(12, (window.innerHeight - size.height) / 2)}px`;
      spotlight.hidden = true;
      overlay.classList.add("tour-dim");
      return;
    }

    spotlight.hidden = false;
    overlay.classList.remove("tour-dim");
    target.scrollIntoView({ block: "nearest", inline: "nearest" });

    const rect = target.getBoundingClientRect();
    const ring = 6;
    spotlight.style.left = `${rect.left - ring}px`;
    spotlight.style.top = `${rect.top - ring}px`;
    spotlight.style.width = `${rect.width + ring * 2}px`;
    spotlight.style.height = `${rect.height + ring * 2}px`;

    const size = popover.getBoundingClientRect();
    const gap = 14;
    const maxLeft = window.innerWidth - size.width - 12;
    let left;
    let top;

    if (rect.right + gap + size.width <= window.innerWidth - 12) {
      left = rect.right + gap;
      top = clamp(rect.top, 12, window.innerHeight - size.height - 12);
    } else if (rect.bottom + gap + size.height <= window.innerHeight - 12) {
      left = clamp(rect.left, 12, maxLeft);
      top = rect.bottom + gap;
    } else {
      left = clamp(rect.left, 12, maxLeft);
      top = Math.max(12, rect.top - gap - size.height);
    }

    popover.style.left = `${Math.max(12, left)}px`;
    popover.style.top = `${clamp(top, 12, Math.max(12, window.innerHeight - size.height - 12))}px`;
  }

  function step() {
    const current = STEPS[index];
    if (current.view && window.AdminApp) window.AdminApp.openView(current.view);
    revealSidebar();
    target = current.target ? document.querySelector(current.target) : null;

    popover.innerHTML = `
      <p class="tour-step">Step ${index + 1} of ${STEPS.length}</p>
      <h2></h2>
      <div class="tour-dots">${STEPS.map((_, i) => `<i${i === index ? ' class="on"' : ""}></i>`).join("")}</div>
      <p class="tour-body"></p>
      <div class="tour-actions">
        <button type="button" class="tour-skip">Skip tour</button>
        <button type="button" class="tour-back"${index === 0 ? " hidden" : ""}>Back</button>
        <button type="button" class="tour-next">${index === STEPS.length - 1 ? "Finish" : "Next"}</button>
      </div>`;
    popover.querySelector("h2").textContent = current.title;
    popover.querySelector(".tour-body").textContent = current.body;
    popover.querySelector(".tour-skip").onclick = finish;
    popover.querySelector(".tour-back").onclick = back;
    popover.querySelector(".tour-next").onclick = next;

    position();
  }

  function next() {
    if (index >= STEPS.length - 1) { finish(); return; }
    index += 1;
    step();
  }

  function back() {
    if (index === 0) return;
    index -= 1;
    step();
  }

  function onKeyDown(event) {
    if (event.key === "Escape") { event.preventDefault(); finish(); }
    else if (event.key === "ArrowRight") { event.preventDefault(); next(); }
    else if (event.key === "ArrowLeft") { event.preventDefault(); back(); }
  }

  function finish() {
    if (!active) return;
    active = false;
    index = 0;
    target = null;

    if (overlay) overlay.remove();
    if (spotlight) spotlight.remove();
    if (popover) popover.remove();
    overlay = spotlight = popover = null;

    document.removeEventListener("keydown", onKeyDown, true);
    window.removeEventListener("resize", position);
    window.removeEventListener("scroll", position, true);

    setCollapsed(saved.collapsed);
    setSidebarOpen(saved.sidebarOpen);

    try {
      localStorage.setItem(DONE_KEY, "1");
      localStorage.removeItem(PENDING_KEY);
    } catch (_) {}
  }

  function start() {
    if (active) return;

    const aside = sidebar();
    saved.collapsed = dashboard.classList.contains("sidebar-collapsed");
    saved.sidebarOpen = Boolean(aside && aside.classList.contains("open"));
    active = true;
    index = 0;

    overlay = document.createElement("div");
    overlay.className = "tour-overlay";
    spotlight = document.createElement("div");
    spotlight.className = "tour-spotlight";
    spotlight.hidden = true;
    popover = document.createElement("div");
    popover.className = "tour-popover";
    popover.setAttribute("role", "dialog");
    popover.setAttribute("aria-modal", "true");
    popover.setAttribute("aria-label", "WeddingHub guided tour");

    document.body.appendChild(overlay);
    document.body.appendChild(spotlight);
    document.body.appendChild(popover);

    document.addEventListener("keydown", onKeyDown, true);
    window.addEventListener("resize", position);
    window.addEventListener("scroll", position, true);

    step();
  }

  const replay = document.getElementById("startTour");
  if (replay) replay.addEventListener("click", () => { finish(); start(); });

  // Only a signed-in owner who has never finished the tour sees it, and only after the
  // dashboard has rendered so the sections it highlights actually exist.
  try {
    const signedIn = Boolean(localStorage.getItem("weddinghub_session_token"));
    if (signedIn && localStorage.getItem(PENDING_KEY) === "1" && localStorage.getItem(DONE_KEY) !== "1") {
      window.setTimeout(start, 700);
    }
  } catch (_) {}
})();
