<template>
  <section class="page">
    <header class="topbar">
      <!-- Active Profile Dropdown Selector -->
      <div class="profile-selector-container">
        <label for="active-profile-select">Current Profile:</label>
        <select
          id="active-profile-select"
          v-model="userSettings.active_profile_id"
          class="topbar-select"
          @change="changeActiveProfile($event)"
        >
          <option value="">-- No Profile Selected --</option>
          <option v-for="p in profiles" :key="p.id" :value="p.id">
            {{ p.name }}
          </option>
        </select>
      </div>
    </header>

    <!-- Escape Hatch Status Banner -->
    <article v-if="userSettings.skip_to_reading_active" class="card info-banner">
      <div class="info-content">
        <span class="info-icon">⚡</span>
        <div class="info-text">
          <p class="info-title">"Skip to Reading" Escape Hatch Active</p>
          <p class="info-subtitle">
            Review tasks have been pushed to the background so you can focus on reading new
            chapters.
          </p>
        </div>
      </div>
    </article>

    <!-- Socratic Rescue Active Banner -->
    <article v-if="hasSocraticRescueTask" class="card rescue-banner">
      <div class="rescue-content">
        <span class="rescue-icon">🛡</span>
        <div class="rescue-text">
          <p class="rescue-title">Concept Rescue Active</p>
          <p class="rescue-subtitle">
            Your study queue is locked because you failed the quiz twice on this topic. You must
            complete the Socratic tutor rescue session to unblock your timeline.
          </p>
        </div>
      </div>
    </article>

    <!-- Flashcard creation confirmation banner -->
    <article v-if="flashcardsJustCreated" class="card flashcard-success-banner">
      <div class="flashcard-success-content">
        <span class="flashcard-success-icon">✓</span>
        <div class="flashcard-success-text">
          <p class="flashcard-success-title">Flashcards generated successfully!</p>
          <p class="flashcard-success-subtitle">
            {{ flashcardsJustCreated }} cards scheduled for spaced repetition.
          </p>
        </div>
      </div>
    </article>

    <!-- Action error banner -->
    <article v-if="actionError" class="card error-banner">
      <div class="error-content">
        <span class="error-icon">⚠</span>
        <div class="error-text">
          <p class="error-title">Error starting task</p>
          <p class="error-subtitle">{{ actionError }}</p>
        </div>
      </div>
    </article>

    <article class="status-strip">
      <div>
        <p class="eyebrow">Study Queue</p>
        <h1>Today's Tasks</h1>
      </div>

      <div class="header-actions">
        <!-- Escape Hatch Quick Toggle Button -->
        <button
          class="escape-hatch-toggle"
          :class="{ active: userSettings.skip_to_reading_active }"
          @click="toggleEscapeHatch"
        >
          {{ userSettings.skip_to_reading_active ? 'Disable Escape Hatch' : 'Skip to Reading' }}
        </button>

        <div v-if="dueReviewCards > 0" class="review-stats">
          <p class="review-count">{{ dueReviewCards }} cards due for review</p>
          <p class="review-hint">Spaced repetition strengthens long-term retention</p>
        </div>
      </div>
    </article>

    <template v-if="loading">
      <article class="card state-card">
        <h2>Loading study workspace...</h2>
        <p class="muted">Querying SQLite database & syncing with cloud.</p>
      </article>
    </template>

    <template v-else-if="error">
      <article class="card state-card error-card">
        <h2>Agenda unavailable</h2>
        <p class="muted">{{ error }}</p>
      </article>
    </template>

    <template v-else>
      <!-- Telemetry Widget for active profile — only show when a deadline is set -->
      <section v-if="activeProfilePace && activeProfilePace.has_deadline" class="telemetry-widget">
        <div class="telemetry-card card">
          <h2 class="telemetry-header">Profile Study Pacing ({{ activeProfileName }})</h2>
          <div class="telemetry-grid">
            <div class="telemetry-item">
              <div class="telemetry-title-row">
                <span class="telemetry-doc-title"
                  >Target Exam Deadline: {{ activeProfilePace.deadline }}</span
                >
                <span
                  class="telemetry-days-left"
                  :class="{ warning: activeProfilePace.days_remaining <= 3 }"
                >
                  ({{ formatDaysRemaining(activeProfilePace.days_remaining) }})
                </span>
              </div>
              <div class="telemetry-metric-row">
                <div class="telemetry-metric">
                  <span class="metric-value">{{ activeProfilePace.daily_pace }}</span>
                  <span class="metric-label">words / day</span>
                </div>
                <div class="telemetry-metric">
                  <span class="metric-value">{{
                    activeProfilePace.sessions_per_day.toFixed(1)
                  }}</span>
                  <span class="metric-label">sessions / day</span>
                </div>
                <div class="telemetry-progress-info">
                  <div class="progress-details">
                    <span
                      >Remaining words:
                      <strong>{{ activeProfilePace.remaining_words }}</strong></span
                    >
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Dashboard Layout Grid -->
      <div class="dashboard-grid">
        <!-- Main Panel (Tasks / States) -->
        <div class="dashboard-main">
          <!-- Task List (Interleaved Bookshelf Tasks) -->
          <div v-if="tasks.length > 0" class="task-list">
            <article v-for="task in tasks" :key="task.id" class="card task-card">
              <div class="task-header">
                <span class="task-type" :class="task.action_type.toLowerCase()">{{
                  formatTaskType(task.action_type)
                }}</span>
                <span
                  v-if="task.action_type !== 'flashcard_sync' && task.estimate_minutes > 0"
                  class="task-estimate"
                  >{{ task.estimate_minutes }} min</span
                >
              </div>
              <h3>{{ task.title }}</h3>
              <p class="task-meta">
                {{
                  task.meta
                    ? task.meta
                    : task.start_page !== undefined &&
                        task.start_page !== null &&
                        task.end_page !== undefined &&
                        task.end_page !== null
                      ? 'Pages ' + task.start_page + '-' + task.end_page
                      : 'Pages N/A'
                }}
              </p>
              <button
                type="button"
                class="primary-btn"
                :class="{ 'sync-btn': task.action_type === 'flashcard_sync' }"
                :aria-label="'Start task ' + (task.title || task.id)"
                :disabled="task.action_type === 'flashcard_sync' && isSyncing"
                @click="startTask(task)"
              >
                <span v-if="task.action_type === 'flashcard_sync' && isSyncing">Syncing...</span>
                <span v-else-if="task.action_type === 'flashcard_sync'">Sync</span>
                <span v-else>Start</span>
              </button>
            </article>
          </div>

          <div v-else-if="hasActiveStudyContent" class="card state-card victory-card">
            <h2>Tasks Complete!</h2>
            <p class="muted">You've completed all tasks for today. Great work!</p>
          </div>

          <div v-else class="card onboarding-card">
            <div class="onboarding-content">
              <h2>Your study queue is empty</h2>
              <p class="onboarding-desc">
                Upload a PDF textbook and the app builds a study queue of reading tasks, quizzes, and
                flashcards for you.
              </p>
              <div class="onboarding-steps">
                <div class="onboarding-step">
                  <span class="step-number">1</span>
                  <span class="step-label">Upload a PDF</span>
                </div>
                <div class="onboarding-divider"></div>
                <div class="onboarding-step">
                  <span class="step-number">2</span>
                  <span class="step-label">Read chapters</span>
                </div>
                <div class="onboarding-divider"></div>
                <div class="onboarding-step">
                  <span class="step-number">3</span>
                  <span class="step-label">Quiz & review</span>
                </div>
              </div>
              <button class="primary-btn onboarding-cta" @click="goToNotebooks">
                Upload your first textbook
              </button>
            </div>
          </div>
        </div>

        <!-- Sidebar Panel (Forecast Chart) -->
        <div v-if="timelineData && timelineData.length > 0" class="dashboard-sidebar">
          <section class="forecast-widget">
            <div class="forecast-card card">
              <div class="forecast-header-row">
                <div>
                  <h2 class="forecast-header">Flashcard Review Forecast</h2>
                  <p class="forecast-subtitle">Review load by date vs daily session limit</p>
                </div>
                <div class="forecast-legend">
                  <span class="legend-item"><span class="legend-dot due-dot"></span>Due Cards</span>
                  <span class="legend-item">
                    <span class="legend-line" :class="{ active: isThresholdExceeded }"></span>
                    Limit ({{ maxFlashcardsLimit }})
                  </span>
                </div>
              </div>

              <div class="chart-container">
                <svg class="forecast-chart" viewBox="0 0 400 300" preserveAspectRatio="xMidYMid meet">
                  <!-- Definitions for Gradients -->
                  <defs>
                    <linearGradient id="chartGrad" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stop-color="var(--primary)" stop-opacity="0.25"/>
                      <stop offset="100%" stop-color="var(--primary)" stop-opacity="0.0"/>
                    </linearGradient>
                  </defs>

                  <!-- Grid Lines -->
                  <line x1="0" y1="75" x2="400" y2="75" class="chart-grid-line" />
                  <line x1="0" y1="150" x2="400" y2="150" class="chart-grid-line" />
                  <line x1="0" y1="225" x2="400" y2="225" class="chart-grid-line" />

                  <!-- Horizontal Limit Line -->
                  <line
                    x1="0"
                    :y1="limitLineY"
                    x2="400"
                    :y2="limitLineY"
                    class="limit-line"
                    :class="{ active: isThresholdExceeded }"
                  />

                  <!-- Shading Area under the curve -->
                  <path :d="areaPathData" fill="url(#chartGrad)" />

                  <!-- Main Line Path -->
                  <path :d="linePathData" fill="none" stroke="var(--primary)" stroke-width="2.5" stroke-linecap="round" />

                  <!-- Data Points (interactive dots) -->
                  <g v-for="(pt, idx) in chartPoints" :key="idx">
                    <circle
                      :cx="pt.x"
                      :cy="pt.y"
                      r="5"
                      class="chart-dot"
                      :class="{ 'exceeds-limit': pt.exceeds }"
                      @mouseenter="hoveredPoint = pt"
                      @mouseleave="hoveredPoint = null"
                    />
                  </g>
                </svg>

                <!-- Tooltip overlay -->
                <div
                  v-if="hoveredPoint"
                  class="chart-tooltip"
                  :style="{ left: hoveredPoint.tooltipX + '%', top: hoveredPoint.percentY + '%' }"
                >
                  <div class="tooltip-date">{{ hoveredPoint.dayLabel }} ({{ hoveredPoint.date }})</div>
                  <div class="tooltip-value">
                    <strong>{{ hoveredPoint.count }}</strong> due cards
                    <span v-if="hoveredPoint.exceeds" class="tooltip-warn">⚠️ Overload</span>
                  </div>
                </div>
              </div>

              <!-- X Axis Labels -->
              <div class="chart-x-axis">
                <div v-for="(pt, idx) in chartPoints" :key="idx" class="x-label-container" :style="{ left: pt.percentX + '%' }">
                  <span class="x-label">{{ pt.dayLabel }}</span>
                  <span class="x-sublabel" :class="{ exceeds: pt.exceeds }">{{ pt.count }}</span>
                </div>
              </div>
            </div>
          </section>
        </div>
      </div>
    </template>

    <!-- Dev Mode Bypass Panel -->
    <div v-if="appEnv === 'dev'" class="dev-panel card">
      <header class="dev-header">
        <h4>🛠 Dev Tools</h4>
        <span class="dev-badge">APP_ENV = dev</span>
      </header>
      <div class="dev-actions">
        <button type="button" class="dev-btn" :disabled="forcingRescue" @click="forceRescueState">
          {{ forcingRescue ? 'Forcing...' : 'Force Socratic Rescue' }}
        </button>
        <button type="button" class="dev-btn" :disabled="forcingSync" @click="forceSyncTask">
          {{ forcingSync ? 'Forcing...' : 'Force Flashcard Sync' }}
        </button>
      </div>
      <p v-if="devMessage" class="dev-message">{{ devMessage }}</p>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  getTodayPlan,
  getProfiles,
  getUserSettings,
  updateUserSettings,
  getProfileDailyPace,
  triggerCloudSync,
  getAppEnv,
  devForceSocraticRescue,
  devForceFlashcardSync,
  getNotebooks,
  getFlashcardDueTimeline,
} from '../services/appApi'

const router = useRouter()
const route = useRoute()

const loading = ref(true)
const error = ref('')
const actionError = ref('')
const tasks = ref([])
const hasActiveStudyContent = ref(false)
const dueReviewCards = ref(0)

const profiles = ref([])
const userSettings = ref({
  max_flashcards_per_session: 30,
  study_start_time: '17:00',
  study_end_time: '18:00',
  reminders_enabled: true,
  active_profile_id: '',
  skip_to_reading_active: false,
  cloud_sync_url: '',
  cloud_api_token: '',
  theme: '',
  rag_enabled: false,
  rag_notebook_chapter: true,
  rag_entire_notebook: true,
  rag_queue_study: true,
  default_remedial_strategy: 'CLASSIC',
})
const activeProfilePace = ref(null)
const lastPersistedProfile = ref('')

const timelineData = ref([])
const hoveredPoint = ref(null)

const maxFlashcardsLimit = computed(() => {
  return userSettings.value.max_flashcards_per_session || 30
})

const isThresholdExceeded = computed(() => {
  return timelineData.value.some((d) => d.card_count > maxFlashcardsLimit.value)
})

const chartPoints = computed(() => {
  if (!timelineData.value || timelineData.value.length === 0) return []

  const counts = timelineData.value.map((d) => d.card_count)
  const rawMax = Math.max(...counts, maxFlashcardsLimit.value, 10)
  const yAxisMax = rawMax * 1.25

  return timelineData.value.map((d, i) => {
    // scale to 400x300 viewport
    const x = 30 + (i / (timelineData.value.length - 1)) * 340
    const y = 250 - (d.card_count / yAxisMax) * 200
    const exceeds = d.card_count > maxFlashcardsLimit.value
    return {
      x,
      y,
      percentX: 7 + (i / (timelineData.value.length - 1)) * 86,
      tooltipX: 7 + (i / (timelineData.value.length - 1)) * 86,
      tooltipY: y,
      percentY: (y / 300) * 100,
      dayLabel: d.day_label,
      date: d.date,
      count: d.card_count,
      exceeds,
    }
  })
})

const linePathData = computed(() => {
  const pts = chartPoints.value
  if (pts.length === 0) return ''
  return pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ')
})

const areaPathData = computed(() => {
  const pts = chartPoints.value
  if (pts.length === 0) return ''
  const linePath = linePathData.value
  return `${linePath} L ${pts[pts.length - 1].x} 270 L ${pts[0].x} 270 Z`
})

const limitLineY = computed(() => {
  if (!timelineData.value || timelineData.value.length === 0) return 150
  const counts = timelineData.value.map((d) => d.card_count)
  const rawMax = Math.max(...counts, maxFlashcardsLimit.value, 10)
  const yAxisMax = rawMax * 1.25
  return 250 - (maxFlashcardsLimit.value / yAxisMax) * 200
})

const flashcardsJustCreated = computed(() => {
  const created = Number.parseInt(route.query.flashcardsCreated, 10)
  return isNaN(created) || created <= 0 ? 0 : created
})

const activeProfileName = computed(() => {
  const p = profiles.value.find((pr) => pr.id === userSettings.value.active_profile_id)
  return p ? p.name : 'Unknown'
})

const appEnv = ref('')
const forcingRescue = ref(false)
const forcingSync = ref(false)
const devMessage = ref('')
const isSyncing = ref(false)

const hasSocraticRescueTask = computed(() => {
  return tasks.value.some((t) => t.action_type === 'socratic_remedial')
})

onMounted(async () => {
  try {
    const envRes = await getAppEnv()
    if (envRes && envRes.env) {
      appEnv.value = envRes.env
    }
  } catch (err) {
    console.error('Failed to get APP_ENV:', err)
  }
  if (flashcardsJustCreated.value > 0) {
    const newQuery = { ...route.query }
    delete newQuery.flashcardsCreated
    await router.replace({ query: newQuery })
  }
  await loadAgenda()
})

async function loadAgenda() {
  try {
    loading.value = true
    error.value = ''
    actionError.value = ''

    // 1. Fetch settings and profiles — abort on failure so dependent steps
    //    don't run against stale/default data.
    const settingsRes = await getUserSettings()
    if (settingsRes.error) {
      error.value = settingsRes.error
      return
    }
    userSettings.value = settingsRes
    lastPersistedProfile.value = settingsRes.active_profile_id || ''

    const profilesRes = await getProfiles()
    if (profilesRes.error) {
      error.value = profilesRes.error
      return
    }
    profiles.value = profilesRes.profiles || []

    // 2. Fetch today's plan
    const response = await getTodayPlan()
    if (response.error) {
      error.value = response.error
      return
    }

    tasks.value = response.tasks || []
    dueReviewCards.value = response.due_review_cards || 0

    // 3. Determine if there is any active study content (drives the empty state)
    // Uses active_notebook_count from backend so "Tasks Complete!" branch
    // is reachable when users have active textbooks but zero remaining tasks.
    const activeNotebookCount = response.active_notebook_count || 0
    hasActiveStudyContent.value = response.tasks.length > 0 || activeNotebookCount > 0

    // 4. Load pace for active profile
    // Guard: only request pacing if the active_profile_id resolves to a known profile.
    // An orphaned ID (deleted profile still persisted in settings) would hit the backend
    // and return { error: "profile not found" }; we skip the call entirely instead.
    const knownProfile = profiles.value.find((pr) => pr.id === userSettings.value.active_profile_id)
    if (userSettings.value.active_profile_id && knownProfile) {
      try {
        const pace = await getProfileDailyPace(userSettings.value.active_profile_id)
        if (!pace.error) {
          activeProfilePace.value = pace
        } else {
          // API returned a business-logic error; clear stale data so the widget
          // shows nothing rather than outdated metrics from a previous request.
          activeProfilePace.value = null
        }
      } catch (err) {
        // Network / runtime failure: clear stale data to avoid misleading display.
        console.error('Failed to get profile daily pace', err)
        activeProfilePace.value = null
      }
    } else {
      activeProfilePace.value = null
    }

    // 5. Fetch flashcard review forecast timeline
    try {
      const timelineRes = await getFlashcardDueTimeline()
      if (timelineRes && !timelineRes.error) {
        timelineData.value = timelineRes.timeline || []
      } else {
        timelineData.value = []
      }
    } catch (err) {
      console.error('Failed to get flashcard due timeline', err)
      timelineData.value = []
    }
  } catch (err) {
    error.value = err.message || 'Failed to load tasks'
  } finally {
    loading.value = false
  }
}

async function changeActiveProfile(event) {
  const newProfileID = event?.target?.value ?? ''
  const oldProfileID = lastPersistedProfile.value
  try {
    loading.value = true
    const res = await updateUserSettings(
      userSettings.value.max_flashcards_per_session,
      userSettings.value.study_start_time,
      userSettings.value.study_end_time,
      userSettings.value.reminders_enabled,
      newProfileID,
      userSettings.value.skip_to_reading_active,
      userSettings.value.cloud_sync_url,
      userSettings.value.cloud_api_token,
      userSettings.value.theme || '',
      userSettings.value.rag_enabled || false,
      userSettings.value.rag_notebook_chapter,
      userSettings.value.rag_entire_notebook,
      userSettings.value.rag_queue_study,
      userSettings.value.default_remedial_strategy
    )
    if (res && res.error) {
      userSettings.value.active_profile_id = oldProfileID
      actionError.value = res.error
      return
    }
    lastPersistedProfile.value = newProfileID
    window.dispatchEvent(new CustomEvent('settings-updated'))
    await loadAgenda()
  } catch (err) {
    userSettings.value.active_profile_id = oldProfileID
    actionError.value = 'Failed to switch active profile'
  } finally {
    loading.value = false
  }
}

async function toggleEscapeHatch() {
  const previousSkipToReading = userSettings.value.skip_to_reading_active
  try {
    loading.value = true
    userSettings.value.skip_to_reading_active = !userSettings.value.skip_to_reading_active
    const res = await updateUserSettings(
      userSettings.value.max_flashcards_per_session,
      userSettings.value.study_start_time,
      userSettings.value.study_end_time,
      userSettings.value.reminders_enabled,
      userSettings.value.active_profile_id,
      userSettings.value.skip_to_reading_active,
      userSettings.value.cloud_sync_url,
      userSettings.value.cloud_api_token,
      userSettings.value.theme || '',
      userSettings.value.rag_enabled || false,
      userSettings.value.rag_notebook_chapter,
      userSettings.value.rag_entire_notebook,
      userSettings.value.rag_queue_study,
      userSettings.value.default_remedial_strategy
    )
    if (res && res.error) {
      userSettings.value.skip_to_reading_active = previousSkipToReading
      actionError.value = res.error
      return
    }
    window.dispatchEvent(new CustomEvent('settings-updated'))
    await loadAgenda()
  } catch (err) {
    userSettings.value.skip_to_reading_active = previousSkipToReading
    actionError.value = 'Failed to toggle escape hatch'
  } finally {
    loading.value = false
  }
}

function formatDaysRemaining(days) {
  if (days === 0) return 'today!'
  if (days < 0) return 'passed'
  return `${days} days left`
}

async function runFlashcardSyncInline(task) {
  try {
    isSyncing.value = true
    actionError.value = ''
    const res = await triggerCloudSync()
    if (res && res.error) {
      actionError.value = `Cloud Sync Failed: ${res.error}. Please check your connection.`
    } else {
      await loadAgenda()
    }
  } catch (err) {
    actionError.value = `Cloud Sync Error: ${err.message || err}`
  } finally {
    isSyncing.value = false
  }
}

function formatTaskType(type) {
  const t = (type || '').toLowerCase()
  if (t === 'reading') return 'Reading'
  if (t === 'flashcard_review') return 'Review'
  if (t === 'quiz') return 'Quiz'
  if (t === 'examiner') return 'Examiner'
  if (t === 'reread') return 'Reread'
  if (t === 'socratic_remedial') return 'Concept Rescue'
  if (t === 'flashcard_sync') return 'Cloud Sync'
  return type
}

async function forceRescueState() {
  forcingRescue.value = true
  devMessage.value = ''
  try {
    const nbsRes = await getNotebooks()
    const notebooks = Array.isArray(nbsRes) ? nbsRes.filter((n) => !n.error) : []
    if (notebooks.length === 0) {
      devMessage.value = 'No notebooks found. Please upload a notebook first.'
      forcingRescue.value = false
      return
    }

    const validNb = notebooks.find((n) => n.topic_id)
    if (!validNb) {
      devMessage.value = 'No notebook with a linked topic found. Confirm syllabus first.'
      forcingRescue.value = false
      return
    }

    const res = await devForceSocraticRescue(validNb.id, validNb.topic_id)
    if (res && res.error) {
      devMessage.value = 'Error: ' + res.error
    } else {
      devMessage.value = 'Successfully forced Socratic Rescue state!'
      await loadAgenda()
    }
  } catch (err) {
    devMessage.value = 'Error: ' + err.message
  } finally {
    forcingRescue.value = false
  }
}

async function forceSyncTask() {
  forcingSync.value = true
  devMessage.value = ''
  try {
    const nbsRes = await getNotebooks()
    const notebooks = Array.isArray(nbsRes) ? nbsRes.filter((n) => !n.error) : []
    let nbId = 'system_default'
    if (notebooks.length > 0) {
      nbId = notebooks[0].id
    }
    const res = await devForceFlashcardSync(nbId)
    if (res && res.error) {
      devMessage.value = 'Error: ' + res.error
    } else {
      devMessage.value = 'Successfully forced Flashcard Sync task!'
      await loadAgenda()
    }
  } catch (err) {
    devMessage.value = 'Error: ' + err.message
  } finally {
    forcingSync.value = false
  }
}

function goToNotebooks() {
  router.push('/notebooks')
}

function startTask(task) {
  let routePath = '/dashboard'
  const query = {
    topicId: task.topic_id,
    notebookId: task.notebook_id,
    startPage: task.start_page,
    endPage: task.end_page,
    taskId: task.id,
  }

  const action = (task.action_type || '').toLowerCase()
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
  } else if (action === 'socratic_remedial') {
    routePath = '/socratic-rescue'
  } else if (action === 'flashcard_sync') {
    runFlashcardSyncInline(task)
    return
  } else {
    actionError.value = `Unknown task action type: ${task.action_type}`
    return
  }

  router.push({ path: routePath, query })
}
</script>

<style scoped>
.page {
  display: grid;
  gap: 16px;
  font-family: 'Inter', sans-serif;
}

.topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.profile-selector-container {
  display: flex;
  align-items: center;
  gap: 8px;
}

.profile-selector-container label {
  font-size: 13px;
  font-weight: 600;
  color: var(--muted-text);
}

.topbar-select {
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 10px 14px;
  font-size: 14px;
  font-family: inherit;
  font-weight: 600;
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

.eyebrow {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-weight: 700;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 20px;
}

.escape-hatch-toggle {
  background: var(--surface-container-low);
  color: var(--on-surface);
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  padding: 10px 18px;
  font-weight: 700;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}

.escape-hatch-toggle.active {
  background: linear-gradient(135deg, #e67e22, #d35400);
  border-color: #d35400;
  color: white;
  box-shadow: 0 0 12px rgba(230, 126, 34, 0.3);
}

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

/* Banners */
.info-banner {
  background: rgba(230, 126, 34, 0.1);
  border: 1px solid rgba(230, 126, 34, 0.2);
}

.info-content {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px;
}

.info-icon {
  width: 40px;
  height: 40px;
  background: #e67e22;
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 700;
  flex-shrink: 0;
}

.info-title {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 700;
  color: #e67e22;
}

.info-subtitle {
  margin: 0;
  font-size: 13px;
}

.card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 16px;
}

.state-card {
  padding: 40px;
  text-align: center;
}

.state-card h2 {
  margin: 0 0 8px;
  font-size: 24px;
}

.muted {
  color: var(--muted-text);
}

/* Onboarding empty state */
.onboarding-card {
  padding: 48px 40px;
  text-align: center;
}

.onboarding-content {
  max-width: 420px;
  margin: 0 auto;
}

.onboarding-card h2 {
  margin: 0 0 12px;
  font-size: 26px;
  font-family: 'Manrope', sans-serif;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.onboarding-desc {
  margin: 0 0 32px;
  font-size: 15px;
  color: var(--muted-text);
  line-height: 1.6;
}

.onboarding-steps {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0;
  margin-bottom: 36px;
}

.onboarding-step {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.step-number {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: rgba(108, 92, 231, 0.1);
  color: var(--primary);
  font-weight: 700;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.step-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--on-surface);
}

.onboarding-divider {
  width: 40px;
  height: 1px;
  background: var(--outline-variant);
  margin: 0 12px;
  margin-bottom: 20px;
}

.onboarding-cta {
  padding: 14px 32px;
  font-size: 15px;
  border-radius: 14px;
}

/* Telemetry styles */
.telemetry-widget {
  margin-bottom: 8px;
}

.telemetry-card {
  padding: 24px;
}

.telemetry-header {
  margin: 0 0 16px;
  font-size: 18px;
  font-weight: 700;
}

.telemetry-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.telemetry-item {
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  padding: 16px;
  background: var(--surface-container-low);
}

.telemetry-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 12px;
}

.telemetry-doc-icon {
  font-size: 16px;
}

.telemetry-doc-title {
  flex: 1;
}

.telemetry-days-left {
  font-size: 12px;
  color: var(--muted-text);
}

.telemetry-days-left.warning {
  color: #eb5e55;
  font-weight: 700;
}

.telemetry-metric-row {
  display: flex;
  align-items: center;
  gap: 24px;
}

.telemetry-metric {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.metric-value {
  font-size: 28px;
  font-family: 'Manrope', sans-serif;
  font-weight: 800;
  color: var(--primary);
  line-height: 1;
}

.metric-label {
  font-size: 12px;
  color: var(--muted-text);
  font-weight: 600;
}

.telemetry-progress-info {
  flex: 1;
  text-align: right;
  font-size: 13px;
  color: var(--muted-text);
}

/* Task cards */
.task-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.task-card {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  transition:
    transform 0.2s,
    border-color 0.2s;
}

.task-card:hover {
  transform: translateY(-2px);
  border-color: var(--primary);
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.task-type {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--primary);
  background: rgba(108, 92, 231, 0.1);
  padding: 4px 8px;
  border-radius: 6px;
}

.task-estimate {
  font-size: 12px;
  color: var(--muted-text);
}

.task-card h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  line-height: 1.3;
}

.task-meta {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
  flex: 1;
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

.flashcard-success-banner {
  background: rgba(46, 204, 113, 0.1);
  border-color: rgba(46, 204, 113, 0.2);
}

.flashcard-success-icon {
  background: #2ecc71;
  color: white;
}

.flashcard-success-title {
  color: #2ecc71;
}

.error-card h2 {
  color: #eb5e55;
}

.rescue-banner {
  background: rgba(211, 84, 0, 0.1);
  border: 1px solid rgba(211, 84, 0, 0.2);
}

.rescue-content {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px;
}

.rescue-icon {
  width: 40px;
  height: 40px;
  background: #d35400;
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 700;
  flex-shrink: 0;
}

.rescue-title {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 700;
  color: #d35400;
}

.rescue-subtitle {
  margin: 0;
  font-size: 13px;
}

.task-type.flashcard_sync {
  color: #c0392b;
  background: rgba(192, 41, 43, 0.1);
}

.task-type.socratic_remedial {
  color: #d35400;
  background: rgba(211, 84, 0, 0.1);
}

.primary-btn.sync-btn {
  background: linear-gradient(135deg, #c0392b, #e74c3c);
  box-shadow: 0 4px 10px rgba(192, 41, 43, 0.15);
}

.dev-panel {
  margin-top: 32px;
  padding: 20px;
  border-color: #f1c40f;
  background: rgba(241, 196, 15, 0.05);
}

.dev-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.dev-header h4 {
  margin: 0;
  font-size: 16px;
}

.dev-badge {
  font-size: 11px;
  font-weight: 700;
  background: #f1c40f;
  color: #2c3e50;
  padding: 2px 8px;
  border-radius: 6px;
}

.dev-actions {
  display: flex;
  gap: 12px;
}

.dev-btn {
  background: #34495e;
  color: white;
  border: none;
  border-radius: 8px;
  padding: 8px 16px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.2s;
}

.dev-btn:hover {
  opacity: 0.9;
}

.dev-message {
  margin: 10px 0 0;
  font-size: 12px;
  font-weight: 600;
  color: #16a085;
}

/* Dashboard Two-Column Layout Grid */
.dashboard-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 24px;
}

@media (min-width: 1024px) {
  .dashboard-grid {
    grid-template-columns: 1fr 380px;
    align-items: start;
  }
}

.dashboard-main {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dashboard-sidebar {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* Forecast Widget Styles */
.forecast-widget {
  margin-bottom: 8px;
}

.forecast-card {
  padding: 24px;
  position: relative;
  display: flex;
  flex-direction: column;
  aspect-ratio: 1 / 1.05;
  min-height: 380px;
}

.forecast-header-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
}

.forecast-header {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: var(--on-surface);
}

.forecast-subtitle {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted-text);
}

.forecast-legend {
  display: flex;
  gap: 12px;
  align-items: center;
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-text);
  margin-top: 4px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.legend-dot.due-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--primary);
}

.legend-line {
  width: 16px;
  height: 2px;
  background: var(--muted-text);
  opacity: 0.6;
}

.legend-line.active {
  background: #ff4d4f;
  opacity: 1;
  box-shadow: 0 0 4px rgba(255, 77, 79, 0.4);
}

.chart-container {
  position: relative;
  width: 100%;
  flex: 1;
  min-height: 180px;
  background: var(--surface-container-lowest);
  border-radius: 12px;
}

.forecast-chart {
  width: 100%;
  height: 100%;
  display: block;
}

.chart-grid-line {
  stroke: var(--outline-variant);
  stroke-width: 1px;
  stroke-opacity: 0.6;
  stroke-dasharray: 2 4;
}

.limit-line {
  stroke: var(--muted-text);
  stroke-width: 1.5px;
  stroke-dasharray: 4 4;
  opacity: 0.5;
  transition: all 0.3s ease;
}

.limit-line.active {
  stroke: #ff4d4f;
  stroke-width: 2px;
  stroke-dasharray: none;
  opacity: 1;
  filter: drop-shadow(0 0 2px rgba(255, 77, 79, 0.6));
}

.chart-dot {
  fill: var(--surface-container-lowest);
  stroke: var(--primary);
  stroke-width: 2.5px;
  cursor: pointer;
  transition: r 0.15s ease, fill 0.15s ease;
}

.chart-dot:hover {
  r: 7px;
  fill: var(--primary);
}

.chart-dot.exceeds-limit {
  stroke: #ff4d4f;
}

.chart-dot.exceeds-limit:hover {
  fill: #ff4d4f;
}

/* Tooltip */
.chart-tooltip {
  position: absolute;
  transform: translate(-50%, -100%);
  margin-top: -12px;
  background: var(--surface-bright);
  border: 1px solid var(--outline-variant);
  border-radius: 8px;
  padding: 8px 12px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
  backdrop-filter: blur(8px);
  z-index: 10;
  pointer-events: none;
  min-width: 120px;
  transition: left 0.1s ease, top 0.1s ease;
}

.tooltip-date {
  font-size: 11px;
  color: var(--muted-text);
  font-weight: 500;
  margin-bottom: 2px;
}

.tooltip-value {
  font-size: 13px;
  color: var(--on-surface);
}

.tooltip-warn {
  display: inline-block;
  margin-left: 4px;
  font-size: 10px;
  color: #ff4d4f;
  font-weight: 700;
}

/* X Axis */
.chart-x-axis {
  position: relative;
  height: 36px;
  margin-top: 4px;
  width: 100%;
}

.x-label-container {
  position: absolute;
  transform: translateX(-50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.x-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-text);
}

.x-sublabel {
  font-size: 12px;
  font-weight: 700;
  color: var(--on-surface);
  margin-top: 2px;
}

.x-sublabel.exceeds {
  color: #ff4d4f;
}
</style>
