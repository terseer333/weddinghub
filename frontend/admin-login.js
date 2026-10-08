(() => {
    "use strict";

    // The administrator sign-in page. It posts to /api/admin/login, which refuses any account
    // other than the one configured as the administrator, and stores the session the same way
    // the wedding workspace does.
    const form = document.querySelector("[data-admin-login]");
    if (!form) return;

    const API = window.WeddingHubAPI;
    const message = document.getElementById("admin-form-message");
    const submitButton = form.querySelector('button[type="submit"]');

    if (new URLSearchParams(location.search).get("denied")) {
        message.textContent = "That account is not the platform administrator.";
    }

    // An unreachable API is reported as an error rather than letting anyone through.
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

    form.addEventListener("submit", async (event) => {
        event.preventDefault();
        if (!form.reportValidity()) return;
        if (!API || typeof API.adminLogin !== "function") {
            message.textContent = "WeddingHub could not load its API client. Reload the page and try again.";
            return;
        }

        submitButton.disabled = true;
        message.textContent = "Checking administrator credentials\u2026";
        try {
            await API.adminLogin({
                email: form.elements.email.value.trim(),
                password: form.elements.password.value
            });
            message.textContent = "Welcome. Opening the admin dashboard\u2026";
            window.setTimeout(() => window.location.assign("admin.html"), 350);
        } catch (error) {
            message.textContent = error && error.status === 429
                ? "Too many failed attempts from here. Wait a few minutes, then try again."
                : describeError(error);
            submitButton.disabled = false;
        }
    });
})();
