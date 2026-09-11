/**
 * ADMIN DASHBOARD ENHANCEMENTS
 * 
 * Extends the existing admin dashboard to separately manage:
 * - Committee Members (planning role)
 * - Guests (attendance role)
 * 
 * Add these functions to dashboard.js
 */

/**
 * Enhanced Overview Rendering
 * Shows separate metrics for committee members and guests
 */
function renderEnhancedOverview() {
  const committeeMembers = data.committee?.members || [];
  const guests = data.guests || [];
  
  const committeeAccepted = committeeMembers.filter(m => m.status === 'accepted');
  const committeePending = committeeMembers.filter(m => m.status === 'pending');
  const committeeDeclined = committeeMembers.filter(m => m.status === 'declined');
  
  const guestAttending = guests.filter(g => g.rsvp === 'attending');
  const guestPending = guests.filter(g => g.rsvp === 'pending');
  const guestDeclined = guests.filter(g => g.rsvp === 'declined');

  const overviewContent = $('#view-overview');
  if (!overviewContent) return;

  const currentHTML = overviewContent.innerHTML;
  
  // Inject committee metrics panel before guest metrics
  const newPanel = document.createElement('div');
  newPanel.id = 'committeeMetricsPanel';
  newPanel.className = 'metrics-panel committee-panel';
  newPanel.innerHTML = `
    <div class="panel-header">
      <h3>Committee Members</h3>
      <span class="panel-badge">Planning Team</span>
    </div>
    <div class="metrics-grid">
      <div class="mini-metric">
        <span class="metric-icon">✓</span>
        <div>
          <span class="metric-label">Accepted</span>
          <span class="metric-number">${committeeAccepted.length}</span>
        </div>
      </div>
      <div class="mini-metric">
        <span class="metric-icon">◐</span>
        <div>
          <span class="metric-label">Pending</span>
          <span class="metric-number">${committeePending.length}</span>
        </div>
      </div>
      <div class="mini-metric">
        <span class="metric-icon">✗</span>
        <div>
          <span class="metric-label">Declined</span>
          <span class="metric-number">${committeeDeclined.length}</span>
        </div>
      </div>
    </div>
    <div class="panel-members-list">
      ${committeeAccepted.map(m => `
        <div class="member-status-row accepted">
          <span class="status-icon">✓</span>
          <span class="member-name">${m.name}</span>
          <span class="member-role">${m.role}</span>
        </div>
      `).join('')}
      ${committeePending.map(m => `
        <div class="member-status-row pending">
          <span class="status-icon">○</span>
          <span class="member-name">${m.name}</span>
          <span class="member-role">${m.role}</span>
        </div>
      `).join('')}
      ${committeeDeclined.length > 0 ? `
        <div class="members-divider"></div>
        ${committeeDeclined.map(m => `
          <div class="member-status-row declined">
            <span class="status-icon">✗</span>
            <span class="member-name">${m.name}</span>
            <span class="member-role">${m.role}</span>
          </div>
        `).join('')}
      ` : ''}
    </div>
  `;

  // Insert into overview
  const existingPanel = overviewContent.querySelector('#committeeMetricsPanel');
  if (existingPanel) {
    existingPanel.replaceWith(newPanel);
  } else {
    overviewContent.insertBefore(newPanel, overviewContent.querySelector('.metrics-panel'));
  }
}

/**
 * Render Committee Members Management View
 */
function renderCommitteeManagement() {
  const view = $('#view-committee') || createAdminView('committee', 'Committee Members');
  const committee = data.committee || {};
  const members = committee.members || [];

  view.innerHTML = `
    <div class="admin-section">
      <div class="section-header">
        <h2>Committee Members</h2>
        <button id="inviteCommitteeMemberBtn" class="button secondary">+ Invite Committee Member</button>
      </div>

      <div class="committee-summary">
        <div class="summary-stat">
          <span class="stat-value">${members.length}</span>
          <span class="stat-label">Total Invited</span>
        </div>
        <div class="summary-stat">
          <span class="stat-value">${members.filter(m => m.status === 'accepted').length}</span>
          <span class="stat-label">Accepted</span>
        </div>
        <div class="summary-stat">
          <span class="stat-value">${members.filter(m => m.status === 'pending').length}</span>
          <span class="stat-label">Pending</span>
        </div>
        <div class="summary-stat">
          <span class="stat-value">${members.filter(m => m.status === 'declined').length}</span>
          <span class="stat-label">Declined</span>
        </div>
      </div>

      <div class="committee-table">
        <div class="table-row table-head">
          <span>Name</span>
          <span>Role</span>
          <span>Email</span>
          <span>Status</span>
          <span>Invitation</span>
          <span>Actions</span>
        </div>
        ${members.length ? members.map(member => `
          <div class="table-row committee-member-row" data-member-id="${member.id}">
            <span class="member-name">${member.name}</span>
            <span class="member-role">${member.role || 'Committee Member'}</span>
            <span class="member-email">${member.email}</span>
            <span class="status-badge status-${member.status}">${member.status}</span>
            <span class="invitation-status">${member.invitationStatus}</span>
            <span class="actions-cell">
              <button class="action-btn edit-btn" data-member-id="${member.id}">Edit</button>
              <button class="action-btn remove-btn" data-member-id="${member.id}">Remove</button>
            </span>
          </div>
        `).join('') : '<div class="empty-row"><p>No committee members invited yet.</p></div>'}
      </div>
    </div>
  `;

  // Event listeners
  const inviteBtn = view.querySelector('#inviteCommitteeMemberBtn');
  if (inviteBtn) {
    inviteBtn.addEventListener('click', showCommitteeInviteModal);
  }

  view.querySelectorAll('.edit-btn').forEach(btn => {
    btn.addEventListener('click', () => editCommitteeMember(btn.getAttribute('data-member-id')));
  });

  view.querySelectorAll('.remove-btn').forEach(btn => {
    btn.addEventListener('click', () => removeCommitteeMember(btn.getAttribute('data-member-id')));
  });
}

/**
 * Enhanced Guests Management
 * Now separates committee members from regular guests
 */
function renderEnhancedGuestManagement() {
  const view = $('#view-guests') || createAdminView('guests', 'Guests');
  const guests = (data.guests || []).filter(g => !data.committee?.members?.some(m => m.email === g.email));

  view.innerHTML = `
    <div class="admin-section">
      <div class="section-header">
        <h2>Guest List</h2>
        <button id="inviteGuestBtn" class="button secondary">+ Invite Guest</button>
      </div>

      <div class="guest-summary">
        <div class="summary-stat">
          <span class="stat-value">${guests.length}</span>
          <span class="stat-label">Total Guests</span>
        </div>
        <div class="summary-stat">
          <span class="stat-value">${guests.filter(g => g.rsvp === 'attending').length}</span>
          <span class="stat-label">Attending</span>
        </div>
        <div class="summary-stat">
          <span class="stat-value">${guests.filter(g => g.rsvp === 'pending').length}</span>
          <span class="stat-label">Pending</span>
        </div>
        <div class="summary-stat">
          <span class="stat-value">${guests.filter(g => g.rsvp === 'declined').length}</span>
          <span class="stat-label">Declined</span>
        </div>
      </div>

      <div class="guests-table">
        <div class="table-row table-head">
          <span>Guest</span>
          <span>Category</span>
          <span>Email</span>
          <span>RSVP</span>
          <span>Party</span>
          <span>Actions</span>
        </div>
        ${guests.length ? guests.map(guest => `
          <div class="table-row guest-row" data-guest-id="${guest.id}">
            <span class="guest-name">${guest.name}</span>
            <span class="guest-category">${guest.category}</span>
            <span class="guest-email">${guest.email}</span>
            <span class="rsvp-badge rsvp-${guest.rsvp}">${guest.rsvp || 'pending'}</span>
            <span class="party-size">${guest.partySize || 1}</span>
            <span class="actions-cell">
              <button class="action-btn edit-btn" data-guest-id="${guest.id}">Edit</button>
              <button class="action-btn remove-btn" data-guest-id="${guest.id}">Remove</button>
            </span>
          </div>
        `).join('') : '<div class="empty-row"><p>No guests invited yet.</p></div>'}
      </div>
    </div>
  `;

  const inviteBtn = view.querySelector('#inviteGuestBtn');
  if (inviteBtn) {
    inviteBtn.addEventListener('click', showGuestInviteModal);
  }
}

/**
 * Show modal to invite committee member
 */
function showCommitteeInviteModal() {
  const modal = document.createElement('dialog');
  modal.className = 'invite-modal';
  modal.innerHTML = `
    <div class="modal-content">
      <h3>Invite Committee Member</h3>
      <form id="committeeInviteForm">
        <div class="form-group">
          <label>Name *</label>
          <input type="text" id="committeeName" required>
        </div>
        <div class="form-group">
          <label>Email *</label>
          <input type="email" id="committeeEmail" required>
        </div>
        <div class="form-group">
          <label>Phone</label>
          <input type="tel" id="committeePhone">
        </div>
        <div class="form-group">
          <label>Role *</label>
          <select id="committeeRole" required>
            <option value="">Select a role</option>
            <option value="Best Man">Best Man</option>
            <option value="Maid of Honor">Maid of Honor</option>
            <option value="Bridesmaid">Bridesmaid</option>
            <option value="Groomsman">Groomsman</option>
            <option value="Parent">Parent</option>
            <option value="Other">Other</option>
          </select>
        </div>
        <div class="form-group">
          <label>Permissions</label>
          <div class="checkbox-group">
            <label><input type="checkbox" name="perm" value="edit_wedding"> Edit Wedding Info</label>
            <label><input type="checkbox" name="perm" value="manage_events"> Manage Events</label>
            <label><input type="checkbox" name="perm" value="manage_announcements"> Post Announcements</label>
            <label><input type="checkbox" name="perm" value="view_guests"> View Guest List</label>
            <label><input type="checkbox" name="perm" value="chat"> Committee Chat</label>
          </div>
        </div>
        <div class="form-actions">
          <button type="submit" class="button primary">Send Invitation</button>
          <button type="button" class="button secondary" onclick="this.closest('dialog').close()">Cancel</button>
        </div>
      </form>
    </div>
  `;

  document.body.appendChild(modal);
  modal.showModal();

  $('#committeeInviteForm').addEventListener('submit', (e) => {
    e.preventDefault();
    const committee = data.committee || { members: [] };
    data.committee = committee;

    const permissions = Array.from(document.querySelectorAll('#committeeInviteForm input[name="perm"]:checked'))
      .map(cb => cb.value);

    committee.members.push({
      id: `cmm_${Date.now()}`,
      name: $('#committeeName').value,
      email: $('#committeeEmail').value,
      phone: $('#committeePhone').value,
      role: $('#committeeRole').value,
      status: 'pending',
      token: `committee-${Math.random().toString(36).substr(2, 9)}`,
      permissions,
      invitationStatus: 'sent',
      invitedAt: new Date().toISOString()
    });

    WH.saveData(data);
    modal.close();
    renderCommitteeManagement();
    WH.toast('Committee member invited successfully');
    persist('Committee member added');
  });
}

/**
 * Edit committee member
 */
function editCommitteeMember(memberId) {
  const committee = data.committee || {};
  const member = committee.members?.find(m => m.id === memberId);
  if (!member) return;

  const modal = document.createElement('dialog');
  modal.className = 'edit-modal';
  modal.innerHTML = `
    <div class="modal-content">
      <h3>Edit Committee Member</h3>
      <form id="editCommitteeForm">
        <div class="form-group">
          <label>Name</label>
          <input type="text" value="${member.name}" id="editCommitteeName">
        </div>
        <div class="form-group">
          <label>Role</label>
          <select id="editCommitteeRole">
            <option value="Best Man" ${member.role === 'Best Man' ? 'selected' : ''}>Best Man</option>
            <option value="Maid of Honor" ${member.role === 'Maid of Honor' ? 'selected' : ''}>Maid of Honor</option>
            <option value="Bridesmaid" ${member.role === 'Bridesmaid' ? 'selected' : ''}>Bridesmaid</option>
            <option value="Groomsman" ${member.role === 'Groomsman' ? 'selected' : ''}>Groomsman</option>
            <option value="Parent" ${member.role === 'Parent' ? 'selected' : ''}>Parent</option>
          </select>
        </div>
        <div class="form-group">
          <label>Status</label>
          <select id="editCommitteeStatus">
            <option value="pending" ${member.status === 'pending' ? 'selected' : ''}>Pending</option>
            <option value="accepted" ${member.status === 'accepted' ? 'selected' : ''}>Accepted</option>
            <option value="declined" ${member.status === 'declined' ? 'selected' : ''}>Declined</option>
          </select>
        </div>
        <div class="form-actions">
          <button type="submit" class="button primary">Save Changes</button>
          <button type="button" class="button secondary" onclick="this.closest('dialog').close()">Cancel</button>
        </div>
      </form>
    </div>
  `;

  document.body.appendChild(modal);
  modal.showModal();

  $('#editCommitteeForm').addEventListener('submit', (e) => {
    e.preventDefault();
    member.name = $('#editCommitteeName').value;
    member.role = $('#editCommitteeRole').value;
    member.status = $('#editCommitteeStatus').value;
    WH.saveData(data);
    modal.close();
    renderCommitteeManagement();
    WH.toast('Committee member updated');
    persist('Committee member updated');
  });
}

/**
 * Remove committee member
 */
function removeCommitteeMember(memberId) {
  if (!confirm('Remove this committee member? They will not have access to the committee dashboard.')) return;
  
  const committee = data.committee || {};
  if (committee.members) {
    committee.members = committee.members.filter(m => m.id !== memberId);
  }
  WH.saveData(data);
  renderCommitteeManagement();
  WH.toast('Committee member removed');
  persist('Committee member removed');
}

/**
 * Show guest invite modal (existing, but enhanced)
 */
function showGuestInviteModal() {
  const modal = document.createElement('dialog');
  modal.className = 'invite-modal';
  modal.innerHTML = `
    <div class="modal-content">
      <h3>Invite Guest</h3>
      <form id="guestInviteForm">
        <div class="form-group">
          <label>Guest Name *</label>
          <input type="text" id="guestName" required>
        </div>
        <div class="form-group">
          <label>Email *</label>
          <input type="email" id="guestEmail" required>
        </div>
        <div class="form-group">
          <label>Phone</label>
          <input type="tel" id="guestPhone">
        </div>
        <div class="form-group">
          <label>Category *</label>
          <select id="guestCategory" required>
            <option value="">Select category</option>
            <option value="Family">Family</option>
            <option value="Friends">Friends</option>
            <option value="Colleagues">Colleagues</option>
            <option value="Extended Family">Extended Family</option>
          </select>
        </div>
        <div class="form-actions">
          <button type="submit" class="button primary">Send Invitation</button>
          <button type="button" class="button secondary" onclick="this.closest('dialog').close()">Cancel</button>
        </div>
      </form>
    </div>
  `;

  document.body.appendChild(modal);
  modal.showModal();

  $('#guestInviteForm').addEventListener('submit', (e) => {
    e.preventDefault();
    data.guests = data.guests || [];
    data.guests.push({
      id: `guest_${Date.now()}`,
      name: $('#guestName').value,
      email: $('#guestEmail').value,
      phone: $('#guestPhone').value,
      category: $('#guestCategory').value,
      invitationStatus: 'sent',
      token: `guest-${Math.random().toString(36).substr(2, 9)}`,
      rsvp: 'pending',
      partySize: 1,
      invitedAt: new Date().toISOString()
    });
    WH.saveData(data);
    modal.close();
    renderEnhancedGuestManagement();
    WH.toast('Guest invited successfully');
    persist('Guest added');
  });
}

/**
 * Helper to create admin view if it doesn't exist
 */
function createAdminView(id, title) {
  const container = $('#adminViewsContainer') || document.querySelector('.admin-views');
  if (!container) return null;

  const view = document.createElement('div');
  view.id = `view-${id}`;
  view.className = 'admin-view';
  container.appendChild(view);
  return view;
}

// Hook into existing AdminApp initialization
// Call these in renderAll():
function renderAllEnhanced() {
  renderAll(); // Call original
  renderEnhancedOverview();
  renderCommitteeManagement();
  renderEnhancedGuestManagement();
}
