const WH = WeddingHub;
const API = WeddingHubAPI;
let data = WH.getData();
let guest = WH.guestByToken(data);
let wedding = data.wedding;
const token = WH.query("token");
const adminPreview = WH.query("preview") === "admin";
if (adminPreview && !guest) guest = { id:"preview", name:"Guest preview", token:"", rsvp:"pending", invitationStatus:"pending", partySize:1 };
const modal = document.getElementById("rsvpModal");
const content = document.getElementById("rsvpContent");

async function loadInvitation() {
  if (token && await API.connect()) {
    try {
      const view = await API.invitation(token);
      data = API.mergeAPI(data, view);
      wedding = data.wedding;
      guest = data.guests.find(item => item.token === token);
      if (!guest) {
        guest = {
          id: view.invitation.id,
          name: view.invitation.guest_name,
          email: view.invitation.guest_email || "",
          category: "Guest",
          invitationStatus: view.invitation.status,
          token,
          rsvp: view.invitation.status === "accepted" ? "attending" : view.invitation.status === "declined" ? "declined" : "pending",
          partySize: 1
        };
        data.guests.push(guest);
      } else if (view.invitation.status !== "pending") {
        guest.invitationStatus = view.invitation.status;
      }
      if (guest.invitationStatus === "sent") {
        guest.invitationStatus = "opened";
        guest.openedAt = new Date().toISOString();
      }
      WH.saveData(data);
    } catch (error) {
      console.warn("Using local invitation fallback:", error);
    }
  }
  if (guest && guest.invitationStatus === "sent") {
    guest.invitationStatus = "opened";
    guest.openedAt = new Date().toISOString();
    WH.saveData(data);
  }
  renderInvitation();
}

function renderInvitation() {
  document.getElementById("inviteHero").style.backgroundImage = `url('${wedding.heroImage}')`;
  document.getElementById("brideName").textContent = wedding.brideName;
  document.getElementById("groomName").textContent = wedding.groomName;
  document.getElementById("inviteMessage").textContent = wedding.message;
  document.getElementById("date").textContent = WH.formatDate(wedding.date).toUpperCase();
  document.getElementById("location").textContent = `${wedding.city} · ${wedding.state}`;
  document.getElementById("verse").textContent = wedding.verse;
  document.getElementById("closingNames").textContent = `${wedding.brideName} & ${wedding.groomName}`;
  const chosen = WH.templates().find(item => item.id === wedding.templateId);
  document.body.classList.add(`invite-tone-${chosen?.tone || 0}`);

  if (!guest) {
    document.getElementById("personalGreeting").textContent = "This invitation link is invalid or has expired.";
    document.getElementById("inviteActions").innerHTML = '<a class="button ivory" href="../index.html">Return to WeddingHub</a>';
  } else if (adminPreview) {
    document.getElementById("personalGreeting").textContent = "Guest preview · this is how your invitation currently appears.";
    document.getElementById("inviteActions").innerHTML = '<a class="button ivory" href="dashboard.html">Return to editor</a>';
    document.getElementById("bottomAccept").hidden = true;
  } else {
    document.getElementById("personalGreeting").textContent = `Dear ${guest.name.split(" ")[0]}, this celebration would not be complete without you.`;
    updateResponseState();
  }

  WH.countdown(wedding.date, value => {
    document.getElementById("countdown").innerHTML = value.passed
      ? "<h3>Today we celebrate love</h3>"
      : [["Days", value.days], ["Hours", value.hours], ["Minutes", value.minutes], ["Seconds", value.seconds]]
          .map(([label, number]) => `<div><strong>${String(number).padStart(2, "0")}</strong><span>${label}</span></div>`).join("");
  });

  const publishedEvents = WH.published(data.events);
  document.getElementById("eventCards").innerHTML = publishedEvents.length ? publishedEvents.map((event, index) => `
    <article><p class="eyebrow">${index === 0 ? "Ceremony" : "Celebration"}</p><span class="event-number">0${index + 1}</span>
    <h2>${WH.escape(event.name)}</h2><p><strong>${WH.formatDate(event.date)} · ${event.time}</strong></p>
    <p>${WH.escape(event.venue)}<br>${WH.escape(event.address)}</p><small>${WH.escape(event.description)}</small>
    <a href="https://maps.google.com/?q=${encodeURIComponent(event.address)}" target="_blank">View location ↗</a></article>`).join("") : '<div class="invitation-empty"><span>✦</span><h2>Celebration details are coming soon</h2><p>The couple will publish the event schedule here.</p></div>';
}

function updateResponseState() {
  if (!guest) return;
  const note = document.getElementById("acceptedNote");
  if (guest.rsvp === "attending") {
    document.getElementById("inviteActions").innerHTML = `<a class="button ivory" href="guest-dashboard.html?token=${guest.token}">Open your guest dashboard</a>`;
    note.textContent = `Your RSVP is confirmed for ${guest.partySize} ${guest.partySize === 1 ? "guest" : "guests"}.`;
  } else if (guest.rsvp === "declined") {
    note.textContent = "Thank you for letting the couple know. You can change your response anytime.";
  }
}

function openRSVP(mode) {
  if (!guest) return;
  content.innerHTML = `<p class="eyebrow">Your response</p><h2>${mode === "attending" ? "Wonderful, we can’t wait!" : "We’ll miss you"}</h2>
    <p>This response stays connected to your private invitation.</p><form id="rsvpForm" class="modal-form">
    <label>Your name<input value="${WH.escape(guest.name)}" disabled></label>
    ${mode === "attending" ? `<label>Number attending<select name="partySize">${[1,2,3,4].map(number => `<option ${guest.partySize === number ? "selected" : ""}>${number}</option>`).join("")}</select></label>` : ""}
    <label>Message for the couple<textarea name="message" rows="3" placeholder="Share a wish or note..."></textarea></label>
    <button class="button primary">Confirm response</button></form>`;
  modal.classList.add("open");
  document.getElementById("rsvpForm").onsubmit = event => submitResponse(event, mode);
}

async function submitResponse(event, mode) {
  event.preventDefault();
  const form = new FormData(event.target);
  const partySize = mode === "attending" ? Number(form.get("partySize") || 1) : 0;
  const message = String(form.get("message") || "").trim();
  try {
    if (API.isOnline()) {
      await API.respond(token, mode === "attending" ? "accept" : "decline");
      if (mode === "attending") {
        await API.updateRSVP(token, "attending", partySize);
        if (message) await API.sendMessage(token, message);
      }
    }
  } catch (error) {
    console.warn("API response sync failed; response remains local:", error);
  }
  guest.rsvp = mode;
  guest.invitationStatus = mode === "attending" ? "accepted" : "declined";
  guest.partySize = partySize;
  if (message) data.messages.unshift({ id: `msg_${Date.now()}`, guestId: guest.id, name: guest.name, message, status: "pending", date: new Date().toISOString().slice(0, 10) });
  WH.saveData(data);
  modal.classList.remove("open");
  updateResponseState();
  WH.toast(mode === "attending" ? "Invitation accepted — your dashboard is unlocked" : "Your response has been saved");
  if (mode === "attending") setTimeout(() => location.href = `guest-dashboard.html?token=${guest.token}`, 600);
}

document.getElementById("acceptButton").addEventListener("click", () => { if(!adminPreview) openRSVP("attending"); });
document.getElementById("declineButton").addEventListener("click", () => { if(!adminPreview) openRSVP("declined"); });
document.getElementById("bottomAccept").addEventListener("click", () => openRSVP("attending"));
document.getElementById("closeModal").addEventListener("click", () => modal.classList.remove("open"));
modal.addEventListener("click", event => { if (event.target === modal) modal.classList.remove("open"); });
loadInvitation();