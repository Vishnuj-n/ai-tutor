<template>
  <section class="page">
    <header class="topbar">
      <div class="search-shell">Search knowledge base...</div>
      <div class="profile">Academic Profile</div>
    </header>

    <div class="hero">
      <h1>Good Morning.</h1>
      <p>
        Your plan today is <strong>{{ plan.totalMinutes }}</strong> minutes with a balanced learning
        loop.
      </p>
    </div>

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
          <p class="muted">{{ currentTask.meta || defaultTaskMeta(currentTask) }}</p>
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
        <div v-for="item in focusItems" :key="item.id" class="focus-item">
          <div class="dot"></div>
          <div>
            <p class="focus-title">{{ item.title }}</p>
            <p class="focus-meta">{{ item.meta || defaultTaskMeta(item) }}</p>
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

const currentTask = computed(() => tasks.value[0] || null)
const focusItems = computed(() => tasks.value.slice(1))
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

.hero h1 {
  margin: 0;
  font-family: 'Manrope', sans-serif;
  font-size: 48px;
  letter-spacing: -0.02em;
  line-height: 1;
  color: var(--on-surface);
}

.hero p {
  margin: 8px 0 0;
  font-size: 18px;
  color: var(--muted-text);
}

.hero strong {
  color: var(--primary);
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

.primary-btn {
  margin-top: 16px;
  border: 0;
  border-radius: 12px;
  padding: 10px 24px;
  color: var(--on-primary);
  font-size: 15px;
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
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

.focus-meta {
  margin: 2px 0 0;
  color: var(--muted-text);
  text-transform: uppercase;
  font-size: 11px;
  letter-spacing: 0.08em;
}

@media (max-width: 1200px) {
  .hero h1 {
    font-size: 42px;
  }

  .hero p {
    font-size: 17px;
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
