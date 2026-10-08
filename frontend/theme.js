/**
 * WeddingHub theme controller.
 * Loaded synchronously in <head> so the saved/system theme is applied to
 * <html data-theme> before first paint (no flash), then mounts a floating
 * on/off toggle on every page.
 */
(() => {
  "use strict";
  const KEY = "weddinghub_theme";
  const root = document.documentElement;
  const mq = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;

  function stored() {
    try {
      const value = localStorage.getItem(KEY);
      return value === "dark" || value === "light" ? value : null;
    } catch (_) {
      return null;
    }
  }
  function system() { return mq && mq.matches ? "dark" : "light"; }
  function current() { return stored() || system(); }

  function apply(theme) {
    const dark = theme === "dark";
    root.setAttribute("data-theme", dark ? "dark" : "light");
    root.style.colorScheme = dark ? "dark" : "light";
    const button = document.getElementById("whThemeToggle");
    if (button) {
      button.setAttribute("aria-pressed", String(dark));
      button.setAttribute("aria-label", dark ? "Switch to light mode" : "Switch to dark mode");
      button.title = dark ? "Switch to light mode" : "Switch to dark mode";
      button.textContent = dark ? "☀" : "☾";
    }
  }

  function set(theme, persist) {
    if (persist) {
      try { localStorage.setItem(KEY, theme); } catch (_) {}
    }
    apply(theme);
  }
  function toggle() { set(current() === "dark" ? "light" : "dark", true); }

  function mount() {
    if (document.getElementById("whThemeToggle")) return;
    const button = document.createElement("button");
    button.id = "whThemeToggle";
    button.type = "button";
    button.className = "wh-theme-toggle";
    button.addEventListener("click", toggle);
    document.body.appendChild(button);
    apply(current());
  }

  apply(current());
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount);
  } else {
    mount();
  }

  if (mq) {
    const onChange = () => { if (!stored()) apply(current()); };
    if (mq.addEventListener) mq.addEventListener("change", onChange);
    else if (mq.addListener) mq.addListener(onChange);
  }

  window.WeddingHubTheme = {
    get: current,
    set: theme => set(theme, true),
    toggle,
    reset: () => { try { localStorage.removeItem(KEY); } catch (_) {} apply(current()); }
  };
})();
