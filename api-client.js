(() => {
  const API_URL_KEY = "weddinghub_api_url";
  const API_ID_KEY = "weddinghub_api_wedding_id";
  const baseURL = () => (localStorage.getItem(API_URL_KEY) || "http://localhost:8080").replace(/\/$/, "");
  let online = false;

  async function request(path, options = {}) {
    const response = await fetch(baseURL() + path, {
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

  function toAPI(data) {
    const w = data.wedding;
    return {
      slug: w.slug, title: `${w.brideName} & ${w.groomName}`, partner_one: w.brideName,
      partner_two: w.groomName, date: new Date(w.date).toISOString(), status: w.status,
      venue: w.venue, address: w.address, city: w.city, state: w.state, country: w.country,
      message: w.message, verse: w.verse, dress_code: w.dressCode, hero_image: w.heroImage,
      template_id: w.templateId, admins: [], guests: [], invitations: [], rsvps: [], guest_messages: [],
      events: data.events.map(e => ({ id: apiID(e.id), name: e.name, description: e.description,
        starts_at: new Date(`${e.date}T${e.time || "00:00"}:00`).toISOString(), venue: e.venue,
        address: e.address, status: e.status })),
      photos: data.photos.map((p, i) => ({ id: apiID(p.id), url: p.url, alt_text: p.caption,
        caption: p.caption, sort_order: i + 1, status: p.status })),
      story_sections: data.stories.map((s, i) => ({ id: apiID(s.id), title: s.title,
        body: s.content, sort_order: s.order || i + 1, status: s.status })),
      announcements: data.announcements.map(a => ({ id: apiID(a.id), title: a.title, body: a.message,
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
      heroImage: w.hero_image || data.wedding.heroImage, templateId: w.template_id || data.wedding.templateId };
    if (w.events) data.events = w.events.map(e => ({ id: e.id, name: e.name, description: e.description,
      date: String(e.starts_at).slice(0, 10), time: new Date(e.starts_at).toLocaleTimeString([], {hour:"2-digit",minute:"2-digit",hour12:false}),
      venue: e.venue, address: e.address, status: e.status }));
    if (w.photos) data.photos = w.photos.map(p => ({ id: p.id, url: p.url, caption: p.caption || p.alt_text, status: p.status }));
    if (w.story_sections) data.stories = w.story_sections.map((s, i) => ({ id: s.id, year: data.stories[i]?.year || "Our story",
      title: s.title, content: s.body, order: s.sort_order, status: s.status }));
    if (w.announcements) data.announcements = w.announcements.map(a => ({ id: a.id, title: a.title,
      message: a.body, date: String(a.published_at || new Date().toISOString()).slice(0, 10), status: a.status }));
    WeddingHub.saveData(data);
    return data;
  }

  async function bootstrap(data) {
    if (!await connect()) return { online: false, data };
    let weddingID = localStorage.getItem(API_ID_KEY);
    try {
      if (weddingID) {
        try {
          const remote = await request(`/api/weddings/${weddingID}`);
          return { online: true, data: mergeAPI(data, remote), weddingID };
        } catch (error) {
          console.info("Stored API wedding is no longer available; re-bootstrapping.", error);
          localStorage.removeItem(API_ID_KEY);
          weddingID = null;
        }
      }
      const weddings = await request("/api/weddings");
      let remote = weddings.find(w => w.slug === data.wedding.slug);
      if (!remote) remote = await request("/api/weddings", { method: "POST", body: JSON.stringify(toAPI(data)) });
      weddingID = remote.id; localStorage.setItem(API_ID_KEY, weddingID);
      for (const guest of data.guests) {
        const created = await createInvitation(weddingID, guest);
        guest.token = created.token;
        guest.apiInvitationId = created.invitation.id;
        if (guest.rsvp === "attending") {
          await respond(guest.token, "accept");
          await updateRSVP(guest.token, "attending", guest.partySize);
        } else if (guest.rsvp === "declined") await respond(guest.token, "decline");
      }
      WeddingHub.saveData(data);
      return { online: true, data: mergeAPI(data, remote), weddingID };
    } catch (error) { console.warn("WeddingHub API bootstrap failed:", error); return { online: false, data, error }; }
  }

  async function saveWedding(data) {
    if (!online && !await connect()) return null;
    const id = localStorage.getItem(API_ID_KEY);
    if (!id) return (await bootstrap(data)).data;
    const remote = await request(`/api/weddings/${id}`, { method: "PUT", body: JSON.stringify(toAPI(data)) });
    return mergeAPI(data, remote);
  }
  async function createInvitation(id, guest) { return request(`/api/weddings/${id}/invitations`, { method: "POST", body: JSON.stringify({ guest_name: guest.name, guest_email: guest.email || "", max_party_size: Math.max(1, guest.partySize || 1) }) }); }
  async function addGuest(guest) { const id = localStorage.getItem(API_ID_KEY); if (!online || !id) return null; return createInvitation(id, guest); }
  async function respond(token, decision) { return request(`/api/invitations/${encodeURIComponent(token)}/${decision}`, { method: "POST" }); }
  async function updateRSVP(token, status, partySize) { return request(`/api/guest/${encodeURIComponent(token)}/rsvp`, { method: "PUT", body: JSON.stringify({ status, party_size: partySize }) }); }
  async function invitation(token) { return request(`/api/invitations/${encodeURIComponent(token)}`); }
  async function dashboard(token) { return request(`/api/guest/${encodeURIComponent(token)}/dashboard`); }
  async function sendMessage(token, message) { return request(`/api/guest/${encodeURIComponent(token)}/messages`, { method: "POST", body: JSON.stringify({ message }) }); }
  function isOnline() { return online; }

  window.WeddingHubAPI = { connect, bootstrap, saveWedding, addGuest, respond, updateRSVP, invitation, dashboard, sendMessage, isOnline, mergeAPI, baseURL };
})();