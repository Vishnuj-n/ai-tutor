<template>
  <section class="page">
    <p class="eyebrow">Settings</p>
    <h1>Preferences</h1>
    <article class="panel form-grid">
      <label>
        Base URL
        <input type="text" placeholder="https://api.example.com" />
      </label>
      <label>
        Model
        <input type="text" placeholder="gpt-4o-mini" />
      </label>
      <label>
        API Key
        <input type="password" placeholder="sk-..." />
      </label>
    </article>
  </section>
</template>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-weight: 700;
}

h1 {
  margin: 0;
  font-size: 46px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 24px;
}

.form-grid {
  display: grid;
  gap: 12px;
  max-width: 560px;
}

label {
  display: grid;
  gap: 8px;
  font-weight: 600;
  color: var(--on-surface);
}

input {
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 11px 12px;
  font-size: 14px;
}

input:focus {
  border-color: var(--primary);
  outline: none;
}
</style>
