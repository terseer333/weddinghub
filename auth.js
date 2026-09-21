(() => {
    "use strict";

    const form = document.querySelector("[data-auth-form]");
    if (!form) return;

    const API = window.WeddingHubAPI;
    const message = document.getElementById("form-message");
    const submitButton = form.querySelector('button[type="submit"]');
    const mode = form.dataset.authForm; // "login" or "signup"
    const profileKey = "weddinghub_local_profile";
    const weddingDate = form.elements.weddingDate;
    if (weddingDate) weddingDate.min = new Date().toISOString().slice(0, 10);

    // Credentials are verified by the API, so an unreachable backend is reported as an
    // error rather than silently letting the visitor through.
    function describeError(error) {
        const text = String((error && error.message) || "").trim();
        if (!text || error instanceof TypeError || /failed to fetch|network ?error|load failed/i.test(text)) {
            return "Cannot reach WeddingHub to verify your sign-in. Start the server and try again.";
        }
        if (/no api url|api url is not configured/i.test(text)) {
            return "This page is not running on the WeddingHub server, so sign-in cannot be verified. Start the backend and open http://localhost:8080 instead.";
        }
        return text;
    }

    function storeProfile(profile) {
        localStorage.setItem(profileKey, JSON.stringify(profile));
        localStorage.setItem("weddinghub_user", JSON.stringify(profile));
    }

    function openWorkspace() {
        window.setTimeout(() => window.location.assign("dashboard.html"), 450);
    }

    // The dashboard offers its guided tour to anyone who has never completed it, so a
    // first sign-in on this browser queues it up.
    function markTourPending() {
        try {
            if (localStorage.getItem("weddinghub_tour_done") !== "1") {
                localStorage.setItem("weddinghub_tour_pending", "1");
            }
        } catch (_) {}
    }

    // Signing up starts a fresh wedding for this account.
    function initializeWedding(response) {
        storeProfile({
            fullName: response.user.display_name || form.elements.fullName.value.trim() || "Wedding Admin",
            email: response.user.email
        });
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

    form.addEventListener("submit", async (event) => {
        event.preventDefault();
        if (!form.reportValidity()) return;

        if (!API) {
            message.textContent = "WeddingHub could not load its API client. Reload the page and try again.";
            return;
        }

        submitButton.disabled = true;
        message.textContent = mode === "signup" ? "Creating your account\u2026" : "Checking your credentials\u2026";

        const email = form.elements.email.value.trim();
        const password = form.elements.password.value;

        try {
            if (mode === "signup") {
                const response = await API.signup({
                    email,
                    password,
                    display_name: form.elements.fullName.value.trim()
                });
                initializeWedding(response);
                message.textContent = "Account created. Opening your workspace\u2026";
            } else {
                const response = await API.login({ email, password });
                storeProfile({
                    fullName: response.user.display_name || email,
                    email: response.user.email
                });
                message.textContent = "Welcome back. Opening your workspace\u2026";
            }
            markTourPending();
            openWorkspace();
        } catch (error) {
            message.textContent = describeError(error);
            submitButton.disabled = false;
        }
    });
})();
