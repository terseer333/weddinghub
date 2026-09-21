/**
 * WeddingHub landing page enhancements.
 * Loaded synchronously in <head>, like theme.js, so the hero headline can be
 * masked before first paint and then typed out on DOMContentLoaded.
 */
(() => {
  "use strict";
  const root = document.documentElement;
  const MASK_CLASS = "hero-typing";
  const CHAR_DELAY = 46;
  const PUNCTUATION_PAUSE = 340;
  const PAUSE_AFTER = /[.,;:!?]/;
  const reduceMotion = window.matchMedia
    ? window.matchMedia("(prefers-reduced-motion: reduce)").matches
    : false;

  if (reduceMotion) return;
  root.classList.add(MASK_CLASS);

  function typeHero() {
    const title = document.getElementById("hero-title");
    if (!title) return;
    const segments = Array.from(title.querySelectorAll(".hero-type-seg"));
    if (!segments.length) return;

    const parts = segments.map(el => ({ el, text: el.textContent }));
    const headline = parts.map(part => part.text).join("");
    parts.forEach(part => { part.el.textContent = ""; });

    title.setAttribute("aria-label", headline);
    segments.forEach(el => el.setAttribute("aria-hidden", "true"));
    title.classList.add("is-typing");

    let partIndex = 0;
    let charIndex = 0;
    let delay = CHAR_DELAY;

    function step() {
      const part = parts[partIndex];
      if (!part) {
        title.classList.add("type-complete");
        root.classList.remove(MASK_CLASS);
        return;
      }
      charIndex += 1;
      part.el.textContent = part.text.slice(0, charIndex);
      if (charIndex === part.text.length) {
        partIndex += 1;
        charIndex = 0;
        delay = CHAR_DELAY;
      } else {
        delay = PAUSE_AFTER.test(part.text.charAt(charIndex - 1)) ? PUNCTUATION_PAUSE : CHAR_DELAY;
      }
      window.setTimeout(step, delay);
    }

    step();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", typeHero);
  } else {
    typeHero();
  }
})();
