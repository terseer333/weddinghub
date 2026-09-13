/**
 * WeddingHub Template Engine & Catalog Service
 * Provides access to 100+ wedding invitation templates, fonts, colors, and decorations.
 */
(() => {
  "use strict";

  const FAVORITES_KEY = "weddinghub_template_favorites";

  const CATEGORIES = [
    { id: "all", label: "All Templates" },
    { id: "luxury-floral", label: "Luxury Floral" },
    { id: "modern-minimalist", label: "Modern Minimalist" },
    { id: "romantic", label: "Romantic" },
    { id: "royal-wedding", label: "Royal Wedding" },
    { id: "traditional-wedding", label: "Traditional Wedding" },
    { id: "rustic-wedding", label: "Rustic Wedding" },
    { id: "tropical-wedding", label: "Tropical Wedding" },
    { id: "elegant-simple", label: "Elegant Simple" }
  ];

  const FONTS_CATALOG = {
    couple: [
      { id: "great-vibes", name: "Great Vibes", family: "'Great Vibes', cursive", sample: "Francis & Wilhemina" },
      { id: "allura", name: "Allura", family: "'Allura', cursive", sample: "Francis & Wilhemina" },
      { id: "alex-brush", name: "Alex Brush", family: "'Alex Brush', cursive", sample: "Francis & Wilhemina" },
      { id: "dancing-script", name: "Dancing Script", family: "'Dancing Script', cursive", sample: "Francis & Wilhemina" },
      { id: "parisienne", name: "Parisienne", family: "'Parisienne', cursive", sample: "Francis & Wilhemina" },
      { id: "cinzel-dec", name: "Cinzel Decorative", family: "'Cinzel Decorative', serif", sample: "FRANCIS & WILHEMINA" },
      { id: "pinyon-script", name: "Pinyon Script", family: "'Pinyon Script', cursive", sample: "Francis & Wilhemina" }
    ],
    heading: [
      { id: "playfair-display", name: "Playfair Display", family: "'Playfair Display', serif" },
      { id: "cormorant-garamond", name: "Cormorant Garamond", family: "'Cormorant Garamond', serif" },
      { id: "libre-baskerville", name: "Libre Baskerville", family: "'Libre Baskerville', serif" },
      { id: "marcellus", name: "Marcellus", family: "'Marcellus', serif" },
      { id: "bodoni-moda", name: "Bodoni Moda", family: "'Bodoni Moda', serif" },
      { id: "prata", name: "Prata", family: "'Prata', serif" },
      { id: "montserrat", name: "Montserrat", family: "'Montserrat', sans-serif" }
    ],
    body: [
      { id: "cormorant-garamond", name: "Cormorant Garamond", family: "'Cormorant Garamond', serif" },
      { id: "playfair-display", name: "Playfair Display", family: "'Playfair Display', serif" },
      { id: "libre-baskerville", name: "Libre Baskerville", family: "'Libre Baskerville', serif" },
      { id: "montserrat", name: "Montserrat", family: "'Montserrat', sans-serif" },
      { id: "lato", name: "Lato", family: "'Lato', sans-serif" }
    ]
  };

  const COLOR_PALETTES = [
    { name: "Sage & Gold (Reference)", background: "#2d4030", text: "#f7f4ed", accent: "#d4af37", border: "#e5c158" },
    { name: "Royal Black & Gold", background: "#11110f", text: "#f5eedb", accent: "#d4af37", border: "#e5c158" },
    { name: "Navy & Gilded", background: "#0e1b2e", text: "#fdf9ed", accent: "#e6be58", border: "#ffd700" },
    { name: "Burgundy Velvet", background: "#3d101c", text: "#fff2eb", accent: "#d4af37", border: "#e8c35a" },
    { name: "Rose Blush", background: "#fbf2f0", text: "#52333b", accent: "#b86f7f", border: "#dcaeb7" },
    { name: "Modern Pure White", background: "#ffffff", text: "#1a1a1a", accent: "#666666", border: "#d4af37" },
    { name: "Kraft & Flora", background: "#e8d7be", text: "#4a3c2a", accent: "#826646", border: "#9c7c57" },
    { name: "Emerald Forest", background: "#143023", text: "#fdfbf5", accent: "#d4af37", border: "#e5c158" },
    { name: "Champagne Ivory", background: "#f8f1df", text: "#46371f", accent: "#b28a3e", border: "#c6a15b" }
  ];

  const DECORATION_STYLES = {
    floral: [
      { id: "sage-botanical-corners", name: "Sage Botanical Corners (Luxury 3D)", asset: "assets/templates/sage-botanical-corners.svg" },
      { id: "botanical-arch", name: "Botanical Floral Arch", asset: "assets/templates/botanical-arch.svg" },
      { id: "rose-corners", name: "Blush Rose Corners", asset: "assets/templates/rose-corners.svg" },
      { id: "burgundy-flowers", name: "Burgundy Velvet Blooms", asset: "assets/templates/burgundy-flowers.svg" },
      { id: "wildflower-garden", name: "Wildflower Meadow", asset: "assets/templates/wildflower-garden.svg" },
      { id: "tropical-leaves", name: "Tropical Palm Fronds", asset: "assets/templates/tropical-leaves.svg" },
      { id: "traditional-flourish", name: "Traditional Heraldic Flourish", asset: "assets/templates/traditional-flourish.svg" },
      { id: "luxury-frame", name: "Gilded Baroque Frame", asset: "assets/templates/luxury-frame.svg" },
      { id: "minimal", name: "Minimal / No Floral", asset: "" }
    ],
    borders: [
      { id: "double-gold", name: "Double Gold Metallic Frame" },
      { id: "single-gold", name: "Single Gold Foil Border" },
      { id: "thin-line", name: "Fine Hairline Rule" },
      { id: "royal-crest", name: "Royal Crest Border" },
      { id: "none", name: "No Border" }
    ]
  };

  let templatesCache = null;

  async function loadTemplates() {
    if (templatesCache && templatesCache.length > 0) return templatesCache;
    const sources = [
      "../templates/templates.json",
      "templates/templates.json",
      "/templates/templates.json"
    ];
    for (const url of sources) {
      try {
        const resp = await fetch(url);
        if (resp.ok) {
          const list = await resp.json();
          if (Array.isArray(list) && list.length > 0) {
            templatesCache = list;
            return list;
          }
        }
      } catch (_) {}
    }
    // Fallback if fetch is blocked: minimal seed array
    templatesCache = [
      {
        id: "luxury-sage-download",
        name: "Sage Botanical Luxe",
        category: "luxury-floral",
        categoryLabel: "Luxury Floral",
        description: "Deep sage green with 3D gold botanical flourishes and double gold frame",
        fonts: { couple: "Great Vibes", heading: "Playfair Display", body: "Cormorant Garamond" },
        colors: { background: "#2d4030", text: "#f7f4ed", accent: "#d4af37", border: "#e5c158", secondary: "#a3b49e" },
        decorations: { floralStyle: "sage-botanical-corners", borderStyle: "double-gold", layout: "centered-classic", frameGlow: true, datePill: true },
        layout: "centered-classic",
        premium: true
      }
    ];
    return templatesCache;
  }

  function allTemplatesSync() {
    return templatesCache || [];
  }

  function getTemplateSync(id) {
    const list = allTemplatesSync();
    return list.find(t => t.id === id) || list[0] || null;
  }

  function getFavorites() {
    try {
      return JSON.parse(localStorage.getItem(FAVORITES_KEY) || "[]");
    } catch (_) {
      return [];
    }
  }

  function isFavorite(id) {
    return getFavorites().includes(id);
  }

  function toggleFavorite(id) {
    let favs = getFavorites();
    if (favs.includes(id)) {
      favs = favs.filter(item => item !== id);
    } else {
      favs.push(id);
    }
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(favs));
    return favs.includes(id);
  }

  function filterTemplates(list, { category = "all", query = "", favoritesOnly = false } = {}) {
    const favs = favoritesOnly ? getFavorites() : [];
    const q = (query || "").trim().toLowerCase();
    return list.filter(t => {
      if (category && category !== "all" && t.category !== category) return false;
      if (favoritesOnly && !favs.includes(t.id)) return false;
      if (q) {
        const hay = `${t.name} ${t.category} ${t.categoryLabel || ""} ${t.description || ""}`.toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
  }

  window.WeddingTemplates = {
    loadTemplates,
    all: allTemplatesSync,
    get: getTemplateSync,
    categories: () => CATEGORIES,
    fonts: () => FONTS_CATALOG,
    palettes: () => COLOR_PALETTES,
    decorations: () => DECORATION_STYLES,
    getFavorites,
    isFavorite,
    toggleFavorite,
    filter: filterTemplates
  };

  // Eager load templates on startup
  loadTemplates();
})();
