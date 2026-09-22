// Inserts Google Fonts preconnect hints before the first stylesheet link of each page.
const fs = require("fs");
const files = [
  "pages/login.html",
  "pages/signup.html",
  "pages/dashboard.html",
  "pages/event.html",
  "pages/guest-dashboard.html",
  "pages/committee-dashboard.html",
];
const hint =
  '<link rel="preconnect" href="https://fonts.googleapis.com"><link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>';
for (const file of files) {
  let source = fs.readFileSync(file, "utf8");
  if (source.includes("preconnect")) {
    console.log("already has preconnect:", file);
    continue;
  }
  const at = source.indexOf('<link rel="stylesheet"');
  if (at < 0) {
    console.log("no stylesheet link:", file);
    continue;
  }
  fs.writeFileSync(file, source.slice(0, at) + hint + source.slice(at));
  console.log("updated:", file);
}
