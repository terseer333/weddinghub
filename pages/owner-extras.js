(() => {
  const owner = window.WeddingHubOwner;
  if (!owner) return;
  const page = document.body.dataset.page;
  const root = document.getElementById("owner-content");
  const esc = owner.esc;
  const intro = (eyebrow, title, text, action = "") => `<header class="page-intro"><div><span class="owner-eyebrow">${eyebrow}</span><h1>${title}</h1><p>${text}</p></div>${action}</header>`;
  const state = '<div class="owner-state" id="extra-state">Loading your wedding details…</div><section class="owner-card" id="extra-main" hidden></section>';
  const ready = () => { document.getElementById("extra-state").hidden = true; document.getElementById("extra-main").hidden = false; };
  const failed = error => { const box = document.getElementById("extra-state"); box.textContent = error.message || "Could not load this page."; box.classList.add("error"); };

  async function messages() {
    root.innerHTML = `${intro("Guests", "Guest messages", "Read notes sent by guests with their RSVP.")} ${state}`;
    try {
      const data = await owner.loadWorkspace(); const box = document.getElementById("extra-main"); ready();
      const messages = data.guest_messages || [];
      box.innerHTML = `<div class="owner-card-head"><div><h2>Inbox</h2><p>${messages.filter(m => !m.read).length} unread · ${messages.length} total</p></div></div><div class="owner-list">${messages.length ? messages.map(m => { const guest = (data.invitations || []).find(i => i.id === m.invitation_id); return `<article class="owner-row" data-searchable><div class="owner-row-main"><strong>${esc(guest?.guest_name || "Guest")}</strong><p>${esc(m.body)}</p><small>${esc(m.created_at ? new Date(m.created_at).toLocaleString() : "")}</small></div><span class="owner-badge ${m.read ? "accepted" : "pending"}">${m.read ? "Read" : "Unread"}</span><div class="owner-row-actions"><button data-read="${esc(m.id)}" data-value="${!m.read}">${m.read ? "Mark unread" : "Mark read"}</button><button data-delete="${esc(m.id)}">Delete</button></div></article>`; }).join("") : '<p class="owner-empty">Guest messages will appear here.</p>'}</div>`;
      box.addEventListener("click", async event => { const read = event.target.closest("[data-read]"); const remove = event.target.closest("[data-delete]"); try { if (read) await owner.api.setOwnerMessageRead(read.dataset.read, read.dataset.value === "true"); if (remove && confirm("Delete this guest message?")) await owner.api.deleteOwnerMessage(remove.dataset.delete); if (read || remove) { owner.toast("Inbox updated."); messages(); } } catch (e) { owner.toast(e.message, "error"); } });
    } catch (e) { failed(e); }
  }

  async function content(kind) {
    const announcements = kind === "announcements";
    const title = announcements ? "Announcements" : "Photos & story";
    root.innerHTML = `${intro("Content", title, announcements ? "Share updates with guests or your planning committee." : "Tell your story for guests visiting your wedding page.", `<button class="owner-button primary" id="add-item" type="button">Add ${announcements ? "announcement" : "story section"}</button>`)} ${state}`;
    try {
      let data = await owner.loadWorkspace(); ready();
      const box = document.getElementById("extra-main");
      const key = announcements ? "announcements" : "story_sections";
      const list = () => data[key] || [];
      function draw() {
        box.innerHTML = `<div class="owner-card-head"><div><h2>${announcements ? "Updates" : "Story sections"}</h2><p>Draft items stay private until published.</p></div></div><div class="owner-list">${list().length ? list().map(item => `<article class="owner-row"><div class="owner-row-main"><strong>${esc(item.title)}</strong><p>${esc(announcements ? item.body : item.body)}</p><small>${announcements ? esc(item.audience || "public") : esc(item.photo_url || "")}</small></div><span class="owner-badge ${esc(item.status)}">${esc(item.status)}</span><div class="owner-row-actions"><button data-edit="${esc(item.id)}">Edit</button><button data-toggle="${esc(item.id)}">${item.status === "published" ? "Unpublish" : "Publish"}</button><button data-remove="${esc(item.id)}">Delete</button></div></article>`).join("") : '<p class="owner-empty">Nothing here yet. Add your first item above.</p>'}</div>`;
      }
      function onClick(event) {
        const edit = event.target.closest("[data-edit]"), toggle = event.target.closest("[data-toggle]"), remove = event.target.closest("[data-remove]");
        if (edit) formFor(list().find(x => x.id === edit.dataset.edit));
        else if (toggle) { data[key] = list().map(x => x.id === toggle.dataset.toggle ? { ...x, status: x.status === "published" ? "draft" : "published" } : x); save(); }
        else if (remove && confirm("Delete this item?")) { data[key] = list().filter(x => x.id !== remove.dataset.remove); save(); }
      }
      async function save() { try { data = await owner.saveWorkspace(data); draw(); owner.toast("Content saved."); } catch (e) { owner.toast(e.message || "Could not save content.", "error"); } }
      function formFor(item = {}) {
        const dialog = document.createElement("dialog"); dialog.className = "owner-dialog";
        dialog.innerHTML = `<form method="dialog"><div class="owner-card-head"><h2>${item.id ? "Edit" : "Add"} ${announcements ? "announcement" : "story section"}</h2><button type="button" class="owner-button" data-close>Close</button></div><div class="owner-form"><label class="owner-field">Title<input name="title" maxlength="${announcements ? 200 : 160}" required></label>${announcements ? '<label class="owner-field">Audience<select name="audience"><option value="public">Guests</option><option value="committee">Committee</option></select></label>' : '<label class="owner-field">Photo URL<input name="photo_url" placeholder="https://…"></label>'}<label class="owner-field wide">${announcements ? "Message" : "Story"}<textarea name="body" required></textarea></label><div class="owner-actions wide"><button class="owner-button primary" type="submit">Save</button></div></div></form>`;
        document.body.append(dialog); const form = dialog.querySelector("form"); form.elements.title.value = item.title || ""; form.elements.body.value = item.body || ""; if (announcements) form.elements.audience.value = item.audience || "public"; else form.elements.photo_url.value = item.photo_url || "";
        dialog.querySelector("[data-close]").onclick = () => dialog.close(); form.onsubmit = event => { event.preventDefault(); const value = { ...item, id: item.id || "", title: form.elements.title.value.trim(), body: form.elements.body.value.trim(), status: item.status || "draft" }; if (announcements) { value.audience = form.elements.audience.value; value.author_name ||= "Wedding owner"; } else value.photo_url = form.elements.photo_url.value.trim(); data[key] = item.id ? list().map(x => x.id === item.id ? value : x) : [...list(), value]; dialog.close(); dialog.remove(); save(); }; dialog.addEventListener("close", () => dialog.remove(), { once: true }); dialog.showModal();
      }
      box.addEventListener("click", onClick);
      document.getElementById("add-item").onclick = () => formFor(); draw();
    } catch (e) { failed(e); }
  }

  async function committee() {
    root.innerHTML = `${intro("Planning", "Planning committee", "Invite helpers, manage your team, and keep planning tasks moving.", '<button class="owner-button primary" id="invite-member">Invite member</button> <button class="owner-button" id="add-task">Add task</button>')} ${state}`;
    try {
      let data = await owner.loadWorkspace(); ready(); const box = document.getElementById("extra-main");
      function draw() {
        box.innerHTML = `<div class="owner-grid"><article class="owner-card"><div class="owner-card-head"><h2>Members</h2></div><div class="owner-list">${(data.committee_members || []).map(m => `<article class="owner-row"><div class="owner-row-main"><strong>${esc(m.name)}</strong><p>${esc(m.title || "Committee member")} · ${esc(m.email || "No email")}</p></div><span class="owner-badge ${esc(m.invitation_status || "accepted")}">${esc(m.invitation_status || "Member")}</span><div class="owner-row-actions"><button data-remove-member="${esc(m.id)}">Remove</button></div></article>`).join("") || '<p class="owner-empty">No committee members yet.</p>'}</div></article><article class="owner-card"><div class="owner-card-head"><h2>Planning tasks</h2></div><div class="owner-list">${(data.planning_tasks || []).map(t => `<article class="owner-row"><div class="owner-row-main"><strong>${esc(t.title)}</strong><p>${esc(t.assigned_to || "Unassigned")} · ${esc(t.due_on || "No due date")}</p></div><span class="owner-badge ${esc(t.status)}">${esc(t.status)}</span><div class="owner-row-actions"><button data-toggle-task="${esc(t.id)}">${t.status === "done" ? "Reopen" : "Complete"}</button><button data-remove-task="${esc(t.id)}">Delete</button></div></article>`).join("") || '<p class="owner-empty">No tasks yet.</p>'}</div></article></div>`;
      }
      document.getElementById("invite-member").onclick = async () => { const name = prompt("Committee member name:"); if (!name?.trim()) return; const email = prompt("Email address (optional):") || ""; const title = prompt("Committee role:") || "Committee member"; try { const created = await owner.api.createOwnerInvitation({ type: "committee", guest_name: name.trim(), guest_email: email.trim(), committee_title: title.trim(), max_party_size: 1 }); owner.toast(`Invitation created${created.token ? `; save this invite token: ${created.token}` : "."}`); data = await owner.loadWorkspace(); draw(); } catch (e) { owner.toast(e.message, "error"); } };
      document.getElementById("add-task").onclick = async () => { const title = prompt("Task title:"); if (!title?.trim()) return; try { await owner.api.createOwnerTask({ title: title.trim(), status: "todo" }); data = await owner.loadWorkspace(); draw(); } catch (e) { owner.toast(e.message, "error"); } };
      box.addEventListener("click", async event => { const rm = event.target.closest("[data-remove-member]"), done = event.target.closest("[data-toggle-task]"), del = event.target.closest("[data-remove-task]"); try { if (rm && confirm("Remove this committee member?")) await owner.api.deleteOwnerCommitteeMember(rm.dataset.removeMember); if (done) { const task = data.planning_tasks.find(x => x.id === done.dataset.toggleTask); await owner.api.updateOwnerTask(task.id, { ...task, status: task.status === "done" ? "todo" : "done" }); } if (del && confirm("Delete this task?")) await owner.api.deleteOwnerTask(del.dataset.removeTask); data = await owner.loadWorkspace(); draw(); } catch (e) { owner.toast(e.message, "error"); } }); draw();
    } catch (e) { failed(e); }
  }

  if (page === "messages") messages();
  if (page === "committee") committee();
  if (page === "story") content("story");
  if (page === "announcements") content("announcements");
})();
