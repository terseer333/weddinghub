/**
 * WeddingHub Professional Invitation Card Renderer & Canvas Generator
 * Visual reference style: assets/templates/download.webp (Sage Botanical Luxe)
 */
(() => {
  "use strict";
  const WH = window.WeddingHub;

  // Font family mapping
  const FONT_MAP = {
    "Great Vibes": "'Great Vibes', cursive",
    "Allura": "'Allura', cursive",
    "Alex Brush": "'Alex Brush', cursive",
    "Dancing Script": "'Dancing Script', cursive",
    "Parisienne": "'Parisienne', cursive",
    "Cinzel Decorative": "'Cinzel Decorative', serif",
    "Pinyon Script": "'Pinyon Script', cursive",
    "Playfair Display": "'Playfair Display', serif",
    "Cormorant Garamond": "'Cormorant Garamond', serif",
    "Libre Baskerville": "'Libre Baskerville', serif",
    "Marcellus": "'Marcellus', serif",
    "Bodoni Moda": "'Bodoni Moda', serif",
    "Prata": "'Prata', serif",
    "Montserrat": "'Montserrat', sans-serif",
    "Lato": "'Lato', sans-serif"
  };

  function resolveFont(name, fallback = "serif") {
    if (!name) return fallback;
    return FONT_MAP[name] || `'${name}', ${fallback}`;
  }

  function resolveTemplate(data, id) {
    const tid = id || data?.wedding?.cardConfig?.template_id || data?.wedding?.templateId || "luxury-sage-download";
    if (window.WeddingTemplates) {
      return WeddingTemplates.get(tid);
    }
    return {
      id: "luxury-sage-download",
      name: "Sage Botanical Luxe",
      category: "luxury-floral",
      fonts: { couple: "Great Vibes", heading: "Playfair Display", body: "Cormorant Garamond" },
      colors: { background: "#2d4030", text: "#f7f4ed", accent: "#d4af37", border: "#e5c158" },
      decorations: { floralStyle: "sage-botanical-corners", borderStyle: "double-gold", datePill: true }
    };
  }

  function resolveCardConfig(data, templateId, overrideConfig) {
    const template = resolveTemplate(data, templateId);
    const savedConfig = data?.wedding?.cardConfig || {};
    const config = Object.assign({}, template, savedConfig, overrideConfig || {});

    // Deep merge fonts and colors
    config.fonts = Object.assign({}, template.fonts, savedConfig.fonts, overrideConfig?.fonts);
    config.colors = Object.assign({}, template.colors, savedConfig.colors, overrideConfig?.colors);
    config.decorations = Object.assign({}, template.decorations, savedConfig.decorations, overrideConfig?.decorations);
    return { template, config };
  }

  /**
   * Generates HTML markup for a luxury physical-look wedding invitation card
   */
  function card(data, templateId, extraClass = "", overrideConfig = null) {
    const wedding = data?.wedding || {};
    const { template, config } = resolveCardConfig(data, templateId, overrideConfig);

    const bride = wedding.brideName || wedding.partner_one || "Bride";
    const groom = wedding.groomName || wedding.partner_two || "Groom";
    const venue = wedding.venue || "Royal Palace Hall";
    const address = wedding.address || "";
    const city = wedding.city || "";
    const state = wedding.state || "";
    const fullLocation = [venue, address, city, state].filter(Boolean).join(" · ");
    const message = wedding.message || "request the pleasure of your company at the celebration of their marriage";

    // Date formatting matching download.webp
    let dayName = "SATURDAY";
    let dayNum = "29";
    let monthYear = "MARCH 2026";
    let timeString = wedding.ceremonyTime ? `AT ${wedding.ceremonyTime}` : "AT 10:00 AM";

    if (wedding.date) {
      try {
        const d = new Date(wedding.date);
        if (!isNaN(d.getTime())) {
          dayName = d.toLocaleDateString("en-US", { weekday: "long" }).toUpperCase();
          dayNum = String(d.getDate());
          monthYear = `${d.toLocaleDateString("en-US", { month: "long" }).toUpperCase()} ${d.getFullYear()}`;
          if (!wedding.ceremonyTime) {
            timeString = `AT ${d.toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit" }).toUpperCase()}`;
          }
        }
      } catch (_) {}
    }

    const fontCouple = resolveFont(config.fonts?.couple, "cursive");
    const fontHeading = resolveFont(config.fonts?.heading, "serif");
    const fontBody = resolveFont(config.fonts?.body, "serif");

    const bg = config.colors?.background || "#2d4030";
    const text = config.colors?.text || "#f7f4ed";
    const accent = config.colors?.accent || "#d4af37";
    const border = config.colors?.border || "#e5c158";

    const floralStyle = config.decorations?.floralStyle || "sage-botanical-corners";
    const borderStyle = config.decorations?.borderStyle || "double-gold";

    const styleVars = `
      --card-bg: ${bg};
      --card-text: ${text};
      --card-accent: ${accent};
      --card-border: ${border};
      --font-couple: ${fontCouple};
      --font-heading: ${fontHeading};
      --font-body: ${fontBody};
    `;

    return `
      <article class="luxury-invitation-card ${extraClass}"
               style="${styleVars}"
               data-template-id="${template.id}"
               data-floral="${floralStyle}"
               data-border="${borderStyle}">
        <div class="card-frame-overlay" aria-hidden="true"></div>
        <div class="card-floral-layer" aria-hidden="true"></div>
        <div class="card-inner-content">
          <p class="card-kicker">YOU ARE INVITED TO THE WEDDING OF</p>
          <div class="card-couple-names">
            <span class="name-one">${WH.escape(bride)}</span>
            <span class="name-amp">&amp;</span>
            <span class="name-two">${WH.escape(groom)}</span>
          </div>
          <div class="card-date-banner">
            <span class="date-divider"></span>
            <span class="date-day-name">${dayName}</span>
            <span class="date-day-num">${dayNum}</span>
            <span class="date-time">${timeString}</span>
            <span class="date-divider"></span>
          </div>
          <p class="date-month-year">${monthYear}</p>
          <p class="card-venue-line">${WH.escape(venue)}</p>
          ${address || city ? `<p class="card-city-line">${WH.escape([address, city].filter(Boolean).join(", "))}</p>` : ""}
          <p class="card-invitation-message">${WH.escape(message)}</p>
          <div class="card-rsvp-block">
            <p class="rsvp-title">${wedding.dressCode ? `Dress Code · ${WH.escape(wedding.dressCode)}` : "RSVP to share our joyful celebration"}</p>
            <p class="rsvp-detail"><span class="rsvp-contact">Kindly respond</span> via your personal wedding link</p>
          </div>
        </div>
      </article>
    `;
  }

  /**
   * Generates a high-resolution, print-ready PNG blob using Canvas
   */
  async function imageBlob(data, templateId, overrideConfig = null) {
    const wedding = data?.wedding || {};
    const { template, config } = resolveCardConfig(data, templateId, overrideConfig);

    const canvas = document.createElement("canvas");
    canvas.width = 1200;
    canvas.height = 1600;
    const ctx = canvas.getContext("2d");

    const bg = config.colors?.background || "#2d4030";
    const text = config.colors?.text || "#f7f4ed";
    const accent = config.colors?.accent || "#d4af37";
    const border = config.colors?.border || "#e5c158";

    // 1. Draw rich background
    ctx.fillStyle = bg;
    ctx.fillRect(0, 0, 1200, 1600);

    // Subtle paper grain
    const grad = ctx.createRadialGradient(600, 800, 50, 600, 800, 900);
    grad.addColorStop(0, "rgba(255, 255, 255, 0.05)");
    grad.addColorStop(1, "rgba(0, 0, 0, 0.2)");
    ctx.fillStyle = grad;
    ctx.fillRect(0, 0, 1200, 1600);

    // 2. Draw metallic gold double border
    const borderStyle = config.decorations?.borderStyle || "double-gold";
    if (borderStyle !== "none") {
      ctx.strokeStyle = border;
      ctx.lineWidth = 4;
      ctx.strokeRect(60, 60, 1080, 1480);
      if (borderStyle === "double-gold") {
        ctx.lineWidth = 2;
        ctx.strokeRect(80, 80, 1040, 1440);
      }
    }

    // 3. Try to render floral overlay if SVG can be loaded
    const floral = config.decorations?.floralStyle || "sage-botanical-corners";
    if (floral && floral !== "minimal") {
      try {
        const img = new Image();
        img.crossOrigin = "anonymous";
        const svgSrc = `assets/templates/${floral}.svg`;
        await new Promise((resolve, reject) => {
          img.onload = () => { ctx.drawImage(img, 0, 0, 1200, 1600); resolve(); };
          img.onerror = () => resolve(); // continue even if image load fails
          img.src = svgSrc;
        });
      } catch (_) {}
    }

    // 4. Draw Typography
    ctx.textAlign = "center";

    // Kicker
    ctx.fillStyle = accent;
    ctx.font = `600 24px ${resolveFont(config.fonts?.heading, "serif")}`;
    ctx.letterSpacing = "4px";
    ctx.fillText("YOU ARE INVITED TO THE WEDDING OF", 600, 260);

    // Couple Names
    ctx.fillStyle = text;
    ctx.font = `88px ${resolveFont(config.fonts?.couple, "cursive")}`;
    const bride = wedding.brideName || wedding.partner_one || "Bride";
    const groom = wedding.groomName || wedding.partner_two || "Groom";
    ctx.fillText(`${bride}`, 600, 430);

    ctx.fillStyle = accent;
    ctx.font = `italic 42px ${resolveFont(config.fonts?.heading, "serif")}`;
    ctx.fillText("&", 600, 500);

    ctx.fillStyle = text;
    ctx.font = `88px ${resolveFont(config.fonts?.couple, "cursive")}`;
    ctx.fillText(`${groom}`, 600, 590);

    // Date Banner
    let dayName = "SATURDAY";
    let dayNum = "29";
    let timeStr = wedding.ceremonyTime ? `AT ${wedding.ceremonyTime}` : "AT 10:00 AM";
    let monthYear = "MARCH 2026";
    if (wedding.date) {
      try {
        const d = new Date(wedding.date);
        if (!isNaN(d.getTime())) {
          dayName = d.toLocaleDateString("en-US", { weekday: "long" }).toUpperCase();
          dayNum = String(d.getDate());
          monthYear = `${d.toLocaleDateString("en-US", { month: "long" }).toUpperCase()} ${d.getFullYear()}`;
          if (!wedding.ceremonyTime) {
            timeStr = `AT ${d.toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit" }).toUpperCase()}`;
          }
        }
      } catch (_) {}
    }

    // Divider rules
    ctx.strokeStyle = accent;
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(320, 780);
    ctx.lineTo(430, 780);
    ctx.moveTo(770, 780);
    ctx.lineTo(880, 780);
    ctx.stroke();

    ctx.fillStyle = text;
    ctx.font = `600 24px ${resolveFont(config.fonts?.heading, "serif")}`;
    ctx.fillText(dayName, 370, 755);
    ctx.fillText(timeStr, 830, 755);

    ctx.fillStyle = accent;
    ctx.font = `72px ${resolveFont(config.fonts?.heading, "serif")}`;
    ctx.fillText(dayNum, 600, 780);

    ctx.font = `600 26px ${resolveFont(config.fonts?.heading, "serif")}`;
    ctx.fillText(monthYear, 600, 850);

    // Venue & Location
    ctx.fillStyle = text;
    ctx.font = `34px ${resolveFont(config.fonts?.heading, "serif")}`;
    ctx.fillText(wedding.venue || "Royal Palace Hall", 600, 960);

    ctx.font = `26px ${resolveFont(config.fonts?.body, "serif")}`;
    const cityState = [wedding.address, wedding.city, wedding.state].filter(Boolean).join(", ");
    if (cityState) {
      ctx.fillText(cityState, 600, 1010);
    }

    // Invitation Message
    ctx.fillStyle = text;
    ctx.font = `italic 28px ${resolveFont(config.fonts?.body, "serif")}`;
    wrapText(ctx, wedding.message || "request the pleasure of your company at their wedding celebration", 600, 1140, 840, 42);

    // RSVP Line
    ctx.strokeStyle = accent;
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(350, 1340);
    ctx.lineTo(850, 1340);
    ctx.stroke();

    ctx.fillStyle = text;
    ctx.font = `italic 26px ${resolveFont(config.fonts?.body, "serif")}`;
    ctx.fillText(wedding.dressCode ? `Dress Code · ${wedding.dressCode}` : "Kindly RSVP via your personal wedding link", 600, 1400);

    return new Promise(resolve => canvas.toBlob(resolve, "image/png", 0.95));
  }

  function wrapText(ctx, text, x, y, maxWidth, lineHeight) {
    const words = String(text).split(" ");
    const lines = [];
    let line = "";
    for (const word of words) {
      const test = `${line}${word} `;
      if (ctx.measureText(test).width > maxWidth && line) {
        lines.push(line);
        line = `${word} `;
      } else {
        line = test;
      }
    }
    lines.push(line);
    lines.forEach((val, idx) => ctx.fillText(val.trim(), x, y + idx * lineHeight));
  }

  function accessPanel(guest, url) {
    return `
      <section class="guest-access-panel">
        <p class="eyebrow">Guest access</p>
        <h3>${WH.escape(guest.name)}'s personal invitation link</h3>
        <p>This private link opens the invitation card, wedding details, and RSVP experience.</p>
        <div class="access-link-row">
          <input value="${WH.escape(url)}" readonly aria-label="Personal guest access link">
          <button class="button secondary" data-share-copy="${WH.escape(url)}">Copy link</button>
        </div>
        <div class="share-actions">
          <button class="button primary" data-native-share>Share invitation</button>
          <a class="button whatsapp" data-whatsapp-share target="_blank" rel="noopener">WhatsApp</a>
          <button class="button secondary" data-download-card>Download card (PNG)</button>
        </div>
      </section>
    `;
  }

  window.WeddingInvitation = {
    selected: resolveTemplate,
    resolveCardConfig,
    card,
    accessPanel,
    imageBlob
  };
})();