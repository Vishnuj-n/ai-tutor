<template>
  <div class="onboarding-overlay">
    <div class="onboarding-card">
      <div class="header-section">
        <div class="logo-orb">{{ appInitials }}</div>
        <h1>Welcome to {{ BRANDING.appName }}</h1>
        <p class="subtitle">Set up your persistent study workspace in seconds</p>
      </div>

      <div class="progress-bar">
        <div class="progress-fill" :style="{ width: `${(step - 1) * 20}%` }"></div>
      </div>

      <!-- Step 1: Profile and Goal -->
      <div v-if="step === 1" class="step-container">
        <h2>1. Create Your Study Profile</h2>
        <p class="description">
          Profiles group your textbooks and deadlines. E.g. "UPSC Prep" or "Semester Finals".
        </p>

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
          <input id="profile-deadline" v-model="profileDeadline" type="date" required />
        </div>

        <div class="form-group">
          <label for="max-flashcards">Max Flashcards per Session</label>
          <input
            id="max-flashcards"
            v-model.number="maxFlashcards"
            type="number"
            min="5"
            max="200"
            step="5"
            required
          />
          <p class="hint" style="margin-top: 4px; font-size: 0.85rem; opacity: 0.7">
            Caps spacing repetition reviews active in any single study session.
          </p>
        </div>

        <div class="time-range-section">
          <div class="time-range-header">
            <label>Study Schedule</label>
            <span v-if="studyDuration" class="duration-badge">{{ studyDuration }}</span>
          </div>

          <div class="time-range-container">
            <div class="time-input-group">
              <label for="study-start-time" class="time-label">Start</label>
              <div class="time-input-wrapper">
                <svg class="time-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="12" cy="12" r="10"/>
                  <polyline points="12,6 12,12 16,14"/>
                </svg>
                <input
                  id="study-start-time"
                  v-model="studyStartTime"
                  type="time"
                  class="time-input"
                  required
                />
              </div>
            </div>

            <div class="time-connector">
              <svg viewBox="0 0 24 8" fill="none" stroke="currentColor" stroke-width="1.5">
                <path d="M0 4 L20 4 M16 1 L20 4 L16 7"/>
              </svg>
            </div>

            <div class="time-input-group">
              <label for="study-end-time" class="time-label">End</label>
              <div class="time-input-wrapper">
                <svg class="time-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="12" cy="12" r="10"/>
                  <polyline points="12,6 12,12 16,14"/>
                </svg>
                <input
                  id="study-end-time"
                  v-model="studyEndTime"
                  type="time"
                  class="time-input"
                  required
                />
              </div>
            </div>
          </div>

          <div class="quick-durations">
            <button
              v-for="preset in durationPresets"
              :key="preset.label"
              type="button"
              class="duration-preset"
              :class="{ active: studyDuration === preset.label }"
              @click="applyDurationPreset(preset)"
            >
              {{ preset.label }}
            </button>
          </div>
        </div>

        <div
          class="form-group check-group"
          style="margin-bottom: 24px; display: flex; align-items: flex-start; gap: 8px"
        >
          <label
            class="checkbox-container"
            style="display: flex; align-items: center; gap: 10px; cursor: pointer"
          >
            <input
              id="reminders-enabled"
              v-model="remindersEnabled"
              type="checkbox"
              style="width: 18px; height: 18px; cursor: pointer"
            />
            <div class="check-label">
              <strong>Enable Study Reminders</strong>
              <p class="hint" style="margin: 2px 0 0 0; font-size: 0.85rem; opacity: 0.7">
                Notify when daily study time starts and ends.
              </p>
            </div>
          </label>
        </div>

        <button class="action-button" :disabled="!isStep1Valid" @click="step = 2">Next Step</button>
      </div>

      <!-- Step 2: LLM Provider -->
      <div v-else-if="step === 2" class="step-container">
        <h2>2. AI Provider</h2>
        <p class="description">
          Choose an OpenAI-compatible provider. API keys are stored in your OS credential manager,
          not SQLite.
        </p>

        <div class="form-group">
          <label for="llm-provider">Provider</label>
          <select
            id="llm-provider"
            v-model="llmFast.provider"
            @change="applyProviderPreset('fast')"
          >
            <option value="groq">Groq</option>
            <option value="openai">ChatGPT / OpenAI</option>
            <option value="openrouter">OpenRouter</option>
            <option value="custom">Custom OpenAI-compatible</option>
          </select>
        </div>

        <div class="form-group">
          <label for="llm-base-url">Base URL</label>
          <input
            id="llm-base-url"
            v-model="llmFast.base_url"
            type="url"
            placeholder="https://api.groq.com/openai"
          />
        </div>

        <div class="form-group">
          <label for="llm-model">Model</label>
          <input
            id="llm-model"
            v-model="llmFast.model"
            type="text"
            placeholder="openai/gpt-oss-120b"
          />
        </div>

        <div class="form-group">
          <label for="llm-api-key">API Key</label>
          <input
            id="llm-api-key"
            v-model="llmFastKey"
            type="password"
            placeholder="Paste key to save in OS credential manager"
          />
        </div>

        <label class="inline-check">
          <input v-model="useSameLLMForHeavy" type="checkbox" />
          <span>Use same provider and model for heavy AI tasks</span>
        </label>

        <div v-if="!useSameLLMForHeavy" class="advanced-box">
          <div class="form-group">
            <label for="heavy-provider">Heavy Provider</label>
            <select
              id="heavy-provider"
              v-model="llmHeavy.provider"
              @change="applyProviderPreset('heavy')"
            >
              <option value="groq">Groq</option>
              <option value="openai">ChatGPT / OpenAI</option>
              <option value="openrouter">OpenRouter</option>
              <option value="custom">Custom OpenAI-compatible</option>
            </select>
          </div>
          <div class="form-group">
            <label for="heavy-base-url">Heavy Base URL</label>
            <input id="heavy-base-url" v-model="llmHeavy.base_url" type="url" />
          </div>
          <div class="form-group">
            <label for="heavy-model">Heavy Model</label>
            <input id="heavy-model" v-model="llmHeavy.model" type="text" />
          </div>
          <div class="form-group">
            <label for="heavy-api-key">Heavy API Key</label>
            <input
              id="heavy-api-key"
              v-model="llmHeavyKey"
              type="password"
              placeholder="Leave blank to use the fast key"
            />
          </div>
        </div>

        <div
          v-if="error"
          class="error-banner"
          style="
            margin-bottom: 15px;
            display: flex;
            justify-content: space-between;
            align-items: center;
          "
        >
          <span>{{ error }}</span>
          <button
            type="button"
            style="
              background: none;
              border: none;
              color: inherit;
              font-size: 16px;
              cursor: pointer;
              padding: 0 4px;
              line-height: 1;
            "
            @click="error = ''"
          >
            &times;
          </button>
        </div>

        <div class="button-row">
          <button class="secondary-button" @click="step = 1">Back</button>
          <button
            class="action-button"
            :disabled="llmSaving || presetLoading || !isLLMStepValid"
            @click="saveLLMAndContinue"
          >
            {{ llmSaving ? 'Saving AI Settings...' : 'Next Step' }}
          </button>
        </div>
      </div>

      <!-- Step 3: Cloud Sync Settings -->
      <div v-else-if="step === 3" class="step-container">
        <h2>3. Teacher Cloud Sync (Optional)</h2>
        <p class="description">
          If your teacher sends assigned books and tracks progress, enter the sync details below.
        </p>

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
          <button class="secondary-button" @click="step = 2">Back</button>
          <button class="action-button" @click="step = 4">Next Step</button>
        </div>
      </div>

      <!-- Step 4: RAG Settings -->
      <div v-else-if="step === 4" class="step-container">
        <h2>4. Local AI Retrieval</h2>
        <p class="description">
          Enable smart, context-aware helper tools. This sets up a local search and query system to
          ask questions about your textbooks completely offline.
        </p>

        <div class="rag-options">
          <label class="rag-option-card" :class="{ active: wantRag }">
            <input v-model="wantRag" type="radio" :value="true" :disabled="isSettingUpRag" />
            <div class="option-info">
              <strong>Yes, Enable Local AI Search (Recommended)</strong>
              <p>
                Download and configure the offline search system (~152 MB). Requires Windows x64.
              </p>
            </div>
          </label>

          <label class="rag-option-card" :class="{ active: !wantRag }">
            <input v-model="wantRag" type="radio" :value="false" :disabled="isSettingUpRag" />
            <div class="option-info">
              <strong>No, Skip Offline Search</strong>
              <p>AI Q&A will be limited in the reader, falling back to simple keyword matching.</p>
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
          <button class="secondary-button" :disabled="isSettingUpRag" @click="step = 3">
            Back
          </button>

          <button
            v-if="wantRag && !ragSetupCompleted"
            class="action-button"
            :disabled="isSettingUpRag"
            @click="startRagSetup"
          >
            {{ isSettingUpRag ? 'Setting Up...' : 'Initialize Local AI' }}
          </button>

          <button v-else class="action-button" @click="step = 5">Next Step</button>
        </div>
      </div>

      <!-- Step 5: Aesthetics -->
      <div v-else-if="step === 5" class="step-container">
        <h2>5. Choose Workspace Aesthetic</h2>
        <p class="description">
          Select a visual theme. Changing themes alters the colors of your study desk in real-time.
        </p>

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
          <button class="secondary-button" @click="step = 4">Back</button>
          <button class="action-button" :disabled="loading" @click="completeOnboarding">
            {{ loading ? 'Configuring...' : 'Initialize Workspace' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onUnmounted, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { BRANDING } from '../config/branding'
import {
  createProfile,
  updateUserSettings,
  initializeRAG,
  updateLLMSettings,
  saveLLMAPIKey,
  getLLMProviderPreset,
} from '../services/appApi'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

const router = useRouter()
const step = ref(1)
const loading = ref(false)
const error = ref('')

const appInitials = computed(() => {
  const name = BRANDING.appName || ''
  const matches = name.match(/[A-Z]/g)
  if (matches && matches.length > 0) {
    return matches.slice(0, 2).join('')
  }
  const words = name.split(/[\s-_]+/)
  if (words.length > 1) {
    return (words[0][0] + words[1][0]).toUpperCase()
  }
  return name.slice(0, 2).toUpperCase() || 'APP'
})

const profileName = ref('')
const profileDeadline = ref('')
const maxFlashcards = ref(30)
const studyStartTime = ref('17:00')
const studyEndTime = ref('18:00')

const durationPresets = [
  { label: '30 min', minutes: 30 },
  { label: '1 hour', minutes: 60 },
  { label: '1.5 hours', minutes: 90 },
  { label: '2 hours', minutes: 120 },
  { label: '3 hours', minutes: 180 },
]

const studyDuration = computed(() => {
  if (!studyStartTime.value || !studyEndTime.value) return ''
  const [startH, startM] = studyStartTime.value.split(':').map(Number)
  const [endH, endM] = studyEndTime.value.split(':').map(Number)
  const startMinutes = startH * 60 + startM
  const endMinutes = endH * 60 + endM
  const diff = endMinutes - startMinutes
  if (diff <= 0) return ''

  if (diff < 60) return `${diff} min`
  const hours = Math.floor(diff / 60)
  const mins = diff % 60
  if (mins === 0) return hours === 1 ? '1 hour' : `${hours} hours`
  return `${hours}h ${mins}m`
})

function applyDurationPreset(preset) {
  const [h, m] = studyStartTime.value.split(':').map(Number)
  const startMinutes = h * 60 + m
  const endMinutes = startMinutes + preset.minutes
  const endH = Math.floor(endMinutes / 60) % 24
  const endM = endMinutes % 60
  studyEndTime.value = `${String(endH).padStart(2, '0')}:${String(endM).padStart(2, '0')}`
}
const remindersEnabled = ref(true)
const cloudSyncURL = ref('')
const apiToken = ref('')
const selectedTheme = ref('light-classic')
// Theme selection states (background handled automatically via css custom variables)
const llmSaving = ref(false)
const presetLoading = ref(false)
const useSameLLMForHeavy = ref(true)
const llmFastKey = ref('')
const llmHeavyKey = ref('')
// Password visibility state handled natively by browser
const llmFast = ref({
  tier: 'fast',
  provider: 'groq',
  base_url: 'https://api.groq.com/openai',
  model: 'openai/gpt-oss-120b',
  timeout_ms: 60000,
  api_key_source: 'keyring',
  has_api_key: false,
})
const llmHeavy = ref({
  tier: 'heavy',
  provider: 'groq',
  base_url: 'https://api.groq.com/openai',
  model: 'openai/gpt-oss-120b',
  timeout_ms: 90000,
  api_key_source: 'keyring',
  has_api_key: false,
})

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
  return (
    profileName.value.trim() !== '' &&
    profileDeadline.value !== '' &&
    maxFlashcards.value >= 5 &&
    maxFlashcards.value <= 200 &&
    studyStartTime.value !== '' &&
    studyEndTime.value !== '' &&
    studyStartTime.value < studyEndTime.value
  )
})

const isLLMStepValid = computed(() => {
  const fastValid = llmFast.value.base_url.trim() !== '' && llmFast.value.model.trim() !== ''
  const heavyValid =
    useSameLLMForHeavy.value ||
    (llmHeavy.value.base_url.trim() !== '' && llmHeavy.value.model.trim() !== '')
  return fastValid && heavyValid
})

async function applyProviderPreset(tier) {
  presetLoading.value = true
  error.value = ''
  const target = tier === 'heavy' ? llmHeavy.value : llmFast.value
  try {
    const preset = await getLLMProviderPreset(target.provider)
    target.base_url = preset.base_url
    target.model = preset.model
  } catch (err) {
    error.value = err.message || `Failed to load preset for ${target.provider}.`
  } finally {
    presetLoading.value = false
  }
}

async function saveLLMAndContinue() {
  if (presetLoading.value || error.value) return
  error.value = ''
  llmSaving.value = true
  try {
    const fast = { ...llmFast.value, has_api_key: llmFastKey.value.trim() !== '' }
    const heavy = useSameLLMForHeavy.value
      ? {
          ...llmFast.value,
          tier: 'heavy',
          timeout_ms: 90000,
          has_api_key: llmFastKey.value.trim() !== '',
        }
      : {
          ...llmHeavy.value,
          has_api_key: llmHeavyKey.value.trim() !== '' || llmFastKey.value.trim() !== '',
        }

    const settingsRes = await updateLLMSettings({
      use_same_for_heavy: useSameLLMForHeavy.value,
      fast,
      heavy,
    })
    if (settingsRes.error) {
      error.value = settingsRes.error
      return
    }

    if (llmFastKey.value.trim()) {
      const keyRes = await saveLLMAPIKey('fast', llmFastKey.value.trim())
      if (keyRes.error) {
        error.value = keyRes.error
        return
      }
      if (useSameLLMForHeavy.value) {
        const heavyKeyRes = await saveLLMAPIKey('heavy', llmFastKey.value.trim())
        if (heavyKeyRes.error) {
          error.value = heavyKeyRes.error
          return
        }
      }
    }
    if (!useSameLLMForHeavy.value) {
      const heavyKeyValue = llmHeavyKey.value.trim() || llmFastKey.value.trim()
      if (heavyKeyValue) {
        const keyRes = await saveLLMAPIKey('heavy', heavyKeyValue)
        if (keyRes.error) {
          error.value = keyRes.error
          return
        }
      }
    }
    step.value = 3
  } catch (err) {
    error.value = err.message || 'Failed to save AI provider settings.'
  } finally {
    llmSaving.value = false
  }
}

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
      EventsOff('rag-setup-progress')
    }

    if (data.status === 'ready') {
      ragSetupCompleted.value = true
      isSettingUpRag.value = false
      EventsOff('rag-setup-progress')
      setTimeout(() => {
        step.value = 5
      }, 1000)
    }
  })

  initializeRAG()
    .then((res) => {
      if (res.error) {
        ragError.value = res.error
        isSettingUpRag.value = false
        EventsOff('rag-setup-progress')
      }
    })
    .catch((err) => {
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
      maxFlashcards.value,
      studyStartTime.value,
      studyEndTime.value,
      remindersEnabled.value,
      newProfile.id,
      false, // skip to reading off by default
      cloudSyncURL.value.trim(),
      apiToken.value.trim(),
      selectedTheme.value,
      wantRag.value && ragSetupCompleted.value,
      true, // default for ragNotebookChapter
      true, // default for ragEntireNotebook
      true, // default for ragQueueStudy
      'CLASSIC', // default defaultRemedialStrategy
      '' // classroom_code — set later from Settings
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

onMounted(() => {
  selectTheme(selectedTheme.value)
})
</script>

<style scoped>
.onboarding-overlay {
  position: fixed;
  inset: 0;
  background: var(--background);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  padding: 24px; /* 8px grid */
  color: var(--on-surface);
  font-family: 'Inter', sans-serif;
}

.onboarding-card {
  width: 100%;
  max-width: 480px; /* slightly narrower */
  max-height: calc(100vh - 48px); /* Keep 24px margin top and bottom */
  overflow-y: auto;
  border-radius: 16px; /* 2 * 8px - more compact */
  padding: 24px; /* 3 * 8px - reduced padding for a tighter fit */
  box-shadow: 0 16px 32px rgba(45, 51, 56, 0.08);
  box-sizing: border-box;
  background: var(--surface-container-lowest);
  transition: background 0.3s ease;
  border: none; /* No-line rule */
}

/* Custom scrollbar for premium look */
.onboarding-card::-webkit-scrollbar {
  width: 6px;
}

.onboarding-card::-webkit-scrollbar-track {
  background: transparent;
}

.onboarding-card::-webkit-scrollbar-thumb {
  background: var(--outline-variant);
  border-radius: 3px;
}

.header-section {
  text-align: center;
  margin-bottom: 16px; /* 8px grid */
}

.logo-orb {
  width: 48px; /* 8px grid */
  height: 48px; /* 8px grid */
  margin: 0 auto 8px; /* 8px grid */
  background: linear-gradient(135deg, var(--primary-dim), var(--primary));
  color: var(--on-primary);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: 'Manrope', sans-serif;
  font-weight: 800;
  font-size: 16px;
  letter-spacing: -0.05em;
  box-shadow: 0 0 24px rgba(99, 102, 241, 0.15); /* 8px grid shadow size */
}

h1 {
  font-family: 'Manrope', sans-serif; /* Display font */
  font-size: 22px;
  font-weight: 800;
  margin: 0 0 4px; /* 8px grid */
  letter-spacing: -0.02em; /* Spec: -2% tracking */
  color: var(--on-surface);
}

.subtitle {
  color: var(--muted-text);
  font-size: 14px;
  margin: 0;
}

.progress-bar {
  height: 6px; /* 8px grid */
  background: var(--outline-variant);
  border-radius: 3px;
  margin-bottom: 16px; /* 8px grid */
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.step-container {
  display: flex;
  flex-direction: column;
  gap: 16px; /* 8px grid */
}

h2 {
  font-family: 'Manrope', sans-serif; /* Display font */
  font-size: 16px; /* 8px scale */
  font-weight: 700;
  margin: 0;
  letter-spacing: -0.02em; /* Spec: -2% tracking */
}

.description {
  font-size: 13px;
  color: var(--muted-text);
  line-height: 1.5;
  margin: -8px 0 8px; /* 8px grid */
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px; /* 8px grid */
}

.form-group:focus-within label {
  color: var(--primary); /* Spec focus shift */
}

/* Password wrapper styles removed in favor of native browser toggles */

label {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-text);
  transition: color 0.2s ease;
}

input,
select {
  background: var(--surface-container-low);
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent); /* Spec ghost border */
  border-radius: 12px; /* xl: 12px/0.75rem */
  padding: 10px 14px; /* 8px grid friendly - slightly more compact */
  color: var(--on-surface);
  font-size: 14px;
  font-family: inherit;
  transition:
    border-color 0.2s,
    background-color 0.2s;
  box-sizing: border-box;
  width: 100%;
}

input::placeholder {
  color: var(--muted-text);
  opacity: 0.6;
}

select option {
  color: var(--on-surface);
  background: var(--surface-container-lowest);
}

input:focus,
select:focus {
  outline: none;
  border-color: var(--primary);
  background: var(--surface-container);
}

.inline-check {
  display: flex;
  align-items: center;
  gap: 8px; /* 8px grid */
  text-transform: none;
  letter-spacing: 0;
  color: var(--on-surface);
}

.inline-check input {
  width: auto;
}

.advanced-box {
  display: flex;
  flex-direction: column;
  gap: 16px; /* 8px grid */
  padding: 16px; /* 8px grid */
  border: none; /* No-line rule */
  border-radius: 12px; /* xl: 12px */
  background: var(--surface-container-low);
}

.action-button {
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  border: none;
  border-radius: 12px; /* xl: 12px */
  padding: 12px; /* 8px grid - reduced padding */
  color: var(--on-primary);
  font-family: 'Manrope', sans-serif;
  font-weight: 700;
  font-size: 14px;
  cursor: pointer;
  transition:
    opacity 0.2s,
    transform 0.2s;
  width: 100%;
  margin-top: 4px; /* 8px grid */
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
  gap: 16px; /* 8px grid */
  margin-top: 8px; /* 8px grid */
}

.secondary-button {
  background: var(--surface-container-highest);
  border: none;
  border-radius: 12px; /* xl: 12px */
  padding: 12px; /* 8px grid - reduced padding */
  color: var(--primary);
  font-family: 'Manrope', sans-serif;
  font-weight: 700;
  font-size: 14px;
  cursor: pointer;
  transition: background 0.2s;
  flex: 1;
  text-align: center;
}

.secondary-button:hover {
  background: var(--surface-container-low);
}

.error-banner {
  background: rgba(159, 64, 61, 0.06); /* Soft red tinted using spec error color */
  border: none; /* No-line rule */
  color: #9f403d; /* SPEC: error (#9f403d) */
  padding: 16px; /* 8px grid */
  border-radius: 12px; /* xl: 12px */
  font-size: 13px;
  font-weight: 600;
  text-align: center;
}

/* Theme Selector Grid */
.theme-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: 16px; /* 8px grid */
  margin: 8px 0; /* 8px grid */
}

.theme-card {
  background: var(--surface-container-low);
  border: none; /* No-line rule */
  border-radius: 12px; /* xl: 12px */
  padding: 16px; /* 8px grid */
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px; /* 8px grid */
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  width: 100%;
  color: var(--on-surface);
}

.theme-card:hover {
  background: var(--surface-container-lowest);
  box-shadow: 0 8px 16px color-mix(in srgb, var(--on-surface) 6%, transparent);
}

.theme-card.active {
  background: var(--surface-container-lowest);
  box-shadow: 0 0 0 2px var(--primary);
}

.theme-preview {
  width: 100%;
  height: 48px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px; /* 8px grid */
  border: none; /* No-line rule */
}

.preview-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.theme-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-text);
  transition: color 0.2s ease, font-weight 0.2s ease;
}

.theme-card.active .theme-label {
  color: var(--on-surface);
  font-weight: 700;
}

/* Theme Previews */
.theme-preview.light-classic {
  background: #f9f9fb;
}
.theme-preview.light-classic .preview-dot.primary {
  background: #005bc1;
}
.theme-preview.light-classic .preview-dot.surface {
  background: #ebeef2;
}

.theme-preview.light-warm {
  background: #fdfaf6;
}
.theme-preview.light-warm .preview-dot.primary {
  background: #c27d38;
}
.theme-preview.light-warm .preview-dot.surface {
  background: #f3eae1;
}

.theme-preview.dark-indigo {
  background: #0b0d16;
}
.theme-preview.dark-indigo .preview-dot.primary {
  background: #6366f1;
}
.theme-preview.dark-indigo .preview-dot.surface {
  background: #171a2b;
}

.theme-preview.dark-nord {
  background: #2e3440;
}
.theme-preview.dark-nord .preview-dot.primary {
  background: #88c0d0;
}
.theme-preview.dark-nord .preview-dot.surface {
  background: #3b4252;
}

.theme-preview.dark-emerald {
  background: #0a120d;
}
.theme-preview.dark-emerald .preview-dot.primary {
  background: #10b981;
}
.theme-preview.dark-emerald .preview-dot.surface {
  background: #152219;
}

/* RAG Options Stylings */
.rag-options {
  display: flex;
  flex-direction: column;
  gap: 16px; /* 8px grid */
  margin: 24px 0; /* 8px grid */
}

.rag-option-card {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 16px;
  border-radius: 12px; /* xl: 12px */
  background: var(--surface-container-low);
  border: none; /* No-line rule */
  cursor: pointer;
  transition: all 0.2s ease;
  color: var(--on-surface);
}

.rag-option-card:hover {
  background: var(--surface-container-lowest);
  box-shadow: 0 8px 16px color-mix(in srgb, var(--on-surface) 6%, transparent);
}

.rag-option-card.active {
  background: var(--surface-container-lowest);
  box-shadow: 0 0 0 2px var(--primary);
}

.rag-option-card input[type='radio'] {
  margin-top: 4px;
  accent-color: var(--primary);
}

.option-info strong {
  display: block;
  font-size: 14px;
  color: var(--on-surface);
  margin-bottom: 4px;
}

.option-info p {
  font-size: 12px;
  color: var(--muted-text);
  margin: 0;
}

.rag-setup-box {
  background: var(--surface-container-low);
  border: none; /* No-line rule */
  border-radius: 12px; /* xl: 12px */
  padding: 16px;
  margin: 24px 0; /* 8px grid */
  color: var(--on-surface);
}

.setup-header {
  display: flex;
  align-items: center;
  gap: 8px; /* 8px grid */
  margin-bottom: 8px; /* 8px grid */
}

.status-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 8px; /* 8px grid */
  border-radius: 4px;
  background: #a0a0a0;
  color: #121212;
}

.status-badge.checking {
  background: #f59e0b;
  color: #121212;
}
.status-badge.acquiring {
  background: #3b82f6;
  color: #ffffff;
}
.status-badge.verifying {
  background: #8b5cf6;
  color: #ffffff;
}
.status-badge.extracting {
  background: #14b8a6;
  color: #ffffff;
}
.status-badge.initializing {
  background: #06b6d4;
  color: #ffffff;
}
.status-badge.ready {
  background: #10b981;
  color: #ffffff;
}
.status-badge.failed {
  background: #ef4444;
  color: #ffffff;
}

.setup-msg {
  font-size: 13px;
  font-weight: 600;
  color: var(--on-surface);
}

.progress-bar-mini {
  height: 8px; /* 8px grid */
  background: var(--outline-variant);
  border-radius: 4px;
  overflow: hidden;
  margin-bottom: 8px;
}

.progress-fill-mini {
  height: 100%;
  background: var(--primary);
  transition: width 0.3s ease;
}

.setup-detail {
  font-size: 11px;
  color: var(--muted-text);
  margin: 0;
}

/* Time Range Section */
.time-range-section {
  background: var(--surface-container-low);
  border-radius: 16px;
  padding: 20px;
  margin-bottom: 20px;
}

.time-range-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.time-range-header label {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-text);
}

.duration-badge {
  background: color-mix(in srgb, var(--primary) 12%, transparent);
  color: var(--primary);
  font-size: 12px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: 20px;
  letter-spacing: 0.02em;
}

.time-range-container {
  display: flex;
  align-items: flex-end;
  gap: 12px;
}

.time-input-group {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.time-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-text);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.time-input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.time-icon {
  position: absolute;
  left: 14px;
  width: 18px;
  height: 18px;
  color: var(--muted-text);
  pointer-events: none;
  transition: color 0.2s ease;
}

.time-input {
  width: 100%;
  padding: 14px 14px 14px 42px;
  background: var(--surface-container-lowest);
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  border-radius: 12px;
  font-size: 16px;
  font-weight: 600;
  font-family: inherit;
  color: var(--on-surface);
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.time-input:focus {
  outline: none;
  border-color: var(--primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 12%, transparent);
}

.time-input:focus ~ .time-icon,
.time-input-wrapper:focus-within .time-icon {
  color: var(--primary);
}

.time-connector {
  flex-shrink: 0;
  width: 32px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--muted-text);
  opacity: 0.5;
}

.time-connector svg {
  width: 100%;
  height: 8px;
}

.quick-durations {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid color-mix(in srgb, var(--outline-variant) 30%, transparent);
}

.duration-preset {
  background: var(--surface-container-lowest);
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  border-radius: 8px;
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-text);
  cursor: pointer;
  transition: all 0.2s ease;
}

.duration-preset:hover {
  background: var(--surface-container);
  border-color: color-mix(in srgb, var(--primary) 30%, transparent);
  color: var(--on-surface);
}

.duration-preset.active {
  background: color-mix(in srgb, var(--primary) 10%, transparent);
  border-color: color-mix(in srgb, var(--primary) 40%, transparent);
  color: var(--primary);
}

@media (max-width: 480px) {
  .time-range-container {
    flex-direction: column;
    align-items: stretch;
  }

  .time-connector {
    transform: rotate(90deg);
    width: 100%;
    height: 24px;
  }

  .quick-durations {
    justify-content: center;
  }
}
</style>
