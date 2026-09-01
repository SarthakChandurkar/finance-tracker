document.getElementById('register-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      
      const errorEl = document.getElementById('error');
      errorEl.textContent = '';
      
      const btn = e.target.querySelector('button[type="submit"]');
      if (btn) btn.classList.add('btn-loading');
      
      const form = new FormData(e.target);
      try {
        const res = await fetch('/api/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            username: form.get('username'),
            password: form.get('password'),
          }),
        });
        if (res.ok) {
          window.location.href = '/login.html';
          return;
        }
        errorEl.textContent = await res.text();
      } finally {
        if (btn) btn.classList.remove('btn-loading');
      }
    });