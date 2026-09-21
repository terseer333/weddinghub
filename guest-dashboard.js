const WH = WeddingHub;
const API = WeddingHubAPI;
let data = WH.getData();
let guest = WH.guestByToken(data);
let wedding = data.wedding;
const token = WH.query("token");

// Resolve this guest from the API by their invitation token. An accepted guest can
// open their private space directly; an unaccepted link stays behind the gate. Any
// API failure propagates so the page can show a blocking error instead of falling
// back to local data.
async function resolveGuestFromAPI() {
  if (!token) return;
  const invitationView = await API.invitation(token);
  const invitation = invitationView?.invitation;
  if (!invitation || invitation.type === "committee") return;
  data = API.mergeAPI(data, invitationView);
  wedding = data.wedding;
  guest = WH.guestByToken(data) || guest;
  if (!guest) {
    guest = {
      id: invitation.id, name: invitation.guest_name, type: "guest",
      email: invitation.guest_email || "", category: "Guest",
      invitationStatus: invitation.status, token,
      rsvp: invitation.status === "accepted" ? "attending" : invitation.status === "declined" ? "declined" : "pending",
      partySize: invitation.max_party_size > 0 ? invitation.max_party_size : 1
    };
    data.guests.push(guest);
  } else if (invitation.status !== "pending") {
    guest.invitationStatus = invitation.status;
    if (invitation.status === "accepted") guest.rsvp = "attending";
    if (invitation.status === "declined") guest.rsvp = "declined";
  }
  WH.saveData(data);
  if (guest.rsvp === "attending") {
    const view = await API.dashboard(token);
    data = API.mergeAPI(data, view);
    wedding = data.wedding;
    if (view.rsvp) guest.partySize = view.rsvp.party_size;
    WH.saveData(data);
  }
}

async function initializeGuestDashboard() {
  try {
    await resolveGuestFromAPI();
  } catch (error) {
    API.showFatalError(error.status === 404 ? "This invitation link is invalid or has expired." : error.message);
    return;
  }
  if (!guest || guest.rsvp !== "attending") return showAccessGate();
  document.getElementById("accessGate").style.display = "none";
  renderGuestDashboard();
}

function showAccessGate() {
  document.getElementById("guestExperience").style.display = "none";
  document.getElementById("accessGate").innerHTML = `<div><span class="brand-mark">W</span><p class="eyebrow">Private wedding experience</p>
    <h1>${guest ? "Accept your invitation to enter." : "This guest link is invalid."}</h1>
    <p>Only invited guests who are attending can access this private space.</p>
    <a class="button primary" href="${guest ? `event.html?token=${guest.token}` : "../index.html"}">${guest ? "Return to invitation" : "Return home"}</a></div>`;
}

function renderGuestDashboard() {
  const visibility = { story: WH.published(data.stories).length > 0, gallery: WH.published(data.photos).length > 0, events: WH.published(data.events).length > 0, announcements: WH.publicAnnouncements(data.announcements).length > 0 };
  Object.entries(visibility).forEach(([id, visible]) => { const section=document.getElementById(id); if(section)section.hidden=!visible; const link=document.querySelector(`.guest-links a[href="#${id}"]`); if(link)link.hidden=!visible; });
  document.getElementById("guestHero").style.backgroundImage = `url('${wedding.heroImage}')`;
  document.getElementById("detailImage").style.backgroundImage = `url('${data.photos[2]?.url || wedding.heroImage}')`;
  document.getElementById("welcomeName").textContent = `Welcome, ${guest.name.split(" ")[0]}`;
  document.getElementById("heroBride").textContent = wedding.brideName;
  document.getElementById("heroGroom").textContent = wedding.groomName;
  document.getElementById("guestDate").textContent = `${WH.formatDate(wedding.date)} · ${wedding.city}, ${wedding.state}`;
  document.getElementById("weddingMessage").textContent = wedding.message;
  document.getElementById("signature").textContent = `${wedding.brideName} & ${wedding.groomName}`;
  document.getElementById("dressCode").textContent = wedding.dressCode;
  document.getElementById("fullLocation").textContent = `${wedding.venue}, ${wedding.city}`;
  document.getElementById("rsvpSummary").textContent = `Attending · Party of ${guest.partySize}`;
  document.getElementById("footerDate").textContent = `${WH.formatDate(wedding.date)} · ${wedding.city}`;
  document.getElementById("myInvitation").href = `event.html?token=${guest.token}`;

  WH.countdown(wedding.date, value => {
    document.getElementById("heroCountdown").innerHTML = value.passed ? "<strong>Today we celebrate love</strong>" :
      [["DAYS", value.days], ["HRS", value.hours], ["MIN", value.minutes], ["SEC", value.seconds]]
        .map(([label, number]) => `<div><b>${String(number).padStart(2, "0")}</b><span>${label}</span></div>`).join("");
  });

  document.getElementById("storyTimeline").innerHTML = WH.published(data.stories).sort((a,b) => a.order - b.order).map((story, index) => `
    <article class="${index % 2 ? "right" : ""}"><div class="story-year">${story.year}</div><span></span><div class="story-copy">
    ${story.image ? `<img class="story-image" src="${story.image}" alt="${WH.escape(story.title)}" style="width:100%;border-radius:14px;margin-bottom:14px;display:block">` : ''}<small>CHAPTER ${String(index + 1).padStart(2, "0")}</small><h3>${WH.escape(story.title)}</h3><p>${WH.escape(story.content)}</p></div></article>`).join("");
  renderGallery();
  document.getElementById("guestEventList").innerHTML = WH.published(data.events).map((event, index) => {
    const date = new Date(`${event.date}T12:00`);
    return `<article><div class="event-index">0${index + 1}</div><div class="event-date-block"><strong>${date.getDate()}</strong>
      <span>${date.toLocaleString("en", {month:"short"}).toUpperCase()}<br>${date.getFullYear()}</span></div><div class="event-info">
      <small>${event.time}</small><h3>${WH.escape(event.name)}</h3><p>${WH.escape(event.venue)}<br>${WH.escape(event.address)}</p></div>
      <a href="https://maps.google.com/?q=${encodeURIComponent(event.address)}" target="_blank">Map ↗</a></article>`;
  }).join("");
  const announcements = WH.publicAnnouncements(data.announcements);
  document.getElementById("guestAnnouncements").innerHTML = announcements.length ? announcements.map(item => `
    <article><small>${WH.formatDate(item.date)}</small><h3>${WH.escape(item.title)}</h3><p>${WH.escape(item.message)}</p></article>`).join("") : "<p>No announcements yet.</p>";
}

function renderGallery() {
  const photos = WH.published(data.photos);
  if (!photos.length) return;
  let index = 0;
  const show = () => {
    document.getElementById("galleryStage").innerHTML = photos.map((photo, photoIndex) => `<figure class="${photoIndex === index ? "active" : ""}">
      <img src="${photo.url}" alt="${WH.escape(photo.caption)}"><figcaption><span>${String(photoIndex + 1).padStart(2, "0")}</span>${WH.escape(photo.caption)}</figcaption></figure>`).join("");
    document.getElementById("photoPosition").textContent = `${String(index + 1).padStart(2, "0")} / ${String(photos.length).padStart(2, "0")}`;
  };
  show();
  const stage = document.getElementById("galleryStage");
  const next = () => { index = (index + 1) % photos.length; show(); };
  const previous = () => { index = (index - 1 + photos.length) % photos.length; show(); };
  document.getElementById("nextPhoto").onclick = next;
  document.getElementById("prevPhoto").onclick = previous;
  let touchStart = 0;
  stage.ontouchstart = event => { touchStart = event.changedTouches[0].screenX; };
  stage.ontouchend = event => { const distance = event.changedTouches[0].screenX - touchStart; if (Math.abs(distance) > 45) distance < 0 ? next() : previous(); };
  stage.onclick = () => openLightbox(photos[index]);
  window.clearInterval(window.weddingGalleryTimer);
  window.weddingGalleryTimer = window.setInterval(next, 5500);
}

function openLightbox(photo) {
  let dialog = document.getElementById("photoLightbox");
  if (!dialog) {
    dialog = document.createElement("dialog");
    dialog.id = "photoLightbox";
    dialog.className = "photo-lightbox";
    dialog.innerHTML = '<button type="button" aria-label="Close full-screen photo">×</button><img><p></p>';
    document.body.appendChild(dialog);
    dialog.querySelector("button").onclick = () => dialog.close();
    dialog.onclick = event => { if (event.target === dialog) dialog.close(); };
  }
  dialog.querySelector("img").src = photo.url;
  dialog.querySelector("img").alt = photo.caption || "Wedding photo";
  dialog.querySelector("p").textContent = photo.caption || "";
  dialog.showModal();
}

document.getElementById("guestMenu").onclick = () => document.querySelector(".guest-links").classList.toggle("open");
document.querySelectorAll(".guest-links a").forEach(link => link.onclick = () => document.querySelector(".guest-links").classList.remove("open"));
document.getElementById("changeRSVP").onclick = () => location.href = `event.html?token=${guest?.token || ""}`;
document.getElementById("rsvpPill").onclick = () => location.href = `event.html?token=${guest?.token || ""}`;
document.getElementById("wishForm").onsubmit = async event => {
  event.preventDefault();
  const message = document.getElementById("wishMessage").value.trim();
  if (!message) return;
  try { await API.sendMessage(token, message); }
  catch (error) {
    console.warn("Message send failed:", error);
    WH.toast("We couldn't send your message. Please try again.");
    return;
  }
  data.messages.unshift({ id: `msg_${Date.now()}`, guestId: guest.id, name: guest.name, message, status: "pending", date: new Date().toISOString().slice(0, 10) });
  WH.saveData(data);
  event.target.reset();
  WH.toast("Your wish was sent to the couple");
};
initializeGuestDashboard();