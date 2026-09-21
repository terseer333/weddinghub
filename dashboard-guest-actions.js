async function copyInvitation(token){const url=new URL(`event.html?token=${token}`,location.href).href;const guest=AdminApp.data.guests.find(item=>item.token===token);if(guest&&guest.invitationStatus==='pending'){guest.invitationStatus='sent';WH.saveData(AdminApp.data);AdminApp.renderAll()}try{await navigator.clipboard.writeText(url);WH.toast('Invitation link copied and marked as sent')}catch(_){AdminModal.show(`<p class="eyebrow">Invitation link</p><h2>Copy this secure link</h2><label class="copy-field"><input value="${AdminModal.field(url)}" readonly></label>`)}}
function invitationUrl(token) { return new URL(`event.html?token=${token}`, location.href).href; }

function markInvitationSent(guest) {
  if (guest && guest.invitationStatus === 'pending') {
    guest.invitationStatus = 'sent';
    WH.saveData(AdminApp.data);
    AdminApp.renderAll();
  }
}

function openMailto(href) {
  const link = document.createElement('a');
  link.href = href;
  link.rel = 'noopener';
  document.body.appendChild(link);
  link.click();
  link.remove();
}

// WeddingHub's own delivery is attempted first. When the API is offline or has no
// provider configured, delivery falls back to the admin's own mail app or WhatsApp.
// The result is cached so later sends skip a backend that already reported unavailable.
let backendDeliveryReady = null;

function canUseBackend(guest) {
  return Boolean(window.WeddingHubAPI && WeddingHubAPI.isOnline() && guest && guest.apiInvitationId && guest.token);
}

async function tryBackendSend(guest, channels) {
  if (backendDeliveryReady === false || !canUseBackend(guest)) return null;
  try {
    const response = await WeddingHubAPI.sendInvitation(null, guest.apiInvitationId, guest.token, channels);
    const delivered = ((response && response.results) || []).filter(item => item.status === 'sent');
    backendDeliveryReady = delivered.length ? true : false;
    return delivered.length ? delivered : null;
  } catch (error) {
    console.warn('WeddingHub delivery unavailable; using the client delivery fallback:', error);
    backendDeliveryReady = false;
    return null;
  }
}

// Opens WhatsApp in a new tab, falling back to a same-tab navigation when the
// browser blocks a window opened outside the original click.
function openWhatsApp(guest, links) {
  const opened = window.open(links.whatsappHref, '_blank', 'noopener');
  if (!opened) location.href = links.whatsappHref;
  WH.toast(`Opening WhatsApp for ${guest.phone || 'this guest'}`);
}

// Deliver the personal invitation to the guest's registered email address.
async function sendInvitationEmail(guest) {
  const links = WeddingInvitation.shareLinks(guest, invitationUrl(guest.token), AdminApp.data.wedding);
  const delivered = await tryBackendSend(guest, ['email']);
  if (delivered) {
    markInvitationSent(guest);
    WH.toast(`Invitation emailed to ${guest.email}`);
    return links;
  }
  if (!links.hasEmail) { WH.toast('Add an email address for this guest first'); return links; }
  markInvitationSent(guest);
  openMailto(links.emailHref);
  WH.toast(`Opening your email app for ${guest.email}`);
  return links;
}

// Deliver the personal invitation to the guest's registered WhatsApp number.
function sendInvitationWhatsApp(guest) {
  const links = WeddingInvitation.shareLinks(guest, invitationUrl(guest.token), AdminApp.data.wedding);
  if (!links.hasWhatsApp) { WH.toast('Add a WhatsApp number for this guest first'); return links; }
  if (backendDeliveryReady !== false && canUseBackend(guest)) {
    tryBackendSend(guest, ['whatsapp']).then(delivered => {
      markInvitationSent(guest);
      if (delivered) WH.toast(`Invitation sent to ${guest.phone} on WhatsApp`);
      else openWhatsApp(guest, links);
    });
    return links;
  }
  markInvitationSent(guest);
  openWhatsApp(guest, links);
  return links;
}

async function openShare(guest) {
  const url = invitationUrl(guest.token);
  const template = WeddingInvitation.selected(AdminApp.data);
  const links = WeddingInvitation.shareLinks(guest, url, AdminApp.data.wedding);
  AdminModal.show(`<div class="share-layout"><div class="share-heading"><p class="eyebrow">Ready to send</p><h2>${template.name}</h2><p>Email this invitation to ${WH.escape(guest.email || 'the guest')}${links.hasWhatsApp ? ` or send it to ${WH.escape(guest.phone)} on WhatsApp` : ''}. The artwork contains wedding information only; personal access is provided below.</p></div>${WeddingInvitation.card(AdminApp.data, template.id, 'share-invitation-card')}<div class="share-separator"><span>Personal access</span></div>${WeddingInvitation.accessPanel(guest, url, AdminApp.data.wedding)}</div>`);
  const emailButton = document.querySelector('[data-email-share], [data-email-missing]');
  if (emailButton) emailButton.onclick = event => {
    event.preventDefault();
    if (emailButton.hasAttribute('data-email-missing')) { WH.toast('Add an email address for this guest first'); return; }
    sendInvitationEmail(guest);
  };
  const whatsappButton = document.querySelector('[data-whatsapp-share]');
  if (whatsappButton) whatsappButton.onclick = event => { event.preventDefault(); sendInvitationWhatsApp(guest); };
  document.querySelector('[data-share-copy]').onclick = () => copyInvitation(guest.token);
  document.querySelector('[data-download-card]').onclick = async () => { const blob = await WeddingInvitation.imageBlob(AdminApp.data, template.id), link = document.createElement('a'); link.href = URL.createObjectURL(blob); link.download = `${AdminApp.data.wedding.slug || 'wedding'}-invitation.png`; link.click(); URL.revokeObjectURL(link.href); };
  document.querySelector('[data-native-share]').onclick = async () => {
    try {
      const blob = await WeddingInvitation.imageBlob(AdminApp.data, template.id), file = new File([blob], 'wedding-invitation.png', { type: 'image/png' });
      if (navigator.canShare?.({ files: [file] })) await navigator.share({ title: `${AdminApp.data.wedding.brideName} & ${AdminApp.data.wedding.groomName}`, text: links.text, files: [file] });
      else if (navigator.share) await navigator.share({ title: 'Wedding invitation', text: links.text, url });
      else throw new Error('Sharing is not supported');
    } catch (error) { if (error.name !== 'AbortError') WH.toast('Use Email, WhatsApp or Copy Link to share'); }
  };
  markInvitationSent(guest);
}
document.getElementById('shareButton').onclick=()=>{const guest=AdminApp.data.guests.find(item=>item.rsvp==='attending')||AdminApp.data.guests[0];if(guest)openShare(guest);else{AdminApp.openView('guests');WH.toast('Add a guest before sharing an invitation')}};

function openAddCommitteeMemberModal() {
  const roles = AdminApp.data.committeeRoles || [];
  const roleOptions = roles.map(r => `<option value="${r.id}">${WH.escape(r.name)} (${WH.escape(r.description || '')})</option>`).join('');
  const form = `
    <p class="eyebrow">Planning Team</p>
    <h2>Invite Committee Member</h2>
    <p class="modal-intro">Committee members receive private workspace access to manage schedules, tasks, and team chat.</p>
    <form id="committeeMemberForm" class="modal-form">
      <label>Full name<input name="name" required autocomplete="name" placeholder="e.g. Jane Doe"></label>
      <label>Email address<input name="email" type="email" autocomplete="email" placeholder="jane@example.com"></label>
      <label>Phone number<input name="phone" type="tel" autocomplete="tel" placeholder="+1 234 567 8900"></label>
      <label>Committee Role
        <select name="roleId" id="committeeRoleSelect" required>
          ${roleOptions}
          <option value="custom">＋ Create New Custom Role...</option>
        </select>
      </label>
      <div id="customRoleFields" style="display:none;background:#f8fafc;padding:12px;border-radius:8px;border:1px solid #e2e8f0;margin-bottom:12px;">
        <label>Custom Role Name<input name="customName" placeholder="e.g. Decoration Lead, Logistics Coordinator"></label>
        <label>Custom Role Description<input name="customDesc" placeholder="Responsibilities and scope"></label>
      </div>
      <button class="button primary" type="submit">Create Committee Invitation</button>
    </form>
  `;
  AdminModal.show(form);
  const select = document.getElementById('committeeRoleSelect');
  const customFields = document.getElementById('customRoleFields');
  select.onchange = () => { customFields.style.display = select.value === 'custom' ? 'block' : 'none'; };
  document.getElementById('committeeMemberForm').onsubmit = async event => {
    event.preventDefault();
    const fd = new FormData(event.target);
    let roleId = fd.get('roleId');
    let roleTitle = '';
    if (roleId === 'custom') {
      const customName = (fd.get('customName') || '').trim();
      const customDesc = (fd.get('customDesc') || '').trim();
      if (!customName) { WH.toast('Please specify a custom role name'); return; }
      const newRole = { id: `role-${Date.now()}`, name: customName, description: customDesc, is_custom: true };
      try {
        const res = await API.createCommitteeRole(null, newRole);
        if (res && res.id) newRole.id = res.id;
      } catch (_) {}
      AdminApp.data.committeeRoles = AdminApp.data.committeeRoles || [];
      AdminApp.data.committeeRoles.push(newRole);
      roleId = newRole.id;
      roleTitle = customName;
    } else {
      const found = roles.find(r => r.id === roleId);
      roleTitle = found ? found.name : 'Committee member';
    }
    const member = {
      name: fd.get('name'),
      email: fd.get('email') || '',
      phone: fd.get('phone') || '',
      roleId: roleId,
      role_id: roleId,
      title: roleTitle,
      committeeTitle: roleTitle,
      type: 'committee',
      partySize: 1,
      id: `cm_${Date.now()}`,
      token: `invite-${crypto.randomUUID()}`,
      invitationStatus: 'pending'
    };
    try {
      const created = await API.addGuest(member);
      if (created) {
        member.token = created.token;
        member.apiInvitationId = created.invitation.id;
      }
    } catch (err) { console.warn(err); }
    AdminApp.data.committeeMembers = AdminApp.data.committeeMembers || [];
    AdminApp.data.committeeMembers.unshift(member);
    WH.saveData(AdminApp.data);
    AdminApp.renderAll();
    AdminModal.close();
    WH.toast(API.isOnline() ? 'Committee member invited securely' : 'Member saved in offline mode');
  };
}

function openAddRoleModal() {
  const form = `
    <p class="eyebrow">Role Hierarchy</p>
    <h2>Add Custom Committee Role</h2>
    <p class="modal-intro">Define custom committee roles specific to your wedding ceremony and events.</p>
    <form id="newRoleForm" class="modal-form">
      <label>Role Name<input name="name" required placeholder="e.g. Protocol, Media & Live Streaming, Catering Lead"></label>
      <label>Role Description<input name="description" placeholder="Key responsibilities and area of focus"></label>
      <button class="button primary" type="submit">Create Role</button>
    </form>
  `;
  AdminModal.show(form);
  document.getElementById('newRoleForm').onsubmit = async event => {
    event.preventDefault();
    const fd = new FormData(event.target);
    const name = (fd.get('name') || '').trim();
    const description = (fd.get('description') || '').trim();
    if (!name) return;
    const role = { id: `role-${Date.now()}`, name, description, is_custom: true };
    try {
      const res = await API.createCommitteeRole(null, role);
      if (res && res.id) role.id = res.id;
    } catch (_) {}
    AdminApp.data.committeeRoles = AdminApp.data.committeeRoles || [];
    AdminApp.data.committeeRoles.push(role);
    await AdminApp.persist(`Added role: ${name}`);
    AdminModal.close();
  };
}

const addCommitteeMemberBtn = document.getElementById('addCommitteeMember');
if (addCommitteeMemberBtn) addCommitteeMemberBtn.onclick = openAddCommitteeMemberModal;

const addCommitteeRoleBtn = document.getElementById('addCommitteeRoleBtn');
if (addCommitteeRoleBtn) addCommitteeRoleBtn.onclick = openAddRoleModal;

document.getElementById('addGuest').onclick=()=>{
  const roles = AdminApp.data.committeeRoles || [];
  const roleOptions = roles.map(r => `<option value="${r.id}">${WH.escape(r.name)}</option>`).join('');
  const form=`<p class="eyebrow">Personal invitation</p><h2>Add an invite</h2><p class="modal-intro">WeddingHub will create a unique invitation link for this person.</p><form id="guestForm" class="modal-form"><label>Invitation type<select name="type" id="inviteType"><option value="guest">Guest — RSVP & guest dashboard</option><option value="committee">Committee member — plan the wedding together</option></select></label><label>Full name<input name="name" required autocomplete="name"></label><label>Email address<input name="email" type="email" autocomplete="email"></label><label>Phone number<input name="phone" type="tel" autocomplete="tel"></label><label class="check-line"><input name="whatsapp" type="checkbox" checked> This number is on WhatsApp</label><div class="form-grid"><label>Category<select name="category"><option>Family</option><option>Friends</option><option>Colleagues</option><option>VIP</option></select></label><fieldset id="partySizeField" class="form-fieldset"><label>Maximum party size<input name="partySize" type="number" min="1" max="20" value="1" required></label></fieldset><fieldset id="committeeTitleField" class="form-fieldset" hidden><label>Committee role<select name="committeeRoleId">${roleOptions}</select></label></fieldset></div><button class="button primary" type="submit">Create invitation</button></form>`;AdminModal.show(form);const typeSelect=document.getElementById('inviteType');const toggleFields=()=>{const committee=typeSelect.value==='committee';document.getElementById('partySizeField').hidden=committee;document.getElementById('committeeTitleField').hidden=!committee};typeSelect.onchange=toggleFields;toggleFields();document.getElementById('guestForm').onsubmit=async event=>{event.preventDefault();const invite=Object.fromEntries(new FormData(event.target));if(invite.type==='committee'){
    const roleId = invite.committeeRoleId || (roles[0] ? roles[0].id : '');
    if (!roleId) {
      AdminModal.close();
      WH.toast('Create a committee role first, then invite members');
      return;
    }
    const roleObj = roles.find(r => r.id === roleId);
    const roleTitle = roleObj ? roleObj.name : 'Committee member';
    const member=Object.assign({name:invite.name,email:invite.email||'',phone:invite.phone||'',roleId,role_id:roleId,title:roleTitle,committeeTitle:roleTitle,type:'committee',partySize:1},invite);Object.assign(member,{id:`cm_${Date.now()}`,token:`invite-${crypto.randomUUID()}`,invitationStatus:'pending'});try{const created=await API.addGuest(member);if(created){member.token=created.token;member.apiInvitationId=created.invitation.id}}catch(error){console.warn(error)}AdminApp.data.committeeMembers=AdminApp.data.committeeMembers||[];AdminApp.data.committeeMembers.unshift(member);WH.saveData(AdminApp.data);AdminApp.renderAll();AdminModal.close();WH.toast(API.isOnline()?'Committee invitation created securely':'Member saved in offline mode');return}const guest=Object.fromEntries(new FormData(event.target));guest.partySize=Number(guest.partySize);guest.whatsapp=Boolean(guest.whatsapp);Object.assign(guest,{id:`guest_${Date.now()}`,token:`invite-${crypto.randomUUID()}`,invitationStatus:'pending',rsvp:'pending',type:'guest'});try{const created=await API.addGuest(guest);if(created){guest.token=created.token;guest.apiInvitationId=created.invitation.id}}catch(error){console.warn(error)}AdminApp.data.guests.unshift(guest);WH.saveData(AdminApp.data);AdminApp.renderAll();AdminModal.close();WH.toast(API.isOnline()?'Guest invitation created securely':'Guest saved in offline mode');openShare(guest)}};

document.addEventListener('click',async event=>{const share=event.target.closest('[data-share-guest]');if(share){const guest=AdminApp.data.guests.find(item=>item.id===share.dataset.shareGuest);if(guest)return openShare(guest)}const copy=event.target.closest('[data-copy]');if(copy)return copyInvitation(copy.dataset.copy);const button=event.target.closest('[data-action="delete-guest"]');if(button){if(confirm('Remove this guest from your local list?')){AdminApp.data.guests=AdminApp.data.guests.filter(item=>item.id!==button.dataset.id);await AdminApp.persist('Guest removed')}return}
const memberButton=event.target.closest('[data-action="delete-member"]');if(memberButton){if(confirm('Remove this committee member from the planning team?')){const mid=memberButton.dataset.id;const member=(AdminApp.data.committeeMembers||[]).find(item=>item.id===mid);const apiMemberId = member?.apiMembershipId || member?.apiInvitationId || mid;try{await API.deleteCommitteeMember(null,apiMemberId);}catch(_){}AdminApp.data.committeeMembers=(AdminApp.data.committeeMembers||[]).filter(item=>item.id!==mid);await AdminApp.persist('Member removed')}return}
const roleDeleteButton=event.target.closest('[data-action="delete-role"]');if(roleDeleteButton){const roleId=roleDeleteButton.dataset.id;if(confirm('Delete this committee role? Assigned members will become unassigned.')){try{await API.deleteCommitteeRole(null,roleId);}catch(_){}AdminApp.data.committeeRoles=(AdminApp.data.committeeRoles||[]).filter(r=>r.id!==roleId);(AdminApp.data.committeeMembers||[]).forEach(m=>{if((m.roleId||m.role_id)===roleId){m.roleId='';m.role_id='';m.title='Committee member'}});await AdminApp.persist('Role deleted')}}
});

document.addEventListener('click', async event => {
  const emailGuest = event.target.closest('[data-email-guest]');
  if (emailGuest) { const guest = AdminApp.data.guests.find(item => item.id === emailGuest.dataset.emailGuest); if (guest) return sendInvitationEmail(guest); }
  const whatsappGuest = event.target.closest('[data-whatsapp-guest]');
  if (whatsappGuest) { const guest = AdminApp.data.guests.find(item => item.id === whatsappGuest.dataset.whatsappGuest); if (guest) return sendInvitationWhatsApp(guest); }
});

// One composed reminder for every guest for whom WeddingHub could not deliver
// personally. It cannot carry each guest's private link, so it only reminds them.
function bulkReminderMailto(guests) {
  const wedding = AdminApp.data.wedding;
  const couple = [wedding.brideName, wedding.groomName].filter(Boolean).join(' & ') || 'our wedding';
  const subject = `You're invited to the wedding of ${couple}`;
  const body = `You're warmly invited to celebrate the wedding of ${couple}.\n\nWatch out for your personal invitation link from us.\n\nWith love.`;
  return `mailto:?bcc=${encodeURIComponent(guests.map(guest => guest.email).join(','))}&subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(body)}`;
}

// Sends every registered guest their own invitation through WeddingHub. Guests the
// backend could not reach fall back to a single group reminder.
async function emailAllGuests() {
  const guests = (AdminApp.data.guests || []).filter(guest => String(guest.email || '').trim());
  if (!guests.length) { WH.toast('Add guest email addresses first'); return; }
  const undelivered = [];
  let delivered = 0;
  for (const guest of guests) {
    if (await tryBackendSend(guest, ['email'])) delivered++;
    else undelivered.push(guest);
  }
  guests.forEach(markInvitationSent);
  if (!undelivered.length) { WH.toast(`Invitation emails sent to ${delivered} guests`); return; }
  openMailto(bulkReminderMailto(undelivered));
  WH.toast(delivered ? `Emailed ${delivered}; opening a group reminder for the rest` : 'Opening your mail app for a group reminder');
}

const emailAllGuestsButton = document.getElementById('emailAllGuests');
if (emailAllGuestsButton) emailAllGuestsButton.onclick = () => emailAllGuests();
