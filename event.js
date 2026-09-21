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
  if (!token) return startInvitationFlow();
  try {
    await API.requireAPI();
  } catch (error) {
    API.showFatalError(error.message);
    return;
  }
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
    console.warn("Invitation lookup failed:", error);
    API.showFatalError(error.status === 404 ? "This invitation link is invalid or has expired." : error.message);
    return;
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
  const greeting = document.getElementById("personalGreeting");
  const actions = document.getElementById("inviteActions");
  const note = document.getElementById("acceptedNote");
  const guestPanel = document.getElementById("guestPanel");

  // The guest link presents only the designed invitation card plus the acceptance
  // actions. Wedding details, schedule and countdown live in the guest space that
  // opens once the invitation is accepted.
  if (adminPreview) {
    document.body.classList.add("invite-preview-mode");
    const returnBtn = document.getElementById("previewReturnBtn");
    if (returnBtn) returnBtn.hidden = false;
    if (guestPanel) guestPanel.hidden = true;
  } else if (!guest && !member) {
    if (greeting) greeting.textContent = "This invitation link is invalid or has expired.";
    if (actions) actions.innerHTML = '<a class="button primary" href="../index.html">Return to WeddingHub</a>';
    if (note) note.textContent = "";
  } else if (member) {
    if (greeting) greeting.textContent = `Dear ${member.name.split(" ")[0]}, you're invited to help plan and organize this wedding.`;
    updateResponseState();
  } else {
    if (greeting) greeting.textContent = `Dear ${guest.name.split(" ")[0]}, this celebration would not be complete without you.`;
    updateResponseState();
  }

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
  const actions = document.getElementById("inviteActions");
  if (member) {
    if (member.invitationStatus === "accepted") {
      if (actions) actions.innerHTML = `<a class="button ivory" href="committee-dashboard.html?token=${member.token}">Open committee workspace</a>`;
      if (note) note.textContent = "You've joined the wedding committee — your planning workspace is unlocked.";
    } else if (member.invitationStatus === "declined") {
      if (note) note.textContent = "Thank you for letting the couple know.";
    }
    return;
  }
  if (guest.rsvp === "attending") {
    if (actions) actions.innerHTML = `<a class="button ivory" href="guest-dashboard.html?token=${guest.token}">Open your guest dashboard</a>`;
    if (note) note.textContent = `Your RSVP is confirmed for ${guest.partySize} ${guest.partySize === 1 ? "guest" : "guests"}.`;
  } else if (guest.rsvp === "declined") {
    if (note) note.textContent = "Thank you for letting the couple know. You can change your response anytime.";
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
    await API.respond(token, mode === "attending" ? "accept" : "decline", role);
    if (mode === "attending" && role === "guest") {
      await API.updateRSVP(token, "attending", partySize);
      if (message) await API.sendMessage(token, message);
    }
    if (mode === "attending" && role === "committee") {
      if (message) await API.sendCommitteeMessage(remoteWeddingID(), message, token);
    }
  } catch (error) {
    console.warn("Invitation response failed:", error);
    WH.toast("We couldn't reach WeddingHub to save your response. Please try again.");
    return;
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
document.getElementById("closeModal").addEventListener("click", () => modal.classList.remove("open"));
modal.addEventListener("click", event => { if (event.target === modal) modal.classList.remove("open"); });
loadInvitation();