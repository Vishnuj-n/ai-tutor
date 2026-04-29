<template>
  <section class="page">
    <p class="eyebrow">Tools</p>
    <h1>Learning Utilities</h1>

    <article class="panel intro">
      <p>Select a tool to continue.</p>
    </article>

    <!-- Tools Analytics Panel -->
    <div class="tools-analytics">
      <div class="analytics-card">
        <h3 class="analytics-title">Tool Usage</h3>
        <div class="metric-row">
          <span class="metric-label">Available Tools</span>
          <span class="metric-value">3</span>
        </div>
        <div class="metric-row">
          <span class="metric-label">Sessions Today</span>
          <span class="metric-value">12</span>
        </div>
        <div class="metric-row">
          <span class="metric-label">Active Users</span>
          <span class="metric-value">8</span>
        </div>
      </div>
      <div class="analytics-card">
        <h3 class="analytics-title">Learning Impact</h3>
        <div class="metric-row">
          <span class="metric-label">Concepts Mapped</span>
          <span class="metric-value">47</span>
        </div>
        <div class="metric-row">
          <span class="metric-label">Mnemonics Created</span>
          <span class="metric-value">23</span>
        </div>
        <div class="metric-row">
          <span class="metric-label">Questions Guided</span>
          <span class="metric-value">156</span>
        </div>
      </div>
    </div>

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
@import url('https://fonts.googleapis.com/css2?family=Manrope:wght@400;500;600;700&family=Inter:wght@400;500;600;700&display=swap');

.page {
  display: grid;
  gap: 20px;
  padding: 3rem 2rem;
  max-width: 720px;
  margin: 0 auto;
  background: var(--background);
  min-height: 100vh;
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-weight: 700;
  font-family: 'Inter', sans-serif;
}

h1 {
  margin: 0;
  font-size: 46px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
  color: var(--on-surface);
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
  font-family: 'Inter', sans-serif;
  font-weight: 500;
}

/* Tools Analytics Panel */
.tools-analytics {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1.5rem;
  margin-top: 2rem;
}

.analytics-card {
  background: var(--surface-container-low);
  border-radius: 0.75rem;
  padding: 1.5rem;
  transition: all 0.3s ease;
}

.analytics-card:hover {
  background: var(--surface-container-lowest);
  box-shadow: 0 20px 40px rgba(45, 51, 56, 0.06);
  transform: translateY(-2px);
}

.analytics-title {
  font-family: 'Manrope', sans-serif;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--on-surface);
  margin: 0 0 1rem 0;
  letter-spacing: -0.2%;
}

.metric-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem 0;
  border-bottom: 1px solid var(--outline-variant);
}

.metric-row:last-child {
  border-bottom: none;
}

.metric-label {
  font-family: 'Inter', sans-serif;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--muted-text);
}

.metric-value {
  font-family: 'Inter', sans-serif;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--on-surface);
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
  box-shadow: 0 20px 40px rgba(45, 51, 56, 0.06);
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
  font-family: 'Inter', sans-serif;
}

h2 {
  margin: 0;
  font-size: 24px;
  font-family: 'Manrope', sans-serif;
  color: var(--on-surface);
}

.card-copy {
  margin: 0;
  color: var(--muted-text);
  line-height: 1.5;
  font-family: 'Inter', sans-serif;
  font-weight: 500;
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
