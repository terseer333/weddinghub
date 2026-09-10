const modal=document.getElementById('modal'),modalContent=document.getElementById('modalContent');
function showModal(markup){modalContent.innerHTML=markup;modal.classList.add('open')}
function closeModal(){modal.classList.remove('open')}
function field(value){return WH.escape(value||'')}
function statusOptions(current){return ['draft','published','hidden'].map(value=>`<option value="${value}" ${value===current?'selected':''}>${value[0].toUpperCase()+value.slice(1)}</option>`).join('')}
document.getElementById('modalClose').onclick=closeModal;modal.onclick=event=>{if(event.target===modal)closeModal()};
document.getElementById('weddingForm').onsubmit=async event=>{event.preventDefault();new FormData(event.target).forEach((value,key)=>AdminApp.data.wedding[key]=value);await AdminApp.persist('Wedding information published')};
document.getElementById('templateGrid').onclick=async event=>{const button=event.target.closest('[data-template]');if(!button)return;AdminApp.data.wedding.templateId=button.dataset.template;await AdminApp.persist('Invitation design selected')};
document.getElementById('apiURL').value=API.baseURL();
document.getElementById('apiSettings').onsubmit=event=>{event.preventDefault();localStorage.setItem('weddinghub_api_url',document.getElementById('apiURL').value.replace(/\/$/,''));localStorage.removeItem('weddinghub_api_wedding_id');location.reload()};
document.getElementById('resetData').onclick=()=>{if(confirm('Clear this browser workspace and restore the product preview?')){WH.resetData();localStorage.removeItem('weddinghub_api_wedding_id');localStorage.removeItem('weddinghub_user');localStorage.removeItem('weddinghub_local_profile');location.href='signup.html'}};
window.AdminModal={show:showModal,close:closeModal,field,statusOptions};