<template>
  <section class="page">
    <header class="topbar">
      <div class="search-shell">Search knowledge base...</div>
      <div class="profile">Academic Profile</div>
    </header>

    <div class="hero">
      <h1>Good Morning.</h1>
      <p>Your weekly progress is at <strong>85%</strong> of your focus goal.</p>
    </div>

    <div class="grid-top">
      <article class="card feature-card">
        <p class="eyebrow">Current Session</p>
        <h2>Deep Dive: Cognitive Architectures in Neural Networks</h2>
        <p class="muted">Continue your review of the primary thesis. 12 pages remaining for today's milestone.</p>
        <button type="button" class="primary-btn">Start Session</button>
      </article>

      <article class="card side-card">
        <p class="eyebrow">Due Reviews</p>
        <p class="big-number">14 <span>cards</span></p>
        <div class="chip-group">
          <p class="eyebrow">Active Topics</p>
          <div class="chips">
            <span>AI Ethics</span>
            <span>Neuroscience</span>
            <span>Philosophy</span>
          </div>
        </div>
      </article>
    </div>

    <div class="grid-bottom">
      <article class="card chart-card">
        <h3>Weekly Insights</h3>
        <div class="bars">
          <div class="bar" style="height: 48%"></div>
          <div class="bar" style="height: 78%"></div>
          <div class="bar" style="height: 64%"></div>
          <div class="bar active" style="height: 88%"></div>
          <div class="bar" style="height: 52%"></div>
        </div>
        <p class="muted">Peak productivity reached at 10:00 AM on Tuesday.</p>
      </article>

      <article class="card list-card">
        <h3>Curated Focus</h3>
        <div class="focus-item" v-for="item in focusItems" :key="item.title">
          <div class="dot"></div>
          <div>
            <p class="focus-title">{{ item.title }}</p>
            <p class="focus-meta">{{ item.meta }}</p>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>

<script setup>
const focusItems = [
  { title: 'Post-Structuralism Basics', meta: '85% mastered - 12 cards' },
  { title: 'Synaptic Plasticity', meta: '42% mastered - 48 cards' },
  { title: 'Advanced Game Theory', meta: 'New topic - 4 cards' },
]
</script>

<style scoped>
.page {
  display: grid;
  gap: 24px;
}

.topbar {
  display: flex;
  justify-content: space-between;
  gap: 16px;
}

.search-shell {
  flex: 1;
  background: var(--surface-container-low);
  color: var(--muted-text);
  border-radius: 12px;
  padding: 13px 16px;
  max-width: 440px;
}

.profile {
  background: var(--surface-container-low);
  border-radius: 12px;
  padding: 13px 16px;
  font-weight: 600;
  color: var(--on-surface);
}

.hero h1 {
  margin: 0;
  font-family: 'Manrope', sans-serif;
  font-size: 54px;
  letter-spacing: -0.02em;
  line-height: 1;
  color: var(--on-surface);
}

.hero p {
  margin: 12px 0 0;
  font-size: 22px;
  color: var(--muted-text);
}

.hero strong {
  color: var(--primary);
}

.grid-top {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 16px;
}

.grid-bottom {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  gap: 16px;
}

.card {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 24px;
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-weight: 700;
}

.feature-card h2 {
  margin: 14px 0 14px;
  max-width: 540px;
  font-size: 44px;
  line-height: 1.1;
  letter-spacing: -0.02em;
  font-family: 'Manrope', sans-serif;
}

.muted {
  margin: 0;
  color: var(--muted-text);
  font-size: 16px;
}

.primary-btn {
  margin-top: 24px;
  border: 0;
  border-radius: 12px;
  padding: 12px 28px;
  color: var(--on-primary);
  font-size: 16px;
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
}

.big-number {
  margin: 18px 0 42px;
  color: var(--on-surface);
  font-size: 68px;
  line-height: 0.95;
  font-family: 'Manrope', sans-serif;
}

.big-number span {
  font-size: 24px;
  color: var(--muted-text);
  font-family: 'Inter', sans-serif;
}

.chip-group {
  display: grid;
  gap: 14px;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.chips span {
  background: var(--surface-container-low);
  color: var(--on-surface);
  border-radius: 999px;
  padding: 7px 10px;
  font-size: 12px;
  font-weight: 600;
}

.chart-card h3,
.list-card h3 {
  margin: 0 0 16px;
  font-family: 'Manrope', sans-serif;
  font-size: 30px;
  letter-spacing: -0.01em;
}

.bars {
  height: 200px;
  background: var(--surface-container-low);
  border-radius: 14px;
  padding: 16px;
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  align-items: end;
  gap: 10px;
  margin-bottom: 16px;
}

.bar {
  background: #b7c6dc;
  border-radius: 8px 8px 4px 4px;
}

.bar.active {
  background: var(--primary);
}

.list-card {
  display: grid;
  gap: 12px;
  align-content: start;
}

.focus-item {
  display: flex;
  gap: 14px;
  align-items: center;
  padding: 14px;
  border-radius: 14px;
  background: var(--surface-container-low);
  transition: background-color 0.2s ease, box-shadow 0.2s ease;
}

.focus-item:hover {
  background: var(--surface-container-lowest);
  box-shadow: 0 2px 8px rgba(45, 51, 56, 0.06);
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--primary);
}

.focus-title {
  margin: 0;
  font-weight: 700;
}

.focus-meta {
  margin: 4px 0 0;
  color: var(--muted-text);
  text-transform: uppercase;
  font-size: 12px;
  letter-spacing: 0.08em;
}

@media (max-width: 1200px) {
  .hero h1 {
    font-size: 46px;
  }

  .hero p {
    font-size: 19px;
  }

  .feature-card h2 {
    font-size: 36px;
  }
}

@media (max-width: 960px) {
  .topbar,
  .grid-top,
  .grid-bottom {
    grid-template-columns: 1fr;
  }

  .big-number {
    font-size: 56px;
  }

  .hero h1 {
    font-size: 38px;
  }

  .hero p {
    font-size: 18px;
  }

  .feature-card h2 {
    font-size: 32px;
  }
}
</style>
