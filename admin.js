(() => {
  "use strict";
  const API = window.WeddingHubAPI;
  const view = document.getElementById("adminView");
  const nav = document.getElementById("adminNav");
  const sidebar = document.getElementById("sidebar");
  const backdrop = document.getElementById("sidebarBackdrop");
  const status = document.getElementById("apiStatus");
  const adminName = document.getElementById("adminName");
  const adminEmail = document.getElementById("adminEmail");
  const esc = value => String(value ?? "").replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
  const date = value => value ? new Date(value).toLocaleDateString(undefined, {year:"numeric",month:"short",day:"numeric"}) : "—";
  const initials = text => String(text || "").trim().split(/\s+/).filter(Boolean).map(x => x[0]).join("").slice(0,2).toUpperCase() || "WA";
  const emptyRow = (span, text) => `<tr><td colspan="${span}" class="admin-empty">${esc(text)}</td></tr>`;
  const table = (heads, body) => `<div class="admin-table-wrap"><table class="admin-table"><thead><tr>${heads.map(x=>`<th>${esc(x)}</th>`).join("")}</tr></thead><tbody>${body}</tbody></table></div>`;
  const notify = text => { const node=document.getElementById("adminNotice"); if(node){node.textContent=text;node.hidden=false;setTimeout(()=>node.hidden=true,4000);} };
  const sections = {
    dashboard:{title:"Dashboard",eyebrow:"Platform administration",blurb:"A summary of the whole WeddingHub platform."},
    users:{title:"Users",eyebrow:"Accounts",blurb:"Review and manage WeddingHub accounts."},
    weddings:{title:"Weddings",eyebrow:"Content",blurb:"Review wedding pages and publication status."},
    security:{title:"Admin Security",eyebrow:"Safety",blurb:"Review administrator sign-ins and audit history."},
    payments:{title:"Payments",eyebrow:"Billing",blurb:"Transactions and subscriptions.",planned:"Payments are not configured yet."},
    analytics:{title:"Analytics",eyebrow:"Reporting",blurb:"How the platform is being used.",planned:"Analytics reports are not built yet."},
    reports:{title:"Reports",eyebrow:"Moderation",blurb:"Reports and complaints from users.",planned:"User reports are not configured yet."},
    notifications:{title:"Notifications",eyebrow:"Communication",blurb:"Announcements to users.",planned:"Platform notifications are not configured yet."},
    support:{title:"Support",eyebrow:"Help desk",blurb:"Requests from users.",planned:"Support tickets are not configured yet."},
    settings:{title:"Website Settings",eyebrow:"Configuration",blurb:"Platform-wide settings.",planned:"Website settings are not built yet."}
  };
  function heading(section){return `<div class="page-heading"><div><p class="eyebrow">${esc(section.eyebrow)}</p><h1>${esc(section.title)}</h1><p>${esc(section.blurb)}</p></div><button class="button secondary" id="refreshAdmin" type="button">Refresh</button></div><p class="admin-notice" id="adminNotice" hidden role="status"></p>`;}
  function stat(label,value){return `<article class="admin-stat"><small>${esc(label)}</small><strong>${esc(value)}</strong></article>`;}
  async function showSection(id){
    const section=sections[id]; if(!section)return;
    nav.querySelectorAll("[data-view]").forEach(button=>button.classList.toggle("active",button.dataset.view===id));
    adminName.textContent=id==="dashboard"?`Welcome, ${adminEmail.textContent}`:section.title;
    if(section.planned){view.innerHTML=heading(section)+`<article class="admin-card platform-empty"><h2>Not available yet</h2><p>${esc(section.planned)}</p></article>`;return;}
    view.innerHTML=heading(section)+`<div class="admin-loading">Loading…</div>`;
    try {
      if(id==="dashboard") await loadDashboard(section);
      if(id==="users") await loadUsers(section);
      if(id==="weddings") await loadWeddings(section);
      if(id==="security") await loadSecurity(section);
    } catch(error) { view.innerHTML=heading(section)+`<article class="admin-card platform-empty"><h2>Could not load this section</h2><p>${esc(error.message)}</p><button class="button secondary" id="refreshAdmin">Try again</button></article>`; }
    const refresh=document.getElementById("refreshAdmin"); if(refresh)refresh.addEventListener("click",()=>showSection(id));
    closeMenu();
  }
  async function loadDashboard(section){
    const result=await API.adminDashboard(),s=result.stats;
    const users=result.recent_users.map(user=>`<tr><td>${esc(user.display_name)}</td><td>${esc(user.email)}</td><td>${date(user.created_at)}</td></tr>`).join("")||emptyRow(3,"No user accounts yet.");
    const weddings=result.recent_weddings.map(w=>`<tr><td><strong>${esc(w.partner_one)} &amp; ${esc(w.partner_two)}</strong><small>${esc(w.slug)}</small></td><td>${esc(w.status)}</td><td>${date(w.created_at||w.date)}</td></tr>`).join("")||emptyRow(3,"No wedding pages yet.");
    view.innerHTML=heading(section)+`<div class="admin-stat-grid">${stat("Registered users",s.total_users)}${stat("Active accounts",s.active_users)}${stat("Suspended accounts",s.suspended_users)}${stat("New this month",s.new_users_this_month)}${stat("Wedding pages",s.total_weddings)}</div><div class="admin-content-grid"><article class="admin-card"><h2>Recent accounts</h2>${table(["Name","Email","Joined"],users)}<button class="text-button" data-section="users">Manage users →</button></article><article class="admin-card"><h2>Recent weddings</h2>${table(["Couple","Status","Created"],weddings)}<button class="text-button" data-section="weddings">Manage weddings →</button></article></div>`;
  }
  async function loadUsers(section){
    const query=window.adminUserQuery||{search:"",status:""};
    const result=await API.adminUsers(query.search,query.status);
    const body=result.users.map(user=>`<tr><td><strong>${esc(user.display_name)}</strong></td><td>${esc(user.email)}</td><td>${date(user.created_at)}</td><td><span class="admin-badge ${esc(user.status)}">${esc(user.status)}</span></td><td><button data-user-action="status" data-id="${esc(user.id)}" data-status="${user.status==="active"?"suspended":"active"}">${user.status==="active"?"Suspend":"Reactivate"}</button><button class="danger" data-user-action="delete" data-id="${esc(user.id)}">Delete</button></td></tr>`).join("")||emptyRow(5,"No accounts match this filter.");
    view.innerHTML=heading(section)+`<div class="admin-filters"><label>Search<input id="adminUserSearch" type="search" value="${esc(query.search)}" placeholder="Name or email"></label><label>Status<select id="adminUserStatus"><option value="">All accounts</option><option value="active" ${query.status==="active"?"selected":""}>Active</option><option value="suspended" ${query.status==="suspended"?"selected":""}>Suspended</option></select></label></div>${table(["Name","Email","Joined","Status","Actions"],body)}`;
    const search=document.getElementById("adminUserSearch"),filter=document.getElementById("adminUserStatus");
    search.addEventListener("input",()=>{clearTimeout(window.adminSearchTimer);window.adminSearchTimer=setTimeout(()=>{window.adminUserQuery={search:search.value,status:filter.value};loadUsers(section).catch(e=>notify(e.message));},250);});
    filter.addEventListener("change",()=>{window.adminUserQuery={search:search.value,status:filter.value};loadUsers(section).catch(e=>notify(e.message));});
  }
  async function loadWeddings(section){
    const result=await API.adminWeddings();
    const body=result.weddings.map(w=>`<tr><td><strong>${esc(w.partner_one)} &amp; ${esc(w.partner_two)}</strong><small>${esc(w.slug)}</small></td><td>${w.owner?`${esc(w.owner.display_name)}<small>${esc(w.owner.email)}</small>`:"<small>Legacy / unlinked</small>"}</td><td>${date(w.date)}</td><td><span class="admin-badge ${esc(w.status)}">${esc(w.status)}</span></td><td><button data-wedding-action="status" data-id="${esc(w.id)}" data-status="${w.status==="hidden"?"published":"hidden"}">${w.status==="hidden"?"Publish":"Hide"}</button><button class="danger" data-wedding-action="delete" data-id="${esc(w.id)}">Delete</button></td></tr>`).join("")||emptyRow(5,"No wedding pages found.");
    view.innerHTML=heading(section)+table(["Couple","Owner","Wedding date","Visibility","Actions"],body);
  }
  async function loadSecurity(section){
    const [logs,attempts]=await Promise.all([API.adminAuditLogs(),API.adminLoginAttempts()]);
    const logRows=logs.logs.map(item=>`<tr><td>${date(item.created_at)}</td><td>${esc(item.actor_email||"—")}</td><td>${esc(item.action)}</td><td>${esc(item.resource_type||"")} ${esc(item.resource_id||"")}</td></tr>`).join("")||emptyRow(4,"No administrator actions recorded.");
    const attemptRows=attempts.attempts.map(item=>`<tr><td>${esc(item.email)}</td><td>${esc(item.address)}</td><td>${item.attempts}</td><td>${date(item.last_attempt_at)}</td></tr>`).join("")||emptyRow(4,"No failed administrator sign-ins recorded.");
    view.innerHTML=heading(section)+`<article class="admin-card"><h2>Recent failed sign-ins</h2>${table(["Email","Address","Attempts","Last attempt"],attemptRows)}</article><article class="admin-card" style="margin-top:16px"><h2>Audit history</h2>${table(["Date","Administrator","Action","Resource"],logRows)}</article>`;
  }
  function closeMenu(){sidebar.classList.remove("open");backdrop.classList.remove("open");document.body.classList.remove("menu-open");}
  nav.addEventListener("click",event=>{const button=event.target.closest("[data-view]");if(button)showSection(button.dataset.view);});
  view.addEventListener("click",async event=>{
    const section=event.target.closest("[data-section]"); if(section){showSection(section.dataset.section);return;}
    const button=event.target.closest("[data-user-action],[data-wedding-action]");if(!button)return;
    try {
      if(button.dataset.userAction==="status"){await API.adminSetUserStatus(button.dataset.id,button.dataset.status);await loadUsers(sections.users);notify(`Account ${button.dataset.status}.`);}
      else if(button.dataset.userAction==="delete"){if(!confirm("Delete this account? This cannot be undone."))return;await API.adminDeleteUser(button.dataset.id);await loadUsers(sections.users);notify("Account deleted.");}
      else if(button.dataset.weddingAction==="status"){await API.adminSetWeddingStatus(button.dataset.id,button.dataset.status);await loadWeddings(sections.weddings);notify(`Wedding ${button.dataset.status}.`);}
      else if(button.dataset.weddingAction==="delete"){if(!confirm("Delete this wedding and all of its associated data? This cannot be undone."))return;await API.adminDeleteWedding(button.dataset.id);await loadWeddings(sections.weddings);notify("Wedding deleted.");}
    } catch(error){notify(error.message);}
  });
  document.getElementById("menuButton").addEventListener("click",()=>{const open=!sidebar.classList.contains("open");sidebar.classList.toggle("open",open);backdrop.classList.toggle("open",open);document.body.classList.toggle("menu-open",open);});
  document.getElementById("sidebarClose").addEventListener("click",closeMenu);backdrop.addEventListener("click",closeMenu);
  document.getElementById("adminSignOut").addEventListener("click",async()=>{try{await API.adminLogout();}catch(_){}window.location.replace("admin-login.html");});
  async function start(){
    if(!API){return;}
    try{const account=await API.adminMe();const name=account.display_name||account.email||"Administrator";adminName.textContent=name;adminEmail.textContent=account.email||"";document.getElementById("adminAvatar").textContent=initials(name);status.textContent="● Connected";status.classList.add("online");showSection("dashboard");}
    catch(error){if(error.status===401||error.status===403){window.location.replace("admin-login.html"+(error.status===403?"?denied=1":""));return;}API.showFatalError(error.message);}
  }
  start();
})();
