/**
 * WeddingHub Professional Invitation Card Renderer & Canvas Generator
 * Visual reference style: assets/templates/download.webp (Sage Botanical Luxe)
 * Fully data-driven — no hardcoded content fallbacks.
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
      layout: "centered-classic",
      background: { style: "solid" },
      fonts: { couple: "Great Vibes", heading: "Playfair Display", body: "Cormorant Garamond" },
      colors: { background: "#2d4030", text: "#f7f4ed", accent: "#d4af37", border: "#e5c158", secondary: "#b89b3e" },
      decorations: { layout: "centered-classic", floralStyle: "sage-botanical-corners", borderStyle: "double-gold", datePill: true, frameGlow: false }
    };
  }

  function resolveCardConfig(data, templateId, overrideConfig) {
    const template = resolveTemplate(data, templateId);
    const savedConfig = data?.wedding?.cardConfig || {};
    const config = Object.assign({}, template, savedConfig, overrideConfig || {});

    // Deep merge fonts and colors
    config.fonts = Object.assign({}, template.fonts, savedConfig.fonts, overrideConfig?.fonts);
    config.colors = Object.assign({}, template.colors, savedConfig.colors, overrideConfig?.colors);

    // Normalize snake_case decorations from saved server config into camelCase
    const normDec = {};
    for (const key of Object.keys(savedConfig.decorations || {})) {
      normDec[key.replace(/_([a-z])/g, (_, c) => c.toUpperCase())] = savedConfig.decorations[key];
    }
    config.decorations = Object.assign({}, template.decorations, normDec, overrideConfig?.decorations);

    config.background = Object.assign({}, template.background || {}, savedConfig.background, overrideConfig?.background);
    if (config.decorations.background) config.background.style = config.decorations.background;
    config.layout = config.decorations?.layout || config.layout || "centered-classic";
    if (!config.colors.secondary) config.colors.secondary = config.colors.accent;
    return { template, config };
  }

  function parseWeddingDate(value) {
    if (!value) return null;
    const iso = /^\d{4}-\d{2}-\d{2}$/.test(String(value)) ? `${value}T00:00:00` : value;
    const d = new Date(iso);
    if (isNaN(d.getTime())) return null;
    return {
      dayName: d.toLocaleDateString("en-US", { weekday: "long" }).toUpperCase(),
      dayNum: String(d.getDate()),
      monthYear: `${d.toLocaleDateString("en-US", { month: "long" }).toUpperCase()} ${d.getFullYear()}`
    };
  }

  /**
   * Generates HTML markup for a luxury physical-look wedding invitation card
   */
  function card(data, templateId, extraClass = "", overrideConfig = null) {
    const wedding = data?.wedding || {};
    const { template, config } = resolveCardConfig(data, templateId, overrideConfig);

    let nameOne = wedding.brideName || wedding.partner_one || "";
    let nameTwo = wedding.groomName || wedding.partner_two || "";
    if (!nameOne && nameTwo) { nameOne = nameTwo; nameTwo = ""; }

    const venue = wedding.venue || "";
    const address = wedding.address || "";
    const city = wedding.city || "";
    const state = wedding.state || "";
    const fullLocation = [address, city, state].filter(Boolean).join(", ");
    const message = wedding.message || "";

    const dateData = parseWeddingDate(wedding.date);
    const timeString = wedding.ceremonyTime ? `AT ${wedding.ceremonyTime}` : "";

    const fontCouple = resolveFont(config.fonts?.couple, "cursive");
    const fontHeading = resolveFont(config.fonts?.heading, "serif");
    const fontBody = resolveFont(config.fonts?.body, "serif");

    const bg = config.colors?.background || "#2d4030";
    const text = config.colors?.text || "#f7f4ed";
    const accent = config.colors?.accent || "#d4af37";
    const border = config.colors?.border || "#e5c158";
    const secondary = config.colors?.secondary || accent;

    const layout = config.layout;
    const backgroundStyle = (config.background && config.background.style) || "solid";
    const floralStyle = config.decorations?.floralStyle || "sage-botanical-corners";
    const borderStyle = config.decorations?.borderStyle || "double-gold";
    const frameGlow = Boolean(config.decorations?.frameGlow || config.frameGlow);
    const datePill = Boolean(config.decorations?.datePill || config.datePill);

    const styleVars = `
      --card-bg: ${bg};
      --card-text: ${text};
      --card-accent: ${accent};
      --card-border: ${border};
      --card-secondary: ${secondary};
      --font-couple: ${fontCouple};
      --font-heading: ${fontHeading};
      --font-body: ${fontBody};
    `;

    const namesBlock = (nameOne || nameTwo) ? `
          <div class="card-couple-names">
            ${nameOne ? `<span class="name-one">${WH.escape(nameOne)}</span>` : ''}
            ${nameOne && nameTwo ? '<span class="name-amp">&amp;</span>' : ''}
            ${nameTwo ? `<span class="name-two">${WH.escape(nameTwo)}</span>` : ''}
          </div>` : "";

    const dateBlock = dateData ? `
          <div class="card-date-banner">
            <span class="date-divider"></span>
            <span class="date-day-name">${dateData.dayName}</span>
            <span class="date-day-num">${dateData.dayNum}</span>
            ${timeString ? `<span class="date-time">${timeString}</span>` : ''}
            <span class="date-divider"></span>
          </div>
          <p class="date-month-year">${dateData.monthYear}</p>` : "";

    const venueBlock = venue ? `
          <p class="card-venue-line">${WH.escape(venue)}</p>
          ${fullLocation ? `<p class="card-city-line">${WH.escape(fullLocation)}</p>` : ''}` : "";

    const messageBlock = message ? `
          <p class="card-invitation-message">${WH.escape(message)}</p>` : "";

    return `
      <article class="luxury-invitation-card ${extraClass}"
               style="${styleVars}"
               data-template-id="${template.id}"
               data-layout="${layout}"
               data-background="${backgroundStyle}"
               data-floral="${floralStyle}"
               data-border="${borderStyle}"
               data-glow="${frameGlow}"
               data-pill="${datePill}">
        ${frameGlow ? '<div class="card-glow-layer" aria-hidden="true"></div>' : ''}
        <div class="card-background-texture" aria-hidden="true"></div>
        <div class="card-frame-overlay" aria-hidden="true"></div>
        <div class="card-floral-layer" aria-hidden="true"></div>
        <div class="card-inner-content">
          <p class="card-kicker">YOU ARE INVITED TO THE WEDDING OF</p>
          ${namesBlock}
          ${dateBlock}
          ${venueBlock}
          ${messageBlock}
          <div class="card-rsvp-block">
            <p class="rsvp-title">${wedding.dressCode ? `Dress Code · ${WH.escape(wedding.dressCode)}` : "RSVP to share our joyful celebration"}</p>
            <p class="rsvp-detail"><span class="rsvp-contact">Kindly respond</span> via your personal wedding link</p>
          </div>
        </div>
      </article>
    `;
  }

  // ---- Canvas helpers ----------------------------------------------------
  function hexToRgb(hex) {
    const h = String(hex).replace("#", "");
    if (h.length !== 6) return { r: 0, g: 0, b: 0 };
    return {
      r: parseInt(h.slice(0, 2), 16),
      g: parseInt(h.slice(2, 4), 16),
      b: parseInt(h.slice(4, 6), 16)
    };
  }
  function rgb(r, g, b, a = 1) { return `rgba(${r}, ${g}, ${b}, ${a})`; }
  function mix(hexA, hexB, t) {
    const a = hexToRgb(hexA), b = hexToRgb(hexB);
    return rgb(
      Math.round(a.r + (b.r - a.r) * t),
      Math.round(a.g + (b.g - a.g) * t),
      Math.round(a.b + (b.b - a.b) * t)
    );
  }
  function shade(hex, amt) {
    const c = hexToRgb(hex);
    const f = (v) => Math.max(0, Math.min(255, Math.round(v * (1 + amt))));
    return rgb(f(c.r), f(c.g), f(c.b));
  }
  function roundRect(ctx, x, y, w, h, r) {
    ctx.beginPath();
    ctx.moveTo(x + r, y);
    ctx.arcTo(x + w, y, x + w, y + h, r);
    ctx.arcTo(x + w, y + h, x, y + h, r);
    ctx.arcTo(x, y + h, x, y, r);
    ctx.arcTo(x, y, x + w, y, r);
    ctx.closePath();
  }

  /**
   * Generates a high-resolution, print-ready PNG blob using Canvas
   */
  async function imageBlob(data, templateId, overrideConfig = null) {
    const wedding = data?.wedding || {};
    const { template, config } = resolveCardConfig(data, templateId, overrideConfig);

    const W = 1200, H = 1600;
    const canvas = document.createElement("canvas");
    canvas.width = W;
    canvas.height = H;
    const ctx = canvas.getContext("2d");

    const bg = config.colors?.background || "#2d4030";
    const text = config.colors?.text || "#f7f4ed";
    const accent = config.colors?.accent || "#d4af37";
    const border = config.colors?.border || "#e5c158";
    const secondary = config.colors?.secondary || accent;
    const layout = config.layout;
    const backgroundStyle = (config.background && config.background.style) || "solid";
    const frameGlow = Boolean(config.decorations?.frameGlow || config.frameGlow);
    const datePill = Boolean(config.decorations?.datePill || config.datePill);

    // 1. Draw rich background
    ctx.fillStyle = bg;
    ctx.fillRect(0, 0, W, H);

    if (backgroundStyle === "gradient" || backgroundStyle === "duotone") {
      const grad = ctx.createLinearGradient(0, 0, W, H);
      grad.addColorStop(0, bg);
      grad.addColorStop(1, mix(bg, secondary, 0.45));
      ctx.fillStyle = grad;
      ctx.fillRect(0, 0, W, H);
    }

    // Subtle paper grain / linen texture
    const vignette = ctx.createRadialGradient(W / 2, H / 2, 50, W / 2, H / 2, 950);
    vignette.addColorStop(0, "rgba(255, 255, 255, 0.05)");
    vignette.addColorStop(1, "rgba(0, 0, 0, 0.2)");
    ctx.fillStyle = vignette;
    ctx.fillRect(0, 0, W, H);

    if (backgroundStyle === "paper") {
      ctx.fillStyle = "rgba(255, 255, 255, 0.035)";
      for (let y = 0; y < H; y += 4) {
        ctx.fillRect(0, y, W, 1);
      }
    }

    // 2. Ambient glow behind content
    if (frameGlow) {
      const ga = hexToRgb(accent);
      const glow = ctx.createRadialGradient(W / 2, H * 0.44, 40, W / 2, H * 0.44, 520);
      glow.addColorStop(0, rgb(ga.r, ga.g, ga.b, 0.30));
      glow.addColorStop(0.55, rgb(ga.r, ga.g, ga.b, 0.08));
      glow.addColorStop(1, rgb(ga.r, ga.g, ga.b, 0));
      ctx.fillStyle = glow;
      ctx.fillRect(0, 0, W, H);
    }

    // 3. Draw metallic gold double border
    const borderStyle = config.decorations?.borderStyle || "double-gold";
    if (borderStyle !== "none") {
      ctx.strokeStyle = border;
      ctx.lineWidth = 4;
      ctx.strokeRect(60, 60, W - 120, H - 120);
      if (borderStyle === "double-gold") {
        ctx.lineWidth = 2;
        ctx.strokeRect(80, 80, W - 160, H - 160);
      }
    }

    // 4. Try to render floral overlay if SVG can be loaded
    const floral = config.decorations?.floralStyle || "sage-botanical-corners";
    if (floral && floral !== "minimal") {
      try {
        const img = new Image();
        img.crossOrigin = "anonymous";
        const svgSrc = `assets/templates/${floral}.svg`;
        await new Promise((resolve) => {
          img.onload = () => { ctx.drawImage(img, 0, 0, W, H); resolve(); };
          img.onerror = () => resolve(); // continue even if image load fails
          img.src = svgSrc;
        });
      } catch (_) {}
    }

    // 5. Draw Typography (centered, print-safe composition)
    ctx.textAlign = "center";

    const bride = wedding.brideName || wedding.partner_one || "";
    const groom = wedding.groomName || wedding.partner_two || "";
    let nameOne = bride, nameTwo = groom;
    if (!nameOne && nameTwo) { nameOne = nameTwo; nameTwo = ""; }
    const dateData = parseWeddingDate(wedding.date);
    const ceremonyTime = wedding.ceremonyTime || "";

    // Kicker
    ctx.fillStyle = accent;
    ctx.font = `600 24px ${resolveFont(config.fonts?.heading, "serif")}`;
    ctx.fillText("YOU ARE INVITED TO THE WEDDING OF", W / 2, 260);

    // Couple Names
    if (nameOne) {
      ctx.fillStyle = text;
      ctx.font = `88px ${resolveFont(config.fonts?.couple, "cursive")}`;
      ctx.fillText(nameOne, W / 2, 430);
    }
    if (nameOne && nameTwo) {
      ctx.fillStyle = accent;
      ctx.font = `italic 42px ${resolveFont(config.fonts?.heading, "serif")}`;
      ctx.fillText("&", W / 2, 500);
    }
    if (nameTwo) {
      ctx.fillStyle = text;
      ctx.font = `88px ${resolveFont(config.fonts?.couple, "cursive")}`;
      ctx.fillText(nameTwo, W / 2, 590);
    }

    if (dateData) {
      // Divider rules
      ctx.strokeStyle = accent;
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.moveTo(320, 780);
      ctx.lineTo(430, 780);
      ctx.moveTo(770, 780);
      ctx.lineTo(880, 780);
      ctx.stroke();

      // Day number with optional pill accent
      if (datePill) {
        ctx.fillStyle = mix(bg, secondary, 0.28);
        roundRect(ctx, W / 2 - 52, 720, 104, 64, 30);
        ctx.fill();
        ctx.strokeStyle = secondary;
        ctx.lineWidth = 2;
        roundRect(ctx, W / 2 - 52, 720, 104, 64, 30);
        ctx.stroke();
      }

      ctx.fillStyle = text;
      ctx.font = `600 24px ${resolveFont(config.fonts?.heading, "serif")}`;
      if (dateData.dayName) ctx.fillText(dateData.dayName, 370, 755);
      if (ceremonyTime) ctx.fillText(`AT ${ceremonyTime}`, 830, 755);

      ctx.fillStyle = accent;
      ctx.font = `72px ${resolveFont(config.fonts?.heading, "serif")}`;
      ctx.fillText(dateData.dayNum, W / 2, 780);

      ctx.font = `600 26px ${resolveFont(config.fonts?.heading, "serif")}`;
      ctx.fillText(dateData.monthYear, W / 2, 850);
    }

    // Venue & Location
    if (wedding.venue) {
      ctx.fillStyle = text;
      ctx.font = `34px ${resolveFont(config.fonts?.heading, "serif")}`;
      ctx.fillText(wedding.venue, W / 2, 960);

      const cityState = [wedding.address, wedding.city, wedding.state].filter(Boolean).join(", ");
      if (cityState) {
        ctx.font = `26px ${resolveFont(config.fonts?.body, "serif")}`;
        ctx.fillText(cityState, W / 2, 1010);
      }
    }

    // Invitation Message
    if (wedding.message) {
      ctx.fillStyle = text;
      ctx.font = `italic 28px ${resolveFont(config.fonts?.body, "serif")}`;
      wrapText(ctx, wedding.message, W / 2, 1140, 840, 42);
    }

    // RSVP Line
    ctx.strokeStyle = accent;
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(350, 1340);
    ctx.lineTo(850, 1340);
    ctx.stroke();

    ctx.fillStyle = text;
    ctx.font = `italic 26px ${resolveFont(config.fonts?.body, "serif")}`;
    ctx.fillText(wedding.dressCode ? `Dress Code · ${wedding.dressCode}` : "Kindly RSVP via your personal wedding link", W / 2, 1400);

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