(() => {
  const form = document.getElementById('recovery-form');
  const message = document.getElementById('form-message');
  const button = form.querySelector('button[type="submit"]');
  form.addEventListener('submit', async event => {
    event.preventDefault();
    if (!form.reportValidity()) return;
    button.disabled = true;
    message.textContent = 'Sending your request…';
    try {
      const result = await WeddingHubAPI.forgotPassword(form.elements.email.value.trim());
      message.textContent = result.message || 'If an account exists for that email, a password reset link will be sent. Check your inbox and spam folder.';
    } catch (error) {
      message.textContent = error.message || 'We could not reach WeddingHub. Check your connection and try again.';
      button.disabled = false;
    }
  });
})();
