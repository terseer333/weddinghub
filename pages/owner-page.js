(() => {
  const owner = window.WeddingHubOwner;
  if (!owner || document.body.dataset.page === "dashboard") return;
  const root = document.getElementById("owner-content");
  const escape = owner.esc;
  const page = document.body.dataset.page;

  function intro(eyebrow, title, description, action = "") {
    return `<header class="page-intro"><div><span class="owner-eyebrow">${eyebrow}</span><h1>${title}</h1><p>${description}</p></div>${action}</header>`;
  }
  function stateMarkup() {
    return '<div class="owner-state" id="page-loading">Loading your wedding details…</div><div class="owner-state error" id="page-error" hidden></div>';
  }
  function setState(type, message) {
    const loading = document.getElementById("page-loading");
    const error = document.getElementById("page-error");
    if (loading) loading.hidden = type !== "loading";
    if (error) {
      error.hidden = type !== "error";
      error.textContent = message || "";
    }
  }
  function localDateTime(value) {
    if (!value) return "";
    const date = new Date(value);
    if (Number.isNaN(date.valueOf())) return "";
    const pad = number => String(number).padStart(2, "0");
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
  }
  function progress(data) {
    const steps = [
      ["Couple names", data.partner_one && data.partner_two],
      ["Wedding date", Boolean(data.date)],
      ["Venue", Boolean(data.venue)],
      ["Location", Boolean(data.address)],
      ["Guest message", Boolean(data.message)]
    ];
    const done = steps.filter(([, complete]) => complete).length;
    const panel = document.getElementById("setup-progress");
    if (!panel) return;
    panel.querySelector("strong").textContent = `${Math.round(done / steps.length * 100)}%`;
    panel.querySelector(".owner-progress span").style.width = `${done / steps.length * 100}%`;
    panel.querySelector("#setup-step-list").innerHTML = steps.map(([name, complete]) => `<li>${complete ? "✓" : "○"} ${escape(name)}</li>`).join("");
  }

  async function weddingInfo() {
    root.innerHTML = `${intro("Your celebration", "Wedding information", "Add the details your guests need and save the changes to your wedding page.")} ${stateMarkup()}<div class="owner-grid" id="page-main" hidden><article class="owner-card" style="grid-column:span 8"><div class="owner-card-head"><div><h2>The details</h2><p>Fields marked required are used to identify your wedding.</p></div></div><form id="wedding-form" class="owner-form"><label class="owner-field">Partner one<input name="partner_one" maxlength="100" autocomplete="given-name" required></label><label class="owner-field">Partner two<input name="partner_two" maxlength="100" autocomplete="given-name" required></label><label class="owner-field">Wedding date and time<input name="date" type="datetime-local"></label><label class="owner-field">Venue name<input name="venue" maxlength="250"></label><label class="owner-field wide">Address<input name="address" maxlength="500" autocomplete="street-address"></label><label class="owner-field">City<input name="city" maxlength="120"></label><label class="owner-field">State or region<input name="state" maxlength="120"></label><label class="owner-field">Country<input name="country" maxlength="120"></label><label class="owner-field">Dress code<input name="dress_code" maxlength="500"></label><label class="owner-field wide">A message for your guests<textarea name="message" maxlength="10000" placeholder="A short welcome for the people celebrating with you"></textarea></label><div class="owner-actions wide"><button class="owner-button primary" type="submit">Save wedding information</button><span class="owner-hint" id="save-note" aria-live="polite"></span></div></form></article><aside class="owner-card" id="setup-progress" style="grid-column:span 4"><span class="owner-eyebrow">Setup progress</span><h2><strong>0%</strong></h2><div class="owner-progress"><span style="width:0%"></span></div><ul id="setup-step-list" class="setup-step-list"></ul><p>Complete these details to help guests plan for your celebration.</p></aside></div>`;
    setState("loading");
    try {
      let data = await owner.loadWorkspace();
      const form = document.getElementById("wedding-form");
      for (const key of ["partner_one", "partner_two", "venue", "address", "city", "state", "country", "dress_code", "message"]) form.elements[key].value = data[key] || "";
      form.elements.date.value = localDateTime(data.date);
      document.getElementById("page-main").hidden = false;
      setState("ready");
      progress(data);
      form.addEventListener("submit", async event => {
        event.preventDefault();
        const button = form.querySelector("button[type=submit]");
        button.disabled = true;
        const formData = new FormData(form);
        data.partner_one = String(formData.get("partner_one") || "").trim();
        data.partner_two = String(formData.get("partner_two") || "").trim();
        data.title = `${data.partner_one} & ${data.partner_two}`;
        const date = String(formData.get("date") || "");
        data.date = date ? new Date(date).toISOString() : null;
        for (const key of ["venue", "address", "city", "state", "country", "dress_code", "message"]) data[key] = String(formData.get(key) || "").trim();
        try {
          data = await owner.saveWorkspace(data);
          progress(data);
          owner.toast("Wedding information saved.");
          document.getElementById("save-note").textContent = "Saved to your wedding page.";
        } catch (error) {
          owner.toast(error.message || "Could not save wedding information.", "error");
        } finally { button.disabled = false; }
      });
    } catch (error) {
      setState("error", error.message || "Could not load wedding information.");
    }
  }

  async function cardStudio() {
    root.innerHTML = `${intro("Invitations", "Card Studio", "Choose a design, adjust the fonts and colors, and preview your invitation as you edit.", '<a class="owner-button" href="preview-card.html">Guest preview</a>')} ${stateMarkup()}<div class="owner-grid card-studio-page" id="page-main" hidden><article class="owner-card studio-gallery-card"><div class="owner-card-head"><div><h2>1. Choose a design</h2><p>Start with a style you both like. You can change it any time.</p></div><label class="owner-field">Category<select id="template-filter" class="owner-control"><option value="">All categories</option></select></label></div><label class="owner-field studio-search-field">Search designs<input id="template-search" type="search" placeholder="Try botanical, classic, or gold"></label><div class="template-grid" id="template-grid"></div><p class="owner-empty" id="templates-empty" hidden>No templates match your search.</p></article><article class="owner-card studio-edit-card"><div class="owner-card-head"><div><h2>2. Make it yours</h2><p>Changes appear in the preview right away. Save when you’re happy.</p></div></div><div class="invitation-preview" id="card-mini-preview" aria-live="polite"><span class="owner-eyebrow">You are invited</span><h2 id="preview-couple"></h2><p id="preview-date"></p><p id="preview-venue"></p></div><form id="card-form" class="owner-form"><h3 class="studio-form-heading">Personalize the look</h3><label class="owner-field">Names font<select name="font-couple"></select></label><label class="owner-field">Heading font<select name="font-heading"></select></label><label class="owner-field">Details font<select name="font-body"></select></label><label class="owner-field">Background color<input name="color-background" type="color"></label><label class="owner-field">Text color<input name="color-text" type="color"></label><label class="owner-field">Accent color<input name="color-accent" type="color"></label><div class="owner-actions wide"><button class="owner-button primary" type="submit">Save invitation design</button></div></form></article></div>`;
    setState("loading");
    try {
      const [data, templatesResponse, fontsResponse] = await Promise.all([
        owner.loadWorkspace(),
        fetch(`${owner.api.baseURL()}/api/templates`),
        fetch(`${owner.api.baseURL()}/api/fonts`)
      ]);
      if (!templatesResponse.ok || !fontsResponse.ok) throw new Error("The template catalog could not be loaded.");
      const templates = await templatesResponse.json();
      const fonts = await fontsResponse.json();
      let selected = templates.find(template => template.id === (data.card_config?.template_id || data.template_id)) || templates[0] || null;
      let config = data.card_config || (selected ? { template_id: selected.id, fonts: selected.fonts, colors: selected.colors, decorations: { ...(selected.decorations || {}), background: selected.background?.style || "solid" } } : { template_id: "", fonts: {}, colors: {}, decorations: {} });
      document.getElementById("page-main").hidden = false;
      setState("ready");
      const form = document.getElementById("card-form");
      for (const category of [...new Set(templates.map(template => template.category).filter(Boolean))]) {
        const option = document.createElement("option"); option.value = category; option.textContent = category; document.getElementById("template-filter").append(option);
      }
      for (const key of ["couple", "heading", "body"]) {
        const select = form.elements[`font-${key}`];
        fonts.filter(font => font.category === key).forEach(font => { const option = document.createElement("option"); option.value = font.name; option.textContent = font.name; select.append(option); });
      }
      const safeColor = value => /^#[0-9a-f]{6}$/i.test(value || "") ? value : "#173d32";
      function draw() {
        document.getElementById("template-grid").innerHTML = templates.filter(template => {
          const text = `${template.name} ${template.category}`.toLowerCase();
          return (!document.getElementById("template-filter").value || template.category === document.getElementById("template-filter").value) && text.includes(document.getElementById("template-search").value.toLowerCase());
        }).map(template => {
          const floralStyle = template.decorations?.floralStyle || "minimalist-frame";
          const floralAsset = floralStyle === "minimal" ? "minimalist-frame" : floralStyle;
          return `<button class="template-choice" type="button" data-template="${escape(template.id)}" aria-pressed="${selected?.id === template.id}"><span class="template-swatch" style="--swatch-bg:${safeColor(template.colors?.background)};--swatch-accent:${safeColor(template.colors?.accent)};--swatch-art:url('../assets/templates/${escape(floralAsset)}.svg')"></span><strong>${escape(template.name)}</strong><small>${escape(template.category || "Wedding")}</small></button>`;
        }).join("");
        document.getElementById("templates-empty").hidden = Boolean(document.getElementById("template-grid").children.length);
        const card = document.getElementById("card-mini-preview");
        const wedding = { brideName: data.partner_one, groomName: data.partner_two, date: data.date, venue: data.venue, address: data.address, city: data.city, state: data.state, message: data.message, dressCode: data.dress_code, cardConfig: config };
        card.innerHTML = window.WeddingInvitation
          ? window.WeddingInvitation.card({ wedding, events: [] }, selected?.id, "owner-card-preview", config)
          : `<div class="card-preview-fallback" style="background:${safeColor(config.colors?.background)};color:${safeColor(config.colors?.text)};border-color:${safeColor(config.colors?.accent)}"><strong>${escape(data.partner_one || "Partner one")} &amp; ${escape(data.partner_two || "Partner two")}</strong><span>${escape(selected?.name || "Wedding invitation")}</span></div>`;
      }
      for (const key of ["couple", "heading", "body"]) form.elements[`font-${key}`].value = config.fonts?.[key] || "";
      for (const key of ["background", "text", "accent"]) form.elements[`color-${key}`].value = safeColor(config.colors?.[key]);
      document.getElementById("template-grid").addEventListener("click", event => {
        const button = event.target.closest("[data-template]");
        if (!button) return;
        selected = templates.find(template => template.id === button.dataset.template) || selected;
        config = { ...config, template_id: selected.id, fonts: { ...selected.fonts }, colors: { ...selected.colors }, decorations: { ...selected.decorations, background: selected.background?.style || "solid" } };
        for (const key of ["couple", "heading", "body"]) if (selected.fonts?.[key]) form.elements[`font-${key}`].value = selected.fonts[key];
        for (const key of ["background", "text", "accent"]) form.elements[`color-${key}`].value = safeColor(selected.colors?.[key]);
        draw();
      });
      document.getElementById("template-search").addEventListener("input", draw);
      document.getElementById("template-filter").addEventListener("change", draw);
      form.addEventListener("input", () => {
        config.fonts = { couple: form.elements["font-couple"].value, heading: form.elements["font-heading"].value, body: form.elements["font-body"].value };
        config.colors = { background: form.elements["color-background"].value, text: form.elements["color-text"].value, accent: form.elements["color-accent"].value };
        draw();
      });
      form.addEventListener("submit", async event => {
        event.preventDefault();
        if (!selected) return owner.toast("Choose a template first.", "error");
        const button = form.querySelector("button[type=submit]"); button.disabled = true;
        config.template_id = selected.id; data.template_id = selected.id; data.card_config = config;
        try { data = await owner.saveWorkspace(data); owner.toast("Invitation design saved."); }
        catch (error) { owner.toast(error.message || "Could not save the invitation design.", "error"); }
        finally { button.disabled = false; }
      });
      draw();
    } catch (error) { setState("error", error.message || "Could not load Card Studio."); }
  }

  async function cardPreview() {
    root.innerHTML = `${intro("Invitations", "Preview your invitation", "See the guest-facing card with the design currently saved for your wedding.", '<a class="owner-button" href="card-studio.html">← Back to Card Studio</a>')} ${stateMarkup()}<section id="page-main" class="owner-card" hidden><div class="owner-card-head"><div><h2>Guest preview</h2><p>The card uses the saved template and wedding details.</p></div><div class="owner-actions"><button class="owner-button" type="button" data-preview-size="desktop">Desktop</button><button class="owner-button" type="button" data-preview-size="mobile">Mobile</button></div></div><div class="preview-frame"><iframe id="guest-preview" title="Guest invitation card preview" src="about:blank"></iframe></div><div class="owner-form" style="margin-top:18px"><label class="owner-field wide">Guest link to copy or share<select id="preview-invitation" class="owner-control"></select></label><div class="owner-actions wide"><button class="owner-button primary" id="copy-invitation" type="button">Generate & copy guest link</button><button class="owner-button" id="share-invitation" type="button">Share link</button></div><p class="owner-hint wide">For security, invitation links are stored as one-way hashes. Generating a replacement link invalidates the previous link for that guest.</p></div></section>`;
    setState("loading");
    try {
      const data = await owner.loadWorkspace();
      const local = window.WeddingHub.getData() || {};
      window.WeddingHub.saveData(owner.api.mergeAPI(local, data));
      const invitations = (data.invitations || []).filter(invitation => invitation.type !== "committee");
      const select = document.getElementById("preview-invitation");
      select.innerHTML = '<option value="">Choose a guest</option>' + invitations.map(invitation => `<option value="${escape(invitation.id)}">${escape(invitation.guest_name)}</option>`).join("");
      document.getElementById("page-main").hidden = false;
      setState("ready");
      document.getElementById("guest-preview").src = "event.html?preview=admin";
      document.querySelectorAll("[data-preview-size]").forEach(button => button.addEventListener("click", () => {
        const frame = document.querySelector(".preview-frame"); frame.classList.toggle("mobile", button.dataset.previewSize === "mobile");
      }));
      async function guestURL() {
        if (!select.value) throw new Error("Choose a guest first.");
        const link = await owner.api.refreshOwnerInvitationLink(select.value);
        const url = new URL(`event.html?token=${encodeURIComponent(link.token)}`, location.href).href;
        return url;
      }
      document.getElementById("copy-invitation").addEventListener("click", async () => {
        try { const url = await guestURL(); await navigator.clipboard.writeText(url); owner.toast("A new guest link was copied."); }
        catch (error) { owner.toast(error.message || "Could not create the guest link.", "error"); }
      });
      document.getElementById("share-invitation").addEventListener("click", async () => {
        try {
          const url = await guestURL();
          if (navigator.share) await navigator.share({ title: "Wedding invitation", url });
          else { await navigator.clipboard.writeText(url); owner.toast("A new guest link was copied."); }
        } catch (error) { if (error.name !== "AbortError") owner.toast(error.message || "Could not share the guest link.", "error"); }
      });
    } catch (error) { setState("error", error.message || "Could not load invitation preview."); }
  }

  async function guestsPage() {
    root.innerHTML = `${intro("Guests", "Guests & RSVP", "Manage invitations, track responses, and share guest links.", '<button class="owner-button primary" id="add-guest" type="button">Add guest</button>')} ${stateMarkup()}<section id="page-main" hidden><div class="owner-stat-grid" id="guest-stats"></div><article class="owner-card"><div class="owner-card-head"><div><h2>Guest list</h2><p>Search by name or filter by RSVP status.</p></div><div class="owner-actions"><button class="owner-button" id="export-guests" type="button">Export CSV</button></div></div><div class="owner-rsvp"><div class="owner-donut" id="guest-donut"><div><strong id="guest-donut-total">0</strong><small>invited</small></div></div><div class="owner-rsvp-legend"><span>Attending <strong id="guest-attending">0</strong></span><span>Pending <strong id="guest-pending">0</strong></span><span>Declined <strong id="guest-declined">0</strong></span></div></div><div class="owner-tabs" id="guest-filters"><button type="button" data-status="all" aria-pressed="true">All</button><button type="button" data-status="attending" aria-pressed="false">Attending</button><button type="button" data-status="pending" aria-pressed="false">Pending</button><button type="button" data-status="declined" aria-pressed="false">Declined</button></div><div class="owner-list" id="guest-list"></div><div class="owner-empty" id="guest-empty" hidden><strong>No guests yet?</strong>Add your first guest to start collecting RSVPs.</div></article><dialog class="owner-dialog" id="guest-dialog"><form id="guest-form" method="dialog"><div class="owner-card-head"><div><span class="owner-eyebrow">Invitation</span><h2 id="guest-dialog-title">Add guest</h2></div><button class="owner-button" type="button" id="close-guest-dialog">Close</button></div><div class="owner-form"><label class="owner-field">Guest name<input name="guest_name" maxlength="160" required></label><label class="owner-field">Party size<input name="max_party_size" type="number" min="1" max="20" value="1" required></label><label class="owner-field">Email<input name="guest_email" type="email" maxlength="254"></label><label class="owner-field">Phone<input name="guest_phone" type="tel" maxlength="40"></label><div class="owner-actions wide"><button class="owner-button primary" type="submit">Save guest</button></div></div></form></dialog><section class="owner-card" id="guest-link-card" hidden><div class="owner-card-head"><div><h2>Guest invitation link</h2><p>This link is shown once when created. You can generate a replacement from the guest row later.</p></div></div><a id="guest-link" class="owner-control" target="_blank" rel="noopener"></a><div class="owner-actions"><button class="owner-button primary" id="copy-new-link" type="button">Copy link</button></div></section></section>`;
    setState("loading");
    let data;
    const tokens = new Map();
    let filter = "all";
    let editing = "";
    const responseFor = invitation => (data.rsvps || []).find(response => response.invitation_id === invitation.id);
    const stateFor = invitation => {
      if (invitation.status === "declined") return "declined";
      if (invitation.status !== "accepted") return "pending";
      const response = responseFor(invitation);
      if (!response || response.status === "maybe") return "pending";
      return response.status === "not_attending" ? "declined" : response.status;
    };
    const guests = () => (data.invitations || []).filter(invitation => invitation.type !== "committee");
    function render() {
      const rows = guests();
      const counts = { total: 0, attending: 0, pending: 0, declined: 0 };
      rows.forEach(invitation => {
        const party = Math.max(1, invitation.max_party_size || 1);
        counts.total += party;
        const status = stateFor(invitation);
        if (status === "attending") counts.attending += responseFor(invitation)?.party_size || party;
        else if (status === "declined") counts.declined += party;
        else counts.pending += party;
      });
      document.getElementById("guest-stats").innerHTML = [["Invited", counts.total], ["Attending", counts.attending], ["Pending", counts.pending], ["Declined", counts.declined]].map(([label, count]) => `<div class="owner-stat"><strong>${count}</strong><span>${label}</span></div>`).join("");
      const percentage = count => counts.total ? count / counts.total * 100 : 0;
      const donut = document.getElementById("guest-donut");
      donut.classList.toggle("empty", counts.total === 0);
      donut.style.setProperty("--att", percentage(counts.attending));
      donut.style.setProperty("--pen", percentage(counts.pending));
      donut.style.setProperty("--dec", percentage(counts.declined));
      document.getElementById("guest-donut-total").textContent = counts.total;
      document.getElementById("guest-attending").textContent = counts.attending;
      document.getElementById("guest-pending").textContent = counts.pending;
      document.getElementById("guest-declined").textContent = counts.declined;
      const visible = rows.filter(invitation => filter === "all" || stateFor(invitation) === filter);
      document.getElementById("guest-list").innerHTML = visible.map(invitation => {
        const status = stateFor(invitation);
        const party = responseFor(invitation)?.party_size || invitation.max_party_size || 1;
        return `<article class="owner-row" data-searchable><div class="owner-row-main"><strong>${escape(invitation.guest_name)}</strong><p>${escape(invitation.guest_email || "No email provided")} · Party ${party}</p><small>${escape(invitation.guest_phone || "No phone provided")}</small></div><span class="owner-badge ${escape(status)}">${escape(status.replace("_", " "))}</span><div class="owner-row-actions"><button type="button" data-edit-guest="${escape(invitation.id)}">Edit</button><button type="button" data-copy-guest="${escape(invitation.id)}">Copy link</button>${status === "pending" ? `<button type="button" data-remind-guest="${escape(invitation.id)}">Send reminder</button>` : ""}<button type="button" data-delete-guest="${escape(invitation.id)}">Delete</button></div></article>`;
      }).join("");
      const none = document.getElementById("guest-empty");
      none.hidden = rows.length > 0;
      document.getElementById("guest-list").hidden = visible.length === 0;
    }
    function showGeneratedLink(token) {
      const panel = document.getElementById("guest-link-card");
      const anchor = document.getElementById("guest-link");
      anchor.href = new URL(`event.html?token=${encodeURIComponent(token)}`, location.href).href;
      anchor.textContent = anchor.href;
      panel.hidden = false;
      panel.scrollIntoView({ behavior: "smooth", block: "center" });
      document.getElementById("copy-new-link").onclick = async () => {
        try { await navigator.clipboard.writeText(anchor.href); owner.toast("Invitation link copied."); }
        catch (_) { owner.toast("Copy the link shown above.", "error"); }
      };
    }
    async function createFreshLink(id) {
      const link = await owner.api.refreshOwnerInvitationLink(id);
      tokens.set(id, link.token);
      showGeneratedLink(link.token);
      return link.token;
    }
    try {
      data = await owner.loadWorkspace();
      document.getElementById("page-main").hidden = false;
      setState("ready");
      render();
      const dialog = document.getElementById("guest-dialog");
      const form = document.getElementById("guest-form");
      document.getElementById("add-guest").addEventListener("click", () => {
        editing = ""; form.reset(); form.elements.max_party_size.value = 1;
        document.getElementById("guest-dialog-title").textContent = "Add guest";
        dialog.showModal();
      });
      document.getElementById("close-guest-dialog").addEventListener("click", () => dialog.close());
      document.getElementById("guest-filters").addEventListener("click", event => {
        const button = event.target.closest("[data-status]"); if (!button) return;
        filter = button.dataset.status;
        document.querySelectorAll("#guest-filters button").forEach(item => item.setAttribute("aria-pressed", String(item === button)));
        render();
      });
      form.addEventListener("submit", async event => {
        event.preventDefault();
        const submit = form.querySelector("button[type=submit]"); submit.disabled = true;
        const invitation = { guest_name: form.elements.guest_name.value.trim(), guest_email: form.elements.guest_email.value.trim(), guest_phone: form.elements.guest_phone.value.trim(), max_party_size: Number(form.elements.max_party_size.value), type: "guest" };
        try {
          if (editing) {
            await owner.api.updateOwnerInvitation(editing, invitation);
            owner.toast("Guest invitation updated.");
          } else {
            const created = await owner.api.createOwnerInvitation(invitation);
            tokens.set(created.invitation.id, created.token);
            showGeneratedLink(created.token);
            owner.toast("Guest added. Copy and share the invitation link.");
          }
          data = await owner.loadWorkspace(); render(); dialog.close();
        } catch (error) { owner.toast(error.message || "Could not save this guest.", "error"); }
        finally { submit.disabled = false; }
      });
      document.getElementById("guest-list").addEventListener("click", async event => {
        const edit = event.target.closest("[data-edit-guest]");
        const copy = event.target.closest("[data-copy-guest]");
        const remind = event.target.closest("[data-remind-guest]");
        const remove = event.target.closest("[data-delete-guest]");
        try {
          if (edit) {
            editing = edit.dataset.editGuest;
            const invitation = guests().find(item => item.id === editing);
            if (!invitation) return;
            form.elements.guest_name.value = invitation.guest_name || "";
            form.elements.guest_email.value = invitation.guest_email || "";
            form.elements.guest_phone.value = invitation.guest_phone || "";
            form.elements.max_party_size.value = invitation.max_party_size || 1;
            document.getElementById("guest-dialog-title").textContent = "Edit guest";
            dialog.showModal();
          } else if (copy) {
            const token = await createFreshLink(copy.dataset.copyGuest);
            try { await navigator.clipboard.writeText(new URL(`event.html?token=${encodeURIComponent(token)}`, location.href).href); owner.toast("A replacement guest link was copied."); }
            catch (_) { owner.toast("A replacement link is displayed below the guest list."); }
          } else if (remind) {
            const id = remind.dataset.remindGuest;
            let token = tokens.get(id);
            if (!token) token = await createFreshLink(id);
            const invitation = guests().find(item => item.id === id);
            const channels = [invitation?.guest_email ? "email" : "", invitation?.guest_phone ? "whatsapp" : ""].filter(Boolean);
            const sent = await owner.api.sendOwnerInvitation(id, token, channels);
            const result = (sent.results || []).some(item => item.status === "sent");
            owner.toast(result ? "Reminder sent." : "No reminder channel is configured for this guest.", result ? "success" : "error");
          } else if (remove && window.confirm("Delete this invitation and its RSVP?")) {
            await owner.api.deleteOwnerInvitation(remove.dataset.deleteGuest);
            data = await owner.loadWorkspace(); render(); owner.toast("Guest invitation deleted.");
          }
        } catch (error) { owner.toast(error.message || "That guest action failed.", "error"); }
      });
      document.getElementById("export-guests").addEventListener("click", () => {
        const cell = value => { let text = String(value ?? ""); if (/^[=+@-]/.test(text)) text = `'${text}`; return `"${text.replaceAll('"', '""')}"`; };
        const csv = [["Name", "Email", "Phone", "Party size", "RSVP"], ...guests().map(item => [item.guest_name, item.guest_email, item.guest_phone, item.max_party_size, stateFor(item)])].map(row => row.map(cell).join(",")).join("\r\n");
        const link = document.createElement("a"); link.href = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" })); link.download = "wedding-guests.csv"; link.click(); URL.revokeObjectURL(link.href);
      });
    } catch (error) { setState("error", error.message || "Could not load guests."); }
  }

  async function eventsPage() {
    root.innerHTML = `${intro("Planning", "Events", "Build the schedule your guests will see. Only published events appear on the guest page.", '<button class="owner-button primary" id="add-event" type="button">Add event</button>')} ${stateMarkup()}<section id="page-main" class="owner-grid" hidden><article class="owner-card"><div class="owner-card-head"><div><h2>Your wedding schedule</h2><p>Keep dates, venue details, and visibility current.</p></div></div><div id="event-list" class="owner-list"></div><div id="event-empty" class="owner-empty" hidden><strong>No events yet</strong>Add your ceremony, reception, or another celebration.</div></article></section><dialog class="owner-dialog" id="event-dialog"><form id="event-form" method="dialog"><div class="owner-card-head"><div><span class="owner-eyebrow">Wedding schedule</span><h2 id="event-dialog-title">Add event</h2></div><button class="owner-button" type="button" id="close-event-dialog">Close</button></div><div class="owner-form"><label class="owner-field">Event name<input name="name" maxlength="160" required></label><label class="owner-field">Visibility<select name="status"><option value="draft">Draft</option><option value="published">Published</option></select></label><label class="owner-field">Date and time<input name="starts_at" type="datetime-local" required></label><label class="owner-field">End date and time<input name="ends_at" type="datetime-local"></label><label class="owner-field">Venue<input name="venue" maxlength="250"></label><label class="owner-field">Address<input name="address" maxlength="500"></label><label class="owner-field wide">Description<textarea name="description" maxlength="4000"></textarea></label><div class="owner-actions wide"><button class="owner-button primary" type="submit">Save event</button></div></div></form></dialog>`;
    setState("loading");
    let data;
    let editing = "";
    function render() {
      const events = data.events || [];
      document.getElementById("event-list").innerHTML = events.map(event => `<article class="owner-row" data-searchable><div class="owner-row-main"><strong>${escape(event.name)}</strong><p>${escape(new Date(event.starts_at).toLocaleString())} · ${escape(event.venue || "Venue to be confirmed")}</p><small>${escape(event.description || event.address || "")}</small></div><span class="owner-badge ${escape(event.status)}">${escape(event.status)}</span><div class="owner-row-actions"><button type="button" data-edit-event="${escape(event.id)}">Edit</button><button type="button" data-toggle-event="${escape(event.id)}">${event.status === "published" ? "Unpublish" : "Publish"}</button><button type="button" data-delete-event="${escape(event.id)}">Delete</button></div></article>`).join("");
      document.getElementById("event-empty").hidden = events.length > 0;
      document.getElementById("event-list").hidden = events.length === 0;
    }
    try {
      data = await owner.loadWorkspace();
      document.getElementById("page-main").hidden = false;
      setState("ready"); render();
      const dialog = document.getElementById("event-dialog");
      const form = document.getElementById("event-form");
      document.getElementById("add-event").addEventListener("click", () => { editing = ""; form.reset(); form.elements.status.value = "draft"; document.getElementById("event-dialog-title").textContent = "Add event"; dialog.showModal(); });
      document.getElementById("close-event-dialog").addEventListener("click", () => dialog.close());
      form.addEventListener("submit", async event => {
        event.preventDefault();
        const submit = form.querySelector("button[type=submit]"); submit.disabled = true;
        try {
          const starts = form.elements.starts_at.value;
          const ends = form.elements.ends_at.value;
          const item = { id: editing, name: form.elements.name.value.trim(), description: form.elements.description.value.trim(), starts_at: new Date(starts).toISOString(), ends_at: ends ? new Date(ends).toISOString() : null, venue: form.elements.venue.value.trim(), address: form.elements.address.value.trim(), status: form.elements.status.value };
          if (editing) data.events = data.events.map(existing => existing.id === editing ? item : existing); else data.events = [...(data.events || []), item];
          data = await owner.saveWorkspace(data); render(); dialog.close(); owner.toast("Event saved.");
        } catch (error) { owner.toast(error.message || "Could not save this event.", "error"); }
        finally { submit.disabled = false; }
      });
      document.getElementById("event-list").addEventListener("click", async event => {
        const edit = event.target.closest("[data-edit-event]");
        const toggle = event.target.closest("[data-toggle-event]");
        const remove = event.target.closest("[data-delete-event]");
        try {
          if (edit) {
            editing = edit.dataset.editEvent;
            const item = data.events.find(candidate => candidate.id === editing); if (!item) return;
            for (const key of ["name", "description", "venue", "address", "status"]) form.elements[key].value = item[key] || "";
            form.elements.starts_at.value = localDateTime(item.starts_at);
            form.elements.ends_at.value = localDateTime(item.ends_at);
            document.getElementById("event-dialog-title").textContent = "Edit event";
            dialog.showModal();
          } else if (toggle) {
            data.events = data.events.map(item => item.id === toggle.dataset.toggleEvent ? { ...item, status: item.status === "published" ? "draft" : "published" } : item);
            data = await owner.saveWorkspace(data); render(); owner.toast("Event visibility updated.");
          } else if (remove && window.confirm("Delete this event?")) {
            data.events = data.events.filter(item => item.id !== remove.dataset.deleteEvent);
            data = await owner.saveWorkspace(data); render(); owner.toast("Event deleted.");
          }
        } catch (error) { owner.toast(error.message || "That event action failed.", "error"); }
      });
    } catch (error) { setState("error", error.message || "Could not load events."); }
  }

  if (page === "wedding-info") weddingInfo();
  if (page === "card-studio") cardStudio();
  if (page === "preview-card") cardPreview();
  if (page === "guests") guestsPage();
  if (page === "events") eventsPage();
})();
