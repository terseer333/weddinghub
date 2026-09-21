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
  const ERASE_DELAY = 24;
  const PUNCTUATION_PAUSE = 340;
  const HOLD_AFTER_TYPE = 2400;
  const HOLD_AFTER_ERASE = 700;
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

    // The headline box keeps the height of the complete copy, so erasing the
    // text each cycle never pushes the rest of the page around.
    function reserveHeight() {
      const shown = parts.map(part => part.el.textContent);
      const previous = title.style.visibility;
      title.style.visibility = "hidden";
      title.style.minHeight = "";
      parts.forEach(part => { part.el.textContent = part.text; });
      const height = title.offsetHeight;
      parts.forEach((part, i) => { part.el.textContent = shown[i]; });
      title.style.visibility = previous;
      title.style.minHeight = height + "px";
    }

    reserveHeight();
    parts.forEach(part => { part.el.textContent = ""; });

    title.setAttribute("aria-label", headline);
    segments.forEach(el => el.setAttribute("aria-hidden", "true"));
    title.classList.add("is-typing");
    root.classList.remove(MASK_CLASS);
    window.addEventListener("resize", reserveHeight);

    let partIndex = 0;
    let charIndex = 0;

    // Types the headline out, holds it, erases it, then types again, looping
    // for as long as the page stays open.
    function typeNext() {
      const part = parts[partIndex];
      if (!part) {
        window.setTimeout(erasePrev, HOLD_AFTER_TYPE);
        return;
      }
      charIndex += 1;
      part.el.textContent = part.text.slice(0, charIndex);
      if (charIndex === part.text.length) {
        partIndex += 1;
        charIndex = 0;
        window.setTimeout(typeNext, CHAR_DELAY);
        return;
      }
      const last = part.text.charAt(charIndex - 1);
      window.setTimeout(typeNext, PAUSE_AFTER.test(last) ? PUNCTUATION_PAUSE : CHAR_DELAY);
    }

    function erasePrev() {
      if (partIndex === 0 && charIndex === 0) {
        window.setTimeout(typeNext, HOLD_AFTER_ERASE);
        return;
      }
      if (charIndex === 0) {
        partIndex -= 1;
        charIndex = parts[partIndex].text.length;
      }
      charIndex -= 1;
      const part = parts[partIndex];
      part.el.textContent = part.text.slice(0, charIndex);
      window.setTimeout(erasePrev, ERASE_DELAY);
    }

    typeNext();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", typeHero);
  } else {
    typeHero();
  }
})();
