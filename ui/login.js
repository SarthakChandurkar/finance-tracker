window.addEventListener('pageshow', (event) => {
      if (event.persisted) window.location.reload();
    });

    document.getElementById('login-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      
      const errorEl = document.getElementById('error');
      errorEl.textContent = '';
      
      const btn = e.target.querySelector('button[type="submit"]');
      if (btn) btn.classList.add('btn-loading');
      
      const form = new FormData(e.target);
      try {
        const res = await fetch('/api/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            username: form.get('username'),
            password: form.get('password'),
          }),
        });
        if (res.ok) {
          window.location.href = '/';
          return;
        }
        if (res.status === 409) {
          window.location.href = '/';
          return;
        }
        errorEl.textContent = await res.text();
      } finally {
        if (btn) btn.classList.remove('btn-loading');
      }
    });