(() => {
  const API_URL_KEY = "weddinghub_api_url";
  const API_ID_KEY = "weddinghub_api_wedding_id";
  const ADMIN_TOKEN_KEY = "weddinghub_admin_token";
  const baseURL = () => {
      const configured = window.WEDDINGHUB_API_URL || localStorage.getItem(API_URL_KEY);
      if (configured) return configured.replace(/\/$/, "");
      return ["localhost", "127.0.0.1"].includes(location.hostname) ? "http://localhost:8080" : "";
    };
  let online = false;

  async function request(path, options = {}) {
    const endpoint = baseURL();
    if (!endpoint) throw new Error("API URL is not configured");
    const response = await fetch(endpoint + path, {
      ...options,
      headers: { "Content-Type": "application/json", ...(options.headers || {}) }
    });
    const body = response.status === 204 ? null : await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body?.error || `API request failed (${response.status})`);
    return body;
  }

  async function connect() {
    try { await request("/healthz"); online = true; return true; }
    catch (_) { online = false; return false; }
  }

  // Admin capability token issued once when the wedding is created.
  function adminToken() { return localStorage.getItem(ADMIN_TOKEN_KEY) || ""; }
  function storeAdminToken(token) { if (token) localStorage.setItem(ADMIN_TOKEN_KEY, token); }
  function authHeader(token) {
    const value = token || adminToken();
    return value ? { Authorization: `Bearer ${value}` } : {};
  }

  // User sessions issued by /api/auth/login/verify. Kept apart from the admin
  // capability token: one browser can hold a session and no wedding admin rights.
  const SESSION_KEY = "weddinghub_session_token";
  function sessionToken() { return localStorage.getItem(SESSION_KEY) || ""; }
  function sessionHeader() {
    const value = sessionToken();
    return value ? { Authorization: `Bearer ${value}` } : {};
  }
  function storeSession(session) {
    if (session?.session_token) localStorage.setItem(SESSION_KEY, session.session_token);
    if (session?.user) localStorage.setItem("weddinghub_user", JSON.stringify({
      fullName: session.user.full_name, email: session.user.email
    }));
  }
  async function logoutAccount() {
    if (sessionToken()) {
      try { await request("/api/auth/logout", { method: "POST", headers: sessionHeader() }); } catch (_) {}
    }
    localStorage.removeItem(SESSION_KEY);
    localStorage.removeItem("weddinghub_user");
    localStorage.removeItem("weddinghub_local_profile");
    localStorage.removeItem(ADMIN_TOKEN_KEY);
    sessionStorage.removeItem("weddinghub_invitation_token");
  }
  function registerAccount(email, fullName, password) {
    return request("/api/auth/register", { method: "POST", body: JSON.stringify({ email, full_name: fullName, password }) });
  }
  function requestLoginCode(email, password) {
    return request("/api/auth/login/start", { method: "POST", body: JSON.stringify({ email, password }) });
  }
  function verifyLoginCode(email, code) {
    return request("/api/auth/login/verify", { method: "POST", body: JSON.stringify({ email, code }) });
  }
  function currentUser() { return request("/api/auth/me", { headers: sessionHeader() }); }

  function toAPI(data) {
    const w = data.wedding;
    return {
      slug: w.slug, title: `${w.brideName} & ${w.groomName}`, partner_one: w.brideName,
      partner_two: w.groomName, date: new Date(w.date).toISOString(), status: w.status,
      venue: w.venue, address: w.address, city: w.city, state: w.state, country: w.country,
      message: w.message, verse: w.verse, dress_code: w.dressCode, hero_image: w.heroImage,
      template_id: w.templateId, card_config: w.cardConfig || null, committee_roles: data.committeeRoles || [],
      admins: [], guests: [], invitations: [], rsvps: [], guest_messages: [],
      events: data.events.map(e => ({ id: apiID(e.id), name: e.name, description: e.description,
        starts_at: new Date(`${e.date}T${e.time || "00:00"}:00`).toISOString(), venue: e.venue,
        address: e.address, status: e.status })),
      photos: data.photos.map((p, i) => ({ id: apiID(p.id), url: p.url, alt_text: p.caption,
        caption: p.caption, sort_order: i + 1, status: p.status })),
      story_sections: data.stories.map((s, i) => ({ id: apiID(s.id), title: s.title,
        body: s.content, sort_order: s.order || i + 1, status: s.status })),
      announcements: data.announcements.map(a => ({ id: apiID(a.id), title: a.title, body: a.message,
        audience: a.audience || "public", author_name: a.authorName || "",
        created_at: a.createdAt ? new Date(a.createdAt).toISOString() : null,
        published_at: a.status === "published" ? new Date(`${a.date}T12:00:00Z`).toISOString() : null,
        status: a.status }))
    };
  }

  function apiID(value) {
    return /^[a-f0-9]{32}$/.test(value || "") ? value : "";
  }

  function mergeAPI(data, source) {
    const w = source.wedding || source;
    data.wedding = { ...data.wedding, id: w.id, slug: w.slug, brideName: w.partner_one,
      groomName: w.partner_two, date: w.date, status: data.wedding.status,
      venue: w.venue || data.wedding.venue, address: w.address || data.wedding.address,
      city: w.city || data.wedding.city, state: w.state || data.wedding.state,
      country: w.country || data.wedding.country, message: w.message || data.wedding.message,
      verse: w.verse || data.wedding.verse, dressCode: w.dress_code || data.wedding.dressCode,
      heroImage: w.hero_image || data.wedding.heroImage, templateId: w.template_id || data.wedding.templateId,
      cardConfig: w.card_config || data.wedding.cardConfig };
    if (w.committee_roles) {
      data.committeeRoles = w.committee_roles.map(r => ({ id: r.id, name: r.name, description: r.description || "" }));
    }
    if (w.events) data.events = w.events.map(e => ({ id: e.id, name: e.name, description: e.description,
      date: String(e.starts_at).slice(0, 10), time: new Date(e.starts_at).toLocaleTimeString([], {hour:"2-digit",minute:"2-digit",hour12:false}),
      venue: e.venue, address: e.address, status: e.status }));
    if (w.photos) data.photos = w.photos.map(p => ({ id: p.id, url: p.url, caption: p.caption || p.alt_text, status: p.status }));
    if (w.story_sections) data.stories = w.story_sections.map((s, i) => ({ id: s.id, year: data.stories[i]?.year || "Our story",
      title: s.title, content: s.body, order: s.sort_order, status: s.status }));
    if (w.announcements) data.announcements = w.announcements.map(a => ({ id: a.id, title: a.title,
      message: a.body, audience: a.audience || "public", authorName: a.author_name || "", createdAt: a.created_at || "",
      date: String(a.published_at || a.created_at || new Date().toISOString()).slice(0, 10), status: a.status }));
    if (w.committee_members) {
      data.committeeMembers = w.committee_members.map((member, index) => {
        const local = data.committeeMembers?.find(item => item.apiInvitationId === member.invitation_id);
        return { id: local?.id || `cm_${member.id || index}`, apiMembershipId: member.id, apiInvitationId: member.invitation_id,
          name: local?.name || member.name, email: local?.email || member.email || "", title: local?.title || member.title || "",
          phone: member.phone || "", invitationStatus: data.committeeMembers?.find(item => item.apiInvitationId === member.invitation_id)?.invitationStatus || "accepted",
          token: local?.token || "", joinedAt: member.joined_at || "" };
      });
    }
    if (w.planning_tasks) data.planningTasks = w.planning_tasks.map(t => ({ id: t.id, title: t.title, details: t.details || "",
      assignedTo: t.assigned_to || "", dueOn: t.due_on || "", status: t.status, createdBy: t.created_by || "" }));
    if (w.committee_chat) data.committeeChat = w.committee_chat.map(c => ({ id: c.id, authorName: c.author_name,
      authorRole: c.author_role, body: c.body, createdAt: c.created_at }));
    if (w.invitations) {
      w.invitations.forEach(invitation => {
        if (invitation.type === "committee") {
          const member = data.committeeMembers?.find(item => item.apiInvitationId === invitation.id || item.email === invitation.guest_email);
          if (!member) return;
          member.apiInvitationId = invitation.id;
          member.invitationStatus = invitation.status;
        } else {
          const guest = data.guests.find(item => item.apiInvitationId === invitation.id || item.email === invitation.guest_email);
          if (!guest) return;
          guest.apiInvitationId = invitation.id;
          if (invitation.status === "accepted") { guest.invitationStatus = "accepted"; guest.rsvp = "attending"; }
          if (invitation.status === "declined") { guest.invitationStatus = "declined"; guest.rsvp = "declined"; }
          const response = (w.rsvps || []).find(item => item.invitation_id === invitation.id);
          if (response) { guest.rsvp = response.status === "not_attending" ? "declined" : response.status; guest.partySize = response.party_size; }
        }
      });
    }
    if (w.guest_messages) {
      const invitations = w.invitations || [];
      const existingStatus = new Map(data.messages.map(item => [item.id, item.status]));
      data.messages = w.guest_messages.map(message => {
        const invitation = invitations.find(item => item.id === message.invitation_id);
        return { id: message.id, guestId: message.invitation_id, name: invitation?.guest_name || "Invited guest",
          message: message.body, status: existingStatus.get(message.id) || "pending", date: String(message.created_at).slice(0, 10) };
      });
    }
    WeddingHub.saveData(data);
    return data;
  }

  async function bootstrap(data) {
    if (!await connect()) return { online: false, data };
    let weddingID = localStorage.getItem(API_ID_KEY);
    const adminHeaders = authHeader();
    try {
      if (weddingID && adminToken()) {
        try {
          const remote = await request(`/api/weddings/${weddingID}`, { headers: adminHeaders });
          return { online: true, data: mergeAPI(data, remote), weddingID };
        } catch (error) {
          console.info("Stored API wedding is no longer available; re-bootstrapping.", error);
          localStorage.removeItem(API_ID_KEY);
          weddingID = null;
        }
      }
      const weddings = await request("/api/weddings");
      let remote = weddings.find(w => w.slug === data.wedding.slug);
      if (!remote) {
        const created = await request("/api/weddings", { method: "POST", body: JSON.stringify(toAPI(data)) });
        storeAdminToken(created.admin_token);
        remote = created.wedding;
      }
      weddingID = remote.id; localStorage.setItem(API_ID_KEY, weddingID);
      for (const guest of data.guests) {
        const created = await createInvitation(weddingID, guest);
        guest.token = created.token;
        guest.apiInvitationId = created.invitation.id;
        if (guest.rsvp === "attending") {
          await respond(guest.token, "accept", "guest");
          await updateRSVP(guest.token, "attending", guest.partySize);
        } else if (guest.rsvp === "declined") await respond(guest.token, "decline");
      }
      for (const member of data.committeeMembers || []) {
        const created = await createInvitation(weddingID, { ...member, type: "committee", committeeTitle: member.title });
        member.token = created.token;
        member.apiInvitationId = created.invitation.id;
        if (member.invitationStatus === "accepted") await respond(member.token, "accept", "committee");
        else if (member.invitationStatus === "declined") await respond(member.token, "decline");
      }
      WeddingHub.saveData(data);
      return { online: true, data: mergeAPI(data, remote), weddingID };
    } catch (error) { console.warn("WeddingHub API bootstrap failed:", error); return { online: false, data, error }; }
  }

  async function saveWedding(data) {
    if (!online && !await connect()) return null;
    const id = localStorage.getItem(API_ID_KEY);
    if (!id) return (await bootstrap(data)).data;
    const remote = await request(`/api/weddings/${id}`, { method: "PUT", body: JSON.stringify(toAPI(data)), headers: authHeader() });
    return mergeAPI(data, remote);
  }
  async function createInvitation(id, guest) {
    return request(`/api/weddings/${id}/invitations`, { method: "POST", headers: authHeader(), body: JSON.stringify({
      type: guest.type || "guest",
      guest_name: guest.name,
      guest_email: guest.email || "",
      guest_phone: guest.phone || "",
      committee_title: guest.committeeTitle || guest.title || "",
      max_party_size: Math.max(1, guest.partySize || 1)
    }) });
  }
  async function addGuest(guest) { const id = localStorage.getItem(API_ID_KEY); if (!online || !id) return null; return createInvitation(id, guest); }
  async function respond(token, decision, role) {
    const body = decision === "accept" && role ? JSON.stringify({ role }) : undefined;
    return request(`/api/invitations/${encodeURIComponent(token)}/${decision}`, { method: "POST", body });
  }
  async function updateRSVP(token, status, partySize) { return request(`/api/guest/${encodeURIComponent(token)}/rsvp`, { method: "PUT", body: JSON.stringify({ status, party_size: partySize }) }); }
  async function invitation(token) { return request(`/api/invitations/${encodeURIComponent(token)}`); }
  async function dashboard(token) { return request(`/api/guest/${encodeURIComponent(token)}/dashboard`); }
  async function sendMessage(token, message) { return request(`/api/guest/${encodeURIComponent(token)}/messages`, { method: "POST", body: JSON.stringify({ message }) }); }

  async function adminOverview() {
    const id = localStorage.getItem(API_ID_KEY);
    if (!online || !id || !adminToken()) return null;
    return request(`/api/weddings/${id}/admin/overview`, { headers: authHeader() });
  }
  async function adminRoster() {
    const id = localStorage.getItem(API_ID_KEY);
    if (!online || !id || !adminToken()) return null;
    return request(`/api/weddings/${id}/admin/roster`, { headers: authHeader() });
  }

  function committeeToken() { return sessionStorage.getItem("weddinghub_invitation_token") || adminToken() || ""; }
  async function committeeDashboard(weddingID, token) {
    return request(`/api/weddings/${encodeURIComponent(weddingID)}/committee/dashboard`, { headers: authHeader(token || committeeToken()) });
  }
  async function committeeChat(weddingID, since, token) {
    const suffix = since ? `?since=${encodeURIComponent(since)}` : "";
    return request(`/api/weddings/${encodeURIComponent(weddingID)}/committee/chat${suffix}`, { headers: authHeader(token || committeeToken()) });
  }
  async function sendCommitteeMessage(weddingID, message, token) {
    return request(`/api/weddings/${encodeURIComponent(weddingID)}/committee/chat`, { method: "POST", headers: authHeader(token || committeeToken()), body: JSON.stringify({ message }) });
  }
  async function createTask(weddingID, task, token) {
    return request(`/api/weddings/${encodeURIComponent(weddingID)}/committee/tasks`, { method: "POST", headers: authHeader(token || committeeToken()), body: JSON.stringify(task) });
  }
  async function updateTask(weddingID, taskID, task, token) {
    return request(`/api/weddings/${encodeURIComponent(weddingID)}/committee/tasks/${encodeURIComponent(taskID)}`, { method: "PUT", headers: authHeader(token || committeeToken()), body: JSON.stringify(task) });
  }
  async function deleteTask(weddingID, taskID, token) {
    return request(`/api/weddings/${encodeURIComponent(weddingID)}/committee/tasks/${encodeURIComponent(taskID)}`, { method: "DELETE", headers: authHeader(token || committeeToken()) });
  }
  async function createAnnouncement(weddingID, announcement, token) {
    return request(`/api/weddings/${encodeURIComponent(weddingID)}/committee/announcements`, { method: "POST", headers: authHeader(token || committeeToken()), body: JSON.stringify(announcement) });
  }
  async function updateAnnouncement(weddingID, announcementID, announcement, token) {
    return request(`/api/weddings/${encodeURIComponent(weddingID)}/committee/announcements/${encodeURIComponent(announcementID)}`, { method: "PUT", headers: authHeader(token || committeeToken()), body: JSON.stringify(announcement) });
  }
  async function deleteAnnouncement(weddingID, announcementID, token) {
    return request(`/api/weddings/${encodeURIComponent(weddingID)}/committee/announcements/${encodeURIComponent(announcementID)}`, { method: "DELETE", headers: authHeader(token || committeeToken()) });
  }
  async function saveCardConfig(weddingID, config) {
    const id = weddingID || localStorage.getItem(API_ID_KEY);
    if (!online || !id) return null;
    return request(`/api/weddings/${encodeURIComponent(id)}/card`, { method: "PUT", headers: authHeader(), body: JSON.stringify(config) });
  }
  async function getCardConfig(weddingID) {
    const id = weddingID || localStorage.getItem(API_ID_KEY);
    if (!online || !id) return null;
    return request(`/api/weddings/${encodeURIComponent(id)}/card`);
  }
  async function createCommitteeRole(weddingID, role) {
    const id = weddingID || localStorage.getItem(API_ID_KEY);
    if (!online || !id) return null;
    return request(`/api/weddings/${encodeURIComponent(id)}/committee/roles`, { method: "POST", headers: authHeader(), body: JSON.stringify(role) });
  }
  async function deleteCommitteeRole(weddingID, roleID) {
    const id = weddingID || localStorage.getItem(API_ID_KEY);
    if (!online || !id) return null;
    return request(`/api/weddings/${encodeURIComponent(id)}/committee/roles/${encodeURIComponent(roleID)}`, { method: "DELETE", headers: authHeader() });
  }
  async function updateCommitteeMember(weddingID, memberID, member) {
    const id = weddingID || localStorage.getItem(API_ID_KEY);
    if (!online || !id) return null;
    return request(`/api/weddings/${encodeURIComponent(id)}/committee/members/${encodeURIComponent(memberID)}`, { method: "PUT", headers: authHeader(), body: JSON.stringify(member) });
  }
  async function deleteCommitteeMember(weddingID, memberID) {
    const id = weddingID || localStorage.getItem(API_ID_KEY);
    if (!online || !id) return null;
    return request(`/api/weddings/${encodeURIComponent(id)}/committee/members/${encodeURIComponent(memberID)}`, { method: "DELETE", headers: authHeader() });
  }
  function isOnline() { return online; }

  window.WeddingHubAPI = { connect, bootstrap, saveWedding, addGuest, respond, updateRSVP, invitation, dashboard, sendMessage,
    adminOverview, adminRoster, committeeDashboard, committeeChat, sendCommitteeMessage, createTask, updateTask, deleteTask,
    createAnnouncement, updateAnnouncement, deleteAnnouncement,
    saveCardConfig, getCardConfig, createCommitteeRole, deleteCommitteeRole, updateCommitteeMember, deleteCommitteeMember,
    isOnline, mergeAPI, toAPI, baseURL, adminToken, storeAdminToken,
    sessionToken, sessionHeader, storeSession, logoutAccount, registerAccount, requestLoginCode, verifyLoginCode, currentUser };
})();