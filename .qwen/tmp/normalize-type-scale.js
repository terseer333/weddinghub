// Normalises px font sizes onto the WeddingHub type scale.
//
// Scale (defined in weddinghub.css :root):
//   --text-2xs: 11px  micro labels (badges, captions)
//   --text-xs:  12px  meta / uppercase labels
//   --text-sm:  13px  secondary UI copy
//   --text-base:14px  small body copy / button text
//   --text-md:  15px  body base
//   15px+            left alone (headings and display type)
//
// Elements under `mock`/`window-bar` are scaled-down illustrations of the
// product (landing page dashboard mockup), so their tiny sizes are deliberate.
const fs = require("fs");

const FILES = [
  "weddinghub.css",
  "landing.css",
  "guest.css",
  "polish.css",
  "tour.css",
  "card-studio.css",
];

const SKIP_SELECTOR = /mock|window-bar/i;

function mapSize(value) {
  const px = parseFloat(value);
  if (px <= 7) return "var(--text-2xs)";
  if (px <= 10) return "var(--text-xs)";
  if (px <= 12) return "var(--text-sm)";
  if (px <= 13) return "var(--text-base)";
  if (px <= 14) return "var(--text-md)";
  return null; // 15px and up stay as they are
}

// rem sizes below 0.7rem (11.2px) are just as unreadable as the px ones.
function mapRem(value) {
  return parseFloat(value) < 0.7 ? "var(--text-xs)" : null;
}

// Literal fixes that the numeric mapping cannot express.
const EXPLICIT = [
  // The invitation greeting is a script accent, which needs a much larger size.
  [".invite-personal{font-size:16px}", ".invite-personal{font-family:var(--script);font-size:30px;line-height:1.2}"],
  // The guest table needs more room now that its text is legible.
  [".guest-table{min-width:720px}", ".guest-table{min-width:880px}"],
];

let total = 0;
for (const file of FILES) {
  let css = fs.readFileSync(file, "utf8");
  const before = css;
  let changed = 0;

  for (const [from, to] of EXPLICIT) {
    if (css.includes(from)) {
      css = css.split(from).join(to);
      changed++;
    }
  }

  // Walk rule blocks so decorative mockups can be skipped by selector.
  css = css.replace(/([^{}]+)\{([^{}]*)\}/g, (match, selector, body) => {
    if (SKIP_SELECTOR.test(selector)) return match;
    const next = body.replace(/font-size:\s*([\d.]+)px/g, (decl, value) => {
      const mapped = mapSize(value);
      if (!mapped) return decl;
      changed++;
      return `font-size:${mapped}`;
    });
    return `${selector}{${next}}`;
  });

  if (css !== before) {
    fs.writeFileSync(file, css);
    total += changed;
    console.log(`${file}: ${changed} declarations updated`);
  } else {
    console.log(`${file}: no change`);
  }
}
console.log(`total: ${total}`);
