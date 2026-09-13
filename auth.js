(() => {
    "use strict";

    const form = document.querySelector("[data-auth-form]");
    if (!form) return;

    const message = document.getElementById("form-message");
    const submitButton = form.querySelector('button[type="submit"]');
    const mode = form.dataset.authForm;
    const profileKey = "weddinghub_local_profile";
        const weddingDate = form.elements.weddingDate;
        if (weddingDate) weddingDate.min = new Date().toISOString().slice(0, 10);

    form.addEventListener("submit", (event) => {
        event.preventDefault();

        if (!form.reportValidity()) return;

        submitButton.disabled = true;
        message.textContent = mode === "signup"
            ? "Local profile created. Opening your workspace…"
            : "Opening your local workspace…";

        if (mode === "signup") {
            const profile = {
                fullName: form.elements.fullName.value.trim(),
                email: form.elements.email.value.trim()
            };
            localStorage.setItem(profileKey, JSON.stringify(profile));
            localStorage.setItem("weddinghub_user", JSON.stringify(profile));
            if (window.WeddingHub) {
                const data = window.WeddingHub.resetData();
                const partnerOne = form.elements.partnerOne.value.trim();
                const partnerTwo = form.elements.partnerTwo.value.trim();
                data.wedding.brideName = partnerOne;
                data.wedding.groomName = partnerTwo;
                data.wedding.date = `${form.elements.weddingDate.value}T10:00:00`;
                data.wedding.slug = `${partnerOne}-and-${partnerTwo}`.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
                data.wedding.message = `${partnerOne} & ${partnerTwo} invite you to share in the joy of their wedding celebration.`;
                data.wedding.venue = "";
                data.wedding.address = "";
                data.wedding.city = "";
                data.wedding.state = "";
                data.wedding.verse = "";
                data.wedding.dressCode = "";
                data.wedding.heroImage = "";
                data.events = [];
                data.photos = [];
                data.stories = [];
                data.announcements = [];
                data.guests = [];
                data.messages = [];
                window.WeddingHub.saveData(data);
                localStorage.removeItem("weddinghub_api_wedding_id");
            }
        } else {
            let profile = null;
            try {
                profile = JSON.parse(localStorage.getItem(profileKey) || localStorage.getItem("weddinghub_user") || "null");
            } catch (_) {}
            if (!profile) {
                const emailVal = form.elements.email.value.trim();
                const defaultName = emailVal.split("@")[0].replace(/[._-]/g, " ") || "Wedding Admin";
                profile = {
                    fullName: defaultName.charAt(0).toUpperCase() + defaultName.slice(1),
                    email: emailVal
                };
                localStorage.setItem(profileKey, JSON.stringify(profile));
                localStorage.setItem("weddinghub_user", JSON.stringify(profile));
            }
        }

        window.setTimeout(() => {
            window.location.assign("dashboard.html");
        }, 450);
    });
})();
