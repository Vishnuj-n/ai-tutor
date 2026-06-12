<template>
  <div class="onboarding-overlay">
    <div class="onboarding-card">
      <div class="header-section">
        <div class="logo-orb">AG</div>
        <h1>Welcome to AntiGravity</h1>
        <p class="subtitle">Set up your persistent study workspace in seconds</p>
      </div>

      <div class="progress-bar">
        <div class="progress-fill" :style="{ width: step === 1 ? '50%' : '100%' }"></div>
      </div>

      <div v-if="step === 1" class="step-container">
        <h2>1. Create Your Study Profile</h2>
        <p class="description">Profiles group your textbooks and deadlines. E.g. "UPSC Prep" or "Semester Finals".</p>

        <div class="form-group">
          <label for="profile-name">Profile Name</label>
          <input
            id="profile-name"
            v-model="profileName"
            type="text"
            placeholder="e.g. UPSC Prep, College Sem 3"
            required
          />
        </div>

        <div class="form-group">
          <label for="profile-deadline">Target Exam Deadline</label>
          <input
            id="profile-deadline"
            v-model="profileDeadline"
            type="date"
            required
          />
        </div>

        <div class="form-group">
          <label for="daily-minutes">Daily Study Goal (Minutes)</label>
          <input
            id="daily-minutes"
            v-model.number="dailyMinutes"
            type="number"
            min="15"
            max="480"
            step="5"
          />
        </div>

        <button class="action-button" :disabled="!isStep1Valid" @click="step = 2">
          Next Step
        </button>
      </div>

      <div v-else-if="step === 2" class="step-container">
        <h2>2. Teacher Cloud Sync (Optional)</h2>
        <p class="description">If your teacher sends assigned books and tracks progress, enter the sync details below.</p>

        <div class="form-group">
          <label for="cloud-url">Cloud Server URL</label>
          <input
            id="cloud-url"
            v-model="cloudSyncURL"
            type="url"
            placeholder="e.g. https://school-tutor.cloud/api/sync"
          />
        </div>

        <div class="form-group">
          <label for="api-token">Authorization API Token</label>
          <input
            id="api-token"
            v-model="apiToken"
            type="password"
            placeholder="Enter your auth token"
          />
        </div>

        <div v-if="error" class="error-banner">{{ error }}</div>

        <div class="button-row">
          <button class="secondary-button" @click="step = 1">Back</button>
          <button class="action-button" :disabled="loading" @click="completeOnboarding">
            {{ loading ? 'Configuring...' : 'Initialize Workspace' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { createProfile, updateUserSettings } from '../services/appApi'

const router = useRouter()
const step = ref(1)
const loading = ref(false)
const error = ref('')

const profileName = ref('')
const profileDeadline = ref('')
const dailyMinutes = ref(90)
const cloudSyncURL = ref('')
const apiToken = ref('')

const isStep1Valid = computed(() => {
  return profileName.value.trim() !== '' && profileDeadline.value !== '' && dailyMinutes.value >= 15
})

async function completeOnboarding() {
  error.value = ''
  loading.value = true
  try {
    // 1. Create the first profile
    const profileRes = await createProfile(profileName.value, profileDeadline.value)
    if (profileRes.error) {
      error.value = profileRes.error
      return
    }

    const newProfile = profileRes.profile

    // 2. Set settings with this profile as active
    const settingsRes = await updateUserSettings(
      dailyMinutes.value,
      newProfile.id,
      false, // skip to reading off by default
      cloudSyncURL.value.trim(),
      apiToken.value.trim()
    )

    if (settingsRes.error) {
      error.value = settingsRes.error
      return
    }

    // 3. Redirect to dashboard
    router.push('/dashboard')
  } catch (err) {
    error.value = err.message || 'Onboarding configuration failed.'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.onboarding-overlay {
  position: fixed;
  inset: 0;
  background: radial-gradient(circle at top left, #1a1b2f, #0c0d14);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  padding: 20px;
  color: #ffffff;
  font-family: 'Inter', sans-serif;
}

.onboarding-card {
  width: 100%;
  max-width: 500px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 24px;
  padding: 40px;
  backdrop-filter: blur(20px);
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.3);
  box-sizing: border-box;
}

.header-section {
  text-align: center;
  margin-bottom: 30px;
}

.logo-orb {
  width: 60px;
  height: 60px;
  margin: 0 auto 16px;
  background: linear-gradient(135deg, #6c5ce7, #a8a5e6);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 800;
  font-size: 20px;
  letter-spacing: -0.05em;
  box-shadow: 0 0 30px rgba(108, 92, 231, 0.4);
}

h1 {
  font-size: 28px;
  font-weight: 800;
  margin: 0 0 8px;
  letter-spacing: -0.03em;
  background: linear-gradient(to right, #ffffff, #e0e0e0);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.subtitle {
  color: #8a8b98;
  font-size: 14px;
  margin: 0;
}

.progress-bar {
  height: 4px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 2px;
  margin-bottom: 30px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(to right, #6c5ce7, #a8a5e6);
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.step-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

h2 {
  font-size: 18px;
  font-weight: 700;
  margin: 0;
}

.description {
  font-size: 13px;
  color: #8a8b98;
  line-height: 1.5;
  margin: -10px 0 10px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

label {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #8a8b98;
}

input {
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  padding: 12px 16px;
  color: #ffffff;
  font-size: 14px;
  font-family: inherit;
  transition: border-color 0.2s, background-color 0.2s;
  box-sizing: border-box;
  width: 100%;
}

input:focus {
  outline: none;
  border-color: #6c5ce7;
  background: rgba(255, 255, 255, 0.05);
}

.action-button {
  background: linear-gradient(to right, #6c5ce7, #8073e4);
  border: none;
  border-radius: 12px;
  padding: 14px;
  color: #ffffff;
  font-weight: 700;
  font-size: 14px;
  cursor: pointer;
  transition: opacity 0.2s, transform 0.2s;
  width: 100%;
  margin-top: 10px;
  text-align: center;
}

.action-button:hover:not(:disabled) {
  opacity: 0.95;
  transform: translateY(-1px);
}

.action-button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.button-row {
  display: flex;
  gap: 12px;
  margin-top: 10px;
}

.secondary-button {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  padding: 14px;
  color: #ffffff;
  font-weight: 700;
  font-size: 14px;
  cursor: pointer;
  transition: background 0.2s;
  flex: 1;
  text-align: center;
}

.secondary-button:hover {
  background: rgba(255, 255, 255, 0.1);
}

.error-banner {
  background: rgba(235, 94, 85, 0.1);
  border: 1px solid rgba(235, 94, 85, 0.2);
  color: #eb5e55;
  padding: 12px;
  border-radius: 12px;
  font-size: 13px;
  text-align: center;
}
</style>
