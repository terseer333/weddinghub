const WH = WeddingHub;
const API = WeddingHubAPI;
const $ = selector => document.querySelector(selector);
const $$ = selector => document.querySelectorAll(selector);
let data = WH.getData();

function initials(value) {
  return String(value || "Wedding Admin").split(/\s+/).map(part => part[0]).join("").slice(0, 2).toUpperCase();
}
function firstName(value) { return String(value || "").trim().split(/\s+/)[0] || "there"; }
function currentAdmin() {
  try { return JSON.parse(localStorage.getItem("weddinghub_user") || localStorage.getItem("weddinghub_local_profile") || localStorage.getItem("weddinghub_demo_user")) || {}; }
  catch (_) { return {}; }
}
function openSidebar() {
  $('#sidebar').classList.add('open');
  $('#sidebarBackdrop').classList.add('open');
  document.body.classList.add('menu-open');
  $('#sidebarClose').focus();
}
function closeSidebar() {
  $('#sidebar').classList.remove('open');
  $('#sidebarBackdrop').classList.remove('open');
  document.body.classList.remove('menu-open');
}
function openView(name) {
  const target = document.getElementById(`view-${name}`) ? name : "overview";
  $$('.admin-view').forEach(view => view.classList.toggle('active', view.id === `view-${target}`));
  $$('.side-link[data-view]').forEach(link => link.classList.toggle('active', link.dataset.view === target));
  closeSidebar();
  history.replaceState(null, '', `#${target}`);
  scrollTo({ top: 0, behavior: 'smooth' });
}
function setupIdentity() {
  const admin = currentAdmin();
  const name = admin.fullName || admin.email?.split('@')[0] || 'Wedding Admin';
  $('#adminName').textContent = name;
  $('#userAvatar').textContent = initials(name);
  $('#welcomeHeading').textContent = `Welcome back, ${firstName(name)}.`;
  $('#switcherNames').textContent = `${data.wedding.brideName} & ${data.wedding.groomName}`;
  $('#switcherDate').textContent = WH.formatDate(data.wedding.date);
  $('#coupleAvatar').textContent = `${initials(data.wedding.brideName)[0] || ''}&${initials(data.wedding.groomName)[0] || ''}`;
}
function completion() {
  const checks = [data.wedding.brideName, data.wedding.groomName, data.wedding.date, data.wedding.venue,
    data.wedding.message, data.events.length, data.photos.length >= 3, data.stories.length, data.guests.length, data.wedding.templateId];
  return Math.round(checks.filter(Boolean).length / checks.length * 100);
}
function renderOverview() {
  const attending = data.guests.filter(guest => guest.rsvp === 'attending');
  const pending = data.guests.filter(guest => guest.rsvp === 'pending');
  const viewed = data.guests.filter(guest => ['opened','accepted','declined'].includes(guest.invitationStatus));
  $('#metricGuests').textContent = data.guests.length;
  $('#metricAttending').textContent = attending.reduce((sum, guest) => sum + Number(guest.partySize || 1), 0);
  $('#metricPending').textContent = pending.length;
  $('#metricViews').textContent = viewed.length;
  const progress = completion();
  $('#progressValue').textContent = `${progress}%`;
  $('#progressRing').style.background = `conic-gradient(#527565 ${progress}%, #d9e2dc 0)`;
  $('#progressBar').style.width = `${progress}%`;
  $('#progressTitle').textContent = progress === 100 ? 'Your wedding experience is ready' : 'Complete your wedding experience';
  $('#progressMessage').textContent = progress === 100 ? 'Your published experience is ready to share with guests.' : `${10 - Math.round(progress / 10)} setup steps remaining.`;
  const next = WH.published(data.events).sort((a,b) => `${a.date}${a.time}`.localeCompare(`${b.date}${b.time}`))[0];
  $('#nextEvent').innerHTML = next ? `<span class="calendar-tile"><b>${new Date(`${next.date}T12:00`).getDate()}</b>${new Date(`${next.date}T12:00`).toLocaleString('en',{month:'short'}).toUpperCase()}</span><div><strong>${WH.escape(next.name)}</strong><small>${next.time} · ${WH.escape(next.venue)}</small></div>` : '<p class="empty-inline">Add a published event to build your schedule.</p>';
  const activities = [...data.guests].reverse().slice(0, 2).map(guest => ({ icon: initials(guest.name), title: guest.name, detail: guest.invitationStatus === 'accepted' ? `accepted · party of ${guest.partySize}` : `invitation ${guest.invitationStatus}` }));
  if (data.messages[0]) activities.push({ icon: '✉', title: data.messages[0].name, detail: 'left a wedding wish' });
  $('#activityList').innerHTML = activities.length ? activities.map(item => `<p><i>${WH.escape(item.icon)}</i><span><strong>${WH.escape(item.title)}</strong> ${WH.escape(item.detail)}<small>Latest activity</small></span></p>`).join('') : '<p class="empty-inline">Guest activity will appear here.</p>';
  const preview = attending[0] || data.guests[0];
  $$('a[href*="event.html?"]').forEach(anchor => anchor.href = preview ? `event.html?token=${preview.token}` : 'event.html?preview=admin');
}
WH.countdown(data.wedding.date, value => {
  $('#adminCountdown').innerHTML = value.passed ? '<strong>Today we celebrate love</strong>' : [['Days',value.days],['Hours',value.hours],['Minutes',value.minutes],['Seconds',value.seconds]].map(([label,number]) => `<div><strong>${String(number).padStart(2,'0')}</strong><span>${label}</span></div>`).join('');
});
function fillWeddingForm() {
  const form = $('#weddingForm');
  Object.entries(data.wedding).forEach(([key,value]) => { if (form.elements[key]) form.elements[key].value = key === 'date' ? String(value).slice(0,16) : value; });
}
function renderTemplates() {
  const query = $('#templateSearch').value.toLowerCase();
  const category = $('#templateCategory').value;
  const templates = WH.templates().filter(item => (!category || item.category === category) && `${item.name} ${item.category}`.toLowerCase().includes(query));
  $('#templateGrid').innerHTML = templates.map(item => `<article class="template-card ${data.wedding.templateId === item.id ? 'selected' : ''}"><div class="template-preview design-${item.design} variant-${item.variant}"><span class="template-ornament top" aria-hidden="true"></span><div class="template-card-inner"><span class="template-label">${item.premium ? 'PREMIUM COLLECTION' : item.category.toUpperCase()}</span><small>Together with their families</small><h3>${WH.escape(data.wedding.brideName)} <i>&</i><br>${WH.escape(data.wedding.groomName)}</h3><p>${WH.formatDate(data.wedding.date)}</p><em>${WH.escape(data.wedding.venue || 'Wedding venue')}</em></div><span class="template-ornament bottom" aria-hidden="true"></span></div><div class="template-meta"><span><strong>${item.name}</strong><small>${item.category}</small></span><div class="template-actions"><button class="preview-template" data-preview-template="${item.id}">Preview</button><button data-template="${item.id}">${data.wedding.templateId === item.id ? 'Selected' : 'Apply'}</button></div></div></article>`).join('');
}
function renderEvents() {
  $('#eventAdminList').innerHTML = data.events.length ? data.events.map(event => `<article class="manage-card"><span class="calendar-tile"><b>${new Date(`${event.date}T12:00`).getDate()}</b>${new Date(`${event.date}T12:00`).toLocaleString('en',{month:'short'}).toUpperCase()}</span><div class="manage-copy"><h3>${WH.escape(event.name)} <i class="status ${event.status}">${event.status}</i></h3><p>${event.time} · ${WH.escape(event.venue)}</p><small>${WH.escape(event.description)}</small></div><div class="row-menu"><button data-action="edit-event" data-id="${event.id}">Edit</button><button data-action="delete-event" data-id="${event.id}">Delete</button></div></article>`).join('') : emptyState('No events yet', 'Add your ceremony, reception or another celebration.');
}
function renderGuests(filter = '') {
  const guests = data.guests.filter(guest => `${guest.name} ${guest.email}`.toLowerCase().includes(filter.toLowerCase()));
  $('#guestTable').innerHTML = '<div class="table-row table-head"><span>Guest</span><span>Category</span><span>Invitation</span><span>RSVP</span><span>Party</span><span>Actions</span></div>' + guests.map(guest => `<div class="table-row"><span class="guest-cell"><i>${initials(guest.name)}</i><span><strong>${WH.escape(guest.name)}</strong><small>${WH.escape(guest.email)}</small></span></span><span>${WH.escape(guest.category)}</span><span><b class="status ${guest.invitationStatus}">${guest.invitationStatus}</b></span><span>${guest.rsvp}</span><span>${guest.partySize}</span><span class="row-menu"><button data-share-guest="${guest.id}">Share</button><button data-copy="${guest.token}">Copy</button><button data-action="delete-guest" data-id="${guest.id}">Delete</button></span></div>`).join('');
}
function renderContent() {
  $('#photoAdminGrid').innerHTML = data.photos.length ? data.photos.map(photo => `<figure><img src="${photo.url}" alt="${WH.escape(photo.caption)}"><figcaption>${WH.escape(photo.caption)} <button data-action="delete-photo" data-id="${photo.id}" aria-label="Delete photo">×</button></figcaption></figure>`).join('') : emptyState('No photos yet','Add a photo URL to begin your gallery.');
  $('#storyAdminList').innerHTML = data.stories.length ? [...data.stories].sort((a,b)=>a.order-b.order).map(story => `<article><span>☷</span><div><small>${WH.escape(story.year)}</small><strong>${WH.escape(story.title)}</strong><p>${WH.escape(story.content)}</p></div><b class="status ${story.status}">${story.status}</b><div class="row-menu"><button data-action="edit-story" data-id="${story.id}">Edit</button><button data-action="delete-story" data-id="${story.id}">Delete</button></div></article>`).join('') : emptyState('No story chapters yet','Tell guests how your journey began.');
}
function renderAnnouncements() {
  $('#announcementList').innerHTML = data.announcements.length ? data.announcements.map(item => `<article class="manage-card"><span class="manage-icon">◉</span><div class="manage-copy"><h3>${WH.escape(item.title)} <i class="status ${item.status}">${item.status}</i></h3><p>${WH.escape(item.message)}</p><small>${WH.formatDate(item.date)}</small></div><div class="row-menu"><button data-action="edit-announcement" data-id="${item.id}">Edit</button><button data-action="delete-announcement" data-id="${item.id}">Delete</button></div></article>`).join('') : emptyState('No announcements','Publish an update when guests need to know something.');
}
function renderMessages() {
  $('#messageList').innerHTML = data.messages.length ? data.messages.map(message => `<article class="manage-card"><span class="activity-avatar">${initials(message.name)}</span><div class="manage-copy"><h3>${WH.escape(message.name)} <i class="status ${message.status}">${message.status}</i></h3><p>“${WH.escape(message.message)}”</p><small>${WH.formatDate(message.date)}</small></div><div class="row-menu"><button data-action="approve-message" data-id="${message.id}">Approve</button><button data-action="hide-message" data-id="${message.id}">Hide</button><button data-action="delete-message" data-id="${message.id}">Delete</button></div></article>`).join('') : emptyState('No guest messages','Wishes and blessings will appear here.');
}
function emptyState(title, text) { return `<div class="empty-state"><span>✦</span><h3>${title}</h3><p>${text}</p></div>`; }
function renderAll() { setupIdentity(); renderOverview(); fillWeddingForm(); renderTemplates(); renderEvents(); renderGuests(); renderContent(); renderAnnouncements(); renderMessages(); }
async function persist(message = 'Changes saved') {
  WH.saveData(data);
  renderAll();
  try { await API.saveWedding(data); WH.toast(API.isOnline() ? `${message} · synced` : `${message} locally`); }
  catch (error) { WH.toast(`${message} locally · API unavailable`); }
}
window.AdminApp = { get data(){return data}, set data(value){data=value}, openView, renderAll, renderTemplates, renderGuests, persist, initials };
$$('.side-link[data-view]').forEach(link => link.onclick = () => openView(link.dataset.view));
$$('[data-jump]').forEach(button => button.onclick = () => openView(button.dataset.jump));
$('#menuButton').onclick = openSidebar;
$('#sidebarClose').onclick = closeSidebar;
$('#sidebarBackdrop').onclick = closeSidebar;
document.addEventListener('keydown', event => { if (event.key === 'Escape') closeSidebar(); });
$('#templateSearch').oninput = renderTemplates;
$('#templateCategory').onchange = renderTemplates;
$('#guestSearch').oninput = event => renderGuests(event.target.value);
const categories = [...new Set(WH.templates().map(item => item.category))];
$('#templateCategory').innerHTML += categories.map(category => `<option>${category}</option>`).join('');
renderAll();
if (location.hash) openView(location.hash.slice(1));
async function syncFromAPI(){const result=await API.bootstrap(data);data=result.data;$('#apiStatus').textContent=result.online?'● API connected':'● Offline mode';$('#apiStatus').classList.toggle('online',result.online);renderAll()}
syncFromAPI();
setInterval(()=>{const active=document.querySelector('.admin-view.active')?.id;if(['view-overview','view-guests','view-messages'].includes(active)&&!document.querySelector('.modal-backdrop.open'))syncFromAPI()},30000);
document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='visible')syncFromAPI()});