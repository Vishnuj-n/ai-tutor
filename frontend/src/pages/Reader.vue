<template>
  <section class="page">
    <div>
      <p class="eyebrow">Reader</p>
      <h1>Operating Systems</h1>
      <p class="muted">Structured topic content appears here with sections and headings.</p>
    </div>

    <div class="split">
      <article class="panel">
        <h2>Content</h2>
        <p>
          Round Robin scheduling is a preemptive algorithm where each process receives a fixed time slice and cycles in queue order.
        </p>
      </article>

      <article class="panel">
        <h2>Ask AI</h2>
        <textarea placeholder="Ask for concept clarification in this topic..."></textarea>
        <button type="button" class="primary-btn">Ask</button>
      </article>
    </div>
  </section>
</template>

<style scoped>
.page {
  display: grid;
  gap: 24px;
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
  margin: 8px 0 0;
  font-size: 46px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.muted {
  margin: 12px 0 0;
  color: var(--muted-text);
}

.split {
  display: grid;
  grid-template-columns: 1.6fr 1fr;
  gap: 16px;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 24px;
}

h2 {
  margin: 0 0 16px;
  font-size: 30px;
  font-family: 'Manrope', sans-serif;
}

textarea {
  width: 100%;
  min-height: 120px;
  border: 0;
  outline: 0;
  border-radius: 12px;
  background: var(--surface-container-low);
  padding: 12px;
  font-family: inherit;
  color: var(--on-surface);
  resize: vertical;
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

@media (max-width: 960px) {
  .split {
    grid-template-columns: 1fr;
  }

  h1 {
    font-size: 34px;
  }
}
</style>
