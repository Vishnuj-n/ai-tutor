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
        <SettingsStudyBudget
          :settings="settings"
          :study-duration="studyDuration"
          :disabled="loading || saving"
          @apply-duration-preset="applyDurationPreset"
        />

        <SettingsQuizRescue
          :settings="settings"
          :disabled="loading || saving"
          @rag-toggle="onRagToggle"
        />

        <SettingsAIProvider
          :llm-settings="llmSettings"
          :llm-fast-key="llmFastKey"
          :llm-heavy-key="llmHeavyKey"
          :disabled="loading || savingLLM"
          @apply-preset="applyProviderPreset"
          @remove-keys="removeLLMKeys"
          @update:llm-fast-key="llmFastKey = $event"
          @update:llm-heavy-key="llmHeavyKey = $event"
        />

        <SettingsTheme :settings="settings" :disabled="loading || saving" />

        <SettingsUpdate />

        <SettingsAccount
          :settings="settings"
          :is-dev="isDev"
          :disabled="loading || saving"
          :login-username="loginUsername"
          :login-password="loginPassword"
          :login-classroom-code="loginClassroomCode"
          :login-error="loginError"
          :logging-in="loggingIn"
          @login="handleLogin"
          @logout="handleLogout"
          @update:login-username="loginUsername = $event"
          @update:login-password="loginPassword = $event"
          @update:login-classroom-code="loginClassroomCode = $event"
        />
      </div>

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
        <SettingsProfilesPanel
          :profiles="profiles"
          :active-profile-id="settings.active_profile_id"
          :format-unix-date="formatUnixDate"
          @add="showAddModal = true"
          @select="setActiveProfile"
          @edit="openEditModal"
          @delete="handleDeleteProfile"
        />

        <SettingsTextbooksPanel
          :notebooks="notebooks"
          :profiles="profiles"
          @assign="handleAssignProfile"
        />
      </div>
    </div>

    <!-- Add Profile Modal -->
    <SettingsProfileModal
      v-if="showAddModal"
      @close="showAddModal = false"
      @save="handleAddProfile"
    />

    <!-- Edit Profile Modal -->
    <SettingsProfileModal
      v-if="showEditModal"
      is-edit
      :initial-name="editProfileName"
      :initial-deadline="editProfileDeadline"
      @close="closeEditModal"
      @save="handleUpdateProfile"
    />

    <!-- RAG Setup Modal -->
    <SettingsRagModal
      v-if="showRagModal"
      :is-setting-up-rag="isSettingUpRag"
      :rag-status="ragStatus"
      :rag-percent="ragPercent"
      :rag-message="ragMessage"
      :rag-detail="ragDetail"
      :rag-error="ragError"
      :rag-setup-completed="ragSetupCompleted"
      @dismiss="handleRagModalDismiss"
      @start="startRagSetup"
      @finish="closeRagModal"
    />
  </section>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { getAppEnv, getCloudConfig, triggerCloudSync } from '../services/appApi'

import { useSettings } from '../composables/useSettings'
import { useLLM } from '../composables/useLLM'
import { useProfiles } from '../composables/useProfiles'
import { useRAG } from '../composables/useRAG'
import { useAuth } from '../composables/useAuth'

import SettingsStudyBudget from '../components/SettingsStudyBudget.vue'
import SettingsQuizRescue from '../components/SettingsQuizRescue.vue'
import SettingsAIProvider from '../components/SettingsAIProvider.vue'
import SettingsTheme from '../components/SettingsTheme.vue'
import SettingsUpdate from '../components/SettingsUpdate.vue'
import SettingsAccount from '../components/SettingsAccount.vue'
import SettingsProfilesPanel from '../components/SettingsProfilesPanel.vue'
import SettingsTextbooksPanel from '../components/SettingsTextbooksPanel.vue'
import SettingsProfileModal from '../components/SettingsProfileModal.vue'
import SettingsRagModal from '../components/SettingsRagModal.vue'

const activeTab = ref('settings')
const isDev = ref(false)
const cloudConfigured = ref(false)
const syncing = ref(false)
const error = ref('')
const success = ref('')

// Composables — all share the same error/success refs
const {
  settings,
  loading,
  saving,
  studyDuration,
  applyDurationPreset,
  loadSettings,
  saveUserSettings,
  cleanup: cleanupSettings,
} = useSettings(error, success)

const {
  llmSettings,
  llmFastKey,
  llmHeavyKey,
  savingLLM,
  applyProviderPreset,
  removeLLMKeys,
  loadLLM,
  cleanup: cleanupLLM,
} = useLLM(loading, error, success)

const {
  profiles,
  notebooks,
  showAddModal,
  showEditModal,
  editProfileName,
  editProfileDeadline,
  openEditModal,
  closeEditModal,
  handleAddProfile,
  handleUpdateProfile,
  handleDeleteProfile,
  handleAssignProfile,
  loadProfiles,
  loadNotebooks,
  formatUnixDate,
} = useProfiles(error)

const {
  showRagModal,
  isSettingUpRag,
  ragStatus,
  ragPercent,
  ragMessage,
  ragDetail,
  ragError,
  ragSetupCompleted,
  onRagToggle,
  startRagSetup,
  handleRagModalDismiss,
  closeRagModal,
  cleanup: cleanupRAG,
} = useRAG(settings)

const {
  loginUsername,
  loginPassword,
  loginClassroomCode,
  loginError,
  loggingIn,
  handleLogin,
  handleLogout,
} = useAuth(loadAllData, error, success)

async function loadAllData() {
  loading.value = true
  error.value = ''
  try {
    await Promise.all([loadSettings(), loadLLM(), loadProfiles(), loadNotebooks()])
  } catch (err) {
    error.value = err.message || 'Failed to load settings data'
    console.error('loadAllData error:', err)
  } finally {
    loading.value = false
  }
}

function setActiveProfile(profileID) {
  settings.value.active_profile_id = profileID
  saveUserSettings(true)
}

async function runManualSync() {
  error.value = ''
  success.value = ''
  syncing.value = true
  try {
    const res = await triggerCloudSync()
    if (res.error) {
      error.value = res.error
      return
    }
    await loadAllData()
    success.value = 'Sync completed successfully!'
    setTimeout(() => (success.value = ''), 4000)
  } catch (err) {
    error.value = err.message || 'Failed to sync with cloud'
  } finally {
    syncing.value = false
  }
}

watch(
  () => settings.value.theme,
  (newTheme) => {
    if (newTheme) document.documentElement.setAttribute('data-theme', newTheme)
  }
)

onMounted(async () => {
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
  cleanupSettings()
  cleanupLLM()
  cleanupRAG()
})
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

.profiles-layout {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.error-text {
  color: #ef4444;
}

.success-text {
  color: #10b981;
}
</style>
