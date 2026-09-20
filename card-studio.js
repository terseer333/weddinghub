/**
 * WeddingHub Card Customization Studio Controller
 * Powers the 3-panel design studio: Gallery | Live Preview | Customization Controls
 */
(() => {
  "use strict";

  let currentTemplateId = "luxury-sage-download";
  let activeCustomConfig = {
    fonts: { couple: "Great Vibes", heading: "Playfair Display", body: "Cormorant Garamond" },
    colors: { background: "#2d4030", text: "#f7f4ed", accent: "#d4af37", border: "#e5c158", secondary: "#a3b49e" },
    decorations: { floralStyle: "sage-botanical-corners", borderStyle: "double-gold", layout: "centered-classic", frameGlow: true, datePill: true },
    background: { style: "solid" }
  };

  const FALLBACK_TEMPLATE = {
    name: "Sage Botanical Luxe",
    fonts: { couple: "Great Vibes", heading: "Playfair Display", body: "Cormorant Garamond" },
    colors: { background: "#2d4030", text: "#f7f4ed", accent: "#d4af37", border: "#e5c158" },
    decorations: { floralStyle: "sage-botanical-corners", borderStyle: "double-gold", layout: "centered-classic" },
    background: { style: "solid" }
  };

  function normDecos(deco) {
    const out = {};
    for (const key of Object.keys(deco || {})) {
      out[key.replace(/_([a-z])/g, (_, c) => c.toUpperCase())] = deco[key];
    }
    return out;
  }

  let activeCategory = "all";
  let searchQuery = "";
  let favoritesOnly = false;
  let currentDevice = "desktop"; // "desktop" | "mobile"
  let currentZoom = 1.0;
  let activeTab = "fonts"; // "fonts" | "colors" | "decorations"

  function init() {
    if (!window.AdminApp) return;
    const wedding = AdminApp.data?.wedding || {};
    currentTemplateId = wedding.cardConfig?.template_id || wedding.templateId || "luxury-sage-download";

    // Load existing saved configuration or template defaults
    syncActiveConfigFromData();

    // Render Studio
    renderStudio();

    // Listen to wedding data updates
    window.addEventListener("weddinghub:update", () => {
      syncActiveConfigFromData();
      renderLiveCard();
    });
  }

  function syncActiveConfigFromData() {
    const wedding = (window.AdminApp && AdminApp.data?.wedding) || {};
    currentTemplateId = wedding.cardConfig?.template_id || wedding.templateId || "luxury-sage-download";
    const template = (window.WeddingTemplates && WeddingTemplates.get(currentTemplateId)) || FALLBACK_TEMPLATE;

    const savedDec = normDecos(wedding.cardConfig?.decorations);
    activeCustomConfig = {
      fonts: Object.assign({}, template?.fonts || {}, wedding.cardConfig?.fonts),
      colors: Object.assign({}, template?.colors || {}, wedding.cardConfig?.colors),
      decorations: Object.assign({}, template?.decorations || {}, savedDec),
      background: Object.assign({}, template?.background || {}, {
        style: savedDec.background || (template?.background && template.background.style) || "solid"
      })
    };
    if (!activeCustomConfig.colors.secondary) activeCustomConfig.colors.secondary = activeCustomConfig.colors.accent;
    if (!activeCustomConfig.decorations.layout) activeCustomConfig.decorations.layout = template?.layout || "centered-classic";
  }

  function renderStudio() {
    const container = document.getElementById("cardStudioContainer");
    if (!container) return;

    container.innerHTML = `
      <div class="card-studio-layout">
        <!-- LEFT: Template Selection & Gallery -->
        <aside class="studio-panel template-gallery-panel">
          <div class="studio-panel-header">
            <h2>✦ Template Gallery <span class="badge" id="galleryCountBadge">100+</span></h2>
            <button class="text-button" id="favToggleBtn" title="Show starred favorites">★ Favorites</button>
          </div>
          <div class="gallery-search-box">
            <input type="search" id="studioSearchInput" placeholder="Search templates, styles, colors..." value="${escapeAttr(searchQuery)}">
          </div>
          <div class="category-pills-row" id="studioCategoryPills"></div>
          <div class="template-cards-scroll" id="studioTemplateGrid"></div>
        </aside>

        <!-- MIDDLE: Live Interactive Preview -->
        <section class="studio-panel live-preview-panel">
          <div class="preview-toolbar">
            <div class="toolbar-group">
              <div class="device-switcher">
                <button class="device-btn ${currentDevice === 'desktop' ? 'active' : ''}" data-device="desktop">Desktop Card</button>
                <button class="device-btn ${currentDevice === 'mobile' ? 'active' : ''}" data-device="mobile">📱 Mobile View</button>
              </div>
            </div>
            <div class="toolbar-group zoom-controls">
              <span>Zoom:</span>
              <button class="zoom-btn" id="zoomOutBtn" title="Zoom out">-</button>
              <strong id="zoomLabel">${Math.round(currentZoom * 100)}%</strong>
              <button class="zoom-btn" id="zoomInBtn" title="Zoom in">+</button>
              <button class="zoom-btn" id="zoomResetBtn" title="Reset zoom">↺</button>
            </div>
            <div class="toolbar-group">
              <button class="button secondary compact" id="studioDownloadBtn">↓ Download PNG</button>
              <a class="button primary compact" id="studioOpenPreviewBtn" href="event.html?preview=admin" target="_blank">Preview Card ↗</a>
            </div>
          </div>
          <div class="preview-stage-container" id="previewStageContainer">
            <div class="card-stage-wrapper" id="cardStageWrapper" style="transform: scale(${currentZoom});">
              <div id="cardDeviceContainer">
                <div id="liveCardMount"></div>
              </div>
            </div>
          </div>
        </section>

        <!-- RIGHT: Customization Studio Controls -->
        <aside class="studio-panel card-studio-controls-panel">
          <div class="studio-panel-header">
            <h2>⚙ Customize Card</h2>
            <span class="badge" id="currentTemplateName">Template</span>
          </div>
          <nav class="customizer-tabs">
            <button class="customizer-tab-btn ${activeTab === 'fonts' ? 'active' : ''}" data-tab="fonts">Fonts</button>
            <button class="customizer-tab-btn ${activeTab === 'colors' ? 'active' : ''}" data-tab="colors">Colors</button>
            <button class="customizer-tab-btn ${activeTab === 'decorations' ? 'active' : ''}" data-tab="decorations">Artwork</button>
          </nav>
          <div class="customizer-tab-content" id="customizerTabContent"></div>
          <div class="studio-action-row">
            <button class="button secondary" id="resetDefaultsBtn">Reset</button>
            <button class="button primary" id="saveDesignBtn">Save Design</button>
          </div>
        </aside>
      </div>
    `;

    bindStudioEvents();
    renderCategoryPills();
    renderTemplateGrid();
    renderLiveCard();
    renderTabContent();
  }

  function bindStudioEvents() {
    // Search
    const search = document.getElementById("studioSearchInput");
    if (search) {
      search.oninput = (e) => {
        searchQuery = e.target.value;
        renderTemplateGrid();
      };
    }

    // Favorites filter
    const favBtn = document.getElementById("favToggleBtn");
    if (favBtn) {
      favBtn.onclick = () => {
        favoritesOnly = !favoritesOnly;
        favBtn.classList.toggle("active", favoritesOnly);
        favBtn.textContent = favoritesOnly ? "★ All Templates" : "★ Favorites";
        renderTemplateGrid();
      };
    }

    // Device switcher
    document.querySelectorAll(".device-btn").forEach(btn => {
      btn.onclick = () => {
        currentDevice = btn.dataset.device;
        document.querySelectorAll(".device-btn").forEach(b => b.classList.toggle("active", b === btn));
        renderLiveCard();
      };
    });

    // Zoom controls
    const zoomIn = document.getElementById("zoomInBtn");
    if (zoomIn) zoomIn.onclick = () => setZoom(currentZoom + 0.1);
    const zoomOut = document.getElementById("zoomOutBtn");
    if (zoomOut) zoomOut.onclick = () => setZoom(currentZoom - 0.1);
    const zoomReset = document.getElementById("zoomResetBtn");
    if (zoomReset) zoomReset.onclick = () => setZoom(1.0);

    // Download PNG
    const dlBtn = document.getElementById("studioDownloadBtn");
    if (dlBtn) {
      dlBtn.onclick = async () => {
        try {
          WeddingHub.toast("Rendering print-ready card...");
          const blob = await WeddingInvitation.imageBlob(AdminApp.data, currentTemplateId, activeCustomConfig);
          const a = document.createElement("a");
          a.href = URL.createObjectURL(blob);
          a.download = `${AdminApp.data.wedding.slug || "wedding"}-invitation-card.png`;
          a.click();
          URL.revokeObjectURL(a.href);
          WeddingHub.toast("Card downloaded successfully!");
        } catch (err) {
          console.error(err);
          WeddingHub.toast("Download failed. Please try again.");
        }
      };
    }

    // Persist the in-progress selection before opening the full preview, so the preview
    // reflects it and the return trip to Card Studio → Templates keeps it selected.
    const previewBtn = document.getElementById("studioOpenPreviewBtn");
    if (previewBtn) {
      previewBtn.onclick = () => applyDesignToData();
    }

    // Customizer tabs
    document.querySelectorAll(".customizer-tab-btn").forEach(btn => {
      btn.onclick = () => {
        activeTab = btn.dataset.tab;
        document.querySelectorAll(".customizer-tab-btn").forEach(b => b.classList.toggle("active", b === btn));
        renderTabContent();
      };
    });

    // Save design
    const saveBtn = document.getElementById("saveDesignBtn");
    if (saveBtn) {
      saveBtn.onclick = async () => {
        await saveCustomDesign();
      };
    }

    // Reset defaults
    const resetBtn = document.getElementById("resetDefaultsBtn");
    if (resetBtn) {
      resetBtn.onclick = () => {
        if (confirm("Reset font and color overrides to this template's original design?")) {
          const tmpl = (window.WeddingTemplates && WeddingTemplates.get(currentTemplateId)) || FALLBACK_TEMPLATE;
          activeCustomConfig = {
            fonts: Object.assign({}, tmpl.fonts),
            colors: Object.assign({}, tmpl.colors),
            decorations: Object.assign({}, tmpl.decorations, {
              layout: tmpl.layout || tmpl.decorations?.layout || "centered-classic"
            }),
            background: Object.assign({}, tmpl.background || {}, {
              style: tmpl.decorations?.background || (tmpl.background && tmpl.background.style) || "solid"
            })
          };
          activeCustomConfig.colors.secondary = activeCustomConfig.colors.secondary || activeCustomConfig.colors.accent;
          renderLiveCard();
          renderTabContent();
          WeddingHub.toast("Reset to template default design");
        }
      };
    }
  }

  function setZoom(val) {
    currentZoom = Math.min(Math.max(0.4, Number(val.toFixed(2))), 1.5);
    const wrapper = document.getElementById("cardStageWrapper");
    if (wrapper) wrapper.style.transform = `scale(${currentZoom})`;
    const label = document.getElementById("zoomLabel");
    if (label) label.textContent = `${Math.round(currentZoom * 100)}%`;
  }

  function renderCategoryPills() {
    const row = document.getElementById("studioCategoryPills");
    if (!row) return;
    const cats = WeddingTemplates.categories();
    row.innerHTML = cats.map(c => `
      <button class="cat-pill ${activeCategory === c.id ? 'active' : ''}" data-category="${c.id}">${c.label}</button>
    `).join("");

    row.querySelectorAll(".cat-pill").forEach(btn => {
      btn.onclick = () => {
        activeCategory = btn.dataset.category;
        row.querySelectorAll(".cat-pill").forEach(b => b.classList.toggle("active", b === btn));
        renderTemplateGrid();
      };
    });
  }

  function renderTemplateGrid() {
    const grid = document.getElementById("studioTemplateGrid");
    if (!grid) return;
    const list = WeddingTemplates.filter(WeddingTemplates.all(), {
      category: activeCategory,
      query: searchQuery,
      favoritesOnly
    });

    const badge = document.getElementById("galleryCountBadge");
    if (badge) badge.textContent = `${list.length} Designs`;

    grid.innerHTML = list.map(t => {
      const isFav = WeddingTemplates.isFavorite(t.id);
      const isSelected = t.id === currentTemplateId;
      const layoutTag = (WeddingTemplates.layouts().find(l => l.id === (t.layout || t.decorations?.layout)) || {}).name || "Classic";
      const nameOne = AdminApp.data.wedding.brideName || AdminApp.data.wedding.partner_one || "";
      const nameTwo = AdminApp.data.wedding.groomName || AdminApp.data.wedding.partner_two || "";
      let year = "";
      if (AdminApp.data.wedding.date) {
        try {
          const d = new Date(/^\d{4}-\d{2}-\d{2}$/.test(AdminApp.data.wedding.date) ? `${AdminApp.data.wedding.date}T00:00:00` : AdminApp.data.wedding.date);
          if (!isNaN(d.getTime())) year = String(d.getFullYear());
        } catch (_) {}
      }
      const sample = nameOne && nameTwo ? `${nameOne} & ${nameTwo}` : (nameOne || nameTwo || "Invitation");
      return `
        <article class="template-mini-card ${isSelected ? 'active' : ''}" data-template-id="${t.id}">
          <button class="fav-star-btn ${isFav ? 'active' : ''}" data-fav-id="${t.id}" title="${isFav ? 'Remove favorite' : 'Add to favorites'}">
            ${isFav ? '★' : '☆'}
          </button>
          <span class="mini-layout-tag">${layoutTag}</span>
          <div class="mini-card-thumb" style="background-color: ${t.colors.background}; color: ${t.colors.text}; border: 1px solid ${t.colors.border || '#ccc'};">
            <small style="color: ${t.colors.accent}; font-size: 0.55rem; letter-spacing: 0.1em;">INVITATION</small>
            <span class="mini-name" style="font-family: ${t.fonts.couple};">${escapeHtml(sample)}</span>
            ${year ? `<small style="font-size: 0.55rem; opacity: 0.8;">${year}</small>` : ''}
          </div>
          <div class="mini-card-meta">
            <strong>${escapeHtml(t.name)}</strong>
            <small>${escapeHtml(t.categoryLabel || t.category)}</small>
          </div>
        </article>
      `;
    }).join("");

    // Bind card clicks
    grid.querySelectorAll(".template-mini-card").forEach(cardEl => {
      cardEl.onclick = (e) => {
        if (e.target.closest(".fav-star-btn")) return;
        selectTemplate(cardEl.dataset.templateId);
      };
    });

    // Bind favorite stars
    grid.querySelectorAll(".fav-star-btn").forEach(btn => {
      btn.onclick = (e) => {
        e.stopPropagation();
        const id = btn.dataset.favId;
        const nowFav = WeddingTemplates.toggleFavorite(id);
        btn.classList.toggle("active", nowFav);
        btn.textContent = nowFav ? "★" : "☆";
        if (favoritesOnly && !nowFav) {
          renderTemplateGrid();
        }
      };
    });
  }

  function selectTemplate(templateId) {
    currentTemplateId = templateId;
    const template = (window.WeddingTemplates && WeddingTemplates.get(templateId)) || FALLBACK_TEMPLATE;

    activeCustomConfig.colors = Object.assign({}, template.colors);
    activeCustomConfig.colors.secondary = template.colors.secondary || template.colors.accent;
    activeCustomConfig.decorations = Object.assign({}, template.decorations, {
      layout: template.layout || template.decorations?.layout || "centered-classic",
      frameGlow: Boolean(template.decorations?.frameGlow),
      datePill: Boolean(template.decorations?.datePill)
    });
    activeCustomConfig.fonts = Object.assign({}, template.fonts, activeCustomConfig.fonts);
    activeCustomConfig.background = Object.assign({}, template.background || {}, {
      style: activeCustomConfig.decorations.background || (template.background && template.background.style) || "solid"
    });

    // Update active indicators
    document.querySelectorAll(".template-mini-card").forEach(el => {
      el.classList.toggle("active", el.dataset.templateId === templateId);
    });

    const badge = document.getElementById("currentTemplateName");
    if (badge) badge.textContent = template.name;

    renderLiveCard();
    renderTabContent();
    WeddingHub.toast(`Applied design: ${template.name}`);
  }

  function renderLiveCard() {
    const mount = document.getElementById("liveCardMount");
    const deviceContainer = document.getElementById("cardDeviceContainer");
    if (!mount || !deviceContainer) return;

    const html = WeddingInvitation.card(AdminApp.data, currentTemplateId, "", activeCustomConfig);

    if (currentDevice === "mobile") {
      deviceContainer.className = "phone-mockup-frame";
      deviceContainer.innerHTML = `
        <div class="phone-notch"></div>
        <div class="phone-screen">
          ${html}
        </div>
      `;
    } else {
      deviceContainer.className = "";
      deviceContainer.innerHTML = html;
    }
  }

  function renderTabContent() {
    const container = document.getElementById("customizerTabContent");
    if (!container) return;

    if (activeTab === "fonts") {
      const fonts = WeddingTemplates.fonts();
      container.innerHTML = `
        <div class="customizer-section">
          <h3>Couple Names Font</h3>
          <div class="font-field-row">
            <select class="font-select-input" id="fontCoupleSelect">
              ${fonts.couple.map(f => `
                <option value="${f.name}" ${activeCustomConfig.fonts.couple === f.name ? 'selected' : ''}>
                  ${f.name} (Script)
                </option>
              `).join("")}
            </select>
          </div>
        </div>

        <div class="customizer-section">
          <h3>Heading &amp; Date Font</h3>
          <div class="font-field-row">
            <select class="font-select-input" id="fontHeadingSelect">
              ${fonts.heading.map(f => `
                <option value="${f.name}" ${activeCustomConfig.fonts.heading === f.name ? 'selected' : ''}>
                  ${f.name} (Serif / Display)
                </option>
              `).join("")}
            </select>
          </div>
        </div>

        <div class="customizer-section">
          <h3>Details &amp; Body Font</h3>
          <div class="font-field-row">
            <select class="font-select-input" id="fontBodySelect">
              ${fonts.body.map(f => `
                <option value="${f.name}" ${activeCustomConfig.fonts.body === f.name ? 'selected' : ''}>
                  ${f.name} (Body / Details)
                </option>
              `).join("")}
            </select>
          </div>
        </div>
      `;

      // Event bindings
      document.getElementById("fontCoupleSelect").onchange = (e) => {
        activeCustomConfig.fonts.couple = e.target.value;
        renderLiveCard();
      };
      document.getElementById("fontHeadingSelect").onchange = (e) => {
        activeCustomConfig.fonts.heading = e.target.value;
        renderLiveCard();
      };
      document.getElementById("fontBodySelect").onchange = (e) => {
        activeCustomConfig.fonts.body = e.target.value;
        renderLiveCard();
      };
    } else if (activeTab === "colors") {
      const palettes = WeddingTemplates.palettes();
      container.innerHTML = `
        <div class="customizer-section">
          <h3>Curated Luxury Palettes</h3>
          <div class="palette-grid">
            ${palettes.map(p => `
              <div class="palette-chip" data-palette="${p.name}">
                <div class="palette-colors-row">
                  <span style="background: ${p.background}"></span>
                  <span style="background: ${p.accent}"></span>
                  <span style="background: ${p.text}"></span>
                </div>
                <small title="${p.name}">${p.name}</small>
              </div>
            `).join("")}
          </div>
        </div>

        <div class="customizer-section">
          <h3>Custom Card Colors</h3>
          <div class="color-picker-grid">
            <div class="color-picker-item">
              <label>Background Color</label>
              <div class="color-input-wrapper">
                <input type="color" id="colorBgPick" value="${activeCustomConfig.colors.background || '#2d4030'}">
                <input type="text" id="colorBgHex" value="${activeCustomConfig.colors.background || '#2d4030'}">
              </div>
            </div>
            <div class="color-picker-item">
              <label>Text / Ink Color</label>
              <div class="color-input-wrapper">
                <input type="color" id="colorTextPick" value="${activeCustomConfig.colors.text || '#f7f4ed'}">
                <input type="text" id="colorTextHex" value="${activeCustomConfig.colors.text || '#f7f4ed'}">
              </div>
            </div>
            <div class="color-picker-item">
              <label>Accent / Gold Color</label>
              <div class="color-input-wrapper">
                <input type="color" id="colorAccentPick" value="${activeCustomConfig.colors.accent || '#d4af37'}">
                <input type="text" id="colorAccentHex" value="${activeCustomConfig.colors.accent || '#d4af37'}">
              </div>
            </div>
            <div class="color-picker-item">
              <label>Border Line Color</label>
              <div class="color-input-wrapper">
                <input type="color" id="colorBorderPick" value="${activeCustomConfig.colors.border || '#e5c158'}">
                <input type="text" id="colorBorderHex" value="${activeCustomConfig.colors.border || '#e5c158'}">
              </div>
            </div>
            <div class="color-picker-item">
              <label>Secondary Tone</label>
              <div class="color-input-wrapper">
                <input type="color" id="colorSecondaryPick" value="${activeCustomConfig.colors.secondary || activeCustomConfig.colors.accent || '#a3b49e'}">
                <input type="text" id="colorSecondaryHex" value="${activeCustomConfig.colors.secondary || activeCustomConfig.colors.accent || '#a3b49e'}">
              </div>
            </div>
          </div>
        </div>
      `;

      // Palette clicks
      container.querySelectorAll(".palette-chip").forEach(chip => {
        chip.onclick = () => {
          const pal = palettes.find(p => p.name === chip.dataset.palette);
          if (pal) {
            activeCustomConfig.colors = {
              background: pal.background,
              text: pal.text,
              accent: pal.accent,
              border: pal.border
            };
            renderLiveCard();
            renderTabContent();
            WeddingHub.toast(`Applied palette: ${pal.name}`);
          }
        };
      });

      // Color picker sync
      syncColorInputs("colorBgPick", "colorBgHex", (val) => { activeCustomConfig.colors.background = val; renderLiveCard(); });
      syncColorInputs("colorTextPick", "colorTextHex", (val) => { activeCustomConfig.colors.text = val; renderLiveCard(); });
      syncColorInputs("colorAccentPick", "colorAccentHex", (val) => { activeCustomConfig.colors.accent = val; renderLiveCard(); });
      syncColorInputs("colorBorderPick", "colorBorderHex", (val) => { activeCustomConfig.colors.border = val; renderLiveCard(); });
      syncColorInputs("colorSecondaryPick", "colorSecondaryHex", (val) => { activeCustomConfig.colors.secondary = val; renderLiveCard(); });
    } else if (activeTab === "decorations") {
      const decs = WeddingTemplates.decorations();
      const layouts = WeddingTemplates.layouts();
      const backgrounds = WeddingTemplates.backgrounds();
      container.innerHTML = `
        <div class="customizer-section">
          <h3>Card Layout</h3>
          <div class="font-field-row">
            <select class="font-select-input" id="layoutSelect">
              ${layouts.map(l => `
                <option value="${l.id}" ${activeCustomConfig.decorations.layout === l.id ? 'selected' : ''}>
                  ${l.name}
                </option>
              `).join("")}
            </select>
          </div>
        </div>

        <div class="customizer-section">
          <h3>Background Treatment</h3>
          <div class="font-field-row">
            <select class="font-select-input" id="backgroundSelect">
              ${backgrounds.map(b => `
                <option value="${b.id}" ${activeCustomConfig.background.style === b.id ? 'selected' : ''}>
                  ${b.name}
                </option>
              `).join("")}
            </select>
          </div>
        </div>

        <div class="customizer-section">
          <h3>Botanical &amp; Floral Artwork</h3>
          <div class="font-field-row">
            <select class="font-select-input" id="floralStyleSelect">
              ${decs.floral.map(f => `
                <option value="${f.id}" ${activeCustomConfig.decorations.floralStyle === f.id ? 'selected' : ''}>
                  ${f.name}
                </option>
              `).join("")}
            </select>
          </div>
        </div>

        <div class="customizer-section">
          <h3>Card Border Frame</h3>
          <div class="font-field-row">
            <select class="font-select-input" id="borderStyleSelect">
              ${decs.borders.map(b => `
                <option value="${b.id}" ${activeCustomConfig.decorations.borderStyle === b.id ? 'selected' : ''}>
                  ${b.name}
                </option>
              `).join("")}
            </select>
          </div>
        </div>

        <div class="customizer-section">
          <h3>Accent Effects</h3>
          <label class="effect-toggle">
            <input type="checkbox" id="frameGlowToggle" ${activeCustomConfig.decorations.frameGlow ? 'checked' : ''}>
            <span>Ambient gold glow behind text</span>
          </label>
          <label class="effect-toggle">
            <input type="checkbox" id="datePillToggle" ${activeCustomConfig.decorations.datePill ? 'checked' : ''}>
            <span>Rounded date pill</span>
          </label>
        </div>
      `;

      document.getElementById("layoutSelect").onchange = (e) => {
        activeCustomConfig.decorations.layout = e.target.value;
        renderLiveCard();
      };
      document.getElementById("backgroundSelect").onchange = (e) => {
        activeCustomConfig.background.style = e.target.value;
        activeCustomConfig.decorations.background = e.target.value;
        renderLiveCard();
      };
      document.getElementById("floralStyleSelect").onchange = (e) => {
        activeCustomConfig.decorations.floralStyle = e.target.value;
        renderLiveCard();
      };
      document.getElementById("borderStyleSelect").onchange = (e) => {
        activeCustomConfig.decorations.borderStyle = e.target.value;
        renderLiveCard();
      };
      document.getElementById("frameGlowToggle").onchange = (e) => {
        activeCustomConfig.decorations.frameGlow = e.target.checked;
        renderLiveCard();
      };
      document.getElementById("datePillToggle").onchange = (e) => {
        activeCustomConfig.decorations.datePill = e.target.checked;
        renderLiveCard();
      };
    }
  }

  function syncColorInputs(pickerId, hexId, onColorChange) {
    const pick = document.getElementById(pickerId);
    const hex = document.getElementById(hexId);
    if (!pick || !hex) return;
    pick.oninput = (e) => {
      hex.value = e.target.value;
      onColorChange(e.target.value);
    };
    hex.oninput = (e) => {
      if (/^#[0-9a-fA-F]{6}$/.test(e.target.value)) {
        pick.value = e.target.value;
        onColorChange(e.target.value);
      }
    };
  }

  function buildCardConfig() {
    return {
      template_id: currentTemplateId,
      fonts: {
        couple: activeCustomConfig.fonts.couple,
        heading: activeCustomConfig.fonts.heading,
        body: activeCustomConfig.fonts.body
      },
      colors: {
        background: activeCustomConfig.colors.background,
        text: activeCustomConfig.colors.text,
        accent: activeCustomConfig.colors.accent,
        border: activeCustomConfig.colors.border,
        secondary: activeCustomConfig.colors.secondary || activeCustomConfig.colors.accent
      },
      decorations: {
        floral_style: activeCustomConfig.decorations.floralStyle,
        border_style: activeCustomConfig.decorations.borderStyle,
        layout: activeCustomConfig.decorations.layout || "centered-classic",
        frame_glow: Boolean(activeCustomConfig.decorations.frameGlow),
        date_pill: Boolean(activeCustomConfig.decorations.datePill),
        background: activeCustomConfig.background.style || "solid"
      }
    };
  }

  // Writes the studio's current template and customizations into the shared wedding
  // record and persists locally, so the selection survives the Preview Card round trip.
  function applyDesignToData() {
    const wedding = AdminApp.data.wedding;
    wedding.templateId = currentTemplateId;
    wedding.cardConfig = buildCardConfig();
    WeddingHub.saveData(AdminApp.data);
  }

  async function saveCustomDesign() {
    applyDesignToData();
    const wedding = AdminApp.data.wedding;

    await AdminApp.persist("Invitation card design saved");

    // Also push the design through the dedicated card config endpoint
    try {
      if (window.WeddingHubAPI) {
        const res = await WeddingHubAPI.saveCardConfig(wedding.id, wedding.cardConfig);
        if (res) WeddingHub.toast("Card design published to all invitation links!");
      }
    } catch (err) {
      console.warn("Card config endpoint sync failed (local save still applied):", err);
    }
  }

  function escapeHtml(val) {
    const div = document.createElement("div");
    div.textContent = val ?? "";
    return div.innerHTML;
  }

  function escapeAttr(val) {
    return String(val ?? "").replace(/"/g, "&quot;");
  }

  window.CardStudio = {
    init,
    renderStudio,
    selectTemplate,
    saveCustomDesign
  };
})();
