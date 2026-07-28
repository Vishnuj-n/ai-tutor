<template>
  <article class="panel">
    <h2>Application Updates</h2>
    <div class="update-section">
      <div class="status-info">
        <p class="current-ver">
          Current Version: <strong>v{{ currentVersion }}</strong>
        </p>
        <p v-if="updateChecked && !updateAvailable" class="status-text success">
          ✓ Your application is up to date!
        </p>
        <p v-if="updateChecked && updateAvailable" class="status-text warning">
          ⚠️ A new version (<strong>v{{ latestVersion }}</strong
          >) is available!
        </p>
        <p v-if="error" class="status-text error">Error checking updates: {{ error }}</p>
      </div>

      <div class="action-buttons">
        <button type="button" class="btn-check" :disabled="checking" @click="performCheck">
          {{ checking ? 'Checking...' : 'Check for Updates' }}
        </button>

        <button v-if="updateAvailable" type="button" class="btn-redirect" @click="redirectToRepo">
          Get Update (Redirect to Repository)
        </button>
      </div>
    </div>
  </article>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { checkForUpdates, openRepoURL } from '../services/appApi'

const checking = ref(false)
const updateChecked = ref(false)
const updateAvailable = ref(false)
const currentVersion = ref('1.2.0')
const latestVersion = ref('')
const error = ref('')

async function performCheck() {
  checking.value = true
  error.value = ''
  try {
    const res = await checkForUpdates()
    if (res?.current_version) {
      currentVersion.value = String(res.current_version).replace(/^v+/, '')
    }
    if (res?.latest_version) {
      latestVersion.value = String(res.latest_version).replace(/^v+/, '')
    }
    updateAvailable.value = !!res?.update_available
    updateChecked.value = true
    if (res?.error) {
      error.value = res.error
    }
  } catch (err) {
    error.value = err.message || 'Failed to check updates'
  } finally {
    checking.value = false
  }
}

function redirectToRepo() {
  openRepoURL()
}

onMounted(() => {
  // Grab the app version initially
  checkForUpdates()
    .then((res) => {
      if (res?.current_version) {
        currentVersion.value = String(res.current_version).replace(/^v+/, '')
      }
      if (res?.error) {
        error.value = res.error
      }
    })
    .catch(() => {})
})
</script>

<style scoped>
.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 28px;
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  box-shadow: 0 4px 20px color-mix(in srgb, var(--on-surface) 3%, transparent);
}

h2 {
  font-size: 20px;
  margin: 0 0 16px;
  font-weight: 700;
}

.update-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.current-ver {
  margin: 0;
  font-size: 14px;
}

.status-text {
  margin: 8px 0 0 0;
  font-size: 14px;
  font-weight: 500;
}

.status-text.success {
  color: #10b981;
}

.status-text.warning {
  color: #f59e0b;
}

.status-text.error {
  color: #ef4444;
}

.action-buttons {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

button {
  border: none;
  border-radius: 8px;
  padding: 10px 18px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-check {
  background: var(--surface-container-highest);
  color: var(--on-surface);
}

.btn-check:hover:not(:disabled) {
  background: var(--surface-container-high);
}

.btn-check:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-redirect {
  background: var(--primary);
  color: var(--on-primary, #ffffff);
}

.btn-redirect:hover {
  background: var(--primary-hover, color-mix(in srgb, var(--primary) 85%, #000));
}
</style>
