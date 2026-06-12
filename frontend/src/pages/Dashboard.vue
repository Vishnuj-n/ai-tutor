<template>
  <section class="page">
    <header class="topbar">
      <div class="search-shell">Search knowledge base...</div>
      
      <!-- Active Profile Dropdown Selector -->
      <div class="profile-selector-container">
        <label for="active-profile-select">Current Profile:</label>
        <select
          id="active-profile-select"
          v-model="userSettings.active_profile_id"
          @change="changeActiveProfile"
          class="topbar-select"
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
          <p class="info-subtitle">Review tasks have been pushed to the background so you can focus on reading new chapters.</p>
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
          <button
            type="button"
            class="primary-btn"
            :aria-label="'Start task ' + (task.title || task.id)"
            @click="startTask(task)"
          >
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
        <p class="muted">Activate textbooks in the shelf below or upload new textbooks to begin.</p>
      </div>

      <!-- The Smart Shelf: Active vs Dormant Warehouse -->
      <section class="shelf-section">
        <h2> The Smart Shelf</h2>
        <p class="section-description">Hard-gated to a maximum of 4 active textbooks to prevent study fatigue.</p>
        
        <div class="shelf-grid">
          <!-- Active Textbooks Column -->
          <article class="shelf-column active-column">
            <h3>Active Lane ({{ activeNotebooks.length }} / 4)</h3>
            <div v-if="activeNotebooks.length === 0" class="shelf-empty">
              No textbooks active. Activate books below to populate your study queue.
            </div>
            <div v-else class="shelf-list">
              <div v-for="nb in activeNotebooks" :key="nb.id" class="shelf-card active-card">
                <div class="card-details">
                  <h4>{{ nb.title }}</h4>
                  <p class="card-meta">
                    Priority: <strong>{{ nb.priority }}</strong> · {{ nb.page_count }} pages
                  </p>
                  <span v-if="getProfileName(nb.profile_id)" class="profile-tag">
                    {{ getProfileName(nb.profile_id) }}
                  </span>
                </div>
                <button class="shelf-action-btn sleep-btn" @click="setStudyStatus(nb.id, 'dormant')">
                  Sleep
                </button>
              </div>
            </div>
          </article>

          <!-- Dormant Textbook Warehouse Column -->
          <article class="shelf-column dormant-column">
            <h3>Dormant Warehouse</h3>
            <div v-if="dormantNotebooks.length === 0" class="shelf-empty">
              Dormant warehouse is empty. Upload books to store them here.
            </div>
            <div v-else class="shelf-list">
              <div v-for="nb in dormantNotebooks" :key="nb.id" class="shelf-card dormant-card">
                <div class="card-details">
                  <h4>{{ nb.title }}</h4>
                  <p class="card-meta">
                    Priority: <strong>{{ nb.priority }}</strong> · {{ nb.page_count }} pages
                  </p>
                  <span v-if="getProfileName(nb.profile_id)" class="profile-tag">
                    {{ getProfileName(nb.profile_id) }}
                  </span>
                </div>
                <button
                  class="shelf-action-btn activate-btn"
                  :disabled="activeNotebooks.length >= 4"
                  @click="setStudyStatus(nb.id, 'active')"
                >
                  Activate
                </button>
              </div>
            </div>
          </article>
        </div>
      </section>
    </template>
  </section>
</template>

<script setup>
import { onMounted, ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  getTodayPlan,
  getNotebooks,
  getProfiles,
  getUserSettings,
  updateUserSettings,
  updateNotebookStudyStatus,
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
const notebooks = ref([])
const userSettings = ref({
  daily_study_minutes: 90,
  active_profile_id: '',
  skip_to_reading_active: false,
  cloud_sync_url: '',
  cloud_api_token: ''
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

const activeNotebooks = computed(() => {
  return notebooks.value.filter(nb => nb.study_status === 'active')
})

const dormantNotebooks = computed(() => {
  // If active profile is selected, show dormant notebooks for that profile
  return notebooks.value.filter(nb => {
    const isDormant = nb.study_status === 'dormant' || !nb.study_status
    if (userSettings.value.active_profile_id) {
      return isDormant && nb.profile_id === userSettings.value.active_profile_id
    }
    return isDormant
  })
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

    // 1. Fetch settings and profiles
    const settingsRes = await getUserSettings()
    if (!settingsRes.error) {
      userSettings.value = settingsRes
    }

    const profilesRes = await getProfiles()
    if (!profilesRes.error) {
      profiles.value = profilesRes.profiles || []
    }

    // 2. Fetch today's plan
    const response = await getTodayPlan()
    if (response.error) {
      error.value = response.error
      return
    }

    tasks.value = response.tasks || []
    dueReviewCards.value = response.due_review_cards || 0

    // 3. Fetch textbooks
    const notebooksList = await getNotebooks('')
    notebooks.value = Array.isArray(notebooksList) ? notebooksList.filter((nb) => !nb?.error) : []
    hasActiveStudyContent.value = notebooks.value.some((nb) => nb.study_status === 'active')

    // 4. Load pace for active profile
    if (userSettings.value.active_profile_id) {
      try {
        const pace = await getProfileDailyPace(userSettings.value.active_profile_id)
        if (!pace.error) {
          activeProfilePace.value = pace
        }
      } catch (err) {
        console.error('Failed to get profile daily pace', err)
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
  try {
    loading.value = true
    await updateUserSettings(
      userSettings.value.daily_study_minutes,
      userSettings.value.active_profile_id,
      userSettings.value.skip_to_reading_active,
      userSettings.value.cloud_sync_url,
      userSettings.value.cloud_api_token
    )
    await loadAgenda()
  } catch (err) {
    alert('Failed to switch active profile')
  } finally {
    loading.value = false
  }
}

async function toggleEscapeHatch() {
  try {
    loading.value = true
    userSettings.value.skip_to_reading_active = !userSettings.value.skip_to_reading_active
    await updateUserSettings(
      userSettings.value.daily_study_minutes,
      userSettings.value.active_profile_id,
      userSettings.value.skip_to_reading_active,
      userSettings.value.cloud_sync_url,
      userSettings.value.cloud_api_token
    )
    await loadAgenda()
  } catch (err) {
    alert('Failed to toggle escape hatch')
  } finally {
    loading.value = false
  }
}

async function setStudyStatus(notebookID, status) {
  try {
    loading.value = true
    const res = await updateNotebookStudyStatus(notebookID, status)
    if (res.error) {
      alert(res.error)
      return
    }
    await loadAgenda()
  } catch (err) {
    alert('Failed to update textbook status')
  } finally {
    loading.value = false
  }
}

function getProfileName(profileID) {
  const p = profiles.value.find(pr => pr.id === profileID)
  return p ? p.name : ''
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

/* Shelf layouts */
.shelf-section {
  margin-top: 32px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.shelf-section h2 {
  margin: 0;
  font-size: 22px;
}

.section-description {
  margin: 0 0 16px;
  font-size: 14px;
  color: var(--muted-text);
}

.shelf-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.shelf-column {
  background: var(--surface-container-low);
  border: 1px solid var(--outline-variant);
  border-radius: 16px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.shelf-column h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--on-surface);
}

.shelf-empty {
  text-align: center;
  padding: 30px;
  color: var(--muted-text);
  font-size: 13px;
  border: 1px dashed var(--outline-variant);
  border-radius: 12px;
}

.shelf-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.shelf-card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  padding: 14px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.card-details h4 {
  margin: 0 0 4px;
  font-size: 14px;
  font-weight: 700;
}

.card-meta {
  margin: 0 0 6px;
  font-size: 12px;
  color: var(--muted-text);
}

.profile-tag {
  background: var(--surface-container-high);
  color: var(--on-surface);
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
}

.shelf-action-btn {
  border: none;
  border-radius: 8px;
  padding: 6px 12px;
  font-weight: 700;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
}

.sleep-btn {
  background: rgba(235, 94, 85, 0.1);
  color: #eb5e55;
}

.sleep-btn:hover {
  background: #eb5e55;
  color: white;
}

.activate-btn {
  background: rgba(108, 92, 231, 0.1);
  color: var(--primary);
}

.activate-btn:hover:not(:disabled) {
  background: var(--primary);
  color: var(--on-primary);
}

.activate-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
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
