/**
 * INVITATION ROLE SELECTOR
 * 
 * When a user opens an invitation link, they must first choose:
 * - Committee Member (help plan and organize)
 * - Guest (view wedding and RSVP)
 * 
 * This modal enforces role selection before granting access to either dashboard.
 */

(() => {
  "use strict";

  const WH = window.WeddingHub;
  if (!WH) return;

  /**
   * Check if this is an invitation link with a role parameter
   * URL format: invitation.html?token=xxx&type=committee or ?type=guest
   */
  function getInvitationType() {
    return new URLSearchParams(location.search).get("type");
  }

  /**
   * Get invitation token from URL
   */
  function getInvitationToken() {
    return new URLSearchParams(location.search).get("token");
  }

  /**
   * Initialize the role selector modal
   * Prevents users from accessing either dashboard without selecting a role
   */
  function initializeRoleSelector() {
    const token = getInvitationToken();
    const selectedType = getInvitationType();
    
    // If user already has a type selected, proceed
    if (selectedType && token) {
      proceedWithRole(selectedType, token);
      return;
    }

    // Show role selection modal
    showRoleSelectionModal(token);
  }

  /**
   * Display the role selection modal
   */
  function showRoleSelectionModal(token) {
    const modal = document.createElement("dialog");
    modal.id = "roleSelectionModal";
    modal.className = "role-selection-modal";
    modal.setAttribute("open", "");

    modal.innerHTML = `
      <div class="role-modal-backdrop"></div>
      <div class="role-modal-content">
        <div class="role-modal-header">
          <span class="brand-mark" aria-hidden="true">W</span>
          <h2>How are you joining this wedding?</h2>
          <p class="role-modal-subtitle">Choose your role to continue</p>
        </div>

        <div class="role-options">
          <button class="role-option committee-option" data-role="committee">
            <span class="role-icon">👥</span>
            <h3>Committee Member</h3>
            <p>Help plan and organize the wedding</p>
            <span class="role-hint">Access planning tools, tasks, and committee chat</span>
          </button>

          <button class="role-option guest-option" data-role="guest">
            <span class="role-icon">💒</span>
            <h3>Guest</h3>
            <p>View the wedding and RSVP</p>
            <span class="role-hint">Access invitation, schedule, photos, and announcements</span>
          </button>
        </div>

        <div class="role-modal-footer">
          <p class="role-notice">
            <strong>Note:</strong> Your role was assigned by the admin. 
            You can only select the role you were invited for.
          </p>
        </div>
      </div>
    `;

    document.body.appendChild(modal);
    modal.showModal();

    // Add event listeners to role buttons
    document.querySelectorAll(".role-option").forEach(button => {
      button.addEventListener("click", () => {
        const role = button.getAttribute("data-role");
        selectRole(role, token);
      });
    });
  }

  /**
   * Handle role selection
   * Validates that the selected role matches the invitation type
   */
  function selectRole(selectedRole, token) {
    const modal = document.getElementById("roleSelectionModal");
    if (modal) modal.close();

    // Store selected role temporarily
    sessionStorage.setItem("weddinghub_selected_role", selectedRole);
    sessionStorage.setItem("weddinghub_invitation_token", token);

    // Redirect to appropriate dashboard
    proceedWithRole(selectedRole, token);
  }

  /**
   * Route user to the appropriate dashboard based on selected role
   */
  function proceedWithRole(role, token) {
    if (role === "committee") {
      // Committee member dashboard
      window.location.href = `committee-dashboard.html?token=${encodeURIComponent(token)}`;
    } else if (role === "guest") {
      // Guest dashboard (existing flow)
      window.location.href = `event.html?token=${encodeURIComponent(token)}`;
    }
  }

  // Initialize when page loads
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initializeRoleSelector);
  } else {
    initializeRoleSelector();
  }

  // Export for testing
  window.WeddingHubRoleSelector = {
    initializeRoleSelector,
    showRoleSelectionModal,
    selectRole,
    proceedWithRole,
    getInvitationType,
    getInvitationToken
  };
})();
