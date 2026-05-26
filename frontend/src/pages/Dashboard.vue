<template>
  <section class="page">
    <header class="topbar">
      <div class="search-shell">Search knowledge base...</div>
      <div class="profile">Academic Profile</div>
    </header>

    <!-- Flashcard creation confirmation banner -->
    <article v-if="flashcardsJustCreated" class="card flashcard-success-banner">
      <div class="flashcard-success-content">
        <span class="flashcard-success-icon">✓</span>
        <div class="flashcard-success-text">
          <p class="flashcard-success-title">Flashcards generated successfully!</p>
          <p class="flashcard-success-subtitle">{{ flashcardsJustCreated }} cards scheduled for spaced repetition. They'll appear here when due.</p>
        </div>
      </div>
    </article>

    <article class="status-strip">
      <div>
        <p class="eyebrow">Today's Mission</p>
        <h1>Daily Agenda</h1>
      </div>
      <div v-if="dueReviewCards > 0" class="review-stats">
        <p class="review-count">{{ dueReviewCards }} cards due for review</p>
        <p class="review-hint">Spaced repetition strengthens long-term retention</p>
      </div>
    </article>

    <template v-if="loading">
      <article class="card state-card">
        <h2>Loading your agenda...</h2>
        <p class="muted">Preparing today's tasks.</p>
      </article>
    </template>

    <template v-else-if="error">
      <article class="card state-card error-card">
        <h2>Agenda unavailable</h2>
        <p class="muted">{{ error }}</p>
      </article>
    </template>

    <template v-else-if="tasks.length === 0 && hasActiveStudyContent">
      <article class="card state-card victory-card">
        <h2>Mission Complete!</h2>
        <p class="muted">You've completed all tasks for today. Great work!</p>
      </article>
    </template>

    <template v-else-if="tasks.length === 0">
      <article class="card state-card">
        <h2>No tasks yet</h2>
        <p class="muted">Upload and confirm a notebook syllabus to generate your first agenda tasks.</p>
      </article>
    </template>

    <template v-else>
      <div class="task-list">
        <article v-for="task in tasks" :key="task.id" class="card task-card">
          <div class="task-header">
            <span class="task-type">{{ task.action_type }}</span>
            <span class="task-estimate">{{ task.estimate_minutes }} min</span>
          </div>
          <h3>{{ task.title }}</h3>
          <p class="task-meta">{{ task.meta ? task.meta : (task.start_page !== undefined && task.start_page !== null && task.end_page !== undefined && task.end_page !== null ? 'Pages ' + task.start_page + '-' + task.end_page : 'Pages N/A') }}</p>
          <button type="button" class="primary-btn" :aria-label="'Start task ' + (task.title || task.id)" @click="startTask(task)">
            Start
          </button>
        </article>
      </div>
    </template>
  </section>
</template>

<script setup>
import { onMounted, ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { getTodayPlan, getNotebooks } from '../services/appApi'

const router = useRouter()
const route = useRoute()

const loading = ref(true)
const error = ref('')
const tasks = ref([])
const hasActiveStudyContent = ref(false)
const dueReviewCards = ref(0)

// Show confirmation when flashcards were just created from quiz completion
const flashcardsJustCreated = computed(() => {
  const created = parseInt(route.query.flashcardsCreated, 10)
  return isNaN(created) || created <= 0 ? 0 : created
})

onMounted(async () => {
  console.warn('[DASHBOARD] onMounted loading queue')
  // Clear flashcardsCreated query param after first render to prevent reappearing on reload/back
  if (flashcardsJustCreated.value > 0) {
    const newQuery = { ...route.query }
    delete newQuery.flashcardsCreated
    await router.replace({ query: newQuery })
  }
  await loadAgenda()
})

async function loadAgenda() {
  try {
    console.warn('[DASHBOARD] loadAgenda refetch start')
    loading.value = true
    error.value = ''

    const response = await getTodayPlan()
    console.warn('[DASHBOARD] loadAgenda backend response', response)
    if (response.error) {
      error.value = response.error
      return
    }

    tasks.value = response.tasks || []
    dueReviewCards.value = response.due_review_cards || 0
    console.warn('[DASHBOARD] loadAgenda task list length', tasks.value.length)
    console.warn('[DASHBOARD] loadAgenda top pending task', tasks.value[0] || null)
    console.warn('[DASHBOARD] loadAgenda task ids', tasks.value.map((task) => ({ id: task.id, action_type: task.action_type, status: task.status, topic_id: task.topic_id, notebook_id: task.notebook_id })))
    const actionCounts = tasks.value.reduce((acc, task) => {
      const key = String(task?.action_type || '').toLowerCase() || 'unknown'
      acc[key] = (acc[key] || 0) + 1
      return acc
    }, {})
    const reviewCount = actionCounts.flashcard_review || 0
    console.warn('[FLASHCARD_PIPELINE] frontend_task_rendering', {
      totalTasks: tasks.value.length,
      reviewTasks: reviewCount,
      actionCounts,
      reviewMinutes: response.review_minutes,
      dueReviewCards: response.due_review_cards,
    })
    const notebooks = await getNotebooks('')
    const notebookList = Array.isArray(notebooks) ? notebooks.filter((nb) => !nb?.error) : []
    hasActiveStudyContent.value = notebookList.some((nb) => {
      const status = String(nb?.status || '').toLowerCase()
      return status === 'active' || status === 'chunked' || status === 'indexed'
    })
  } catch (err) {
    console.error('[DASHBOARD] loadAgenda catch', err)
    error.value = err.message || 'Failed to load daily agenda'
  } finally {
    loading.value = false
  }
}

function startTask(task) {
  // Normalize task routing from agenda values (case-insensitive)
  let routePath = '/dashboard'
  const query = {
    topicId: task.topic_id,
    notebookId: task.notebook_id,
    startPage: task.start_page,
    endPage: task.end_page,
    taskId: task.id,
  }

  const action = (task.action_type || '').toLowerCase()
  console.warn('[FLASHCARD_PIPELINE] frontend_task_start_click', {
    taskID: task.id,
    actionType: action,
    notebookID: task.notebook_id,
    topicID: task.topic_id,
  })

  if (action === 'reading') {
    routePath = '/reader'
  } else if (action === 'flashcard_review') {
    routePath = '/flashcards'
  } else if (action === 'quiz') {
    routePath = '/quiz'
  } else if (action === 'examiner' || action === 'written') {
    routePath = '/examiner'
  } else if (action === 'reread') {
    routePath = '/reader'
  } else {
    // Unknown action type: surface feedback and fall back to dashboard
    const display = task.action_type || '(empty)'
    if (import.meta.env.DEV) {
      console.warn(`Unknown task action: ${display} for task ${task.id}. Redirecting to dashboard.`)
    }
    routePath = '/dashboard'
  }

  console.warn('[DASHBOARD] startTask navigation', { routePath, query, task })
  router.push({ path: routePath, query })
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

/* Review stats in header */
.review-stats {
  text-align: right;
}

.review-count {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--primary);
  font-family: 'Manrope', sans-serif;
}

.review-hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted-text);
}

/* Flashcard success banner */
.flashcard-success-banner {
  background: color-mix(in srgb, #16a34a 10%, var(--surface-container-lowest));
  border: 1px solid color-mix(in srgb, #16a34a 25%, transparent);
}

.flashcard-success-content {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 4px;
}

.flashcard-success-icon {
  width: 40px;
  height: 40px;
  background: #16a34a;
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 700;
  flex-shrink: 0;
}

.flashcard-success-text {
  flex: 1;
}

.flashcard-success-title {
  margin: 0 0 4px;
  font-size: 16px;
  font-weight: 600;
  color: #16a34a;
}

.flashcard-success-subtitle {
  margin: 0;
  font-size: 14px;
  color: var(--on-surface);
}

.state-card {
  text-align: center;
  padding: 48px 24px;
}

.state-card h2 {
  margin: 0 0 12px;
  font-size: 32px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.error-card h2 {
  color: #b42318;
}

.victory-card h2 {
  color: var(--primary);
}

.task-list {
  display: grid;
  gap: 12px;
}

.task-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.task-type {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  background: rgba(64, 95, 171, 0.15);
  color: var(--primary);
  border-radius: 999px;
  padding: 5px 10px;
}

.task-estimate {
  font-size: 12px;
  color: var(--muted-text);
}

.task-card h3 {
  margin: 0;
  font-size: 20px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.01em;
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

.muted {
  margin: 0;
  color: var(--muted-text);
  font-size: 15px;
}

.task-meta {
  margin: 0;
  color: var(--muted-text);
  font-size: 13px;
  letter-spacing: 0.02em;
}

.primary-btn {
  margin-top: 8px;
  border: 0;
  border-radius: 12px;
  padding: 10px 24px;
  color: var(--on-primary);
  font-size: 15px;
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  transition: transform 0.14s ease, filter 0.14s ease;
  align-self: flex-start;
}

.primary-btn:active {
  transform: scale(0.95);
}

@media (max-width: 1200px) {
  .status-strip h1 {
    font-size: 42px;
  }

  .state-card h2 {
    font-size: 28px;
  }
}

@media (max-width: 960px) {
  .topbar {
    grid-template-columns: 1fr;
  }

  .status-strip {
    align-items: start;
    flex-direction: column;
  }

  .status-strip h1 {
    font-size: 38px;
  }

  .state-card h2 {
    font-size: 26px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .primary-btn {
    transition: none;
  }
}
</style>
