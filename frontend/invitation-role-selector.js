// Invitation role selector: every invitation link first asks which access the
// invitee wants (Committee Member or Guest), then routes them into the matching
// invitation flow. The backend independently validates the chosen role against
// the invitation type on acceptance.
(() => {
  "use strict";
  const WH = window.WeddingHub;

  const ROLE_KEY = "weddinghub_selected_role";
  const TOKEN_KEY = "weddinghub_invitation_token";

  function selectedRole() { return sessionStorage.getItem(ROLE_KEY); }
  function selectedToken() { return sessionStorage.getItem(TOKEN_KEY); }
  function remember(role, token) {
    sessionStorage.setItem(ROLE_KEY, role);
    if (token) sessionStorage.setItem(TOKEN_KEY, token);
  }
  function clearSelection() { sessionStorage.removeItem(ROLE_KEY); }

  function close() {
    document.querySelectorAll(".role-selector").forEach(element => element.remove());
  }

  // show presents the "How are you joining this wedding?" screen.
  // invitationType is the role the admin issued this link for ("guest" | "committee").
  // Choosing the wrong role is blocked and explained; choosing the right one invokes onSelected.
  function show({ invitationType = "guest", invitedName = "", onSelected } = {}) {
    close();
    const expected = invitationType === "committee" ? "committee" : "guest";
    const backdrop = document.createElement("div");
    backdrop.className = "role-selector";
    backdrop.setAttribute("role", "dialog");
    backdrop.setAttribute("aria-modal", "true");
    backdrop.innerHTML = `
      <div class="role-selector-card">
        <div class="role-selector-head">
          <span class="role-selector-brand">W</span>
          <p class="eyebrow">Wedding invitation</p>
          <h2>How are you joining this wedding?</h2>
          <p>${invitedName ? `Dear ${WH.escape(String(invitedName).split(" ")[0])}, choose the access you were invited for.` : "Choose the access you were invited for."}</p>
        </div>
        <div class="role-selector-options">
          <button type="button" class="role-selector-option committee" data-role="committee">
            <span class="role-selector-icon" aria-hidden="true">◈</span>
            <strong>Committee Member</strong>
            <small>Help plan and organize the wedding</small>
          </button>
          <button type="button" class="role-selector-option guest" data-role="guest">
            <span class="role-selector-icon" aria-hidden="true">♙</span>
            <strong>Guest</strong>
            <small>View the wedding and RSVP</small>
          </button>
        </div>
        <p class="role-selector-error" id="roleSelectorError" hidden></p>
        <p class="role-selector-note">Your role is assigned by the admin and is validated when you accept the invitation.</p>
      </div>`;
    document.body.appendChild(backdrop);

    function reject(role) {
      const label = expected === "committee" ? "committee member" : "guest";
      const error = backdrop.querySelector("#roleSelectorError");
      error.textContent = expected === "committee"
        ? `This invitation is for a ${label}. Choosing "Guest" with a committee invitation would not unlock the planning workspace.`
        : `This invitation is for a ${label}. Accepting as a committee member is not allowed for a guest invitation.`;
      error.hidden = false;
      backdrop.querySelectorAll(".role-selector-option").forEach(option => option.classList.toggle("wrong", option.dataset.role === role));
    }

    function choose(role) {
      if (role !== expected) return reject(role);
      remember(role, selectedToken());
      close();
      if (typeof onSelected === "function") onSelected(role);
    }

    backdrop.querySelector('[data-role="committee"]').onclick = () => choose("committee");
    backdrop.querySelector('[data-role="guest"]').onclick = () => choose("guest");
    return backdrop;
  }

  window.WeddingRoleSelector = { show, close, selectedRole, selectedToken, remember, clearSelection };
})();