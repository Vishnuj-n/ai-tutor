<template>
  <section class="page">
    <header class="page-header">
      <p class="eyebrow">Index</p>
      <h1>Learning Utilities</h1>
      <p class="intro-text">Curated academic capabilities and cognitive aids.</p>
    </header>

    <div class="utilities-container">
      <div class="utility-list">
        <button type="button" class="utility-item" @click="go('/socratic')">
          <div class="utility-meta">
            <span class="utility-icon" aria-hidden="true">◎</span>
            <span class="utility-category">Assistant</span>
          </div>
          <div class="utility-content">
            <h2>Socratic Tutor</h2>
            <p>Engage in guided, grounded dialogues tied to your material.</p>
          </div>
        </button>

        <button type="button" class="utility-item" @click="go('/tools/acronym-generator')">
          <div class="utility-meta">
            <span class="utility-icon" aria-hidden="true">◇</span>
            <span class="utility-category">Memory</span>
          </div>
          <div class="utility-content">
            <h2>Acronym Generator</h2>
            <p>Create compact mnemonic acronyms from key ideas.</p>
          </div>
        </button>

        <button type="button" class="utility-item" @click="go('/tools/mindmap-generator')">
          <div class="utility-meta">
            <span class="utility-icon" aria-hidden="true">◍</span>
            <span class="utility-category">Visual</span>
          </div>
          <div class="utility-content">
            <h2>Mindmap Generator</h2>
            <p>Generate concept maps to connect sections quickly.</p>
          </div>
        </button>
      </div>
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
  display: flex;
  flex-direction: column;
  padding: 64px 48px;
  width: 100%;
  max-width: 1024px;
  margin: 0 auto;
  min-height: 100vh;
  gap: 48px;
  background: var(--background);
}

.page-header {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-weight: 600;
  font-family: 'Inter', sans-serif;
}

h1 {
  margin: 0;
  font-size: 36px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
  color: var(--on-surface);
  font-weight: 500;
}

.intro-text {
  margin: 0;
  color: var(--muted-text);
  font-size: 16px;
  font-family: 'Inter', sans-serif;
  font-weight: 400;
  line-height: 1.5;
  max-width: 600px;
}

.utilities-container {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 16px;
}

.utility-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.utility-item {
  display: grid;
  grid-template-columns: 140px 1fr;
  align-items: flex-start;
  gap: 32px;
  width: 100%;
  text-align: left;
  background: transparent;
  border: none;
  padding: 24px 32px;
  border-radius: 12px;
  cursor: pointer;
  transition: background 0.2s ease;
  position: relative;
}

.utility-item:hover,
.utility-item:focus-visible {
  background: var(--surface-container-low);
  outline: none;
}

.utility-item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%) scaleY(0);
  width: 3px;
  height: 32px;
  background: var(--primary);
  border-radius: 0 4px 4px 0;
  transition: transform 0.2s ease;
  opacity: 0.6;
}

.utility-item:hover::before,
.utility-item:focus-visible::before {
  transform: translateY(-50%) scaleY(1);
}

.utility-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--muted-text);
  margin-top: 2px; /* Visual alignment with heading */
}

.utility-icon {
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--muted-text);
}

.utility-category {
  font-size: 13px;
  font-family: 'Inter', sans-serif;
  font-weight: 500;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.utility-content {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-width: 640px;
}

h2 {
  margin: 0;
  font-size: 18px;
  font-family: 'Inter', sans-serif;
  font-weight: 600;
  color: var(--on-surface);
}

.utility-content p {
  margin: 0;
  font-size: 15px;
  color: var(--muted-text);
  font-family: 'Inter', sans-serif;
  font-weight: 400;
  line-height: 1.6;
}

@media (max-width: 768px) {
  .utility-item {
    grid-template-columns: 1fr;
    gap: 12px;
    padding: 20px 24px;
  }

  .utility-meta {
    margin-top: 0;
  }

  .page {
    padding: 40px 24px;
  }
}
</style>
