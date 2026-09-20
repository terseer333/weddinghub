const WH = WeddingHub;
const API = WeddingHubAPI;
let data = WH.getData();
let guest = WH.guestByToken(data) || null;
let member = WH.committeeMemberByToken(data) || null;
let wedding = data.wedding;
const token = WH.query("token");
const adminPreview = WH.query("preview") === "admin";
if (adminPreview && !guest) guest = { id: "preview", name: "Guest preview", token: "", rsvp: "pending", invitationStatus: "pending", partySize: 1, type: "guest" };
const modal = document.getElementById("rsvpModal");
const content = document.getElementById("rsvpContent");

// The role this link was issued for: committee members live in data.committeeMembers,
// guests live in data.guests. Online invitations carry a validated type from the API.
function inviteeType() {
  if (member) return "committee";
  return guest?.type === "committee" ? "committee" : "guest";
}
function invitee() { return member || guest; }

async function loadInvitation() {
  if (token && await API.connect()) {
    try {
      const view = await API.invitation(token);
      data = API.mergeAPI(data, view);
      wedding = data.wedding;
      const invitationType = view.invitation?.type === "committee" ? "committee" : (view.invitation?.type || "guest");
      if (invitationType === "committee") {
        member = data.committeeMembers.find(item => item.token === token);
        if (!member) {
          member = {
            id: view.invitation.id, name: view.invitation.guest_name, title: view.invitation.committee_title || "",
            email: view.invitation.guest_email || "", invitationStatus: view.invitation.status, token, type: "committee",
            apiInvitationId: view.invitation.id
          };
          data.committeeMembers = data.committeeMembers || [];
          data.committeeMembers.push(member);
        } else {
          if (view.invitation.status !== "pending") member.invitationStatus = view.invitation.status;
        }
      } else {
        guest = data.guests.find(item => item.token === token);
        if (!guest) {
          guest = {
            id: view.invitation.id, name: view.invitation.guest_name, type: "guest",
            email: view.invitation.guest_email || "", category: "Guest",
            invitationStatus: view.invitation.status, token,
            rsvp: view.invitation.status === "accepted" ? "attending" : view.invitation.status === "declined" ? "declined" : "pending",
            partySize: view.invitation.max_party_size > 0 ? view.invitation.max_party_size : 1
          };
          data.guests.push(guest);
        } else if (view.invitation.status !== "pending") {
          guest.invitationStatus = view.invitation.status;
        }
      }
      if ((guest || member)?.invitationStatus === "sent") {
        (guest || member).invitationStatus = "opened";
        (guest || member).openedAt = new Date().toISOString();
      }
      WH.saveData(data);
    } catch (error) {
      console.warn("Using local invitation fallback:", error);
    }
  } else if ((guest || member)?.invitationStatus === "sent") {
    (guest || member).invitationStatus = "opened";
    (guest || member).openedAt = new Date().toISOString();
    WH.saveData(data);
  }
  startInvitationFlow();
}

// Every invitation link asks which role the person is joining as before the
// invitation can be accepted. The selected role must match the invitation type.
function startInvitationFlow() {
  if (adminPreview) return renderInvitation();
  const inviteePerson = invitee();
  if (!inviteePerson) return renderInvitation();
  if (!token) return renderInvitation();
  const role = inviteeType();
  if ((role === "committee" && member?.invitationStatus === "accepted") ||
      (role === "guest" && guest?.rsvp === "attending")) return renderInvitation();
  const selected = WeddingRoleSelector.selectedRole();
  if (selected === role) return renderInvitation();
  WeddingRoleSelector.show({ invitationType: role, invitedName: inviteePerson.name, onSelected: () => renderInvitation() });
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

  const inviteePerson = invitee();
  if (!guest && !member) {
    document.getElementById("personalGreeting").textContent = "This invitation link is invalid or has expired.";
    document.getElementById("inviteActions").innerHTML = '<a class="button ivory" href="../index.html">Return to WeddingHub</a>';
  } else if (adminPreview) {
    document.getElementById("personalGreeting").textContent = "Guest preview · this is how your invitation currently appears.";
    document.getElementById("inviteActions").innerHTML = '<a class="button ivory" href="dashboard.html#designs">Return to editor</a>';
    document.getElementById("bottomAccept").hidden = true;
  } else if (member) {
    document.getElementById("personalGreeting").textContent = `Dear ${member.name.split(" ")[0]}, you're invited to help plan and organize this wedding.`;
    document.getElementById("bottomAccept").hidden = false;
    updateResponseState();
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

  const cardMount = document.getElementById("invitationCardMount");
  if (cardMount && window.WeddingInvitation) {
    cardMount.innerHTML = WeddingInvitation.card(data);
  }
  const downloadBtn = document.getElementById("downloadCardBtn");
  if (downloadBtn && window.WeddingInvitation) {
    downloadBtn.onclick = async () => {
      try {
        downloadBtn.disabled = true;
        downloadBtn.textContent = "Generating card...";
        const blob = await WeddingInvitation.imageBlob(data);
        const link = document.createElement("a");
        link.href = URL.createObjectURL(blob);
        link.download = `${wedding.slug || "wedding"}-invitation-card.png`;
        link.click();
        URL.revokeObjectURL(link.href);
        WH.toast("Invitation card downloaded successfully!");
      } catch (err) {
        console.error(err);
        WH.toast("Could not download invitation card.");
      } finally {
        downloadBtn.disabled = false;
        downloadBtn.textContent = "↓ Download Invitation Card";
      }
    };
  }
}

function updateResponseState() {
  if (!member && !guest) return;
  const note = document.getElementById("acceptedNote");
  if (member) {
    if (member.invitationStatus === "accepted") {
      document.getElementById("inviteActions").innerHTML = `<a class="button ivory" href="committee-dashboard.html?token=${member.token}">Open committee workspace</a>`;
      note.textContent = "You've joined the wedding committee — your planning workspace is unlocked.";
      document.getElementById("bottomAccept").hidden = true;
    } else if (member.invitationStatus === "declined") {
      note.textContent = "Thank you for letting the couple know.";
    }
    return;
  }
  if (guest.rsvp === "attending") {
    document.getElementById("inviteActions").innerHTML = `<a class="button ivory" href="guest-dashboard.html?token=${guest.token}">Open your guest dashboard</a>`;
    note.textContent = `Your RSVP is confirmed for ${guest.partySize} ${guest.partySize === 1 ? "guest" : "guests"}.`;
    document.getElementById("bottomAccept").hidden = true;
  } else if (guest.rsvp === "declined") {
    note.textContent = "Thank you for letting the couple know. You can change your response anytime.";
  }
}

function openRSVP(mode) {
  if (!guest && !member) return;
  const role = WeddingRoleSelector.selectedRole() || inviteeType();
  const name = (member || guest).name;
  const messageField = `<label>Message for the couple<textarea name="message" rows="3" placeholder="Share a wish or note..."></textarea></label>`;
  if (role === "committee") {
    content.innerHTML = `<p class="eyebrow">Join the planning team</p><h2>${mode === "attending" ? "Welcome to the committee!" : "We'll miss your help"}</h2>
      <p>Your response confirms you are joining this wedding as a committee member.</p><form id="rsvpForm" class="modal-form">
      <label>Your name<input value="${WH.escape(name)}" disabled></label>
      ${messageField}
      <button class="button primary">Confirm response</button></form>`;
  } else {
    content.innerHTML = `<p class="eyebrow">Your response</p><h2>${mode === "attending" ? "Wonderful, we can't wait!" : "We'll miss you"}</h2>
      <p>This response stays connected to your private invitation.</p><form id="rsvpForm" class="modal-form">
      <label>Your name<input value="${WH.escape(name)}" disabled></label>
      ${mode === "attending" ? `<label>Number attending<select name="partySize">${[1,2,3,4].map(number => `<option ${guest.partySize === number ? "selected" : ""}>${number}</option>`).join("")}</select></label>` : ""}
      ${messageField}
      <button class="button primary">Confirm response</button></form>`;
  }
  modal.classList.add("open");
  document.getElementById("rsvpForm").onsubmit = event => submitResponse(event, mode, role);
}

async function submitResponse(event, mode, role = WeddingRoleSelector.selectedRole() || inviteeType()) {
  event.preventDefault();
  const form = new FormData(event.target);
  const partySize = mode === "attending" && role === "guest" ? Number(form.get("partySize") || 1) : 0;
  const message = String(form.get("message") || "").trim();
  try {
    if (API.isOnline()) {
      await API.respond(token, mode === "attending" ? "accept" : "decline", role);
      if (mode === "attending" && role === "guest") {
        await API.updateRSVP(token, "attending", partySize);
        if (message) await API.sendMessage(token, message);
      }
      if (mode === "attending" && role === "committee") {
        if (message) await API.sendCommitteeMessage(remoteWeddingID(), message, token);
      }
    }
  } catch (error) {
    console.warn("API response sync failed; response remains local:", error);
  }
  if (role === "committee" && member) {
    member.invitationStatus = mode === "attending" ? "accepted" : "declined";
  } else if (guest) {
    guest.rsvp = mode;
    guest.invitationStatus = mode === "attending" ? "accepted" : "declined";
    guest.partySize = partySize;
  }
  if (message && guest) data.messages.unshift({ id: `msg_${Date.now()}`, guestId: guest.id, name: (member || guest).name, message, status: "pending", date: new Date().toISOString().slice(0, 10) });
  WH.saveData(data);
  modal.classList.remove("open");
  updateResponseState();
  WH.toast(mode === "attending" ? "Invitation accepted — your dashboard is unlocked" : "Your response has been saved");
  if (mode === "attending") {
    const target = role === "committee"
      ? `committee-dashboard.html?token=${(member || { token }).token}`
      : `guest-dashboard.html?token=${(guest || { token }).token}`;
    setTimeout(() => location.href = target, 600);
  }
}

function remoteWeddingID() {
  return data.wedding.id || localStorage.getItem("weddinghub_api_wedding_id") || "";
}

document.getElementById("acceptButton").addEventListener("click", () => { if (!adminPreview) openRSVP("attending"); });
document.getElementById("declineButton").addEventListener("click", () => { if (!adminPreview) openRSVP("declined"); });
document.getElementById("bottomAccept").addEventListener("click", () => openRSVP("attending"));
document.getElementById("closeModal").addEventListener("click", () => modal.classList.remove("open"));
modal.addEventListener("click", event => { if (event.target === modal) modal.classList.remove("open"); });
loadInvitation();