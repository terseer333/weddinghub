(() => {
  const WH=WeddingHub;
  function selected(data,id){return WH.templates().find(item=>item.id===(id||data.wedding.templateId))||WH.templates()[0]}
  function card(data,templateId,extraClass=''){
    const wedding=data.wedding,template=selected(data,templateId);
    const event=WH.published(data.events)[0];
    const location=wedding.venue||event?.venue||'Venue to be announced';
    return `<article class="invitation-card design-${template.design} variant-${template.variant} ${extraClass}" data-template-id="${template.id}">
      <span class="template-ornament top" aria-hidden="true"></span><div class="template-card-inner">
      <span class="template-label">${template.category}</span><small>Together with their families</small>
      <h3>${WH.escape(wedding.brideName)} <i>&</i> ${WH.escape(wedding.groomName)}</h3>
      <p class="card-message">${WH.escape(wedding.message||'request the pleasure of your company at their wedding celebration')}</p>
      <div class="card-date"><strong>${WH.formatDate(wedding.date)}</strong><span>${wedding.ceremonyTime||event?.time||''}</span></div>
      <p class="card-venue">${WH.escape(location)}${wedding.city?` · ${WH.escape(wedding.city)}`:''}</p>
      <div class="card-schedule">${wedding.ceremonyTime?`<span>Ceremony <b>${wedding.ceremonyTime}</b></span>`:''}${wedding.receptionTime?`<span>Reception <b>${wedding.receptionTime}</b></span>`:''}</div>
      ${wedding.dressCode?`<p class="card-dress">Dress code · ${WH.escape(wedding.dressCode)}</p>`:''}</div>
      <span class="template-ornament bottom" aria-hidden="true"></span></article>`
  }
  function accessPanel(guest,url){return `<section class="guest-access-panel"><p class="eyebrow">Guest access</p><h3>${WH.escape(guest.name)}'s personal wedding link</h3><p>This private link opens the invitation, RSVP and guest experience.</p><div class="access-link-row"><input value="${WH.escape(url)}" readonly aria-label="Personal guest access link"><button class="button secondary" data-share-copy="${WH.escape(url)}">Copy link</button></div><div class="share-actions"><button class="button primary" data-native-share>Share invitation</button><a class="button whatsapp" data-whatsapp-share target="_blank" rel="noopener">WhatsApp</a><button class="button secondary" data-download-card>Download card</button></div></section>`}
  async function imageBlob(data,templateId){
    const wedding=data.wedding,canvas=document.createElement('canvas');canvas.width=1200;canvas.height=1600;const ctx=canvas.getContext('2d');
    const theme=selected(data,templateId).design;const dark=['black-gold','burgundy-floral','green-botanical','african-luxury','editorial'].includes(theme);
    ctx.fillStyle=dark?'#17251f':'#f7f1e7';ctx.fillRect(0,0,1200,1600);ctx.strokeStyle=dark?'#caa66b':'#8b6d58';ctx.lineWidth=4;ctx.strokeRect(58,58,1084,1484);ctx.strokeRect(78,78,1044,1444);
    ctx.textAlign='center';ctx.fillStyle=dark?'#d6bd88':'#806650';ctx.font='600 26px serif';ctx.fillText('TOGETHER WITH THEIR FAMILIES',600,310);
    ctx.fillStyle=dark?'#fff9ed':'#263b32';ctx.font='76px serif';wrap(ctx,`${wedding.brideName} & ${wedding.groomName}`,600,560,980,92);
    ctx.fillStyle=dark?'#e5d9c2':'#526159';ctx.font='italic 30px serif';wrap(ctx,wedding.message||'invite you to celebrate their wedding',600,820,840,46);
    ctx.fillStyle=dark?'#d6bd88':'#806650';ctx.font='600 34px serif';ctx.fillText(WH.formatDate(wedding.date).toUpperCase(),600,1080);
    ctx.fillStyle=dark?'#f3eadc':'#354b41';ctx.font='30px sans-serif';ctx.fillText(`${wedding.venue||'Venue to be announced'}${wedding.city?` · ${wedding.city}`:''}`,600,1160);
    if(wedding.dressCode){ctx.font='24px sans-serif';ctx.fillText(`DRESS CODE · ${wedding.dressCode}`,600,1280)}
    return new Promise(resolve=>canvas.toBlob(resolve,'image/png',.94));
  }
  function wrap(ctx,text,x,y,maxWidth,lineHeight){const words=String(text).split(' '),lines=[];let line='';for(const word of words){const test=`${line}${word} `;if(ctx.measureText(test).width>maxWidth&&line){lines.push(line);line=`${word} `}else line=test}lines.push(line);lines.forEach((value,index)=>ctx.fillText(value.trim(),x,y+index*lineHeight))}
  window.WeddingInvitation={selected,card,accessPanel,imageBlob};
})();