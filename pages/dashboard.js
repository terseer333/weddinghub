(() => {
  const API_URL = "/api/dashboard";
  const SAMPLE_DATA = {
    user: { name: "Wedding owner", role: "owner" },
    couple: { names: "Your wedding", date: "" }, unreadMessages: 0,
    guests: { total: 0, households: 0, attending: 0, pending: 0, declined: 0, recentResponses: 0 },
    views: { total: 0, thisWeek: 0 }, setup: { percent: 0, done: [], remaining: 5 },
    committee: { count: 0, tasksDone: 0, tasksTotal: 0, tasksDueToday: 0, members: [], nextTask: null },
    activity: [], events: [], announcements: [], shareUrl: "", storyImage: ""
  };
  const $ = id => document.getElementById(id);
  const esc = value => String(value ?? "").replace(/[&<>"']/g, char => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);
  const token = (() => { try { return localStorage.getItem("weddinghub_session_token") || ""; } catch (_) { return ""; } })();
  const first = value => String(value || "there").trim().split(/\s+/)[0];
  const initials = value => String(value || "WH").trim().split(/\s+/).map(part => part[0]).join("").slice(0, 2).toUpperCase();
  const shortDate = value => {
    if (!value) return "Date to be set";
    const date = new Date(`${value}T12:00:00`);
    return Number.isNaN(date.valueOf()) ? "Date to be set" : date.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
  };
  const dataFallback = () => SAMPLE_DATA;

  function render(data) {
    const names = data.couple?.names || "Your wedding";
    const user = data.user?.name || "Wedding owner";
    $("couple-names").textContent = names;
    $("couple-date").textContent = shortDate(data.couple?.date);
    $("couple-monogram").textContent = initials(names.replace("&", " "));
    $("user-name").textContent = user;
    $("user-role").textContent = data.user?.role === "owner" ? "Wedding owner" : (data.user?.role || "Wedding owner");
    $("user-first-name").textContent = first(user);
    $("user-initials").textContent = initials(user);
    $("today-date").textContent = new Date().toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" });
    $("welcome-sub").textContent = `Here’s what’s happening with ${names}’s wedding.`;

    const guests = data.guests || SAMPLE_DATA.guests;
    $("stat-guests").textContent = guests.total || 0;
    $("stat-guests-note").textContent = `${guests.households || 0} ${guests.households === 1 ? "household" : "households"}`;
    $("stat-attending").textContent = guests.attending || 0;
    $("stat-attending-note").textContent = `${guests.recentResponses || 0} recent responses`;
    $("stat-pending").textContent = guests.pending || 0;
    $("stat-views").textContent = data.views?.total || 0;
    $("stat-views-note").textContent = `${data.views?.thisWeek || 0} this week`;
    $("stat-committee").textContent = data.committee?.count || 0;
    $("stat-committee-note").textContent = `${data.committee?.tasksDueToday || 0} tasks due today`;

    const date = data.couple?.date ? new Date(`${data.couple.date}T00:00:00`) : null;
    const days = date && !Number.isNaN(date.valueOf()) ? Math.max(0, Math.ceil((date - new Date()) / 86400000)) : 0;
    $("stat-days").textContent = days;
    $("days-word").textContent = days === 1 ? "day" : "days";
    $("stat-days-note").textContent = date ? (days ? `Your day is ${shortDate(data.couple.date)}` : "Your celebration is here") : "Set your wedding date";

    const setup = data.setup || SAMPLE_DATA.setup;
    $("setup-percent").textContent = `${setup.percent || 0}%`;
    $("setup-ring").style.setProperty("--pct", setup.percent || 0);
    $("setup-title").textContent = setup.remaining ? "You’re almost ready to share" : "Your wedding space is ready";
    $("setup-text").textContent = setup.remaining ? `${setup.remaining} setup ${setup.remaining === 1 ? "step" : "steps"} left to complete.` : "Your wedding details are ready to share with guests.";
    $("setup-track").firstElementChild.style.width = `${setup.percent || 0}%`;
    $("setup-steps").innerHTML = (setup.done || []).map(item => `<span><svg class="icon" width="14" height="14" viewBox="0 0 24 24"><use href="#i-check"/></svg>${esc(item)}</span>`).join("");

    const total = guests.total || 0;
    const attending = guests.attending || 0;
    const pending = guests.pending || 0;
    const declined = guests.declined || 0;
    const percentages = total ? [attending, pending, declined].map(value => value / total * 100) : [0, 0, 0];
    $("donut").style.setProperty("--att", percentages[0]);
    $("donut").style.setProperty("--pen", percentages[1]);
    $("donut").classList.toggle("empty", total === 0);
    $("donut-total").textContent = total;
    [["lg-att", attending, percentages[0]], ["lg-pen", pending, percentages[1]], ["lg-dec", declined, percentages[2]]].forEach(([id, count, pct]) => {
      $(`${id}-n`).textContent = `${count} ${count === 1 ? "guest" : "guests"}`;
      $(`${id}-p`).textContent = `${Math.round(pct)}%`;
    });
    $("recent-responses").textContent = `${guests.recentResponses || 0} ${guests.recentResponses === 1 ? "response" : "responses"}`;
    $("empty-guests").hidden = total > 0;

    $("activity-list").innerHTML = (data.activity || []).length ? data.activity.map(item => `<article class="activity-row"><span class="avatar ${esc(item.color)}">${esc(item.initials)}</span><p><strong>${esc(item.name)}</strong><span>${esc(item.text)}</span></p><time>${esc(item.time)}</time></article>`).join("") : '<p class="empty-inline">Guest responses and messages will appear here.</p>';
    $("events-list").innerHTML = (data.events || []).length ? data.events.map(item => `<article class="event"><span class="event-date"><strong>${esc(item.day)}</strong><span>${esc(item.month)}</span></span><div><strong>${esc(item.title)}</strong><p>${esc(item.time)} · ${esc(item.place)}</p></div></article>`).join("") : '<p class="empty-inline">Add a published event to build your schedule.</p>';
    const committee = data.committee || SAMPLE_DATA.committee;
    $("member-stack").innerHTML = (committee.members || []).slice(0, 5).map(member => `<span class="avatar ${esc(member.color)}">${esc(member.initials)}</span>`).join("");
    $("committee-count").textContent = `${committee.count || 0} ${committee.count === 1 ? "person" : "people"}`;
    $("tasks-text").textContent = `${committee.tasksDone || 0} of ${committee.tasksTotal || 0}`;
    $("tasks-track").firstElementChild.style.width = committee.tasksTotal ? `${committee.tasksDone / committee.tasksTotal * 100}%` : "0%";
    if (committee.nextTask) {
      $("next-task").hidden = false;
      $("next-task-title").textContent = committee.nextTask.title || "";
      $("next-task-meta").textContent = committee.nextTask.meta || "Next task";
    } else $("next-task").hidden = true;
    $("announcement-list").innerHTML = (data.announcements || []).length ? data.announcements.map(item => `<article class="announcement"><span class="announcement-icon"><svg class="icon" width="16" height="16" viewBox="0 0 24 24"><use href="#i-megaphone"/></svg></span><p><strong>${esc(item.title)}</strong><span>${esc(item.meta)}</span></p><time>${esc(item.time)}</time></article>`).join("") : '<p class="empty-inline">Published announcements will appear here.</p>';
    $("story-img").src = data.storyImage || "/assets/wedding-bg.jpg";
    $("share-btn").disabled = !data.shareUrl;
    $("share-btn").onclick = async () => {
      if (!data.shareUrl) return;
      try { await navigator.clipboard.writeText(new URL(data.shareUrl, location.origin).href); $("share-btn").setAttribute("aria-label", "Invitation link copied"); }
      catch (_) { window.prompt("Copy your invitation link", new URL(data.shareUrl, location.origin).href); }
    };
  }

  function wireNavigation() {
    const sidebar = $("sidebar"), scrim = $("sidebar-scrim");
    const close = () => { sidebar.classList.remove("is-open"); scrim.hidden = true; document.body.classList.remove("menu-open"); };
    $("sidebar-open").addEventListener("click", () => { sidebar.classList.add("is-open"); scrim.hidden = false; document.body.classList.add("menu-open"); });
    $("sidebar-close").addEventListener("click", close);
    scrim.addEventListener("click", close);
    document.addEventListener("keydown", event => { if (event.key === "Escape") close(); });
    $("theme-toggle").addEventListener("click", () => {
      const next = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
      document.documentElement.dataset.theme = next;
      try { localStorage.setItem("weddinghub-theme", next); } catch (_) {}
    });
    const search = $("search-input");
    search.addEventListener("keydown", event => { if (event.key === "Escape") search.blur(); });
  }

  async function load() {
    wireNavigation();
    if (!token) { location.replace("login.html"); return; }
    try {
      const response = await fetch(API_URL, { headers: { Authorization: `Bearer ${token}` }, credentials: "same-origin" });
      if (response.status === 401) { location.replace("login.html"); return; }
      if (!response.ok) throw new Error(`Dashboard request failed: ${response.status}`);
      render(await response.json());
    } catch (error) {
      console.error(error);
      render(dataFallback());
    }
  }
  load();
})();
