(() => {
  const KEY = "weddinghub_demo_v2";
  const hero = "https://images.unsplash.com/photo-1519741497674-611481863552?auto=format&fit=crop&w=1800&q=85";
  const photos = [
    hero,
    "https://images.unsplash.com/photo-1511285560929-80b456fea0bc?auto=format&fit=crop&w=1400&q=85",
    "https://images.unsplash.com/photo-1522673607200-164d1b6ce486?auto=format&fit=crop&w=1400&q=85",
    "https://images.unsplash.com/photo-1520854221256-17451cc331bf?auto=format&fit=crop&w=1400&q=85"
  ];

  const seed = {
    version: 2,
    wedding: {
      id: "wed_blessing_terseer",
      slug: "blessing-and-terseer",
      brideName: "Blessing",
      groomName: "Terseer",
      date: "2026-12-19T10:00:00+01:00",
      ceremonyTime: "10:00",
      receptionTime: "14:00",
      venue: "Royal Palace Hall",
      address: "17 Kashim Ibrahim Way",
      city: "Makurdi",
      state: "Benue State",
      country: "Nigeria",
      message: "With joyful hearts and the blessing of our families, we invite you to share in the beginning of our forever.",
      verse: "I have found the one whom my soul loves. — Song of Solomon 3:4",
      dressCode: "Garden formal · Sage, champagne & warm neutrals",
      heroImage: hero,
      templateId: "black-gold-03",
      status: "published"
    },
    events: [
      { id: "evt_1", name: "Church Ceremony", date: "2026-12-19", time: "10:00", venue: "St. Theresa Cathedral", address: "Makurdi, Benue State", description: "Join us as we exchange our vows before God, family and friends.", status: "published" },
      { id: "evt_2", name: "Wedding Reception", date: "2026-12-19", time: "14:00", venue: "Royal Palace Hall", address: "17 Kashim Ibrahim Way, Makurdi", description: "An afternoon of dining, dancing and celebration.", status: "published" },
      { id: "evt_3", name: "After-party", date: "2026-12-19", time: "20:00", venue: "The Garden Lounge", address: "Makurdi", description: "One more dance under the stars.", status: "draft" }
    ],
    photos: photos.map((url, index) => ({ id: `photo_${index + 1}`, url, caption: ["Our forever begins", "A quiet kind of love", "Always better together", "To all the days ahead"][index], category: index < 2 ? "Engagement" : "Our memories", status: "published" })),
    stories: [
      { id: "story_1", year: "2019", title: "The Beginning", content: "A simple introduction after Sunday service became a conversation neither of us wanted to end.", status: "published", order: 1 },
      { id: "story_2", year: "2021", title: "The Journey", content: "Through new cities, big dreams and ordinary Tuesdays, we learned that home was wherever we were together.", status: "published", order: 2 },
      { id: "story_3", year: "2025", title: "She Said Yes", content: "Under a sky full of stars, surrounded by the people who know us best, forever became our favorite plan.", status: "published", order: 3 },
      { id: "story_4", year: "2026", title: "Forever Begins", content: "We cannot wait to celebrate this next chapter with you.", status: "published", order: 4 }
    ],
    announcements: [
      { id: "ann_1", title: "A little wedding update", message: "Please arrive at the church by 9:30 AM. Ushers will be available to help with seating.", date: "2026-09-08", status: "published" },
      { id: "ann_2", title: "Hotel recommendations", message: "Our accommodation guide will be shared with travelling guests soon.", date: "2026-09-10", status: "draft" }
    ],
    guests: [
      { id: "guest_james", name: "James Aondo", email: "james@example.com", phone: "+234 801 234 5678", category: "Family", invitationStatus: "accepted", token: "demo-james-7fb3c8a1", rsvp: "attending", partySize: 2, openedAt: "2026-09-04" },
      { id: "guest_grace", name: "Grace Ter", email: "grace@example.com", phone: "+234 802 345 6789", category: "Friends", invitationStatus: "opened", token: "demo-grace-91ca48ef", rsvp: "pending", partySize: 1, openedAt: "2026-09-09" },
      { id: "guest_michael", name: "Michael Oche", email: "michael@example.com", phone: "+234 803 456 7890", category: "Colleagues", invitationStatus: "sent", token: "demo-michael-a8452db9", rsvp: "pending", partySize: 1 },
      { id: "guest_ada", name: "Ada Eze", email: "ada@example.com", phone: "+234 804 567 8901", category: "Friends", invitationStatus: "declined", token: "demo-ada-36dc184f", rsvp: "declined", partySize: 1 }
    ],
    messages: [
      { id: "msg_1", guestId: "guest_james", name: "James Aondo", message: "Wishing you both a lifetime full of laughter, grace and beautiful adventures.", status: "approved", date: "2026-09-05" }
    ]
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
  function published(items) { return (items || []).filter(item => item.status === "published"); }
  function escape(value) { const el = document.createElement("div"); el.textContent = value ?? ""; return el.innerHTML; }
  function formatDate(value, options = { day: "numeric", month: "long", year: "numeric" }) { return new Intl.DateTimeFormat("en-GB", options).format(new Date(value)); }
  function toast(message) {
    let el = document.querySelector(".toast");
    if (!el) { el = document.createElement("div"); el.className = "toast"; document.body.appendChild(el); }
    el.textContent = message; el.classList.add("show"); setTimeout(() => el.classList.remove("show"), 2600);
  }
  function templates() {
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
  function countdown(target, callback) {
    const tick = () => {
      const diff = new Date(target) - new Date();
      if (diff <= 0) return callback({ passed: true, days: 0, hours: 0, minutes: 0, seconds: 0 });
      callback({ passed: false, days: Math.floor(diff / 86400000), hours: Math.floor(diff / 3600000) % 24, minutes: Math.floor(diff / 60000) % 60, seconds: Math.floor(diff / 1000) % 60 });
    };
    tick(); return setInterval(tick, 1000);
  }

  window.WeddingHub = { getData, saveData, resetData, query, guestByToken, published, escape, formatDate, toast, templates, countdown };
})();