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

    <!-- Status Banners -->
    <StatusBanner
      v-if="userSettings.skip_to_reading_active"
      variant="info"
      icon="⚡"
      title='"Skip to Reading" Escape Hatch Active'
      subtitle="Review tasks have been pushed to the background so you can focus on reading new chapters."
    />
    <StatusBanner
      v-if="hasSocraticRescueTask"
      variant="rescue"
      icon="🛡"
      title="Concept Rescue Active"
      subtitle="Your study queue is locked because you failed the quiz twice on this topic. You must complete the Socratic tutor rescue session to unblock your timeline."
    />
    <StatusBanner
      v-if="flashcardsJustCreated"
      variant="success"
      icon="✓"
      :title="'Flashcards generated successfully!'"
      :subtitle="flashcardsJustCreated + ' cards scheduled for spaced repetition.'"
    />
    <StatusBanner
      v-if="actionError"
      variant="error"
      icon="⚠"
      title="Error starting task"
      :subtitle="actionError"
    />

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
      <!-- Dashboard Layout Grid -->
      <div class="dashboard-grid">
        <!-- Main Panel (Tasks / States) -->
        <div class="dashboard-main">
          <!-- Task List & Custom Hero Review Card -->
          <div
            v-if="tasks.length > 0"
            class="tasks-container"
            style="display: flex; flex-direction: column; gap: 16px"
          >
            <!-- High-Priority Today's Reviews Card -->
            <ReviewHeroCard
              v-if="reviewTask"
              :task="reviewTask"
              :due-review-cards="dueReviewCards"
              :total-due-review-cards="totalDueReviewCards"
              @start="startTask"
            />

            <!-- Task List (Other Interleaved Tasks) -->
            <div v-if="nonReviewTasks.length > 0" class="task-list">
              <TaskCard
                v-for="task in nonReviewTasks"
                :key="task.id"
                :task="task"
                :is-syncing="isSyncing"
                @start="startTask"
              />
            </div>
          </div>

          <div v-else-if="hasActiveStudyContent" class="card state-card victory-card">
            <h2>Tasks Complete!</h2>
            <p class="muted">You've completed all tasks for today. Great work!</p>
          </div>

          <OnboardingCard v-else @go-to-notebooks="goToNotebooks" />

          <!-- Secondary Telemetry Widget -->
          <TelemetryWidget :pace="activeProfilePace" :profile-name="activeProfileName" />
        </div>

        <!-- Sidebar Panel (Streak Calendar & Forecast Chart) -->
        <div class="dashboard-sidebar">
          <StreakCalendar
            :streak-state="streakState"
            :streak-error="streakError"
            :calendar-days="calendarDays"
            :month-label="currentMonthLabel"
          />
          <ForecastChart :timeline-data="timelineData" :max-flashcards-limit="maxFlashcardsLimit" />
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
          {{ forcingSync ? 'Forcing...' : 'Force Flashcard Generate' }}
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
  getDashboardOverview,
  updateUserSettings,
  getProfileDailyPace,
  retryFlashcardGeneration,
  getAppEnv,
  devForceSocraticRescue,
  devForceFlashcardGenerate,
  getNotebooks,
  getFlashcardDueTimeline,
} from '../services/appApi'
import { buildCalendarDays, MONTH_NAMES } from '../utils/dateFormat'

import StatusBanner from '../components/StatusBanner.vue'
import ReviewHeroCard from '../components/ReviewHeroCard.vue'
import TaskCard from '../components/TaskCard.vue'
import OnboardingCard from '../components/OnboardingCard.vue'
import TelemetryWidget from '../components/TelemetryWidget.vue'
import StreakCalendar from '../components/StreakCalendar.vue'
import ForecastChart from '../components/ForecastChart.vue'

const router = useRouter()
const route = useRoute()

// --- Reactive State ---
const loading = ref(true)
const error = ref('')
const actionError = ref('')
const tasks = ref([])
const hasActiveStudyContent = ref(false)
const dueReviewCards = ref(0)
const totalDueReviewCards = ref(0)

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

const streakState = ref({
  current_streak: 0,
  longest_streak: 0,
  active_dates: [],
  today_completed: false,
})
const streakError = ref('')

const appEnv = ref('')
const forcingRescue = ref(false)
const forcingSync = ref(false)
const devMessage = ref('')
const isSyncing = ref(false)

// --- Calendar computeds ---
const currentDate = new Date()
const currentYear = currentDate.getFullYear()
const currentMonth = currentDate.getMonth()

const currentMonthLabel = computed(() => {
  return `${MONTH_NAMES[currentMonth]} ${currentYear}`
})

const calendarDays = computed(() => {
  return buildCalendarDays(currentYear, currentMonth, streakState.value?.active_dates || [])
})

// --- Task computeds ---
const maxFlashcardsLimit = computed(() => {
  return userSettings.value.max_flashcards_per_session || 30
})

const reviewTask = computed(() => {
  return tasks.value.find((t) => t.id === 'task-review-daily')
})

const nonReviewTasks = computed(() => {
  return tasks.value.filter((t) => t.id !== 'task-review-daily')
})

const flashcardsJustCreated = computed(() => {
  const created = Number.parseInt(route.query.flashcardsCreated, 10)
  return isNaN(created) || created <= 0 ? 0 : created
})

const activeProfileName = computed(() => {
  const p = profiles.value.find((pr) => pr.id === userSettings.value.active_profile_id)
  return p ? p.name : 'Unknown'
})

const hasSocraticRescueTask = computed(() => {
  return tasks.value.some((t) => t.action_type === 'socratic_remedial')
})

// --- Lifecycle ---
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

// --- Data Fetching ---
async function loadAgenda() {
  try {
    loading.value = true
    error.value = ''
    actionError.value = ''

    const tzOffset = new Date().getTimezoneOffset()
    const overview = await getDashboardOverview(tzOffset)

    if (!applyDashboardOverview(overview)) {
      return
    }

    await loadActiveProfilePace()
    await loadFlashcardTimeline(tzOffset)
  } catch (err) {
    error.value = err.message || 'Failed to load tasks'
  } finally {
    loading.value = false
  }
}

function applyDashboardOverview(overview) {
  if (overview.settings?.error) {
    error.value = overview.settings.error
    return false
  }
  if (overview.settings) {
    userSettings.value = overview.settings
    lastPersistedProfile.value = overview.settings.active_profile_id || ''
  }

  if (overview.profiles?.error) {
    error.value = overview.profiles.error
    return false
  }
  if (overview.profiles) {
    profiles.value = overview.profiles.profiles || []
  }

  if (overview.today_plan?.error) {
    error.value = overview.today_plan.error
    return false
  }
  if (overview.today_plan) {
    const response = overview.today_plan
    tasks.value = response.tasks || []
    dueReviewCards.value = response.due_review_cards || 0
    totalDueReviewCards.value = response.total_due_review_cards || 0
    const activeNotebookCount = response.active_notebook_count || 0
    hasActiveStudyContent.value = tasks.value.length > 0 || activeNotebookCount > 0
  }

  if (overview.streak_state?.error) {
    streakError.value = overview.streak_state.error
  } else if (overview.streak_state) {
    streakState.value = overview.streak_state
    streakError.value = ''
  }

  return true
}

async function loadActiveProfilePace() {
  const profileId = userSettings.value.active_profile_id
  const knownProfile = profiles.value.find((pr) => pr.id === profileId)
  if (!profileId || !knownProfile) {
    activeProfilePace.value = null
    return
  }

  try {
    const pace = await getProfileDailyPace(profileId)
    activeProfilePace.value = pace.error ? null : pace
  } catch (err) {
    console.error('Failed to get profile daily pace', err)
    activeProfilePace.value = null
  }
}

async function loadFlashcardTimeline(tzOffset) {
  try {
    const timelineRes = await getFlashcardDueTimeline(tzOffset)
    timelineData.value = timelineRes && !timelineRes.error ? (timelineRes.timeline || []) : []
  } catch (err) {
    console.error('Failed to get flashcard due timeline', err)
    timelineData.value = []
  }
}

// --- User Actions ---
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
      userSettings.value.default_remedial_strategy,
      userSettings.value.classroom_code || '',
      userSettings.value.analytics_enabled || false,
      userSettings.value.anonymous_user_id || ''
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
      userSettings.value.default_remedial_strategy,
      userSettings.value.classroom_code || '',
      userSettings.value.analytics_enabled || false,
      userSettings.value.anonymous_user_id || ''
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

async function runFlashcardSyncInline(task) {
  try {
    isSyncing.value = true
    actionError.value = ''
    const res = await retryFlashcardGeneration(task.id)
    if (res && res.error) {
      actionError.value = `Flashcard Generation Failed: ${res.error}. Please check your connection.`
    } else {
      await loadAgenda()
    }
  } catch (err) {
    actionError.value = `Flashcard Generation Error: ${err.message || err}`
  } finally {
    isSyncing.value = false
  }
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
  } else if (action === 'quiz' || action === 'milestone_exam') {
    routePath = '/quiz'
  } else if (action === 'examiner' || action === 'written') {
    routePath = '/examiner'
  } else if (action === 'reread') {
    routePath = '/reader'
  } else if (action === 'socratic_remedial') {
    routePath = '/socratic-rescue'
  } else if (action === 'flashcard_generate') {
    runFlashcardSyncInline(task)
    return
  } else {
    actionError.value = `Unknown task action type: ${task.action_type}`
    return
  }

  router.push({ path: routePath, query })
}

function goToNotebooks() {
  router.push('/notebooks')
}

// --- Dev Mode ---
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
    const res = await devForceFlashcardGenerate(nbId)
    if (res && res.error) {
      devMessage.value = 'Error: ' + res.error
    } else {
      devMessage.value = 'Successfully forced Flashcard Generate task!'
      await loadAgenda()
    }
  } catch (err) {
    devMessage.value = 'Error: ' + err.message
  } finally {
    forcingSync.value = false
  }
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
  appearance: none;
  -webkit-appearance: none;
  -moz-appearance: none;
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 10px 36px 10px 14px;
  font-size: 14px;
  font-family: inherit;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  background-image: url("data:image/svg+xml;charset=utf-8,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' fill='none' stroke='%2364707d' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m3 5 3 3 3-3'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 14px center;
  background-size: 12px;
}

.topbar-select:hover {
  border-color: var(--primary);
  background-color: var(--surface-container-highest);
}

.topbar-select:focus {
  outline: none;
  border-color: var(--primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 20%, transparent);
}

.topbar-select option {
  background-color: var(--surface-container-lowest);
  color: var(--on-surface);
}

.status-strip {
  display: flex;
  justify-content: space-between;
  align-items: center;
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

.error-card h2 {
  color: #eb5e55;
}

.task-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

/* Dashboard Two-Column Layout Grid */
.dashboard-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 24px;
}

@media (min-width: 1024px) {
  .dashboard-grid {
    grid-template-columns: 1fr 310px;
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

/* Dev Panel */
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
</style>
