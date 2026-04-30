<template>
  <section class="page">
    <p class="eyebrow">Welcome</p>
    <h1>Setup Your Study Profile</h1>
    <p class="subtitle">Configure your study preferences to get started.</p>

    <article class="panel form-grid">
      <label for="student-id">Student ID</label>
      <input
        id="student-id"
        v-model="studentID"
        type="text"
        placeholder="Enter your USN or alias"
        :disabled="loading || saving"
      />
      <p class="hint">Your unique identifier for tracking progress.</p>

      <label for="institutional-sync">Institutional Mode</label>
      <div class="row">
        <label class="toggle-label">
          <input
            id="institutional-sync"
            v-model="institutionalSync"
            type="checkbox"
            :disabled="loading || saving"
          />
          <span>Enable sync to teacher dashboard</span>
        </label>
      </div>
      <p class="hint">When enabled, your study data syncs to your institution's dashboard.</p>

      <label for="dashboard-endpoint">Dashboard Endpoint (optional)</label>
      <input
        id="dashboard-endpoint"
        v-model="dashboardEndpoint"
        type="text"
        placeholder="https://dashboard.example.com/api"
        :disabled="loading || saving || !institutionalSync"
      />
      <p class="hint">Leave blank if you don't know this value.</p>

      <label for="daily-minutes">Daily Study Limit (minutes)</label>
      <input
        id="daily-minutes"
        v-model.number="dailyMinutes"
        type="number"
        min="15"
        max="480"
        step="5"
        :disabled="loading || saving"
      />
      <p class="hint">How much time you want to spend studying each day.</p>

      <button type="button" class="continue-btn" :disabled="loading || saving" @click="saveAndContinue">
        {{ saving ? 'Saving...' : 'Continue to Dashboard' }}
      </button>

      <p v-if="error" class="error-text">{{ error }}</p>
      <p v-if="success" class="success-text">{{ success }}</p>
    </article>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getStudentSettings, upsertStudentSettings } from '../services/appApi'

const router = useRouter()
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const success = ref('')
const studentID = ref('')
const institutionalSync = ref(false)
const dashboardEndpoint = ref('')
const dailyMinutes = ref(90)

onMounted(async () => {
  try {
    loading.value = true
    error.value = ''

    const response = await getStudentSettings()
    if (response.error) {
      error.value = response.error
      return
    }

    // If student_id is already set, redirect to dashboard
    if (response.student_id) {
      router.push('/')
      return
    }

    studentID.value = response.student_id || ''
    institutionalSync.value = response.institutional_sync || false
    dashboardEndpoint.value = response.dashboard_endpoint || ''
    dailyMinutes.value = Number(response.daily_study_minutes) || 90
  } catch (err) {
    error.value = err.message || 'Failed to load settings'
  } finally {
    loading.value = false
  }
})

async function saveAndContinue() {
  error.value = ''
  success.value = ''

  if (!studentID.value.trim()) {
    error.value = 'Student ID is required.'
    return
  }

  const minutes = Number(dailyMinutes.value)
  if (!Number.isInteger(minutes) || minutes < 15 || minutes > 480) {
    error.value = 'Enter a value between 15 and 480 minutes.'
    return
  }

  if (institutionalSync.value && !dashboardEndpoint.value.trim()) {
    error.value = 'Dashboard endpoint is required when institutional sync is enabled.'
    return
  }

  try {
    saving.value = true
    const response = await upsertStudentSettings(
      studentID.value.trim(),
      institutionalSync.value,
      dashboardEndpoint.value.trim(),
      minutes
    )
    if (response.error) {
      error.value = response.error
      return
    }

    success.value = 'Settings saved. Redirecting...'
    setTimeout(() => {
      router.push('/')
    }, 1000)
  } catch (err) {
    error.value = err.message || 'Failed to save settings'
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
  max-width: 600px;
  margin: 0 auto;
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

.subtitle {
  margin: 0;
  color: var(--muted-text);
  font-size: 16px;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 24px;
}

.form-grid {
  display: grid;
  gap: 16px;
}

label {
  font-weight: 600;
  color: var(--on-surface);
}

input[type="text"],
input[type="number"] {
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 11px 12px;
  font-size: 14px;
  width: 100%;
}

input:focus {
  border-color: var(--primary);
  outline: none;
}

input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.row {
  display: flex;
  align-items: center;
}

.toggle-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-weight: 500;
  color: var(--on-surface);
}

.toggle-label input[type="checkbox"] {
  width: 18px;
  height: 18px;
  cursor: pointer;
}

.hint {
  margin: 0;
  color: var(--muted-text);
  font-size: 13px;
}

.continue-btn {
  border: 0;
  border-radius: 12px;
  padding: 12px 20px;
  color: var(--on-primary);
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  cursor: pointer;
  font-size: 15px;
  margin-top: 8px;
}

.continue-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
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
</style>
