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
      <article class="panel form-grid">
        <h2>Study Budget & Routine</h2>
        
        <div class="form-group">
          <label for="daily-minutes">Daily study goal (minutes)</label>
          <input
            id="daily-minutes"
            v-model.number="settings.daily_study_minutes"
            type="number"
            min="15"
            max="480"
            step="5"
            :disabled="loading || saving"
          />
          <p class="hint">Adjusts FSRS review capacities and reading goals to match your daily schedule.</p>
        </div>

        <div class="form-group check-group">
          <label class="checkbox-container">
            <input
              type="checkbox"
              v-model="settings.skip_to_reading_active"
              :disabled="loading || saving"
            />
            <span class="checkmark"></span>
            <div class="check-label">
              <strong>Enable "Skip to Reading" (Escape Hatch)</strong>
              <p class="hint">Temporarily deprioritizes review backlogs, letting you read new material first. FSRS records remain safe.</p>
            </div>
          </label>
        </div>

        <div class="form-group check-group">
          <label class="checkbox-container">
            <input
              type="checkbox"
              v-model="settings.rag_enabled"
              :disabled="loading || saving"
              @change="onRagToggle"
            />
            <span class="checkmark"></span>
            <div class="check-label">
              <strong>Enable Local AI Retrieval (RAG)</strong>
              <p class="hint">Preloads local ONNX embeddings for context-rich Q&A. Unticking unloads RAG from memory instantly.</p>
            </div>
          </label>
        </div>

        <hr class="divider" />

        <h2>Workspace Aesthetics</h2>
        <div class="form-group">
          <label for="theme-select">Aesthetic Theme</label>
          <select
            id="theme-select"
            v-model="settings.theme"
            :disabled="loading || saving"
          >
            <option value="light-classic">Light Classic</option>
            <option value="light-warm">Warm Sepia (Reader)</option>
            <option value="dark-indigo">Deep Indigo Night (Dark Mode)</option>
            <option value="dark-nord">Nord Frost (Cool Dark Mode)</option>
            <option value="dark-emerald">Forest Emerald</option>
          </select>
          <p class="hint">Select a visual theme. Changing themes alters the colors of your study desk instantly.</p>
        </div>

        <hr class="divider" />

        <h2>Teacher Cloud Synchronization</h2>

        <div class="form-group">
          <label for="cloud-url">Sync Server URL</label>
          <input
            id="cloud-url"
            v-model="settings.cloud_sync_url"
            type="url"
            placeholder="https://example.com/api/sync"
            :disabled="loading || saving"
          />
        </div>

        <div class="form-group">
          <label for="cloud-token">Access Token</label>
          <input
            id="cloud-token"
            v-model="settings.cloud_api_token"
            type="password"
            placeholder="Enter authorization token"
            :disabled="loading || saving"
          />
        </div>

        <div class="button-row">
          <button type="button" class="save-btn" :disabled="loading || saving" @click="saveUserSettings">
            {{ saving ? 'Saving Settings...' : 'Save Settings' }}
          </button>
          
          <button
            v-if="settings.cloud_sync_url"
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
      </article>
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
                <p class="deadline">Deadline: <strong>{{ formatUnixDate(profile.deadline_at) }}</strong></p>
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
          <p class="description">Assign uploaded textbooks to study profiles to calculate target deadlines.</p>

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
                    @change="handleAssignProfile(nb.id, $event.target.value)"
                    class="profile-select"
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
          <button class="save-btn" :disabled="!newProfileName || !newProfileDeadline" @click="handleAddProfile">
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
          <button class="save-btn" :disabled="!editProfileName || !editProfileDeadline" @click="handleUpdateProfile">
            Save Changes
          </button>
        </div>
      </div>
    </div>

    <!-- RAG Setup Modal -->
    <div v-if="showRagModal" class="modal-overlay" @click.self="isSettingUpRag ? null : showRagModal = false">
      <div class="modal-card">
        <h2>Local AI Setup (RAG)</h2>
        <p class="description">
          We will run system specs check, stage DLLs, and initialize the ONNX embedding engine.
          This will take a few seconds and run completely on your system.
        </p>

        <div class="rag-setup-box">
          <div class="setup-header">
            <span v-if="ragStatus" class="status-badge" :class="ragStatus">{{ ragStatus.toUpperCase() }}</span>
            <span class="setup-msg">{{ ragMessage }}</span>
          </div>
          
          <div class="progress-bar-mini">
            <div class="progress-fill-mini" :style="{ width: ragPercent + '%' }"></div>
          </div>
          
          <p class="setup-detail">{{ ragDetail }}</p>
          <div v-if="ragError" class="error-banner">{{ ragError }}</div>
        </div>

        <div class="modal-actions">
          <button 
            class="cancel-btn" 
            :disabled="isSettingUpRag" 
            @click="showRagModal = false"
          >
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
          
          <button 
            v-else 
            class="save-btn" 
            @click="closeRagModal"
          >
            Finish
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
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
  initializeRAG
} from '../services/appApi'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

const activeTab = ref('settings')
const loading = ref(true)
const saving = ref(false)
const syncing = ref(false)
const error = ref('')
const success = ref('')

const settings = ref({
  daily_study_minutes: 90,
  active_profile_id: '',
  skip_to_reading_active: false,
  cloud_sync_url: '',
  cloud_api_token: '',
  theme: 'light-classic',
  rag_enabled: false
})

// Watch settings theme to apply it in real-time
watch(() => settings.value.theme, (newTheme) => {
  if (newTheme) {
    document.documentElement.setAttribute('data-theme', newTheme)
  }
})

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
  } else {
    saveUserSettings()
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

function closeRagModal() {
  showRagModal.value = false
  settings.value.rag_enabled = true
  saveUserSettings()
}

onMounted(async () => {
  await loadAllData()
})

onUnmounted(() => {
  EventsOff('rag-setup-progress')
})

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
    settings.value = settingsRes

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

async function saveUserSettings() {
  error.value = ''
  success.value = ''
  try {
    saving.value = true
    const res = await updateUserSettings(
      settings.value.daily_study_minutes,
      settings.value.active_profile_id,
      settings.value.skip_to_reading_active,
      settings.value.cloud_sync_url,
      settings.value.cloud_api_token,
      settings.value.theme,
      settings.value.rag_enabled
    )
    if (res.error) {
      error.value = res.error
      return
    }
    success.value = 'Settings updated successfully.'
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
  await saveUserSettings()
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
    const res = await updateProfile(editProfileId.value, editProfileName.value, editProfileDeadline.value)
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
  if (!confirm('Are you sure you want to delete this profile? Associated books will become unassigned.')) {
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
  gap: 12px;
  border-bottom: 1px solid var(--outline-variant);
  padding-bottom: 8px;
}

.tab-btn {
  background: none;
  border: none;
  color: var(--muted-text);
  font-size: 15px;
  font-weight: 700;
  padding: 8px 16px;
  cursor: pointer;
  border-radius: 8px;
  transition: background 0.2s, color 0.2s;
}

.tab-btn:hover {
  background: var(--surface-container-low);
  color: var(--on-surface);
}

.tab-btn.active {
  color: var(--primary);
  background: var(--surface-container-low);
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 28px;
  border: 1px solid var(--outline-variant);
}

.form-grid {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

label {
  font-weight: 600;
  font-size: 14px;
  color: var(--on-surface);
}

input[type="number"],
input[type="text"],
input[type="url"],
input[type="password"],
input[type="date"],
select {
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 12px 14px;
  font-size: 14px;
  font-family: inherit;
}

input:focus, select:focus {
  border-color: var(--primary);
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
  border: 1px solid var(--outline-variant);
  border-radius: 6px;
  flex-shrink: 0;
  position: relative;
  margin-top: 2px;
}

.checkbox-container:hover input ~ .checkmark {
  background-color: var(--surface-container-high);
}

.checkbox-container input:checked ~ .checkmark {
  background-color: var(--primary);
  border-color: var(--primary);
}

.checkmark:after {
  content: "";
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
  border-top: 1px solid var(--outline-variant);
  margin: 8px 0;
}

.button-row {
  display: flex;
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
}

.sync-btn {
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  padding: 12px 24px;
  color: var(--on-surface);
  font-weight: 700;
  background: var(--surface-container-low);
  cursor: pointer;
  transition: background 0.2s;
}

.sync-btn:hover {
  background: var(--surface-container-high);
}

.hint {
  margin: 0;
  color: var(--muted-text);
  font-size: 13px;
  line-height: 1.4;
}

.error-text {
  color: #a3362f;
  font-size: 13px;
  margin: 0;
}

.success-text {
  color: #256f36;
  font-size: 13px;
  margin: 0;
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

.empty-state {
  text-align: center;
  padding: 40px 20px;
  color: var(--muted-text);
  font-size: 14px;
  background: var(--surface-container-low);
  border-radius: 12px;
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

.edit-btn, .delete-btn {
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
