<template>
  <section class="page">
    <p class="eyebrow">Flashcards</p>
    <h1>Review Session</h1>
    <article class="panel">
      <h2>Placeholder Card</h2>
      <p>FSRS card content appears here.</p>
      <div class="actions">
        <button type="button">Again</button>
        <button type="button">Hard</button>
        <button type="button">Good</button>
        <button type="button">Easy</button>
      </div>
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

.actions {
  margin-top: 18px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.actions button {
  border: 0;
  border-radius: 12px;
  padding: 10px 14px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  font-weight: 600;
}
</style>
