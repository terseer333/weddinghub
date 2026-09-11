/**
 * COMMITTEE DASHBOARD
 * 
 * Dedicated workspace for committee members to plan and organize the wedding.
 * Committee members have access to:
 * - Wedding overview and planning dashboard
 * - Planning tasks and checklist
 * - Committee chat room (private, committee-only)
 * - Announcements and updates
 * - Photos and story (shared with guests)
 * - Events and schedule
 * - Shared wedding information
 */

const WH = window.WeddingHub;
const API = window.WeddingHubAPI;
const $ = selector => document.querySelector(selector);
const $$ = selector => document.querySelectorAll(selector);

let data = WH.getData();
let committeeUser = getCurrentCommitteeUser();
let selectedTab = 'overview';

/**
 * Get current committee user from session/localStorage
 */
function getCurrentCommitteeUser() {
  try {
    const stored = JSON.parse(localStorage.getItem('weddinghub_committee_user') || '{}');
    return stored || {};
  } catch (_) {
    return {};
  }
}

/**
 * Check if user has committee access
 * This should be validated by backend in production
 */
function validateCommitteeAccess(token) {
  // In production, call backend to validate token and role
  const committee = data.committee || {};
  return committee.members && committee.members.some(m => m.token === token);
}

/**
 * Initialize committee dashboard
 */
async function initializeCommitteeDashboard() {
  const token = WH.query('token');
  
  if (!token || !validateCommitteeAccess(token)) {
    showAccessDenied();
    return;
  }

  // Load data from API if available
  if (token && await API.connect()) {
    try {
      const view = await API.committeeDashboard(token);
      data = API.mergeAPI(data, view);
      WH.saveData(data);
    } catch (error) {
      console.warn('Committee API sync failed, using local data:', error);
    }
  }

  renderCommitteeDashboard();
  setupCommitteeChat();
}

/**
 * Show access denied message
 */
function showAccessDenied() {
  document.getElementById('committeeDashboard').style.display = 'none';
  const denied = document.getElementById('accessDenied');
  if (denied) {
    denied.innerHTML = `
      <div class="access-denied-container">
        <span class="brand-mark">W</span>
        <h1>Access Denied</h1>
        <p>You do not have permission to access the committee dashboard.</p>
        <p>If you believe this is an error, please contact the wedding admin.</p>
        <a href="index.html" class="button primary">Return to home</a>
      </div>
    `;
  }
}

/**
 * Render the complete committee dashboard
 */
function renderCommitteeDashboard() {
  renderCommitteeHeader();
  renderCommitteeNav();
  renderCommitteeOverview();
  renderPlanningTasks();
  renderCommitteeAnnouncements();
  renderSharedContent();
}

/**
 * Render committee dashboard header
 */
function renderCommitteeHeader() {
  const header = $('#committeeHeader');
  if (!header) return;

  const name = committeeUser.name || 'Committee Member';
  const initials = name.split(/\s+/).slice(0, 2).map(p => p[0]).join('').toUpperCase();

  header.innerHTML = `
    <div class="committee-header-top">
      <div class="committee-brand">
        <span class="brand-mark">W</span>
        <span>Wedding Planning</span>
      </div>
      <div class="committee-user-info">
        <span class="user-avatar">${initials}</span>
        <span class="user-name">${name}</span>
      </div>
    </div>
    <div class="committee-header-wedding">
      <h1>${data.wedding.brideName} & ${data.wedding.groomName}</h1>
      <p>${WH.formatDate(data.wedding.date)} · ${data.wedding.city}, ${data.wedding.state}</p>
    </div>
  `;
}

/**
 * Render committee navigation tabs
 */
function renderCommitteeNav() {
  const nav = $('#committeeNav');
  if (!nav) return;

  const tabs = [
    { id: 'overview', label: 'Dashboard', icon: '🏠' },
    { id: 'tasks', label: 'Planning Tasks', icon: '✓' },
    { id: 'events', label: 'Events & Schedule', icon: '📅' },
    { id: 'announcements', label: 'Announcements', icon: '📢' },
    { id: 'content', label: 'Photos & Story', icon: '📸' },
    { id: 'chat', label: 'Committee Chat', icon: '💬' }
  ];

  nav.innerHTML = tabs.map(tab => `
    <button class="committee-nav-tab ${selectedTab === tab.id ? 'active' : ''}" data-tab="${tab.id}">
      <span class="tab-icon">${tab.icon}</span>
      <span class="tab-label">${tab.label}</span>
    </button>
  `).join('');

  nav.querySelectorAll('.committee-nav-tab').forEach(tab => {
    tab.addEventListener('click', () => switchTab(tab.getAttribute('data-tab')));
  });
}

/**
 * Switch active tab
 */
function switchTab(tabId) {
  selectedTab = tabId;
  $$('.committee-view').forEach(view => {
    view.classList.remove('active');
  });
  const view = $(`#view-${tabId}`);
  if (view) view.classList.add('active');
  
  $$('.committee-nav-tab').forEach(tab => {
    tab.classList.toggle('active', tab.getAttribute('data-tab') === tabId);
  });
}

/**
 * Render committee overview dashboard
 */
function renderCommitteeOverview() {
  const overview = $('#view-overview');
  if (!overview) return;

  const committee = data.committee || {};
  const guests = data.guests || [];
  const acceptedMembers = (committee.members || []).filter(m => m.status === 'accepted');
  const pendingMembers = (committee.members || []).filter(m => m.status === 'pending');
  const attendingGuests = guests.filter(g => g.rsvp === 'attending');

  overview.innerHTML = `
    <div class="committee-overview">
      <div class="overview-metrics">
        <div class="metric-card">
          <span class="metric-label">Committee Members</span>
          <span class="metric-value">${acceptedMembers.length}</span>
          <span class="metric-detail">Accepted</span>
        </div>
        <div class="metric-card">
          <span class="metric-label">Pending Confirmations</span>
          <span class="metric-value">${pendingMembers.length}</span>
          <span class="metric-detail">Awaiting reply</span>
        </div>
        <div class="metric-card">
          <span class="metric-label">Attending Guests</span>
          <span class="metric-value">${attendingGuests.length}</span>
          <span class="metric-detail">RSVP confirmed</span>
        </div>
        <div class="metric-card">
          <span class="metric-label">Days Until</span>
          <span class="metric-value" id="daysUntilMetric">--</span>
          <span class="metric-detail">Wedding day</span>
        </div>
      </div>

      <div class="overview-sections">
        <section class="overview-section">
          <h3>Committee Members</h3>
          <div class="members-list">
            ${(committee.members || []).map(member => `
              <div class="member-card status-${member.status}">
                <span class="member-status-badge">${member.status === 'accepted' ? '✓' : '○'}</span>
                <div class="member-info">
                  <strong>${member.name}</strong>
                  <small>${member.role || 'Committee Member'}</small>
                </div>
              </div>
            `).join('')}
          </div>
        </section>

        <section class="overview-section">
          <h3>Next Events</h3>
          <div class="events-preview">
            ${WH.published(data.events).slice(0, 2).map(event => `
              <div class="event-preview">
                <span class="event-date">${new Date(event.date + 'T12:00').getDate()}</span>
                <div class="event-details">
                  <strong>${event.name}</strong>
                  <small>${event.time} · ${event.venue}</small>
                </div>
              </div>
            `).join('')}
          </div>
        </section>
      </div>
    </div>
  `;

  // Update countdown
  WH.countdown(data.wedding.date, value => {
    const metric = $('#daysUntilMetric');
    if (metric) metric.textContent = value.passed ? '✓ Today' : value.days;
  });
}

/**
 * Render planning tasks
 */
function renderPlanningTasks() {
  const tasks = $('#view-tasks');
  if (!tasks) return;

  const committee = data.committee || {};
  const planningTasks = committee.tasks || [];

  tasks.innerHTML = `
    <div class="planning-tasks-container">
      <div class="tasks-header">
        <h2>Planning Tasks</h2>
        <button id="addTaskBtn" class="button secondary">+ Add Task</button>
      </div>

      <div class="tasks-list">
        ${planningTasks.length ? planningTasks.map((task, idx) => `
          <div class="task-item status-${task.status}">
            <input type="checkbox" class="task-checkbox" data-task-id="${idx}" ${task.status === 'completed' ? 'checked' : ''}>
            <div class="task-content">
              <h4>${task.title}</h4>
              <p>${task.description || ''}</p>
              ${task.assignee ? `<small>Assigned to: ${task.assignee}</small>` : ''}
              ${task.dueDate ? `<small>Due: ${WH.formatDate(task.dueDate)}</small>` : ''}
            </div>
          </div>
        `).join('') : '<p class="empty-state">No planning tasks yet. Create one to get started.</p>'}
      </div>
    </div>
  `;

  // Add task listeners
  $$('.task-checkbox').forEach(checkbox => {
    checkbox.addEventListener('change', (e) => {
      const taskId = e.target.getAttribute('data-task-id');
      toggleTaskStatus(taskId, e.target.checked);
    });
  });

  const addBtn = $('#addTaskBtn');
  if (addBtn) {
    addBtn.addEventListener('click', showAddTaskModal);
  }
}

/**
 * Toggle task completion status
 */
function toggleTaskStatus(taskId, completed) {
  const committee = data.committee || {};
  if (committee.tasks && committee.tasks[taskId]) {
    committee.tasks[taskId].status = completed ? 'completed' : 'pending';
    WH.saveData(data);
    persist('Task updated');
  }
}

/**
 * Show modal to add new planning task
 */
function showAddTaskModal() {
  const dialog = document.createElement('dialog');
  dialog.className = 'task-modal';
  dialog.innerHTML = `
    <div class="modal-content">
      <h3>Add Planning Task</h3>
      <form id="addTaskForm">
        <input type="text" id="taskTitle" placeholder="Task title" required>
        <textarea id="taskDesc" placeholder="Description (optional)"></textarea>
        <input type="date" id="taskDue" placeholder="Due date (optional)">
        <select id="taskAssignee">
          <option value="">Assign to...</option>
          ${(data.committee.members || []).map(m => `<option value="${m.name}">${m.name}</option>`).join('')}
        </select>
        <div class="modal-buttons">
          <button type="submit" class="button primary">Add Task</button>
          <button type="button" class="button secondary" onclick="this.closest('dialog').close()">Cancel</button>
        </div>
      </form>
    </div>
  `;

  document.body.appendChild(dialog);
  dialog.showModal();

  $('#addTaskForm').addEventListener('submit', (e) => {
    e.preventDefault();
    const committee = data.committee || {};
    if (!committee.tasks) committee.tasks = [];
    
    committee.tasks.push({
      title: $('#taskTitle').value,
      description: $('#taskDesc').value,
      dueDate: $('#taskDue').value,
      assignee: $('#taskAssignee').value,
      status: 'pending',
      createdAt: new Date().toISOString()
    });
    
    WH.saveData(data);
    dialog.close();
    renderPlanningTasks();
    persist('Task added');
  });
}

/**
 * Render committee announcements (shared with guests)
 */
function renderCommitteeAnnouncements() {
  const annSection = $('#view-announcements');
  if (!annSection) return;

  const announcements = data.announcements || [];

  annSection.innerHTML = `
    <div class="announcements-container">
      <div class="announcements-header">
        <h2>Announcements</h2>
        <p class="header-note">These announcements are visible to both committee members and guests</p>
      </div>
      <div class="announcements-list">
        ${announcements.length ? announcements.map(ann => `
          <div class="announcement-item status-${ann.status}">
            <div class="ann-header">
              <h4>${ann.title}</h4>
              <span class="ann-status-badge">${ann.status === 'published' ? '📢' : '⏱'}</span>
            </div>
            <p>${ann.message}</p>
            <small>${WH.formatDate(ann.date)}</small>
          </div>
        `).join('') : '<p class="empty-state">No announcements yet.</p>'}
      </div>
    </div>
  `;
}

/**
 * Render shared content (photos, stories, events)
 */
function renderSharedContent() {
  const content = $('#view-content');
  if (!content) return;

  content.innerHTML = `
    <div class="shared-content-container">
      <section class="content-section">
        <h3>Wedding Photos</h3>
        <div class="content-gallery">
          ${(data.photos || []).slice(0, 6).map(photo => `
            <figure>
              <img src="${photo.url}" alt="${photo.caption}">
              <figcaption>${photo.caption}</figcaption>
            </figure>
          `).join('')}
        </div>
      </section>

      <section class="content-section">
        <h3>Our Story</h3>
        ${(data.stories || []).map(story => `
          <div class="story-card">
            <h4>${story.year}: ${story.title}</h4>
            <p>${story.content}</p>
          </div>
        `).join('')}
      </section>

      <section class="content-section">
        <h3>Events</h3>
        <div class="events-list">
          ${WH.published(data.events || []).map(event => `
            <div class="event-card">
              <strong>${event.name}</strong>
              <small>${WH.formatDate(event.date)} at ${event.time}</small>
              <p>${event.venue}</p>
            </div>
          `).join('')}
        </div>
      </section>
    </div>
  `;
}

/**
 * Setup committee chat room
 */
function setupCommitteeChat() {
  const chatView = $('#view-chat');
  if (!chatView) return;

  const committee = data.committee || {};
  const chatMessages = committee.chatMessages || [];
  const token = WH.query('token');

  chatView.innerHTML = `
    <div class="committee-chat-container">
      <div class="chat-header">
        <h2>Committee Chat Room</h2>
        <p class="chat-notice">Private conversation for committee members only</p>
      </div>

      <div class="chat-messages" id="chatMessages">
        ${chatMessages.length ? chatMessages.map(msg => `
          <div class="chat-message ${msg.senderId === committeeUser.id ? 'own-message' : ''}">
            <div class="message-sender">${msg.senderName}</div>
            <div class="message-content">${WH.escape(msg.content)}</div>
            <div class="message-time">${new Date(msg.timestamp).toLocaleTimeString()}</div>
          </div>
        `).join('') : '<p class="empty-state">No messages yet. Start the conversation!</p>'}
      </div>

      <form class="chat-input-form" id="chatForm">
        <input type="text" id="chatInput" placeholder="Type a message..." required>
        <button type="submit" class="button primary">Send</button>
      </form>
    </div>
  `;

  // Setup chat message sending
  const chatForm = $('#chatForm');
  if (chatForm) {
    chatForm.addEventListener('submit', (e) => {
      e.preventDefault();
      sendChatMessage(token);
    });
  }

  // Auto-scroll to latest message
  const messagesContainer = $('#chatMessages');
  if (messagesContainer) {
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
  }
}

/**
 * Send committee chat message
 */
async function sendChatMessage(token) {
  const input = $('#chatInput');
  const message = input.value.trim();
  if (!message) return;

  const committee = data.committee || {};
  if (!committee.chatMessages) committee.chatMessages = [];

  // Add message locally
  committee.chatMessages.push({
    id: `msg_${Date.now()}`,
    senderId: committeeUser.id,
    senderName: committeeUser.name,
    content: message,
    timestamp: new Date().toISOString()
  });

  input.value = '';
  WH.saveData(data);

  // Send to API
  try {
    if (API.isOnline()) {
      await API.sendCommitteeMessage(token, message);
    }
  } catch (error) {
    console.warn('Message sync failed; saved locally:', error);
  }

  setupCommitteeChat();
}

/**
 * Persist changes
 */
async function persist(message = 'Changes saved') {
  WH.saveData(data);
  try {
    await API.saveWedding(data);
    WH.toast(API.isOnline() ? `${message} · synced` : `${message} locally`);
  } catch (error) {
    WH.toast(`${message} locally · API unavailable`);
  }
}

// Initialize on page load
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initializeCommitteeDashboard);
} else {
  initializeCommitteeDashboard();
}

// Export for testing
window.CommitteeDashboard = {
  initializeCommitteeDashboard,
  renderCommitteeDashboard,
  switchTab,
  sendChatMessage,
  persist
};
