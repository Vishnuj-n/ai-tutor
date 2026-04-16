<template>
  <section class="page">
    <p class="eyebrow">Tools</p>
    <h1>Learning Utilities</h1>

    <article class="panel intro">
      <p>Select a tool to continue.</p>
    </article>

    <div class="tool-stack">
      <button type="button" class="tool-card" @click="go('/socratic')">
        <span class="tool-icon" aria-hidden="true">◎</span>
        <p class="card-eyebrow">Assistant</p>
        <h2>Socratic Tutor</h2>
        <p class="card-copy">Ask guided, grounded questions tied to your material.</p>
      </button>

      <button type="button" class="tool-card" @click="go('/tools/acronym-generator')">
        <span class="tool-icon" aria-hidden="true">◇</span>
        <p class="card-eyebrow">Memory</p>
        <h2>Acronym Generator</h2>
        <p class="card-copy">Create compact mnemonic acronyms from key ideas.</p>
      </button>

      <button type="button" class="tool-card" @click="go('/tools/mindmap-generator')">
        <span class="tool-icon" aria-hidden="true">◍</span>
        <p class="card-eyebrow">Visual</p>
        <h2>Mindmap Generator</h2>
        <p class="card-copy">Generate concept maps to connect sections quickly.</p>
      </button>
    </div>
  </section>
</template>

<script setup>
import { useRouter } from 'vue-router'

const router = useRouter()

function go(path) {
  router.push(path)
}
</script>

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
  padding: 20px 24px;
}

.intro p {
  margin: 0;
  color: var(--muted-text);
  font-size: 15px;
}

.tool-stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
  align-items: flex-start;
}

.tool-card {
  border: 0;
  text-align: left;
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 20px;
  display: grid;
  gap: 8px;
  width: min(100%, 640px);
  cursor: pointer;
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease;
}

.tool-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 24px rgba(31, 38, 53, 0.08);
}

.tool-icon {
  display: inline-grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border-radius: 8px;
  background: var(--surface-container-highest);
  color: var(--on-surface);
  font-size: 12px;
}

.card-eyebrow {
  margin: 0;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.09em;
  text-transform: uppercase;
  color: var(--muted-text);
}

h2 {
  margin: 0;
  font-size: 24px;
  font-family: 'Manrope', sans-serif;
}

.card-copy {
  margin: 0;
  color: var(--muted-text);
  line-height: 1.5;
}

@media (max-width: 960px) {
  .tool-stack {
    align-items: stretch;
  }

  .tool-card {
    width: 100%;
  }
}
</style>
