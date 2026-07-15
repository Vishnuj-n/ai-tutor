<template>
  <article class="card review-hero-card">
    <div class="review-hero-header">
      <span class="review-hero-tag">HIGH PRIORITY</span>
      <span class="review-hero-estimate">{{ task.estimate_minutes }} min review</span>
    </div>
    <div class="review-hero-body">
      <h2>Today's Reviews</h2>
      <div class="review-hero-stats">
        <div class="review-hero-stat">
          <span class="stat-num">{{ dueReviewCards }}</span>
          <span class="stat-lbl">Due Today</span>
        </div>
        <div class="review-hero-stat">
          <span class="stat-num">{{ totalDueReviewCards > dueReviewCards ? totalDueReviewCards - dueReviewCards : 0 }}</span>
          <span class="stat-lbl">Remaining Overdue</span>
        </div>
      </div>
      <p class="review-hero-meta">{{ task.meta }}</p>
    </div>
    <button
      type="button"
      class="primary-btn review-hero-btn"
      @click="$emit('start', task)"
    >
      Start Review
    </button>
  </article>
</template>

<script setup>
defineProps({
  task: { type: Object, required: true },
  dueReviewCards: { type: Number, default: 0 },
  totalDueReviewCards: { type: Number, default: 0 },
})

defineEmits(['start'])
</script>

<style scoped>
.card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 16px;
}

.review-hero-card {
  background: var(--surface-container-low);
  border: 1px solid var(--primary);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.05);
  padding: 24px;
  display: grid;
  grid-template-columns: 1fr;
  gap: 16px;
  position: relative;
  overflow: hidden;
  border-radius: 16px;
}

@media (min-width: 768px) {
  .review-hero-card {
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: 32px;
  }
}

.review-hero-header {
  grid-column: 1 / -1;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.review-hero-tag {
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.1em;
  color: var(--primary);
  background: rgba(108, 92, 231, 0.1);
  padding: 4px 8px;
  border-radius: 6px;
  text-transform: uppercase;
}

.review-hero-estimate {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-text);
}

.review-hero-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.review-hero-body h2 {
  margin: 0;
  font-family: 'Manrope', sans-serif;
  font-size: 28px;
  font-weight: 800;
  color: var(--on-surface);
}

.review-hero-stats {
  display: flex;
  gap: 24px;
  margin-top: 4px;
}

.review-hero-stat {
  display: flex;
  flex-direction: column;
}

.stat-num {
  font-family: 'Manrope', sans-serif;
  font-size: 32px;
  font-weight: 800;
  color: var(--on-surface);
  line-height: 1;
}

.stat-lbl {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-text);
  text-transform: uppercase;
  margin-top: 4px;
}

.review-hero-meta {
  margin: 0;
  font-size: 14px;
  color: var(--muted-text);
}

.primary-btn {
  background: var(--primary);
  color: var(--on-primary);
  border: none;
  border-radius: 10px;
  padding: 10px;
  font-weight: 700;
  cursor: pointer;
  transition: opacity 0.2s;
}

.primary-btn:hover {
  opacity: 0.9;
}

.review-hero-btn {
  justify-self: stretch;
  padding: 14px 28px;
  font-size: 16px;
  height: auto;
}

@media (min-width: 768px) {
  .review-hero-btn {
    justify-self: end;
  }
}
</style>
