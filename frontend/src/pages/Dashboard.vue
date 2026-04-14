<template>
  <section class="page">
    <header class="topbar">
      <div class="search-shell">Search knowledge base...</div>
      <div class="profile">Academic Profile</div>
    </header>

    <article class="status-strip">
      <div>
        <p class="eyebrow">Today</p>
        <h1>{{ plan.totalMinutes }} minute study plan</h1>
      </div>
      <div class="status-badges">
        <span class="badge" :class="planMeta.dataFresh ? 'ok' : 'warn'">
          {{ planMeta.dataFresh ? 'Live data' : 'Stale data' }}
        </span>
        <span class="badge" :class="planMeta.isEstimate ? 'warn' : 'ok'">
          {{ planMeta.isEstimate ? 'Estimated plan' : 'Scheduled plan' }}
        </span>
        <span class="stamp">Updated {{ generatedLabel }}</span>
      </div>
    </article>

    <div class="grid-top">
      <article class="card feature-card">
        <p class="eyebrow">Current Session</p>
        <template v-if="loading">
          <h2>Preparing your schedule...</h2>
          <p class="muted">Building today's priorities from review and learning needs.</p>
        </template>
        <template v-else-if="error">
          <h2>Plan unavailable</h2>
          <p class="muted">{{ error }}</p>
        </template>
        <template v-else-if="currentTask">
          <h2>{{ currentTask.title }}</h2>
          <p class="task-meta">{{ currentTask.meta || defaultTaskMeta(currentTask) }}</p>
          <button type="button" class="primary-btn" @click="startTask(currentTask)">
            Start Session
          </button>
        </template>
        <template v-else>
          <h2>No tasks for now</h2>
          <p class="muted">You're clear for today. Use exploration mode or start a new topic.</p>
        </template>
      </article>

      <article class="card side-card">
        <p class="eyebrow">Due Reviews</p>
        <p class="big-number">{{ plan.dueReviewCards }} <span>cards</span></p>
        <button
          type="button"
          class="review-btn"
          :disabled="loading || !reviewRoute"
          @click="startReviewSession"
        >
          {{ plan.dueReviewCards > 0 ? 'Review Due Cards' : 'All Caught Up' }}
        </button>
        <div class="chip-group">
          <p class="eyebrow">Active Topics</p>
          <div class="chips">
            <span v-for="topic in plan.activeTopics" :key="topic">{{ topic }}</span>
            <span v-if="plan.activeTopics.length === 0">No active topics yet</span>
          </div>
        </div>
      </article>
    </div>

    <div class="grid-bottom">
      <article class="card chart-card">
        <h3>Weekly Insights</h3>
        <div v-if="planMeta.insightsAvailable" class="bars">
          <div class="bar" style="height: 48%"></div>
          <div class="bar" style="height: 78%"></div>
          <div class="bar" style="height: 64%"></div>
          <div class="bar active" style="height: 88%"></div>
          <div class="bar" style="height: 52%"></div>
        </div>
        <div v-else class="insight-placeholder">
          <p class="task-meta">Weekly analytics are not connected yet.</p>
          <p class="muted">Showing task plan only to avoid misleading trends.</p>
        </div>
      </article>

      <article class="card list-card">
        <h3>Curated Focus</h3>
        <div v-for="item in focusItems" :key="item.id" class="focus-item">
          <div class="dot"></div>
          <div>
            <p class="focus-title">{{ item.title }}</p>
            <p class="task-meta">{{ item.meta || defaultTaskMeta(item) }}</p>
          </div>
        </div>
        <p v-if="!loading && focusItems.length === 0" class="muted">No additional tasks queued.</p>
      </article>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getTodayPlan } from '../services/appApi'

const router = useRouter()

const loading = ref(true)
const error = ref('')
const tasks = ref([])
const plan = ref({
  totalMinutes: 90,
  dueReviewCards: 0,
  activeTopics: [],
})
const planMeta = ref({
  generatedAtUnix: 0,
  dataFresh: false,
  isEstimate: true,
  insightsAvailable: false,
})

const generatedLabel = computed(() => {
  if (!planMeta.value.generatedAtUnix) {
    return 'just now'
  }
  const asDate = new Date(planMeta.value.generatedAtUnix * 1000)
  if (Number.isNaN(asDate.getTime())) {
    return 'just now'
  }
  return asDate.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
})

const currentTask = computed(() => tasks.value[0] || null)
const focusItems = computed(() => tasks.value)
const reviewTask = computed(() =>
  tasks.value.find((task) => task.action_type === 'review') || null
)
const reviewRoute = computed(() => {
  if (reviewTask.value?.topic_id) {
    return { path: '/flashcards', query: { topic: reviewTask.value.topic_id } }
  }
  return plan.value.dueReviewCards > 0 ? { path: '/flashcards' } : null
})

onMounted(async () => {
  await loadPlan()
})

async function loadPlan() {
  try {
    loading.value = true
    error.value = ''

    const response = await getTodayPlan()
    if (response.error) {
      error.value = response.error
      return
    }

    plan.value = {
      totalMinutes: response.total_minutes || 90,
      dueReviewCards: response.due_review_cards || 0,
      activeTopics: response.active_topics || [],
    }

    planMeta.value = {
      generatedAtUnix: Number(response.generated_at_unix) || 0,
      dataFresh: Boolean(response.data_fresh),
      isEstimate: Boolean(response.is_estimate),
      insightsAvailable: Boolean(response.insights_available),
    }

    tasks.value = response.tasks || []
  } catch (err) {
    error.value = err.message || 'Failed to load today plan'
  } finally {
    loading.value = false
  }
}

function defaultTaskMeta(task) {
  return `${task.estimate_minutes || 15} min session`
}

function startTask(task) {
  const actionRoutes = {
    review: '/flashcards',
    read: '/reader',
    quiz: '/quiz',
    socratic: '/socratic',
    explore: '/reader',
  }

  const path = actionRoutes[task.action_type] || '/dashboard'
  if (task.topic_id) {
    router.push({ path, query: { topic: task.topic_id } })
    return
  }

  router.push(path)
}

function startReviewSession() {
  if (!reviewRoute.value) {
    return
  }
  router.push(reviewRoute.value)
}
</script>

<style scoped>
.page {
  display: grid;
  gap: 16px;
}

:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.topbar {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.search-shell {
  flex: 1;
  background: var(--surface-container-low);
  color: var(--muted-text);
  border-radius: 12px;
  padding: 11px 14px;
  max-width: 440px;
}

.profile {
  background: var(--surface-container-low);
  border-radius: 12px;
  padding: 11px 14px;
  font-weight: 600;
  color: var(--on-surface);
}

.status-strip {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 12px;
  padding: 8px 2px 2px;
}

.status-strip h1 {
  margin: 0;
  font-family: 'Manrope', sans-serif;
  font-size: 44px;
  letter-spacing: -0.03em;
  line-height: 1;
  color: var(--on-surface);
}

.status-badges {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.badge {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  border-radius: 999px;
  padding: 5px 10px;
}

.badge.ok {
  background: rgba(64, 95, 171, 0.15);
  color: var(--primary);
}

.badge.warn {
  background: rgba(190, 120, 58, 0.16);
  color: #8f4f17;
}

.stamp {
  color: var(--muted-text);
  font-size: 12px;
}

.grid-top {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 12px;
}

.grid-bottom {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  gap: 12px;
}

.card {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 18px;
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
  margin: 10px 0;
  max-width: 540px;
  font-size: 32px;
  line-height: 1.12;
  letter-spacing: -0.02em;
  font-family: 'Manrope', sans-serif;
}

.muted {
  margin: 0;
  color: var(--muted-text);
  font-size: 15px;
}

.task-meta {
  margin: 2px 0 0;
  color: var(--muted-text);
  font-size: 13px;
  letter-spacing: 0.02em;
}

.primary-btn {
  margin-top: 16px;
  border: 0;
  border-radius: 12px;
  padding: 10px 24px;
  color: var(--on-primary);
  font-size: 15px;
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  transition: transform 0.14s ease, filter 0.14s ease;
}

.primary-btn:active,
.review-btn:active {
  transform: scale(0.95);
}

.review-btn {
  border: 0;
  border-radius: 12px;
  padding: 10px 14px;
  font-size: 14px;
  font-weight: 700;
  color: var(--on-primary);
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
}

.review-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.big-number {
  margin: 10px 0 24px;
  color: var(--on-surface);
  font-size: 56px;
  line-height: 0.95;
  font-family: 'Manrope', sans-serif;
}

.big-number span {
  font-size: 18px;
  color: var(--muted-text);
  font-family: 'Inter', sans-serif;
}

.chip-group {
  display: grid;
  gap: 10px;
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
  padding: 6px 10px;
  font-size: 11px;
  font-weight: 600;
}

.chart-card h3,
.list-card h3 {
  margin: 0 0 12px;
  font-family: 'Manrope', sans-serif;
  font-size: 24px;
  letter-spacing: -0.01em;
}

.bars {
  height: 136px;
  background: var(--surface-container-low);
  border-radius: 14px;
  padding: 12px;
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  align-items: end;
  gap: 8px;
  margin-bottom: 10px;
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
  gap: 8px;
  align-content: start;
}

.focus-item {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 10px 12px;
  border-radius: 14px;
  background: var(--surface-container-low);
  transition:
    background-color 0.2s ease,
    box-shadow 0.2s ease;
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

.insight-placeholder {
  display: grid;
  gap: 8px;
  padding: 12px;
  border-radius: 12px;
  background: var(--surface-container-low);
}

@media (max-width: 1200px) {
  .status-strip h1 {
    font-size: 42px;
  }

  .feature-card h2 {
    font-size: 30px;
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

  .status-strip {
    align-items: start;
    flex-direction: column;
  }

  .status-strip h1 {
    font-size: 38px;
  }

  .feature-card h2 {
    font-size: 32px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .focus-item,
  .primary-btn,
  .review-btn {
    transition: none;
  }
}
</style>
