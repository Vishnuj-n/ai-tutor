<template>
  <section class="page">
    <p class="eyebrow">Socratic Tutor</p>
    <h1>Guided Thinking</h1>
    <article class="panel">
      <h2>Placeholder</h2>
      <p>Socratic prompts and responses will render here.</p>
      <button type="button" class="primary-btn">Start Prompt</button>
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

h2 {
  margin: 0 0 8px;
  font-family: 'Manrope', sans-serif;
}

p {
  margin: 0;
  color: var(--muted-text);
}

.primary-btn {
  margin-top: 14px;
  border: 0;
  border-radius: 12px;
  padding: 12px 20px;
  color: var(--on-primary);
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
}
</style>
