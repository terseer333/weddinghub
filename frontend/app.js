(() => {
  const KEY = "weddinghub_demo_v2";
  // A blank starting wedding. Real details arrive from signup and are then loaded from the
  // API, so a fresh deployment never invents a placeholder couple, guests, photos or story.
  // version is bumped so any browser still holding the old demo seed resets to this one.
  const seed = {
    version: 6,
    wedding: {
      id: "",
      slug: "",
      brideName: "",
      groomName: "",
      date: "",
      ceremonyTime: "",
      receptionTime: "",
      venue: "",
      address: "",
      city: "",
      state: "",
      country: "",
      message: "",
      verse: "",
      dressCode: "",
      heroImage: "",
      templateId: "",
      cardConfig: null,
      status: "published"
    },
    events: [],
    photos: [],
    stories: [],
    announcements: [],
    guests: [],
    committeeRoles: [],
    committeeMembers: [],
    planningTasks: [],
    committeeChat: [],
    messages: []
  };

  function getData() {
    try {
      const current = JSON.parse(localStorage.getItem(KEY));
      if (current?.version === seed.version) return current;
    } catch (_) {}
    localStorage.setItem(KEY, JSON.stringify(seed));
    return JSON.parse(JSON.stringify(seed));
  }
  function saveData(data) {
    localStorage.setItem(KEY, JSON.stringify(data));
    window.dispatchEvent(new CustomEvent("weddinghub:update", { detail: data }));
  }
  function resetData() { localStorage.removeItem(KEY); return getData(); }
  function query(name) { return new URLSearchParams(location.search).get(name); }
  function guestByToken(data = getData()) { const token = query("token"); return data.guests.find(g => g.token === token); }
  function committeeMemberByToken(data = getData()) { const token = query("token"); return data.committeeMembers?.find(m => m.token === token); }
  function published(items) { return (items || []).filter(item => item.status === "published"); }
  // publicAnnouncements returns announcements published to guests; committee-only
  // announcements are filtered out so a guest page never renders planning updates.
  function publicAnnouncements(items) { return published(items).filter(item => item.audience !== "committee"); }
  function committeeAnnouncements(items) { return published(items).filter(item => item.audience === "committee"); }
  function escape(value) { const el = document.createElement("div"); el.textContent = value ?? ""; return el.innerHTML; }
  // A blank wedding has no date yet, and Intl throws on an invalid one, so an empty or
  // unparseable value renders as nothing instead of crashing the page.
  function formatDate(value, options = { day: "numeric", month: "long", year: "numeric" }) {
    const date = new Date(value);
    if (!value || Number.isNaN(date.getTime())) return "";
    return new Intl.DateTimeFormat("en-GB", options).format(date);
  }
  function toast(message) {
    let el = document.querySelector(".toast");
    if (!el) { el = document.createElement("div"); el.className = "toast"; document.body.appendChild(el); }
    el.textContent = message; el.classList.add("show"); setTimeout(() => el.classList.remove("show"), 2600);
  }
  function templates() {
    if (window.WeddingTemplates && window.WeddingTemplates.all().length > 0) {
      return window.WeddingTemplates.all();
    }
    const designs = [
      ["romantic-floral","Romantic Floral","Floral"],["rose-elegance","Rose Elegance","Romantic"],
      ["garden-wedding","Secret Garden","Garden wedding"],["botanical","Botanical Arch","Botanical"],
      ["luxury-gold","Gilded Vows","Luxury"],["black-gold","Black & Gold Gala","Black & Gold"],
      ["soft-pastel","Pastel Daydream","Pastel"],["pink-romantic","Blush Romance","Pink romantic"],
      ["burgundy-floral","Burgundy Bloom","Dark Elegant"],["tropical","Tropical Promise","Tropical flowers"],
      ["white-elegant","White Elegance","White & Gold"],["green-botanical","Verdant Vows","Green botanical"],
      ["blue-elegant","Blue Porcelain","Blue elegant"],["rustic","Rustic Wildflower","Rustic wedding"],
      ["traditional","Timeless Tradition","Traditional"],["african-luxury","African Royal","African-inspired"],
      ["modern-minimal","Modern Minimal","Minimalist"],["editorial","The Wedding Edit","Premium editorial"]
    ];
    const editions = ["Signature","Atelier","Grande","Luxe","Classic","Contemporary"];
    return designs.flatMap(([design,name,category],designIndex) => editions.map((edition,variant) => ({
      id: `${design}-${String(variant + 1).padStart(2,"0")}`, design, variant: variant + 1,
      name: `${name} · ${edition}`, category, premium: variant > 2 || [4,5,15,17].includes(designIndex), tone: designIndex % 12
    })));
  }
  // A wedding without a date yet has no countdown, so the callback is never called rather
  // than rendering NaN.
  function countdown(target, callback) {
    const targetTime = new Date(target).getTime();
    if (!target || Number.isNaN(targetTime)) return null;
    const tick = () => {
      const diff = targetTime - new Date();
      if (diff <= 0) return callback({ passed: true, days: 0, hours: 0, minutes: 0, seconds: 0 });
      callback({ passed: false, days: Math.floor(diff / 86400000), hours: Math.floor(diff / 3600000) % 24, minutes: Math.floor(diff / 60000) % 60, seconds: Math.floor(diff / 1000) % 60 });
    };
    tick(); return setInterval(tick, 1000);
  }

  function initials(value) {
    const parts = String(value || "").trim().split(/\s+/).filter(Boolean);
    return parts.map(part => part[0]).join("").slice(0, 2).toUpperCase() || "?";
  }
  // profileAvatar returns a data URL only when it is a real inline image, so a stray
  // value can never be injected into an <img src>.
  function profileAvatar(value) { return /^data:image\//.test(value || "") ? value : ""; }
  function profileById(data, id) { return (data?.profiles || []).find(item => item.id === id) || null; }
  // resizeImage downsamples a chosen photo in the browser so the stored avatar stays
  // small enough for the JSON API, which caps request bodies at 1 MB.
  function resizeImage(file, max = 320, quality = 0.82) {
    return new Promise((resolve, reject) => {
      if (!file || !/^image\//.test(file.type || "")) { reject(new Error("Choose an image file.")); return; }
      const reader = new FileReader();
      reader.onerror = () => reject(new Error("That image could not be read."));
      reader.onload = () => {
        const image = new Image();
        image.onerror = () => reject(new Error("That image could not be read."));
        image.onload = () => {
          const scale = Math.min(1, max / Math.max(image.width, image.height));
          const width = Math.max(1, Math.round(image.width * scale));
          const height = Math.max(1, Math.round(image.height * scale));
          const canvas = document.createElement("canvas");
          canvas.width = width; canvas.height = height;
          canvas.getContext("2d").drawImage(image, 0, 0, width, height);
          resolve(canvas.toDataURL("image/jpeg", quality));
        };
        image.src = reader.result;
      };
      reader.readAsDataURL(file);
    });
  }

  window.WeddingHub = { getData, saveData, resetData, query, guestByToken, committeeMemberByToken, published, publicAnnouncements, committeeAnnouncements, escape, formatDate, toast, templates, countdown, initials, profileAvatar, profileById, resizeImage };
})();