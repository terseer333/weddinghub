(() => {
  "use strict";
  const WH = WeddingHub;
  const API = WeddingHubAPI;
  const $ = id => document.getElementById(id);
  const $$ = selector => document.querySelectorAll(selector);
  const token = WH.query("token");
  const adminPreview = WH.query("preview") === "admin";
  let data = WH.getData();
  let wedding = data.wedding;
  let member = WH.committeeMemberByToken(data) || null;
  const isAdmin = Boolean(API.adminToken());
  let weddingID = localStorage.getItem("weddinghub_api_wedding_id") || data.wedding.id || "";
  let guestStats = {};
  let chatSince = "";
  let chatTimer = null;
  let unreadCount = 0;

  function actor() {
    if (member) {
      const roleObj = (data.committeeRoles || []).find(r => r.id === (member.roleId || member.role_id));
      const roleTitle = roleObj?.name || member.title || member.committeeTitle || "Committee member";
      return { role: member.invitationStatus === "accepted" ? "committee_member" : "pending", name: member.name, title: roleTitle };
    }
    if (isAdmin) return { role: "admin", name: (() => { try { return JSON.parse(localStorage.getItem("weddinghub_user") || localStorage.getItem("weddinghub_local_profile"))?.fullName || "Wedding admin"; } catch (_) { return "Wedding admin"; } })(), title: "Wedding admin" };
    return null;
  }

  function computeGuestStats() {
    const guests = data.guests || [];
    return {
      invited: guests.length,
      accepted: guests.filter(g => g.rsvp === "attending").length,
      pending: guests.filter(g => g.rsvp === "pending").length,
      declined: guests.filter(g => g.rsvp === "declined").length,
      attending: guests.filter(g => g.rsvp === "attending").reduce((sum, g) => sum + Number(g.partySize || 1), 0)
    };
  }

  function initials(value) { return String(value || "C").split(/\s+/).map(p => p[0]).join("").slice(0, 2).toUpperCase(); }

  function showGate(message = "") {
    $("committeeExperience").style.display = "none";
    $("committeeAccessGate").style.display = "grid";
    $("committeeAccessGate").innerHTML = `<div><span class="brand-mark">W</span><p class="eyebrow">Private planning workspace</p><h1>Committee access required</h1><p>${message || "Only the wedding admin and invited committee members can open this workspace."}</p><a class="button primary" href="event.html?token=${encodeURIComponent(token || "")}">Return to invitation</a></div>`;
  }

  async function resolveWeddingID() {
    if (token && (!weddingID || weddingID === data.wedding?.id)) {
      try {
        const view = await API.invitation(token);
        if (view?.wedding?.id) weddingID = view.wedding.id;
      } catch (_) { /* offline fallback below */ }
    }
    if (!weddingID) weddingID = data.wedding?.id || "";
    return weddingID;
  }

  async function syncFromServer() {
    if (!API.isOnline() || !weddingID) return null;
    try {
      const view = await API.committeeDashboard(weddingID, token);
      data = API.mergeAPI(data, view);
      WH.saveData(data);
      guestStats = view.guest_stats || computeGuestStats();
      return view;
    } catch (_) { return null; }
  }

  async function initialize() {
    const person = actor();
    if (!person) return showGate();
    setupIdentity(person);
    const online = await API.connect();
    if (online) {
      $("committeeApiStatus").textContent = "● API connected";
      $("committeeApiStatus").classList.add("online");
      await resolveWeddingID();
      if (!weddingID) return showGate("We could not find this wedding. Reopen it from your invitation link.");
      try {
        const view = await API.committeeDashboard(weddingID, token);
        data = API.mergeAPI(data, view);
        WH.saveData(data);
        guestStats = view.guest_stats || computeGuestStats();
        sessionStorage.setItem("weddinghub_wedding_id", weddingID);
        WeddingRoleSelectorClear();
        startChatPolling();
      } catch (error) {
        console.warn("Committee authorization failed:", error);
        return showGate("Your committee invitation is not yet accepted, or this link does not belong to this wedding's committee.");
      }
    } else {
      if (member && member.invitationStatus !== "accepted" && !isAdmin) return showGate("Accept your committee invitation to open the planning workspace.");
      guestStats = computeGuestStats();
      $("committeeApiStatus").textContent = "● Offline mode";
    }
    renderAll();
  }

  function WeddingRoleSelectorClear() {
    if (typeof WeddingRoleSelector !== "undefined") WeddingRoleSelector.clearSelection();
  }

  function setupIdentity(person) {
    $("committeeUserName").textContent = person.name;
    $("committeeUserRole").textContent = person.title || (person.role === "admin" ? "Wedding admin" : "Planning team");
    $("committeeUserAvatar").textContent = initials(person.name);
    $("committeeNames").textContent = `${wedding.brideName} & ${wedding.groomName}`;
    $("committeeRole").textContent = person.role === "admin" ? "Admin · planning workspace" : "Committee member";
    $("committeeCoupleAvatar").textContent = `${initials(wedding.brideName)[0] || "B"}&${initials(wedding.groomName)[0] || "G"}`;
    $("committeeWelcome").textContent = `Welcome, ${person.name.split(" ")[0]}. Let's plan together.`;
  }

  function openView(name) {
    const target = $(`view-${name}`) ? name : "overview";
    $$(".committee-view").forEach(view => view.classList.toggle("active", view.id === `view-${target}`));
    $$(".committee-link[data-view]").forEach(link => link.classList.toggle("active", link.dataset.view === target));
    closeSidebar();
    history.replaceState(null, "", `#${target}`);
    window.scrollTo({ top: 0, behavior: "smooth" });
    if (name === "chat") markChatRead();
  }
  function openSidebar() {
    $("committeeSidebar").classList.add("open");
    $("committeeBackdrop").classList.add("open");
    document.body.classList.add("menu-open");
  }
  function closeSidebar() {
    $("committeeSidebar").classList.remove("open");
    $("committeeBackdrop").classList.remove("open");
    document.body.classList.remove("menu-open");
  }

  function renderAll() { renderOverview(); renderInformation(); renderEvents(); renderTasks(); renderAnnouncements(); renderPhotos(); renderChat(); }

  function renderOverview() {
    const stats = guestStats || computeGuestStats();
    $("committeeStats").innerHTML = [
      ["Invited", stats.invited ?? 0], ["Accepted", stats.accepted ?? 0], ["Pending", stats.pending ?? 0], ["Attending", stats.attending ?? 0]
    ].map(([label, value]) => `<article><span>${label === "Attending" ? "✓" : "♙"}</span><p>Guests ${label}<strong>${value}</strong></p></article>`).join("");
    WH.countdown(wedding.date, value => {
      $("committeeCountdown").innerHTML = value.passed ? "<strong>Today we celebrate love</strong>" :
        [["Days", value.days], ["Hours", value.hours], ["Minutes", value.minutes], ["Seconds", value.seconds]].map(([label, number]) => `<div><strong>${String(number).padStart(2, "0")}</strong><span>${label}</span></div>`).join("");
    });
    const next = WH.published(data.events).sort((a, b) => `${a.date}${a.time}`.localeCompare(`${b.date}${b.time}`))[0];
    $("committeeNextEvent").innerHTML = next ? `<span class="calendar-tile"><b>${new Date(`${next.date}T12:00`).getDate()}</b>${new Date(`${next.date}T12:00`).toLocaleString("en", { month: "short" }).toUpperCase()}</span><div><strong>${WH.escape(next.name)}</strong><small>${next.time} · ${WH.escape(next.venue)}</small></div>` : "<p class=\"empty-inline\">No published events yet.</p>";
    const open = data.planningTasks.filter(t => t.status !== "done");
    $("committeeTaskPreview").innerHTML = open.length ? open.slice(0, 3).map(t => `<div class="task-mini"><b class="status ${t.status}">${t.status.replace("_", " ")}</b><span><strong>${WH.escape(t.title)}</strong><small>${WH.escape(t.assignedTo || "Unassigned")}${t.dueOn ? ` · due ${WH.escape(t.dueOn)}` : ""}</small></span></div>`).join("") : "<p class=\"empty-inline\">No open tasks yet.</p>";
    const updates = WH.publicAnnouncements(data.announcements).slice(0, 3);
    $("committeeOverviewUpdates").innerHTML = updates.length ? updates.map(a => `<article class="manage-card"><span class="manage-icon">◉</span><div class="manage-copy"><h3>${WH.escape(a.title)}</h3><p>${WH.escape(a.message)}</p><small>${WH.formatDate(a.date)}</small></div></article>`).join("") : "<p class=\"empty-inline\">Publish an update and it appears here and for guests.</p>";
  }

  function renderInformation() {
    const w = wedding;
    const fields = [
      ["Bride & groom", `${w.brideName} & ${w.groomName}`],
      ["Wedding date", WH.formatDate(w.date)],
      ["Venue", w.venue],
      ["Location", [w.address, w.city, w.state, w.country].filter(Boolean).join(", ") || "—"],
      ["Guest message", w.message],
      ["Dress code", w.dressCode],
      ["Verse", w.verse]
    ];
    $("committeeInfo").innerHTML = fields.map(([label, value]) => `<div><small>${label}</small><strong>${WH.escape(value || "—")}</strong></div>`).join("");
  }

  function renderEvents() {
    const events = WH.published(data.events);
    $("committeeEventList").innerHTML = events.length ? events.map((event, index) => `<article class="manage-card"><span class="calendar-tile"><b>${new Date(`${event.date}T12:00`).getDate()}</b>${new Date(`${event.date}T12:00`).toLocaleString("en", { month: "short" }).toUpperCase()}</span><div class="manage-copy"><h3>${WH.escape(event.name)}</h3><p>${event.time} · ${WH.escape(event.venue)}</p><small>${WH.escape(event.address)}</small></div><span class="event-index-sm">0${index + 1}</span></article>`).join("") : "<div class='empty-state'><span>✦</span><h3>No events yet</h3><p>The admin will publish the schedule here.</p></div>";
  }

  function renderTasks() {
    const tasks = data.planningTasks || [];
    $("committeeTaskList").innerHTML = tasks.length ? tasks.map(task => `<article class="manage-card"><span class="calendar-tile task-tile">${["✓","◷","○"][["done","in_progress","todo"].indexOf(task.status) + 1] || "○"}</span><div class="manage-copy"><h3>${WH.escape(task.title)} <i class="status ${task.status}">${task.status.replace("_", " ")}</i></h3><p>${WH.escape(task.details || "")}</p><small>${task.assignedTo ? `Assigned to ${WH.escape(task.assignedTo)}` : "Unassigned"}${task.dueOn ? ` · Due ${WH.escape(task.dueOn)}` : ""}</small></div><div class="row-menu"><button data-task-action="edit" data-id="${task.id}">Edit</button><button data-task-action="delete" data-id="${task.id}">Delete</button></div></article>`).join("") : "<div class='empty-state'><span>✦</span><h3>No tasks yet</h3><p>Add the first planning task for the committee.</p></div>";
  }

  function renderAnnouncements() {
    const items = data.announcements || [];
    $("committeeAnnouncementList").innerHTML = items.length ? items.map(item => `<article class="manage-card"><span class="manage-icon">◉</span><div class="manage-copy"><h3>${WH.escape(item.title)} <i class="status ${item.status}">${item.status}</i> <b class="audience-badge ${item.audience === "committee" ? "committee" : "public"}">${item.audience === "committee" ? "Committee only" : "Public"}</b></h3><p>${WH.escape(item.message)}</p><small>${WH.formatDate(item.date)}${item.authorName ? ` · posted by ${WH.escape(item.authorName)}` : ""}</small></div><div class="row-menu"><button data-announcement-action="edit" data-id="${item.id}">Edit</button><button data-announcement-action="delete" data-id="${item.id}">Delete</button></div></article>`).join("") : "<div class='empty-state'><span>✦</span><h3>No updates yet</h3><p>Publish a note for guests or the committee.</p></div>";
  }

  function renderPhotos() {
    const photos = WH.published(data.photos);
    $("committeeGallery").innerHTML = photos.length ? `<div class="committee-gallery-stage">${photos.map((photo, index) => `<figure class="${index === 0 ? "active" : ""}"><img src="${photo.url}" alt="${WH.escape(photo.caption)}"><figcaption>${WH.escape(photo.caption)}</figcaption></figure>`).join("")}</div>` : "<div class='empty-state'><span>✦</span><h3>No photos yet</h3><p>The admin will add photos when they're ready.</p></div>";
    let index = 0;
    if (photos.length > 1) {
      clearInterval(window.committeeGalleryTimer);
      window.committeeGalleryTimer = setInterval(() => {
        const slides = document.querySelectorAll("#committeeGallery .committee-gallery-stage figure");
        if (!slides.length) return;
        slides[index].classList.remove("active");
        index = (index + 1) % photos.length;
        slides[index].classList.add("active");
      }, 4600);
    }
    const story = WH.published(data.stories).sort((a, b) => a.order - b.order);
    $("committeeStory").innerHTML = story.length ? story.map(s => `<div class="story-line"><small>${WH.escape(s.year)}</small><strong>${WH.escape(s.title)}</strong><p>${WH.escape(s.content)}</p></div>`).join("") : "<div class='empty-state'><span>✦</span><h3>No story yet</h3><p>The couple will share their story here.</p></div>";
  }

  function renderChat() {
    const messages = data.committeeChat || [];
    const stream = $("committeeChatStream");
    const last = messages[messages.length - 1];
    $("chatMemberCount").textContent = `${messages.length} ${messages.length === 1 ? "message" : "messages"}`;
    if (stream) stream.innerHTML = messages.length ? messages.map(message => `
      <div class="chat-message ${message.authorRole === "admin" ? "admin" : ""}">
        <span class="chat-avatar">${initials(message.authorName || "C")}</span>
        <div class="chat-bubble"><p><strong>${WH.escape(message.authorName || "Committee")}</strong>${message.authorRole === "admin" ? " <i>admin</i>" : ""}<time>${chatTime(message.createdAt)}</time></p><span>${WH.escape(message.body)}</span></div>
      </div>`).join("") : '<p class="empty-inline">No messages yet — start the planning conversation.</p>';
    requestAnimationFrame(() => { stream.scrollTop = stream.scrollHeight; });
    chatSince = maxChatTime();
  }

  function chatTime(value) { return value ? new Date(value).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : ""; }
  function maxChatTime() {
    const messages = data.committeeChat || [];
    let max = "";
    messages.forEach(m => { if (m.createdAt && (!max || m.createdAt > max)) max = m.createdAt; });
    return max;
  }
  function appendMessages(incoming) {
    if (!incoming?.length) return;
    let added = 0;
    const existing = new Set((data.committeeChat || []).map(m => m.id));
    incoming.forEach(message => {
      if (existing.has(message.id)) return;
      data.committeeChat = data.committeeChat || [];
      data.committeeChat.push(message);
      existing.add(message.id);
      added++;
    });
    if (added) {
      WH.saveData(data);
      const chatActive = document.querySelector(".committee-view.active")?.id === "view-chat";
      if (chatActive) renderChat(); else bumpUnread(added);
    }
  }
  function bumpUnread(count) {
    unreadCount += count;
    const link = document.querySelector('.committee-link[data-view="chat"]');
    if (link) link.dataset.badge = String(unreadCount);
  }
  function markChatRead() {
    unreadCount = 0;
    document.querySelector('.committee-link[data-view="chat"]')?.removeAttribute("data-badge");
  }
  function startChatPolling() {
    if (chatTimer) clearInterval(chatTimer);
    chatTimer = setInterval(async () => {
      if (!API.isOnline() || !weddingID) return;
      try { const incoming = await API.committeeChat(weddingID, chatSince || undefined, token); appendMessages(incoming); }
      catch (_) { /* transient */ }
    }, 5000);
  }

  function modalContent() { return $("committeeModalContent"); }
  function openModal(markup) { modalContent().innerHTML = markup; $("committeeModal").classList.add("open"); }
  function closeModal() { $("committeeModal").classList.remove("open"); }
  function field(value) { return WH.escape(value || ""); }

  function editTask(id) {
    const item = data.planningTasks.find(t => t.id === id) || { id: `task_${Date.now()}`, title: "", details: "", assignedTo: "", dueOn: "", status: "todo" };
    openModal(`<p class="eyebrow">Internal planning</p><h2>${id ? "Edit" : "Add"} task</h2><form id="committeeTaskForm" class="modal-form"><label>Title<input name="title" value="${field(item.title)}" required></label><label>Details<textarea name="details" rows="3">${field(item.details)}</textarea></label><div class="form-grid"><label>Assigned to<input name="assignedTo" value="${field(item.assignedTo)}"></label><label>Due date<input name="dueOn" type="date" value="${field(item.dueOn)}"></label></div><label>Status<select name="status">${["todo", "in_progress", "done"].map(s => `<option ${s === item.status ? "selected" : ""}>${s}</option>`).join("")}</select></label><button class="button primary" type="submit">Save task</button></form>`);
    $("committeeTaskForm").onsubmit = async event => {
      event.preventDefault();
      Object.assign(item, Object.fromEntries(new FormData(event.target)));
      if (!data.planningTasks.find(t => t.id === item.id)) data.planningTasks.push(item);
      const payload = { title: item.title, details: item.details, assigned_to: item.assignedTo, due_on: item.dueOn, status: item.status };
      try { if (API.isOnline() && weddingID) await API.updateTask(weddingID, item.id, payload, token); }
      catch (_) {}
      await persistLocal();
      closeModal();
    };
  }

  function deleteTask(id) {
    if (!confirm("Remove this planning task?")) return;
    data.planningTasks = data.planningTasks.filter(t => t.id !== id);
    if (API.isOnline() && weddingID) API.deleteTask(weddingID, id, token).catch(() => {});
    persistLocal();
  }

  function editAnnouncement(id) {
    const item = data.announcements.find(a => a.id === id) || { id: `ann_${Date.now()}`, title: "", message: "", date: new Date().toISOString().slice(0, 10), status: "draft", audience: "public" };
    openModal(`<p class="eyebrow">Wedding update</p><h2>${id ? "Edit" : "New"} announcement</h2><form id="committeeAnnouncementForm" class="modal-form"><label>Title<input name="title" value="${field(item.title)}" required></label><label>Message<textarea name="message" rows="5" required>${field(item.message)}</textarea></label><label>Audience<select name="audience"><option value="public" ${item.audience !== "committee" ? "selected" : ""}>Public — shown to guests</option><option value="committee" ${item.audience === "committee" ? "selected" : ""}>Committee only — hidden from guests</option></select></label><label>Visibility<select name="status"><option value="draft" ${item.status === "draft" ? "selected" : ""}>Draft</option><option value="published" ${item.status === "published" ? "selected" : ""}>Published</option><option value="hidden" ${item.status === "hidden" ? "selected" : ""}>Hidden</option></select></label><button class="button primary" type="submit">Save announcement</button></form>`);
    $("committeeAnnouncementForm").onsubmit = async event => {
      event.preventDefault();
      const form = Object.fromEntries(new FormData(event.target));
      Object.assign(item, form);
      if (!data.announcements.find(a => a.id === item.id)) { if (item.authorName === undefined) item.authorName = actor()?.name || ""; data.announcements.unshift(item); }
      const payload = { title: item.title, body: item.message, audience: item.audience, status: item.status };
      try {
        if (API.isOnline() && weddingID) {
          if (id) await API.updateAnnouncement(weddingID, item.id, payload, token);
          else { const created = await API.createAnnouncement(weddingID, payload, token); if (created) item.id = created.id; }
        }
      } catch (_) {}
      await persistLocal();
      closeModal();
    };
  }

  function deleteAnnouncement(id) {
    if (!confirm("Delete this announcement?")) return;
    data.announcements = data.announcements.filter(a => a.id !== id);
    if (API.isOnline() && weddingID) API.deleteAnnouncement(weddingID, id, token).catch(() => {});
    persistLocal();
  }

  async function persistLocal() {
    WH.saveData(data);
    renderAll();
    await syncFromServer();
    renderTasks();
    renderAnnouncements();
  }

  // Event wiring
  $$(".committee-link[data-view]").forEach(link => link.onclick = () => openView(link.dataset.view));
  $$("[data-jump]").forEach(button => button.onclick = () => openView(button.dataset.jump));
  $("committeeMenuButton").onclick = openSidebar;
  $("committeeSidebarClose").onclick = closeSidebar;
  $("committeeBackdrop").onclick = closeSidebar;
  document.addEventListener("keydown", event => { if (event.key === "Escape") closeSidebar(); });
  $("openChatFromOverview").onclick = () => openView("chat");
  $("addCommitteeTask").onclick = () => editTask();
  $("addCommitteeAnnouncement").onclick = () => editAnnouncement();
  $("committeeModalClose").onclick = closeModal;
  $("committeeModal").onclick = event => { if (event.target === $("committeeModal")) closeModal(); };
  $("committeeSignOut").onclick = () => { sessionStorage.removeItem("weddinghub_selected_role"); sessionStorage.removeItem("weddinghub_invitation_token"); location.href = `event.html?token=${encodeURIComponent(token || "")}`; };

  document.addEventListener("click", async event => {
    const taskButton = event.target.closest("[data-task-action]");
    if (taskButton) {
      if (taskButton.dataset.taskAction === "edit") return editTask(taskButton.dataset.id);
      if (taskButton.dataset.taskAction === "delete") return deleteTask(taskButton.dataset.id);
    }
    const announcementButton = event.target.closest("[data-announcement-action]");
    if (announcementButton) {
      if (announcementButton.dataset.announcementAction === "edit") return editAnnouncement(announcementButton.dataset.id);
      if (announcementButton.dataset.announcementAction === "delete") return deleteAnnouncement(announcementButton.dataset.id);
    }
  });

  $("committeeChatForm").onsubmit = async event => {
    event.preventDefault();
    const input = $("committeeChatInput");
    const body = input.value.trim();
    if (!body) return;
    input.value = "";
    const message = { id: `cc_${Date.now()}`, authorName: actor()?.name || "Committee", authorRole: actor()?.role || "committee_member", body, createdAt: new Date().toISOString() };
    data.committeeChat = data.committeeChat || [];
    data.committeeChat.push(message);
    WH.saveData(data);
    renderChat();
    if (API.isOnline() && weddingID) {
      try { await API.sendCommitteeMessage(weddingID, body, token); }
      catch (_) { WH.toast("Message kept locally — API unavailable"); }
    }
  };

  initialize();
  if (location.hash) openView(location.hash.slice(1));
})();