<template>
  <div class="onboarding-overlay">
    <div class="onboarding-card">
      <div class="header-section">
        <div class="logo-orb">AG</div>
        <h1>Welcome to AntiGravity</h1>
        <p class="subtitle">Set up your persistent study workspace in seconds</p>
      </div>

      <div class="progress-bar">
        <div class="progress-fill" :style="{ width: step === 1 ? '25%' : step === 2 ? '50%' : step === 3 ? '75%' : '100%' }"></div>
      </div>

      <!-- Step 1: Profile and Goal -->
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

      <!-- Step 2: Cloud Sync Settings -->
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

        <div class="button-row">
          <button class="secondary-button" @click="step = 1">Back</button>
          <button class="action-button" @click="step = 3">Next Step</button>
        </div>
      </div>

      <!-- Step 3: Local AI Retrieval (RAG) -->
      <div v-else-if="step === 3" class="step-container">
        <h2>3. Local AI Retrieval (RAG)</h2>
        <p class="description">Enable context-rich Q&A. This sets up a local ONNX embedding engine and vec0 search to query books completely offline.</p>

        <div class="rag-options">
          <label class="rag-option-card" :class="{ active: wantRag }">
            <input type="radio" :value="true" v-model="wantRag" :disabled="isSettingUpRag" />
            <div class="option-info">
              <strong>Yes, Enable Local RAG (Recommended)</strong>
              <p>Download and compile local assets (~152 MB). Requires Windows x64.</p>
            </div>
          </label>

          <label class="rag-option-card" :class="{ active: !wantRag }">
            <input type="radio" :value="false" v-model="wantRag" :disabled="isSettingUpRag" />
            <div class="option-info">
              <strong>No, Skip Local RAG</strong>
              <p>AI Q&A will be disabled in the reader. Simple lexical fallbacks are used.</p>
            </div>
          </label>
        </div>

        <!-- Progress block during setup -->
        <div v-if="isSettingUpRag || ragSetupCompleted || ragError" class="rag-setup-box">
          <div class="setup-header">
            <span class="status-badge" :class="ragStatus">{{ ragStatus.toUpperCase() }}</span>
            <span class="setup-msg">{{ ragMessage }}</span>
          </div>
          
          <div class="progress-bar-mini">
            <div class="progress-fill-mini" :style="{ width: ragPercent + '%' }"></div>
          </div>
          
          <p class="setup-detail">{{ ragDetail }}</p>
          
          <div v-if="ragError" class="error-banner">{{ ragError }}</div>
        </div>

        <div class="button-row">
          <button class="secondary-button" :disabled="isSettingUpRag" @click="step = 2">Back</button>
          
          <button 
            v-if="wantRag && !ragSetupCompleted" 
            class="action-button" 
            :disabled="isSettingUpRag" 
            @click="startRagSetup"
          >
            {{ isSettingUpRag ? 'Setting Up...' : 'Initialize Local AI' }}
          </button>
          
          <button 
            v-else 
            class="action-button" 
            @click="step = 4"
          >
            Next Step
          </button>
        </div>
      </div>

      <!-- Step 4: Aesthetic Preferences -->
      <div v-else-if="step === 4" class="step-container">
        <h2>4. Choose Workspace Aesthetic</h2>
        <p class="description">Select a visual theme. Changing themes alters the colors of your study desk in real-time.</p>

        <div class="theme-grid">
          <button
            type="button"
            class="theme-card"
            :class="{ active: selectedTheme === 'light-classic' }"
            @click="selectTheme('light-classic')"
          >
            <div class="theme-preview light-classic">
              <span class="preview-dot primary"></span>
              <span class="preview-dot surface"></span>
            </div>
            <span class="theme-label">Light Classic</span>
          </button>

          <button
            type="button"
            class="theme-card"
            :class="{ active: selectedTheme === 'light-warm' }"
            @click="selectTheme('light-warm')"
          >
            <div class="theme-preview light-warm">
              <span class="preview-dot primary"></span>
              <span class="preview-dot surface"></span>
            </div>
            <span class="theme-label">Warm Sepia</span>
          </button>

          <button
            type="button"
            class="theme-card"
            :class="{ active: selectedTheme === 'dark-indigo' }"
            @click="selectTheme('dark-indigo')"
          >
            <div class="theme-preview dark-indigo">
              <span class="preview-dot primary"></span>
              <span class="preview-dot surface"></span>
            </div>
            <span class="theme-label">Deep Indigo</span>
          </button>

          <button
            type="button"
            class="theme-card"
            :class="{ active: selectedTheme === 'dark-nord' }"
            @click="selectTheme('dark-nord')"
          >
            <div class="theme-preview dark-nord">
              <span class="preview-dot primary"></span>
              <span class="preview-dot surface"></span>
            </div>
            <span class="theme-label">Nord Frost</span>
          </button>

          <button
            type="button"
            class="theme-card"
            :class="{ active: selectedTheme === 'dark-emerald' }"
            @click="selectTheme('dark-emerald')"
          >
            <div class="theme-preview dark-emerald">
              <span class="preview-dot primary"></span>
              <span class="preview-dot surface"></span>
            </div>
            <span class="theme-label">Forest Emerald</span>
          </button>
        </div>

        <div v-if="error" class="error-banner">{{ error }}</div>

        <div class="button-row">
          <button class="secondary-button" @click="step = 3">Back</button>
          <button class="action-button" :disabled="loading" @click="completeOnboarding">
            {{ loading ? 'Configuring...' : 'Initialize Workspace' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { createProfile, updateUserSettings, initializeRAG } from '../services/appApi'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

const router = useRouter()
const step = ref(1)
const loading = ref(false)
const error = ref('')

const profileName = ref('')
const profileDeadline = ref('')
const dailyMinutes = ref(90)
const cloudSyncURL = ref('')
const apiToken = ref('')
const selectedTheme = ref('light-classic')

// RAG onboarding states
const wantRag = ref(true)
const ragStatus = ref('')
const ragPercent = ref(0)
const ragMessage = ref('')
const ragDetail = ref('')
const ragError = ref('')
const ragSetupCompleted = ref(false)
const isSettingUpRag = ref(false)

const isStep1Valid = computed(() => {
  return profileName.value.trim() !== '' && profileDeadline.value !== '' && dailyMinutes.value >= 15
})

function selectTheme(theme) {
  selectedTheme.value = theme
  document.documentElement.setAttribute('data-theme', theme)
}

function startRagSetup() {
  ragError.value = ''
  isSettingUpRag.value = true
  ragStatus.value = 'checking'
  ragPercent.value = 5
  ragMessage.value = 'Checking system specifications...'
  ragDetail.value = ''

  // Always unsubscribe first so retries don't stack duplicate listeners.
  EventsOff('rag-setup-progress')
  EventsOn('rag-setup-progress', (data) => {
    console.log('[Onboarding] RAG setup progress:', data)
    if (data.status) ragStatus.value = data.status
    if (data.percent !== undefined) ragPercent.value = data.percent
    if (data.message) ragMessage.value = data.message
    if (data.detail) ragDetail.value = data.detail
    if (data.errorReason) {
      ragError.value = data.errorReason
      isSettingUpRag.value = false
    }
    
    if (data.status === 'ready') {
      ragSetupCompleted.value = true
      isSettingUpRag.value = false
      EventsOff('rag-setup-progress')
      setTimeout(() => {
        step.value = 4
      }, 1000)
    }
  })

  initializeRAG().then(res => {
    if (res.error) {
      ragError.value = res.error
      isSettingUpRag.value = false
      EventsOff('rag-setup-progress')
    }
  }).catch(err => {
    ragError.value = err.message || 'RAG setup failed.'
    isSettingUpRag.value = false
    EventsOff('rag-setup-progress')
  })
}

onUnmounted(() => {
  EventsOff('rag-setup-progress')
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
      apiToken.value.trim(),
      selectedTheme.value,
      wantRag.value && ragSetupCompleted.value
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

/* Theme Selector Grid */
.theme-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: 12px;
  margin: 10px 0;
}

.theme-card {
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 14px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  width: 100%;
}

.theme-card:hover {
  background: rgba(255, 255, 255, 0.08);
  border-color: rgba(255, 255, 255, 0.2);
  transform: translateY(-2px);
}

.theme-card.active {
  background: rgba(108, 92, 231, 0.1);
  border-color: #6c5ce7;
  box-shadow: 0 0 15px rgba(108, 92, 231, 0.3);
}

.theme-preview {
  width: 100%;
  height: 48px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.preview-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.theme-label {
  font-size: 12px;
  font-weight: 600;
  color: #e0e0e0;
}

.theme-card.active .theme-label {
  color: #ffffff;
}

/* Theme Previews */
.theme-preview.light-classic {
  background: #f9f9fb;
}
.theme-preview.light-classic .preview-dot.primary { background: #005bc1; }
.theme-preview.light-classic .preview-dot.surface { background: #ebeef2; }

.theme-preview.light-warm {
  background: #fdfaf6;
}
.theme-preview.light-warm .preview-dot.primary { background: #c27d38; }
.theme-preview.light-warm .preview-dot.surface { background: #f3eae1; }

.theme-preview.dark-indigo {
  background: #0b0d16;
}
.theme-preview.dark-indigo .preview-dot.primary { background: #6366f1; }
.theme-preview.dark-indigo .preview-dot.surface { background: #171a2b; }

.theme-preview.dark-nord {
  background: #2e3440;
}
.theme-preview.dark-nord .preview-dot.primary { background: #88c0d0; }
.theme-preview.dark-nord .preview-dot.surface { background: #3b4252; }

.theme-preview.dark-emerald {
  background: #0a120d;
}
.theme-preview.dark-emerald .preview-dot.primary { background: #10b981; }
.theme-preview.dark-emerald .preview-dot.surface { background: #152219; }

/* RAG Options Stylings */
.rag-options {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin: 20px 0;
}

.rag-option-card {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 16px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  cursor: pointer;
  transition: all 0.2s ease;
}

.rag-option-card:hover {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(255, 255, 255, 0.15);
}

.rag-option-card.active {
  background: rgba(99, 102, 241, 0.08);
  border-color: #6366f1;
}

.rag-option-card input[type="radio"] {
  margin-top: 4px;
  accent-color: #6366f1;
}

.option-info strong {
  display: block;
  font-size: 14px;
  color: #ffffff;
  margin-bottom: 4px;
}

.option-info p {
  font-size: 12px;
  color: #a0a0a0;
  margin: 0;
}

.rag-setup-box {
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  padding: 16px;
  margin: 20px 0;
}

.setup-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.status-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 4px;
  background: #a0a0a0;
  color: #121212;
}

.status-badge.checking { background: #f59e0b; color: #121212; }
.status-badge.acquiring { background: #3b82f6; color: #ffffff; }
.status-badge.verifying { background: #8b5cf6; color: #ffffff; }
.status-badge.extracting { background: #14b8a6; color: #ffffff; }
.status-badge.initializing { background: #06b6d4; color: #ffffff; }
.status-badge.ready { background: #10b981; color: #ffffff; }
.status-badge.failed { background: #ef4444; color: #ffffff; }

.setup-msg {
  font-size: 13px;
  font-weight: 600;
  color: #ffffff;
}

.progress-bar-mini {
  height: 6px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 3px;
  overflow: hidden;
  margin-bottom: 8px;
}

.progress-fill-mini {
  height: 100%;
  background: #6366f1;
  transition: width 0.3s ease;
}

.setup-detail {
  font-size: 11px;
  color: #888888;
  margin: 0;
}
</style>
