<template>
  <section class="page">
    <header class="topbar">
      <div class="search-shell">Search knowledge base...</div>

      <!-- Active Profile Dropdown Selector -->
      <div class="profile-selector-container">
        <label for="active-profile-select">Current Profile:</label>
        <select id="active-profile-select" v-model="userSettings.active_profile_id" @change="changeActiveProfile"
          class="topbar-select">
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
          <p class="info-subtitle">Review tasks have been pushed to the background so you can focus on reading new
            chapters.</p>
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
        <button class="escape-hatch-toggle" :class="{ active: userSettings.skip_to_reading_active }"
          @click="toggleEscapeHatch">
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
      <!-- Telemetry Widget for active profile -->
      <section v-if="activeProfilePace" class="telemetry-widget">
        <div class="telemetry-card card">
          <h2 class="telemetry-header">Profile Study Pacing ({{ activeProfileName }})</h2>
          <div class="telemetry-grid">
            <div class="telemetry-item">
              <div class="telemetry-title-row">
                <span class="telemetry-doc-title">Target Exam Deadline: {{ activeProfilePace.deadline }}</span>
                <span class="telemetry-days-left" :class="{ warning: activeProfilePace.days_remaining <= 3 }">
                  ({{ formatDaysRemaining(activeProfilePace.days_remaining) }})
                </span>
              </div>
              <div class="telemetry-metric-row">
                <div class="telemetry-metric">
                  <span class="metric-value">{{ activeProfilePace.daily_pace }}</span>
                  <span class="metric-label">words / day</span>
                </div>
                <div class="telemetry-metric">
                  <span class="metric-value">{{ activeProfilePace.sessions_per_day.toFixed(1) }}</span>
                  <span class="metric-label">sessions / day</span>
                </div>
                <div class="telemetry-progress-info">
                  <div class="progress-details">
                    <span>Remaining words: <strong>{{ activeProfilePace.remaining_words }}</strong></span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Task List (Interleaved Bookshelf Tasks) -->
      <div v-if="tasks.length > 0" class="task-list">
        <article v-for="task in tasks" :key="task.id" class="card task-card">
          <div class="task-header">
            <span class="task-type">{{ task.action_type }}</span>
            <span class="task-estimate">{{ task.estimate_minutes }} min</span>
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
          <button type="button" class="primary-btn" :aria-label="'Start task ' + (task.title || task.id)"
            @click="startTask(task)">
            Start
          </button>
        </article>
      </div>

      <div v-else-if="hasActiveStudyContent" class="card state-card victory-card">
        <h2>Tasks Complete!</h2>
        <p class="muted">You've completed all tasks for today. Great work!</p>
      </div>

      <div v-else class="card state-card">
        <h2>No textbooks active</h2>
        <p class="muted">Go to Notebooks to upload and activate textbooks.</p>
      </div>
    </template>
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
  getProfileDailyPace
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
  daily_study_minutes: 90,
  active_profile_id: '',
  skip_to_reading_active: false,
  cloud_sync_url: '',
  cloud_api_token: '',
  theme: '',
  rag_enabled: false
})
const activeProfilePace = ref(null)

const flashcardsJustCreated = computed(() => {
  const created = parseInt(route.query.flashcardsCreated, 10)
  return isNaN(created) || created <= 0 ? 0 : created
})

const activeProfileName = computed(() => {
  const p = profiles.value.find(pr => pr.id === userSettings.value.active_profile_id)
  return p ? p.name : 'Unknown'
})



onMounted(async () => {
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
    hasActiveStudyContent.value = (response.tasks || []).length > 0

    // 4. Load pace for active profile
    // Guard: only request pacing if the active_profile_id resolves to a known profile.
    // An orphaned ID (deleted profile still persisted in settings) would hit the backend
    // and return { error: "profile not found" }; we skip the call entirely instead.
    const knownProfile = profiles.value.find(
      (pr) => pr.id === userSettings.value.active_profile_id
    )
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

  } catch (err) {
    error.value = err.message || 'Failed to load tasks'
  } finally {
    loading.value = false
  }
}

async function changeActiveProfile() {
  const previousActiveProfile = userSettings.value.active_profile_id
  try {
    loading.value = true
    const res = await updateUserSettings(
      userSettings.value.daily_study_minutes,
      userSettings.value.active_profile_id,
      userSettings.value.skip_to_reading_active,
      userSettings.value.cloud_sync_url,
      userSettings.value.cloud_api_token,
      userSettings.value.theme || '',
      userSettings.value.rag_enabled || false
    )
    if (res && res.error) {
      userSettings.value.active_profile_id = previousActiveProfile
      actionError.value = res.error
      return
    }
    await loadAgenda()
  } catch (err) {
    userSettings.value.active_profile_id = previousActiveProfile
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
      userSettings.value.daily_study_minutes,
      userSettings.value.active_profile_id,
      userSettings.value.skip_to_reading_active,
      userSettings.value.cloud_sync_url,
      userSettings.value.cloud_api_token,
      userSettings.value.theme || '',
      userSettings.value.rag_enabled || false
    )
    if (res && res.error) {
      userSettings.value.skip_to_reading_active = previousSkipToReading
      actionError.value = res.error
      return
    }
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

.search-shell {
  flex: 1;
  background: var(--surface-container-low);
  color: var(--muted-text);
  border-radius: 12px;
  padding: 11px 14px;
  max-width: 440px;
  font-size: 14px;
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
  transition: transform 0.2s, border-color 0.2s;
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
</style>
