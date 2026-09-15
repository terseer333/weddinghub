(() => {
    "use strict";

    const form = document.querySelector("[data-auth-form]");
    if (!form) return;

    const message = document.getElementById("form-message");
    const submitButton = form.querySelector('button[type="submit"]');
    const mode = form.dataset.authForm;
    const profileKey = "weddinghub_local_profile";
    const codeField = form.querySelector("[data-auth-code]");
    const codeInput = codeField ? codeField.querySelector("input") : null;
    const resendLink = form.querySelector("[data-auth-resend]");
    const api = () => window.WeddingHubAPI;

    const weddingDate = form.elements.weddingDate;
    if (weddingDate) weddingDate.min = new Date().toISOString().slice(0, 10);

    // Set once the password step succeeds; the form then collects the emailed code.
    let pendingLogin = null;

    const say = (text) => { message.textContent = text; };
    const fail = (error) => {
        say(error && error.message ? error.message : "Something went wrong. Please try again.");
        submitButton.disabled = false;
    };

    function storeProfile(profile) {
        localStorage.setItem(profileKey, JSON.stringify(profile));
        localStorage.setItem("weddinghub_user", JSON.stringify(profile));
    }

    // Seed the browser-local workspace after signup so the dashboard has demo
    // data to render while the API wedding is created on first bootstrap.
    function seedLocalWorkspace() {
        if (!window.WeddingHub) return;
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

    async function connected() {
        try { return Boolean(await api()?.connect()); }
        catch (_) { return false; }
    }

    function hideCredentialFields() {
        form.querySelectorAll("[data-auth-credential]").forEach((field) => {
            field.hidden = true;
            field.querySelectorAll("input, select").forEach((input) => { input.required = false; });
        });
    }

    async function beginCodeStep(email, password) {
        await api().requestLoginCode(email, password);
        pendingLogin = { email, password };
        hideCredentialFields();
        if (codeField) {
            codeField.hidden = false;
            if (codeInput) {
                codeInput.required = true;
                codeInput.focus();
            }
        }
        submitButton.textContent = "Verify code and sign in";
        say(`We emailed a 6-digit code to ${email}. It expires in 10 minutes.`);
        submitButton.disabled = false;
    }

    async function submitCode() {
        const code = codeInput ? codeInput.value.trim() : "";
        if (!/^\d{6}$/.test(code)) {
            say("Enter the 6-digit code from your email.");
            return;
        }
        submitButton.disabled = true;
        try {
            const session = await api().verifyLoginCode(pendingLogin.email, code);
            api().storeSession(session);
            storeProfile({ fullName: session.user.full_name, email: session.user.email });
            if (mode === "signup") seedLocalWorkspace();
            say("Signed in. Opening your workspace…");
            window.setTimeout(() => window.location.assign("dashboard.html"), 400);
        } catch (error) { fail(error); }
    }

    // Offline preview behavior: no API reachable, so continue with the
    // browser-local workspace exactly as before.
    function legacyLocalFlow() {
        submitButton.disabled = true;
        say(mode === "signup"
            ? "Working offline. Creating a local profile…"
            : "Working offline. Opening your local workspace…");

        if (mode === "signup") {
            storeProfile({
                fullName: form.elements.fullName.value.trim(),
                email: form.elements.email.value.trim()
            });
            seedLocalWorkspace();
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
                storeProfile(profile);
            }
        }

        window.setTimeout(() => {
            window.location.assign("dashboard.html");
        }, 450);
    }

    form.addEventListener("submit", (event) => {
        event.preventDefault();
        if (!form.reportValidity()) return;
        if (pendingLogin) { submitCode(); return; }

        submitButton.disabled = true;
        (async () => {
            if (!(await connected())) { legacyLocalFlow(); return; }
            try {
                const email = form.elements.email.value.trim();
                const password = form.elements.password.value;
                if (mode === "signup") {
                    await api().registerAccount(email, form.elements.fullName.value.trim(), password);
                    storeProfile({ fullName: form.elements.fullName.value.trim(), email });
                }
                await beginCodeStep(email, password);
            } catch (error) { fail(error); }
        })();
    });

    if (resendLink) {
        resendLink.addEventListener("click", (event) => {
            event.preventDefault();
            if (!pendingLogin) return;
            say("Sending a new code…");
            api().requestLoginCode(pendingLogin.email, pendingLogin.password)
                .then(() => say("A new code is on its way. It may take a minute to arrive."))
                .catch((error) => fail(error));
        });
    }

    // A still-valid session skips the form entirely.
    (async () => {
        if (!api() || !api().sessionToken()) return;
        try {
            const user = await api().currentUser();
            if (user && user.email) window.location.assign("dashboard.html");
        } catch (_) {}
    })();
})();
