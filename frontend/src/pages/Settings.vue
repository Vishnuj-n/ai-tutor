<template>
  <section class="page">
    <p class="eyebrow">Settings</p>
    <h1>Study Budget</h1>

    <article class="panel form-grid">
      <label for="daily-minutes">Daily study minutes</label>
      <div class="row">
        <input
          id="daily-minutes"
          v-model.number="dailyMinutes"
          type="number"
          min="15"
          max="480"
          step="5"
          :disabled="loading || saving"
        />
        <button type="button" class="save-btn" :disabled="loading || saving" @click="saveSettings">
          {{ saving ? 'Saving...' : 'Save' }}
        </button>
      </div>
      <p class="hint">Used by scheduler math: review budget first, then context-locked reading pages.</p>
      <p v-if="error" class="error-text">{{ error }}</p>
      <p v-if="success" class="success-text">{{ success }}</p>
    </article>

    <article class="panel form-grid">
      <p class="eyebrow">Institutional Sync</p>
      <h2>Privacy & Data Sync</h2>
      
      <div class="info-section">
        <p class="info-label">Student ID:</p>
        <p class="info-value">{{ studentID || 'Not set' }}</p>
      </div>

      <div class="info-section">
        <p class="info-label">Sync Status:</p>
        <p class="info-value">{{ institutionalSync ? 'Enabled' : 'Disabled' }}</p>
      </div>

      <div class="info-section">
        <p class="info-label">Dashboard Endpoint:</p>
        <p class="info-value">{{ dashboardEndpoint || 'Not configured' }}</p>
      </div>

      <div class="privacy-disclosure">
        <h3>Data Privacy</h3>
        <p class="privacy-text">
          <strong>Local Data:</strong> All your study data (flashcards, quiz scores, reading progress) is stored locally on your device in SQLite database.
        </p>
        <p class="privacy-text">
          <strong>Institutional Sync:</strong> When enabled, review events (activity type, score, FSRS state) are sent to your institution's dashboard for progress tracking. This includes your student ID and learning metrics but not your actual study content.
        </p>
        <p class="privacy-text">
          <strong>Control:</strong> You can disable institutional sync at any time. Disabling stops new data from being sent but does not delete previously synced data from your institution's systems.
        </p>
      </div>

      <div class="toggle-section">
        <label class="toggle-label">
          <input
            id="institutional-sync-toggle"
            v-model="institutionalSync"
            type="checkbox"
            :disabled="loading || savingSettings"
          />
          <span>Enable Institutional Sync</span>
        </label>
      </div>

      <div class="edit-section">
        <label for="edit-student-id">Edit Student ID:</label>
        <input
          id="edit-student-id"
          v-model="editStudentID"
          type="text"
          placeholder="Enter new student ID"
          :disabled="loading || savingSettings"
        />
        <button type="button" class="save-btn" :disabled="loading || savingSettings" @click="saveStudentSettings">
          {{ savingSettings ? 'Saving...' : 'Update Settings' }}
        </button>
      </div>

      <p v-if="settingsError" class="error-text">{{ settingsError }}</p>
      <p v-if="settingsSuccess" class="success-text">{{ settingsSuccess }}</p>
    </article>
  </section>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { getDailyStudySettings, updateDailyStudyMinutes, getStudentSettings, upsertStudentSettings } from '../services/appApi'

const loading = ref(true)
const saving = ref(false)
const savingSettings = ref(false)
const error = ref('')
const success = ref('')
const settingsError = ref('')
const settingsSuccess = ref('')
const dailyMinutes = ref(90)
const studentID = ref('')
const institutionalSync = ref(false)
const dashboardEndpoint = ref('')
const editStudentID = ref('')
const successTimeout = ref(null)
const settingsSuccessTimeout = ref(null)

onUnmounted(() => {
  if (successTimeout.value !== null) {
    clearTimeout(successTimeout.value)
    successTimeout.value = null
  }
  if (settingsSuccessTimeout.value !== null) {
    clearTimeout(settingsSuccessTimeout.value)
    settingsSuccessTimeout.value = null
  }
})

onMounted(async () => {
  try {
    loading.value = true
    error.value = ''

    const response = await getDailyStudySettings()
    if (response.error) {
      error.value = response.error
      return
    }

    dailyMinutes.value = Number(response.daily_study_minutes) || 90

    // Load student settings
    const studentResponse = await getStudentSettings()
    if (studentResponse.error) {
      console.error('Failed to load student settings:', studentResponse.error)
    } else {
      studentID.value = studentResponse.student_id || ''
      institutionalSync.value = studentResponse.institutional_sync || false
      dashboardEndpoint.value = studentResponse.dashboard_endpoint || ''
      editStudentID.value = studentID.value
    }
  } catch (err) {
    error.value = err.message || 'Failed to load settings'
  } finally {
    loading.value = false
  }
})

async function saveSettings() {
  error.value = ''
  success.value = ''

  if (successTimeout.value !== null) {
    clearTimeout(successTimeout.value)
    successTimeout.value = null
  }

  const minutes = Number(dailyMinutes.value)
  if (!Number.isInteger(minutes) || minutes < 15 || minutes > 480) {
    error.value = 'Enter a value between 15 and 480 minutes.'
    return
  }

  try {
    saving.value = true
    const response = await updateDailyStudyMinutes(minutes)
    if (response.error) {
      error.value = response.error
      return
    }

    dailyMinutes.value = Number(response.daily_study_minutes) || minutes
    success.value = 'Daily study limit updated.'
    successTimeout.value = setTimeout(() => {
      success.value = ''
      successTimeout.value = null
    }, 4000)
  } catch (err) {
    error.value = err.message || 'Failed to save settings'
  } finally {
    saving.value = false
  }
}

async function saveStudentSettings() {
  settingsError.value = ''
  settingsSuccess.value = ''

  if (settingsSuccessTimeout.value !== null) {
    clearTimeout(settingsSuccessTimeout.value)
    settingsSuccessTimeout.value = null
  }

  if (!editStudentID.value.trim()) {
    settingsError.value = 'Student ID is required.'
    return
  }

  if (institutionalSync.value && !dashboardEndpoint.value.trim()) {
    settingsError.value = 'Dashboard endpoint is required when institutional sync is enabled.'
    return
  }

  try {
    savingSettings.value = true
    const response = await upsertStudentSettings(
      editStudentID.value.trim(),
      institutionalSync.value,
      dashboardEndpoint.value.trim(),
      dailyMinutes.value
    )
    if (response.error) {
      settingsError.value = response.error
      return
    }

    studentID.value = editStudentID.value.trim()
    settingsSuccess.value = 'Student settings updated.'
    settingsSuccessTimeout.value = setTimeout(() => {
      settingsSuccess.value = ''
      settingsSuccessTimeout.value = null
    }, 4000)
  } catch (err) {
    settingsError.value = err.message || 'Failed to save student settings'
  } finally {
    savingSettings.value = false
  }
}
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-weight: 700;
}

h1 {
  margin: 0;
  font-size: 46px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 24px;
}

.form-grid {
  display: grid;
  gap: 12px;
  max-width: 560px;
}

label {
  font-weight: 600;
  color: var(--on-surface);
}

.row {
  display: grid;
  gap: 10px;
  grid-template-columns: 1fr auto;
}

input {
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 11px 12px;
  font-size: 14px;
}

input:focus {
  border-color: var(--primary);
  outline: none;
}

.save-btn {
  border: 0;
  border-radius: 12px;
  padding: 0 20px;
  color: var(--on-primary);
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
}

.hint {
  margin: 0;
  color: var(--muted-text);
  font-size: 13px;
}

.error-text {
  margin: 0;
  color: #a3362f;
  font-size: 13px;
}

.success-text {
  margin: 0;
  color: #256f36;
  font-size: 13px;
}

h2 {
  margin: 0 0 16px 0;
  font-size: 28px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.info-section {
  display: grid;
  grid-template-columns: 140px 1fr;
  gap: 8px;
  padding: 12px 0;
  border-bottom: 1px solid var(--outline-variant);
}

.info-label {
  margin: 0;
  font-weight: 600;
  color: var(--muted-text);
  font-size: 13px;
}

.info-value {
  margin: 0;
  color: var(--on-surface);
  font-size: 14px;
}

.privacy-disclosure {
  margin-top: 20px;
  padding: 16px;
  background: var(--surface-container-low);
  border-radius: 12px;
  border: 1px solid var(--outline-variant);
}

.privacy-disclosure h3 {
  margin: 0 0 12px 0;
  font-size: 16px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.privacy-text {
  margin: 0 0 8px 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--on-surface);
}

.privacy-text:last-child {
  margin-bottom: 0;
}

.toggle-section {
  margin-top: 20px;
}

.toggle-label {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  font-weight: 500;
  color: var(--on-surface);
}

.toggle-label input[type="checkbox"] {
  width: 20px;
  height: 20px;
  cursor: pointer;
}

.edit-section {
  margin-top: 20px;
  display: grid;
  gap: 10px;
}

.edit-section input {
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 11px 12px;
  font-size: 14px;
}

.edit-section input:focus {
  border-color: var(--primary);
  outline: none;
}

.edit-section input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
