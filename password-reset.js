(() => {
  const form = document.getElementById('reset-form');
  const message = document.getElementById('form-message');
  const button = form.querySelector('button[type="submit"]');
  const token = new URLSearchParams(location.search).get('token') || '';
  if (token) history.replaceState(null, '', location.pathname);
  if (!token) {
    message.textContent = 'This password reset link is invalid. Request a new link to continue.';
    button.disabled = true;
  }
  form.addEventListener('submit', async event => {
    event.preventDefault();
    if (!token || !form.reportValidity()) return;
    const password = form.elements.password.value;
    if (password !== form.elements.confirmPassword.value) {
      message.textContent = 'The passwords do not match.';
      form.elements.confirmPassword.focus();
      return;
    }
    button.disabled = true;
    message.textContent = 'Updating your password…';
    try {
      const result = await WeddingHubAPI.resetPassword(token, password);
      message.textContent = result.message || 'Password reset successful.';
      form.hidden = true;
      const login = document.createElement('a');
      login.className = 'auth-submit auth-submit-link';
      login.href = '../pages/login.html';
      login.textContent = 'Return to login';
      message.after(login);
    } catch (error) {
      message.textContent = error.message || 'We could not reset your password. Request a new link and try again.';
      if (error.status === 400 || error.status === 410) button.disabled = true;
      else button.disabled = false;
    }
  });
})();
