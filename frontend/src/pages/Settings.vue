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
  </section>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { getDailyStudySettings, updateDailyStudyMinutes } from '../services/appApi'

const loading = ref(true)
const saving = ref(false)
const error = ref('')
const success = ref('')
const dailyMinutes = ref(90)
const successTimeout = ref(null)

onUnmounted(() => {
  if (successTimeout.value !== null) {
    clearTimeout(successTimeout.value)
    successTimeout.value = null
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
</style>
