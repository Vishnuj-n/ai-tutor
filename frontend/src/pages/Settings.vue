<template>
  <section class="page">
    <p class="eyebrow">Settings & Profiles</p>
    <h1>Workspace Configuration</h1>

    <div class="tabs">
      <button
        class="tab-btn"
        :class="{ active: activeTab === 'settings' }"
        @click="activeTab = 'settings'"
      >
        General Settings
      </button>
      <button
        class="tab-btn"
        :class="{ active: activeTab === 'profiles' }"
        @click="activeTab = 'profiles'"
      >
        Study Profiles
      </button>
    </div>

    <!-- General Settings Tab -->
    <div v-if="activeTab === 'settings'" class="tab-content">
      <div class="settings-panels">
        <!-- Panel 1: Study Routine -->
        <article class="panel form-grid">
          <h2>Study Budget & Routine</h2>

          <div class="form-group">
            <label for="max-flashcards">Max Flashcards per Session</label>
            <input
              id="max-flashcards"
              v-model.number="settings.max_flashcards_per_session"
              type="number"
              min="5"
              max="200"
              step="5"
              :disabled="loading || saving"
              required
            />
            <p class="hint">Caps the number of FSRS reviews active in any single study session.</p>
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
                    v-model="settings.study_start_time"
                    type="time"
                    class="time-input"
                    :disabled="loading || saving"
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
                    v-model="settings.study_end_time"
                    type="time"
                    class="time-input"
                    :disabled="loading || saving"
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
                :disabled="loading || saving"
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
                v-model="settings.reminders_enabled"
                type="checkbox"
                :disabled="loading || saving"
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

          <div class="form-group check-group">
            <label class="checkbox-container">
              <input
                v-model="settings.skip_to_reading_active"
                type="checkbox"
                :disabled="loading || saving"
              />
              <span class="checkmark"></span>
              <div class="check-label">
                <strong>Enable "Skip to Reading" (Escape Hatch)</strong>
                <p class="hint">
                  Temporarily deprioritizes review backlogs, letting you read new material first. FSRS
                  records remain safe.
                </p>
              </div>
            </label>
          </div>
        </article>

        <!-- Panel 2: Quiz Failure Rescue -->
        <article class="panel form-grid">
          <h2>Quiz Failure Rescue</h2>
          <p class="hint" style="margin-top: -10px; margin-bottom: 8px">
            Choose what happens when you fail a quiz. Customize the remediation track to match your
            study style.
          </p>

          <div class="form-group">
            <div class="strategy-options">
              <label
                class="strategy-option"
                :class="{ active: settings.default_remedial_strategy === 'CLASSIC' }"
              >
                <input
                  v-model="settings.default_remedial_strategy"
                  type="radio"
                  value="CLASSIC"
                  :disabled="loading || saving"
                  style="cursor: pointer"
                />
                <div class="option-content">
                  <span class="option-title">Classic Track</span>
                  <span class="option-desc"
                    >Reread first, then Socratic tutor if you fail again (dense text, sequential
                    learning)</span
                  >
                </div>
              </label>

              <label
                class="strategy-option"
                :class="{ active: settings.default_remedial_strategy === 'FAST' }"
              >
                <input
                  v-model="settings.default_remedial_strategy"
                  type="radio"
                  value="FAST"
                  :disabled="loading || saving"
                  style="cursor: pointer"
                />
                <div class="option-content">
                  <span class="option-title">Fast Track</span>
                  <span class="option-desc"
                    >Go directly to Socratic AI tutor (deeper encoding, conceptual topics)</span
                  >
                </div>
              </label>
            </div>
          </div>

          <div class="form-group check-group">
            <label class="checkbox-container">
              <input
                v-model="settings.rag_enabled"
                type="checkbox"
                :disabled="loading || saving"
                @change="onRagToggle"
              />
              <span class="checkmark"></span>
              <div class="check-label">
                <strong>Enable Local AI Retrieval (RAG)</strong>
                <p class="hint">
                  Preloads local ONNX embeddings for context-rich Q&A. Unticking unloads RAG from
                  memory instantly.
                </p>
              </div>
            </label>
          </div>

          <div
            v-if="settings.rag_enabled"
            class="rag-sub-settings"
            style="
              margin-left: 28px;
              display: flex;
              flex-direction: column;
              gap: 12px;
              margin-bottom: 8px;
            "
          >
            <div class="form-group check-group" style="margin-bottom: 0">
              <label class="checkbox-container">
                <input
                  v-model="settings.rag_notebook_chapter"
                  type="checkbox"
                  :disabled="loading || saving"
                />
                <span class="checkmark"></span>
                <div class="check-label">
                  <strong>Enable Tutor from Notebook Chapters</strong>
                  <p class="hint">
                    Allows accessing Socratic RAG directly from notebook chapter details.
                  </p>
                </div>
              </label>
            </div>

            <div class="form-group check-group" style="margin-bottom: 0">
              <label class="checkbox-container">
                <input
                  v-model="settings.rag_entire_notebook"
                  type="checkbox"
                  :disabled="loading || saving"
                />
                <span class="checkmark"></span>
                <div class="check-label">
                  <strong>Enable RAG for Entire Book</strong>
                  <p class="hint">
                    Allows general queries scoped to the selected notebook in the Tutor interface.
                  </p>
                </div>
              </label>
            </div>

            <div class="form-group check-group" style="margin-bottom: 0">
              <label class="checkbox-container">
                <input
                  v-model="settings.rag_queue_study"
                  type="checkbox"
                  :disabled="loading || saving"
                />
                <span class="checkmark"></span>
                <div class="check-label">
                  <strong>Enable Tutor in Queue Study Sessions</strong>
                  <p class="hint">Shows an optional Tutor panel inside active reading tasks.</p>
                </div>
              </label>
            </div>
          </div>
        </article>

        <!-- Panel 3: AI Provider -->
        <article class="panel form-grid">
          <h2>AI Provider</h2>
          <p class="hint" style="margin-top: -10px; margin-bottom: 8px">
            Provider settings are saved in SQLite. API keys are saved in the OS credential manager
            through the backend.
          </p>

          <div class="form-group">
            <label for="settings-llm-provider">Provider</label>
            <select
              id="settings-llm-provider"
              v-model="llmSettings.fast.provider"
              :disabled="loading || savingLLM"
              @change="applyProviderPreset('fast')"
            >
              <option value="groq">Groq</option>
              <option value="openai">ChatGPT / OpenAI</option>
              <option value="openrouter">OpenRouter</option>
              <option value="custom">Custom OpenAI-compatible</option>
            </select>
          </div>

          <div class="form-group">
            <label for="settings-llm-base-url">Base URL</label>
            <input
              id="settings-llm-base-url"
              v-model="llmSettings.fast.base_url"
              type="url"
              :disabled="loading || savingLLM"
            />
          </div>

          <div class="form-group">
            <label for="settings-llm-model">Fast Model</label>
            <input
              id="settings-llm-model"
              v-model="llmSettings.fast.model"
              type="text"
              :disabled="loading || savingLLM"
            />
            <p class="hint">Used for quizzes, flashcards, short scoring, and small reader help.</p>
          </div>

          <div class="form-group">
            <label for="settings-llm-key">Fast API Key</label>
            <input
              id="settings-llm-key"
              v-model="llmFastKey"
              type="password"
              placeholder="Leave blank to keep existing key"
              :disabled="loading || savingLLM"
            />
            <p class="hint">
              {{
                llmSettings.fast.has_api_key
                  ? 'A fast-tier key is stored.'
                  : 'No fast-tier key stored yet.'
              }}
            </p>
          </div>

          <div class="form-group check-group">
            <label class="checkbox-container">
              <input
                v-model="llmSettings.use_same_for_heavy"
                type="checkbox"
                :disabled="loading || savingLLM"
              />
              <span class="checkmark"></span>
              <div class="check-label">
                <strong>Use same provider and model for heavy AI tasks</strong>
                <p class="hint">
                  Heavy tasks include syllabus drafting, Socratic responses, and large-context
                  generation.
                </p>
              </div>
            </label>
          </div>

          <div v-if="!llmSettings.use_same_for_heavy" class="llm-advanced">
            <div class="form-group">
              <label for="settings-heavy-provider">Heavy Provider</label>
              <select
                id="settings-heavy-provider"
                v-model="llmSettings.heavy.provider"
                :disabled="loading || savingLLM"
                @change="applyProviderPreset('heavy')"
              >
                <option value="groq">Groq</option>
                <option value="openai">ChatGPT / OpenAI</option>
                <option value="openrouter">OpenRouter</option>
                <option value="custom">Custom OpenAI-compatible</option>
              </select>
            </div>
            <div class="form-group">
              <label for="settings-heavy-base-url">Heavy Base URL</label>
              <input
                id="settings-heavy-base-url"
                v-model="llmSettings.heavy.base_url"
                type="url"
                :disabled="loading || savingLLM"
              />
            </div>
            <div class="form-group">
              <label for="settings-heavy-model">Heavy Model</label>
              <input
                id="settings-heavy-model"
                v-model="llmSettings.heavy.model"
                type="text"
                :disabled="loading || savingLLM"
              />
            </div>
            <div class="form-group">
              <label for="settings-heavy-key">Heavy API Key</label>
              <input
                id="settings-heavy-key"
                v-model="llmHeavyKey"
                type="password"
                placeholder="Leave blank to keep existing key"
                :disabled="loading || savingLLM"
              />
              <p class="hint">
                {{
                  llmSettings.heavy.has_api_key
                    ? 'A heavy-tier key is stored.'
                    : 'No heavy-tier key stored yet.'
                }}
              </p>
            </div>
          </div>

          <div class="button-row">
            <button
              type="button"
              class="sync-btn"
              :disabled="loading || savingLLM"
              @click="removeLLMKeys"
            >
              Remove Stored Keys
            </button>
          </div>
        </article>

        <!-- Panel 4: Workspace Aesthetics -->
        <article class="panel form-grid">
          <h2>Workspace Aesthetics</h2>
          <div class="form-group">
            <label for="theme-select">Aesthetic Theme</label>
            <select id="theme-select" v-model="settings.theme" :disabled="loading || saving">
              <option value="light-classic">Light Classic</option>
              <option value="light-warm">Warm Sepia (Reader)</option>
              <option value="dark-indigo">Deep Indigo Night (Dark Mode)</option>
              <option value="dark-nord">Nord Frost (Cool Dark Mode)</option>
              <option value="dark-emerald">Forest Emerald</option>
            </select>
            <p class="hint">
              Select a visual theme. Changing themes alters the colors of your study desk instantly.
            </p>
          </div>
        </article>

        <!-- Panel 5: Account & Cloud -->
        <article class="panel form-grid">
          <h2>Account &amp; Cloud</h2>

          <!-- Signed In State -->
          <div v-if="settings.cloud_api_token" class="signed-in-box">
            <div class="status-indicator">
              <span class="pulse-dot active"></span>
              <strong>Cloud Sync Active</strong>
            </div>
            <div class="user-details">
              <p><strong>Username:</strong> {{ settings.student_username || 'Student' }}</p>
              <p><strong>Classroom:</strong> {{ settings.classroom_code }}</p>
            </div>
            <button
              type="button"
              class="sync-btn danger-btn"
              @click="handleLogout"
            >
              🚪 Sign Out
            </button>
          </div>

          <!-- Signed Out State (Login Form) -->
          <div v-else class="login-form-container">
            <p class="field-hint" style="margin-bottom: 1.25rem;">Sign in with your student credentials to enable cloud sync and receive assignments.</p>
            
            <div v-if="loginError" class="login-error-message">
              ⚠️ {{ loginError }}
            </div>

            <div class="form-group">
              <label for="student-username">Student Username / ID</label>
              <input
                id="student-username"
                v-model="loginUsername"
                type="text"
                placeholder="e.g. john_doe"
                :disabled="loggingIn"
              />
            </div>

            <div class="form-group">
              <label for="student-password">Password</label>
              <input
                id="student-password"
                v-model="loginPassword"
                type="password"
                placeholder="••••••••"
                :disabled="loggingIn"
              />
            </div>

            <div class="form-group">
              <label for="student-classroom">Classroom Code</label>
              <input
                id="student-classroom"
                v-model="loginClassroomCode"
                type="text"
                placeholder="e.g. BIO101"
                :disabled="loggingIn"
              />
            </div>

            <button
              type="button"
              class="sync-btn"
              :disabled="loggingIn"
              @click="handleLogin"
            >
              {{ loggingIn ? 'Signing In...' : '🔐 Sign In & Sync' }}
            </button>
          </div>

          <!-- Sync Server URL — dev only -->
          <div v-if="isDev" class="form-group" style="margin-top: 1.5rem; border-top: 1px solid var(--border); padding-top: 1.5rem;">
            <label for="cloud-url">
              Sync Server URL
              <span class="dev-badge">DEV</span>
            </label>
            <input
              id="cloud-url"
              v-model="settings.cloud_sync_url"
              type="url"
              placeholder="https://example.com/api/sync"
              :disabled="loading || saving"
            />
          </div>
        </article>
      </div>

      <!-- Global Actions (Sync actions) -->
      <div class="global-actions">
        <div class="button-row">
          <button
            v-if="cloudConfigured"
            type="button"
            class="sync-btn"
            :disabled="syncing"
            @click="runManualSync"
          >
            {{ syncing ? 'Syncing...' : 'Sync with Cloud Now' }}
          </button>
        </div>

        <p v-if="error" class="error-text">{{ error }}</p>
        <p v-if="success" class="success-text">{{ success }}</p>
      </div>
    </div>

    <!-- Study Profiles Tab -->
    <div v-else-if="activeTab === 'profiles'" class="tab-content">
      <div class="profiles-layout">
        <!-- Profiles List -->
        <article class="panel profiles-panel">
          <div class="panel-header">
            <h2>Profiles & Targets</h2>
            <button class="add-profile-btn" @click="showAddModal = true">+ Create Profile</button>
          </div>

          <div v-if="profiles.length === 0" class="empty-state">
            No profiles defined. Create one to organize textbooks.
          </div>

          <div v-else class="profiles-list">
            <div
              v-for="profile in profiles"
              :key="profile.id"
              class="profile-card"
              :class="{ active: settings.active_profile_id === profile.id }"
            >
              <div class="profile-info">
                <h3>{{ profile.name }}</h3>
                <p class="deadline">
                  Deadline: <strong>{{ formatUnixDate(profile.deadline_at) }}</strong>
                </p>
              </div>
              <div class="profile-actions">
                <button
                  v-if="settings.active_profile_id !== profile.id"
                  class="select-btn"
                  @click="setActiveProfile(profile.id)"
                >
                  Select Active
                </button>
                <span v-else class="active-badge">Active</span>

                <button class="edit-btn" @click="openEditModal(profile)">Edit</button>
                <button class="delete-btn" @click="handleDeleteProfile(profile.id)">Delete</button>
              </div>
            </div>
          </div>
        </article>

        <!-- Textbook Assignments -->
        <article class="panel textbooks-panel">
          <h2>Textbook Assignments</h2>
          <p class="description">
            Assign uploaded textbooks to study profiles to calculate target deadlines.
          </p>

          <div v-if="notebooks.length === 0" class="empty-state">
            No textbooks uploaded. Go to Bookshelf to add them.
          </div>

          <table v-else class="textbooks-table">
            <thead>
              <tr>
                <th>Textbook Title</th>
                <th>Assigned Profile</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="nb in notebooks" :key="nb.id">
                <td class="nb-title">{{ nb.title }}</td>
                <td>
                  <select
                    :value="nb.profile_id || ''"
                    class="profile-select"
                    @change="handleAssignProfile(nb.id, $event.target.value)"
                  >
                    <option value="">-- Unassigned --</option>
                    <option v-for="p in profiles" :key="p.id" :value="p.id">
                      {{ p.name }}
                    </option>
                  </select>
                </td>
                <td>
                  <span class="status-chip" :class="nb.study_status || 'dormant'">
                    {{ nb.study_status || 'dormant' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </article>
      </div>
    </div>

    <!-- Add Profile Modal -->
    <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
      <div class="modal-card">
        <h2>New Study Profile</h2>
        <div class="form-group">
          <label>Profile Name</label>
          <input v-model="newProfileName" type="text" placeholder="e.g. UPSC, Semester Finals" />
        </div>
        <div class="form-group">
          <label>Target Deadline</label>
          <input v-model="newProfileDeadline" type="date" />
        </div>
        <div class="modal-actions">
          <button class="cancel-btn" @click="showAddModal = false">Cancel</button>
          <button
            class="save-btn"
            :disabled="!newProfileName || !newProfileDeadline"
            @click="handleAddProfile"
          >
            Create Profile
          </button>
        </div>
      </div>
    </div>

    <!-- Edit Profile Modal -->
    <div v-if="showEditModal" class="modal-overlay" @click.self="closeEditModal">
      <div class="modal-card">
        <h2>Edit Profile</h2>
        <div class="form-group">
          <label>Profile Name</label>
          <input v-model="editProfileName" type="text" />
        </div>
        <div class="form-group">
          <label>Target Deadline</label>
          <input v-model="editProfileDeadline" type="date" />
        </div>
        <div class="modal-actions">
          <button class="cancel-btn" @click="closeEditModal">Cancel</button>
          <button
            class="save-btn"
            :disabled="!editProfileName || !editProfileDeadline"
            @click="handleUpdateProfile"
          >
            Save Changes
          </button>
        </div>
      </div>
    </div>

    <!-- RAG Setup Modal -->
    <div v-if="showRagModal" class="modal-overlay" @click.self="handleRagModalDismiss">
      <div class="modal-card">
        <h2>Local AI Setup (RAG)</h2>
        <p class="description">
          We will run system specs check, stage DLLs, and initialize the ONNX embedding engine. This
          will take a few seconds and run completely on your system.
        </p>

        <div class="rag-setup-box">
          <div class="setup-header">
            <span v-if="ragStatus" class="status-badge" :class="ragStatus">{{
              ragStatus.toUpperCase()
            }}</span>
            <span class="setup-msg">{{ ragMessage }}</span>
          </div>

          <div class="progress-bar-mini">
            <div class="progress-fill-mini" :style="{ width: ragPercent + '%' }"></div>
          </div>

          <p class="setup-detail">{{ ragDetail }}</p>
          <div v-if="ragError" class="error-banner">{{ ragError }}</div>
        </div>

        <div class="modal-actions">
          <button class="cancel-btn" :disabled="isSettingUpRag" @click="handleRagModalDismiss">
            Cancel
          </button>

          <button
            v-if="!ragSetupCompleted"
            class="save-btn"
            :disabled="isSettingUpRag"
            @click="startRagSetup"
          >
            {{ isSettingUpRag ? 'Setting Up...' : 'Start Setup' }}
          </button>

          <button v-else class="save-btn" @click="closeRagModal">Finish</button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import {
  getUserSettings,
  updateUserSettings,
  getProfiles,
  createProfile,
  updateProfile,
  deleteProfile,
  getNotebooks,
  assignNotebookToProfile,
  triggerCloudSync,
  initializeRAG,
  getLLMSettings,
  updateLLMSettings,
  saveLLMAPIKey,
  deleteLLMAPIKey,
  getLLMProviderPreset,
  getAppEnv,
  loginStudent,
  logoutStudent,
  getCloudConfig,
} from '../services/appApi'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

const activeTab = ref('settings')
const loading = ref(true)
const saving = ref(false)
const savingLLM = ref(false)
const presetLoading = ref(false)
const syncing = ref(false)
const error = ref('')
const success = ref('')
const isDev = ref(false)
const cloudConfigured = ref(false)

const settings = ref({
  max_flashcards_per_session: 30,
  study_start_time: '17:00',
  study_end_time: '18:00',
  reminders_enabled: true,
  active_profile_id: '',
  skip_to_reading_active: false,
  cloud_sync_url: '',
  cloud_api_token: '',
  theme: 'light-classic',
  rag_enabled: false,
  rag_notebook_chapter: true,
  rag_entire_notebook: true,
  rag_queue_study: true,
  default_remedial_strategy: 'CLASSIC',
  classroom_code: '',
})

const llmFastKey = ref('')
const llmHeavyKey = ref('')

const durationPresets = [
  { label: '30 min', minutes: 30 },
  { label: '1 hour', minutes: 60 },
  { label: '1.5 hours', minutes: 90 },
  { label: '2 hours', minutes: 120 },
  { label: '3 hours', minutes: 180 },
]

const studyDuration = computed(() => {
  if (!settings.value.study_start_time || !settings.value.study_end_time) return ''
  const [startH, startM] = settings.value.study_start_time.split(':').map(Number)
  const [endH, endM] = settings.value.study_end_time.split(':').map(Number)
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
  const [h, m] = settings.value.study_start_time.split(':').map(Number)
  const startMinutes = h * 60 + m
  const endMinutes = startMinutes + preset.minutes
  const endH = Math.floor(endMinutes / 60) % 24
  const endM = endMinutes % 60
  settings.value.study_end_time = `${String(endH).padStart(2, '0')}:${String(endM).padStart(2, '0')}`
}
const llmSettings = ref({
  use_same_for_heavy: true,
  fast: {
    tier: 'fast',
    provider: 'groq',
    base_url: 'https://api.groq.com/openai',
    model: 'openai/gpt-oss-120b',
    timeout_ms: 60000,
    api_key_source: 'keyring',
    has_api_key: false,
  },
  heavy: {
    tier: 'heavy',
    provider: 'groq',
    base_url: 'https://api.groq.com/openai',
    model: 'openai/gpt-oss-120b',
    timeout_ms: 90000,
    api_key_source: 'keyring',
    has_api_key: false,
  },
})

// Watch settings theme to apply it in real-time
watch(
  () => settings.value.theme,
  (newTheme) => {
    if (newTheme) {
      document.documentElement.setAttribute('data-theme', newTheme)
    }
  }
)

// Auto-save general settings with debounce
let saveSettingsTimer = null
watch(
  settings,
  () => {
    if (loading.value || showRagModal.value) return

    // JS Validation checks
    const val = settings.value.max_flashcards_per_session
    const isValidMaxCards = typeof val === 'number' && !isNaN(val) && val >= 5 && val <= 200

    const start = settings.value.study_start_time
    const end = settings.value.study_end_time
    const isValidTimeWindow = start && end && start < end

    if (!isValidMaxCards) {
      error.value = 'Max flashcards per session must be between 5 and 200.'
      return
    }
    if (!isValidTimeWindow) {
      error.value = 'Study start time must be strictly earlier than end time.'
      return
    }

    // Clear validation error if it was set
    if (
      error.value === 'Max flashcards per session must be between 5 and 200.' ||
      error.value === 'Study start time must be strictly earlier than end time.'
    ) {
      error.value = ''
    }

    clearTimeout(saveSettingsTimer)
    saveSettingsTimer = setTimeout(() => {
      saveUserSettings()
    }, 800)
  },
  { deep: true }
)

// Auto-save LLM provider settings and keys with debounce
let saveLLMTimer = null
watch(
  [llmSettings, llmFastKey, llmHeavyKey],
  () => {
    if (loading.value || savingLLM.value) return
    clearTimeout(saveLLMTimer)
    saveLLMTimer = setTimeout(() => {
      saveLLMProviderSettings()
    }, 800)
  },
  { deep: true }
)

const profiles = ref([])
const notebooks = ref([])

// Modals
const showAddModal = ref(false)
const newProfileName = ref('')
const newProfileDeadline = ref('')

const showEditModal = ref(false)
const editProfileId = ref('')
const editProfileName = ref('')
const editProfileDeadline = ref('')

// RAG setup state
const showRagModal = ref(false)
const isSettingUpRag = ref(false)
const ragStatus = ref('')
const ragPercent = ref(0)
const ragMessage = ref('')
const ragDetail = ref('')
const ragError = ref('')
const ragSetupCompleted = ref(false)

function onRagToggle() {
  if (settings.value.rag_enabled) {
    settings.value.rag_enabled = false
    showRagModal.value = true
    ragSetupCompleted.value = false
    ragError.value = ''
    ragPercent.value = 0
    ragMessage.value = 'Ready to initialize local AI'
    ragDetail.value = ''
    ragStatus.value = ''
  }
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
    console.log('[Settings] RAG setup progress:', data)
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

function handleRagModalDismiss() {
  if (isSettingUpRag.value) return
  if (ragSetupCompleted.value) {
    closeRagModal()
  } else {
    showRagModal.value = false
  }
}

function closeRagModal() {
  showRagModal.value = false
  settings.value.rag_enabled = true
}

async function applyProviderPreset(tier) {
  presetLoading.value = true
  error.value = ''
  const target = tier === 'heavy' ? llmSettings.value.heavy : llmSettings.value.fast
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

onMounted(async () => {
  // Detect dev mode and cloud config state
  try {
    const envRes = await getAppEnv()
    isDev.value = envRes?.env === 'dev'
  } catch (_) {
    isDev.value = false
  }
  try {
    const cfgRes = await getCloudConfig()
    cloudConfigured.value = cfgRes?.configured === true
  } catch (_) {
    cloudConfigured.value = false
  }
  await loadAllData()
})

onUnmounted(() => {
  EventsOff('rag-setup-progress')
  clearTimeout(saveSettingsTimer)
  clearTimeout(saveLLMTimer)
})

const loginUsername = ref('')
const loginPassword = ref('')
const loginClassroomCode = ref('')
const loginError = ref('')
const loggingIn = ref(false)

async function handleLogin() {
  if (!loginUsername.value.trim() || !loginPassword.value.trim() || !loginClassroomCode.value.trim()) {
    loginError.value = 'All fields are required.'
    return
  }
  loginError.value = ''
  loggingIn.value = true
  try {
    const res = await loginStudent(
      loginUsername.value.trim(),
      loginPassword.value.trim(),
      loginClassroomCode.value.trim().toUpperCase()
    )
    if (res.error) {
      loginError.value = res.error
    } else {
      loginUsername.value = ''
      loginPassword.value = ''
      loginClassroomCode.value = ''
      await loadAllData()
      success.value = 'Successfully signed in and cloud sync enabled!'
    }
  } catch (err) {
    loginError.value = err.message || 'An error occurred during sign in.'
  } finally {
    loggingIn.value = false
  }
}

async function handleLogout() {
  if (confirm('Are you sure you want to sign out? This will disable cloud sync.')) {
    try {
      const res = await logoutStudent()
      if (res.error) {
        error.value = res.error
      } else {
        await loadAllData()
        success.value = 'Signed out successfully.'
      }
    } catch (err) {
      error.value = err.message || 'Failed to sign out.'
    }
  }
}

async function loadAllData() {
  try {
    loading.value = true
    error.value = ''

    // Load settings
    const settingsRes = await getUserSettings()
    if (settingsRes.error) {
      error.value = settingsRes.error
      return
    }
    if (!settingsRes.default_remedial_strategy) {
      settingsRes.default_remedial_strategy = 'CLASSIC'
    }
    settings.value = settingsRes

    const llmRes = await getLLMSettings()
    if (llmRes.error) {
      error.value = llmRes.error
      return
    }
    if (llmRes.settings) {
      llmSettings.value = llmRes.settings
    }

    // Load profiles
    const profilesRes = await getProfiles()
    if (profilesRes.error) {
      error.value = profilesRes.error
      return
    }
    profiles.value = profilesRes.profiles || []

    // Load textbooks
    const notebooksRes = await getNotebooks()
    if (notebooksRes.error) {
      error.value = notebooksRes.error
      return
    }
    notebooks.value = notebooksRes || []
  } catch (err) {
    error.value = err.message || 'Failed to fetch settings data'
  } finally {
    loading.value = false
  }
}

async function saveLLMProviderSettings() {
  if (presetLoading.value || error.value) return
  error.value = ''
  success.value = ''
  try {
    savingLLM.value = true
    const fast = { ...llmSettings.value.fast }
    const heavy = { ...llmSettings.value.heavy }

    const res = await updateLLMSettings({
      use_same_for_heavy: llmSettings.value.use_same_for_heavy,
      fast,
      heavy,
    })
    if (res.error) {
      error.value = res.error
      return
    }

    if (llmFastKey.value.trim()) {
      const keyRes = await saveLLMAPIKey('fast', llmFastKey.value.trim())
      if (keyRes.error) {
        error.value = keyRes.error
        return
      }
      if (llmSettings.value.use_same_for_heavy) {
        const heavyKeyRes = await saveLLMAPIKey('heavy', llmFastKey.value.trim())
        if (heavyKeyRes.error) {
          error.value = heavyKeyRes.error
          return
        }
      }
    }
    if (!llmSettings.value.use_same_for_heavy && llmHeavyKey.value.trim()) {
      const keyRes = await saveLLMAPIKey('heavy', llmHeavyKey.value.trim())
      if (keyRes.error) {
        error.value = keyRes.error
        return
      }
    }

    llmFastKey.value = ''
    llmHeavyKey.value = ''
    await loadAllData()
    success.value = 'AI provider settings updated successfully.'
    setTimeout(() => (success.value = ''), 4000)
  } catch (err) {
    error.value = err.message || 'Failed to save AI provider settings'
  } finally {
    savingLLM.value = false
  }
}

async function removeLLMKeys() {
  if (!confirm('Remove stored LLM API keys from the OS credential manager?')) {
    return
  }
  error.value = ''
  success.value = ''
  try {
    savingLLM.value = true
    const fastRes = await deleteLLMAPIKey('fast')
    if (fastRes.error) {
      error.value = fastRes.error
      return
    }
    const heavyRes = await deleteLLMAPIKey('heavy')
    if (heavyRes.error) {
      error.value = heavyRes.error
      return
    }
    await loadAllData()
    success.value = 'Stored AI provider keys removed.'
    setTimeout(() => (success.value = ''), 4000)
  } catch (err) {
    error.value = err.message || 'Failed to remove stored keys'
  } finally {
    savingLLM.value = false
  }
}

async function saveUserSettings() {
  error.value = ''
  success.value = ''
  try {
    saving.value = true
    const res = await updateUserSettings(
      settings.value.max_flashcards_per_session,
      settings.value.study_start_time,
      settings.value.study_end_time,
      settings.value.reminders_enabled,
      settings.value.active_profile_id,
      settings.value.skip_to_reading_active,
      settings.value.cloud_sync_url,
      settings.value.cloud_api_token,
      settings.value.theme,
      settings.value.rag_enabled,
      settings.value.rag_notebook_chapter,
      settings.value.rag_entire_notebook,
      settings.value.rag_queue_study,
      settings.value.default_remedial_strategy,
      settings.value.classroom_code || ''
    )
    if (res.error) {
      error.value = res.error
      return
    }
    success.value = 'Settings updated successfully.'
    window.dispatchEvent(new CustomEvent('settings-updated'))
    setTimeout(() => (success.value = ''), 4000)
  } catch (err) {
    error.value = err.message || 'Failed to save settings'
  } finally {
    saving.value = false
  }
}

async function runManualSync() {
  error.value = ''
  success.value = ''
  try {
    syncing.value = true
    const res = await triggerCloudSync()
    if (res.error) {
      error.value = res.error
      return
    }
    success.value = 'Sync completed successfully!'
    await loadAllData()
    setTimeout(() => (success.value = ''), 4000)
  } catch (err) {
    error.value = err.message || 'Failed to sync with cloud'
  } finally {
    syncing.value = false
  }
}

async function setActiveProfile(profileID) {
  settings.value.active_profile_id = profileID
}

async function handleAddProfile() {
  try {
    const res = await createProfile(newProfileName.value, newProfileDeadline.value)
    if (res.error) {
      alert(res.error)
      return
    }
    showAddModal.value = false
    newProfileName.value = ''
    newProfileDeadline.value = ''
    await loadAllData()
  } catch (err) {
    alert(err.message || 'Failed to create profile')
  }
}

function openEditModal(profile) {
  editProfileId.value = profile.id
  editProfileName.value = profile.name
  // Format unix to Date string YYYY-MM-DD
  const dateObj = new Date(profile.deadline_at * 1000)
  editProfileDeadline.value = dateObj.toISOString().split('T')[0]
  showEditModal.value = true
}

function closeEditModal() {
  showEditModal.value = false
  editProfileId.value = ''
  editProfileName.value = ''
  editProfileDeadline.value = ''
}

async function handleUpdateProfile() {
  try {
    const res = await updateProfile(
      editProfileId.value,
      editProfileName.value,
      editProfileDeadline.value
    )
    if (res.error) {
      alert(res.error)
      return
    }
    closeEditModal()
    await loadAllData()
  } catch (err) {
    alert(err.message || 'Failed to update profile')
  }
}

async function handleDeleteProfile(id) {
  if (
    !confirm(
      'Are you sure you want to delete this profile? Associated books will become unassigned.'
    )
  ) {
    return
  }
  try {
    const res = await deleteProfile(id)
    if (res.error) {
      alert(res.error)
      return
    }
    await loadAllData()
  } catch (err) {
    alert(err.message || 'Failed to delete profile')
  }
}

async function handleAssignProfile(notebookID, profileID) {
  try {
    const res = await assignNotebookToProfile(notebookID, profileID)
    if (res.error) {
      alert(res.error)
      return
    }
    await loadAllData()
  } catch (err) {
    alert(err.message || 'Failed to assign profile')
  }
}

function formatUnixDate(unix) {
  if (!unix) return 'N/A'
  const d = new Date(unix * 1000)
  return d.toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' })
}
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
  max-width: 1000px;
  margin: 0 auto;
  font-family: 'Inter', sans-serif;
  color: var(--on-surface);
}

h1 {
  margin: 0;
  font-size: 36px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

h2 {
  font-size: 20px;
  margin: 0 0 16px;
  font-weight: 700;
}

.tabs {
  display: flex;
  gap: 8px;
  background: var(--surface-container-low);
  padding: 6px;
  border-radius: 12px;
  width: fit-content;
  margin-bottom: 8px;
}

.tab-btn {
  background: none;
  border: none;
  color: var(--muted-text);
  font-size: 14px;
  font-weight: 700;
  padding: 8px 16px;
  cursor: pointer;
  border-radius: 8px;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.tab-btn:hover {
  color: var(--on-surface);
}

.tab-btn.active {
  color: var(--primary);
  background: var(--surface-container-lowest);
  box-shadow: 0 4px 12px color-mix(in srgb, var(--on-surface) 6%, transparent);
}

.settings-panels {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 28px;
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  box-shadow: 0 4px 20px color-mix(in srgb, var(--on-surface) 3%, transparent);
}

.form-grid {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

label {
  font-weight: 600;
  font-size: 14px;
  color: var(--on-surface);
}

input[type='number'],
input[type='text'],
input[type='url'],
input[type='password'],
input[type='date'],
select {
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 12px 14px;
  font-size: 14px;
  font-family: inherit;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

input:focus,
select:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--primary) 15%, transparent);
  outline: none;
}

.checkbox-container {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  cursor: pointer;
  user-select: none;
}

.checkbox-container input {
  position: absolute;
  opacity: 0;
  cursor: pointer;
  height: 0;
  width: 0;
}

.checkmark {
  width: 20px;
  height: 20px;
  background-color: var(--surface-container-low);
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  border-radius: 6px;
  flex-shrink: 0;
  position: relative;
  margin-top: 2px;
  transition: all 0.2s ease;
}

.checkbox-container:hover input ~ .checkmark {
  background-color: var(--surface-container);
  border-color: color-mix(in srgb, var(--outline-variant) 40%, transparent);
}

.checkbox-container input:checked ~ .checkmark {
  background-color: var(--primary);
  border-color: var(--primary);
}

.checkmark:after {
  content: '';
  position: absolute;
  display: none;
}

.checkbox-container input:checked ~ .checkmark:after {
  display: block;
}

.checkbox-container .checkmark:after {
  left: 6px;
  top: 2px;
  width: 5px;
  height: 10px;
  border: solid white;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}

.check-label {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.divider {
  border: none;
  height: 0;
  margin: 0;
}

.llm-advanced {
  display: grid;
  gap: 16px;
  padding: 16px;
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  border-radius: 12px;
  background: var(--surface-container-low);
}

.button-row {
  display: flex;
  gap: 12px;
}

.global-actions {
  padding: 8px 0;
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.save-btn {
  border: 0;
  border-radius: 12px;
  padding: 12px 24px;
  color: var(--on-primary);
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.save-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px color-mix(in srgb, var(--primary) 25%, transparent);
}

.save-btn:active {
  transform: translateY(0);
}

.sync-btn {
  border: none;
  border-radius: 12px;
  padding: 12px 24px;
  color: var(--primary);
  font-weight: 700;
  background: var(--surface-container-highest);
  cursor: pointer;
  transition: all 0.2s ease;
}

.sync-btn:hover {
  background: var(--surface-container-low);
}

.field-hint {
  margin: 2px 0 8px;
  color: var(--muted-text);
  font-size: 12px;
  line-height: 1.4;
}

.dev-badge {
  display: inline-block;
  margin-left: 6px;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.05em;
  background: color-mix(in srgb, var(--warning, #f0a000) 20%, transparent);
  color: var(--warning, #f0a000);
  vertical-align: middle;
}

/* Profiles tab styles */
.profiles-layout {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.panel-header h2 {
  margin: 0;
}

.add-profile-btn {
  background: var(--primary);
  color: var(--on-primary);
  border: none;
  border-radius: 8px;
  padding: 8px 16px;
  font-weight: 700;
  font-size: 13px;
  cursor: pointer;
}

.profiles-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.profile-card {
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  padding: 16px;
  background: var(--surface-container-low);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.profile-card.active {
  border-color: var(--primary);
  box-shadow: 0 0 10px rgba(108, 92, 231, 0.15);
}

.profile-info h3 {
  margin: 0 0 4px;
  font-size: 16px;
}

.profile-info .deadline {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
}

.profile-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.select-btn {
  background: rgba(108, 92, 231, 0.1);
  color: var(--primary);
  border: none;
  border-radius: 8px;
  padding: 6px 12px;
  font-weight: 700;
  font-size: 12px;
  cursor: pointer;
}

.active-badge {
  background: var(--primary);
  color: var(--on-primary);
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.edit-btn,
.delete-btn {
  background: none;
  border: none;
  color: var(--muted-text);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 6px;
}

.edit-btn:hover {
  background: rgba(255, 255, 255, 0.05);
  color: var(--on-surface);
}

.delete-btn:hover {
  background: rgba(235, 94, 85, 0.1);
  color: #eb5e55;
}

/* Textbooks assignments table */
.textbooks-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 16px;
}

.textbooks-table th {
  text-align: left;
  padding: 10px 12px;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  color: var(--muted-text);
  border-bottom: 1px solid var(--outline-variant);
}

.textbooks-table td {
  padding: 12px;
  border-bottom: 1px solid var(--outline-variant);
  font-size: 14px;
}

.nb-title {
  font-weight: 600;
}

.profile-select {
  padding: 6px 10px;
  font-size: 13px;
  border-radius: 8px;
  width: 100%;
}

.status-chip {
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.status-chip.active {
  background: rgba(37, 111, 54, 0.1);
  color: #256f36;
}

.status-chip.dormant {
  background: rgba(138, 139, 152, 0.1);
  color: var(--muted-text);
}

.status-chip.completed {
  background: rgba(108, 92, 231, 0.1);
  color: var(--primary);
}

/* Modals */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
}

.modal-card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 20px;
  padding: 24px;
  width: 100%;
  max-width: 400px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.2);
}

.modal-card h2 {
  margin: 0;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 10px;
}

.cancel-btn {
  background: none;
  border: 1px solid var(--outline-variant);
  padding: 10px 20px;
  border-radius: 10px;
  font-weight: 700;
  cursor: pointer;
  color: var(--on-surface);
}

.cancel-btn:hover {
  background: var(--surface-container-low);
}

/* RAG Modal Custom CSS */
.rag-setup-box {
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  padding: 16px;
  margin: 20px 0;
  color: #ffffff;
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

.strategy-options {
  display: flex;
  gap: 16px;
  margin-top: 8px;
}

.strategy-option {
  flex: 1;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 16px;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
  background: var(--surface-container-low);
  border: none;
  color: var(--on-surface);
}

.strategy-option:hover {
  background: var(--surface-container-lowest);
  box-shadow: 0 8px 16px color-mix(in srgb, var(--on-surface) 6%, transparent);
}

.strategy-option.active {
  background: var(--surface-container-lowest);
  box-shadow: 0 0 0 2px var(--primary);
}

.strategy-option input[type='radio'] {
  margin-top: 4px;
  accent-color: var(--primary);
  cursor: pointer;
}

.option-title {
  display: block;
  font-size: 1rem;
  font-weight: 600;
  color: var(--on-surface);
}

.option-desc {
  display: block;
  font-size: 0.85rem;
  color: var(--muted-text);
  margin-top: 4px;
  line-height: 1.4;
}

/* Settings-specific overrides for time-range shared styles */
.duration-preset:hover:not(:disabled) {
  background: var(--surface-container);
  border-color: color-mix(in srgb, var(--primary) 30%, transparent);
  color: var(--on-surface);
}

.duration-preset:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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

.signed-in-box {
  background: var(--surface-low);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--success);
}

.pulse-dot.active {
  width: 8px;
  height: 8px;
  background: var(--success);
  border-radius: 50%;
  box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7);
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0% {
    transform: scale(0.95);
    box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7);
  }
  70% {
    transform: scale(1);
    box-shadow: 0 0 0 6px rgba(16, 185, 129, 0);
  }
  100% {
    transform: scale(0.95);
    box-shadow: 0 0 0 0 rgba(16, 185, 129, 0);
  }
}

.user-details {
  font-size: 0.9rem;
  color: var(--on-surface);
  line-height: 1.5;
}

.user-details p {
  margin: 0.25rem 0;
}

.danger-btn {
  background: rgba(239, 68, 68, 0.1) !important;
  border: 1px solid rgba(239, 68, 68, 0.3) !important;
  color: #ef4444 !important;
  transition: all 0.2s ease;
}

.danger-btn:hover {
  background: rgba(239, 68, 68, 0.2) !important;
  border-color: rgba(239, 68, 68, 0.5) !important;
}

.login-form-container {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.login-error-message {
  background: rgba(239, 68, 68, 0.08);
  border: 1px solid rgba(239, 68, 68, 0.2);
  color: #f87171;
  padding: 0.75rem 1rem;
  border-radius: 8px;
  font-size: 0.85rem;
}
</style>
