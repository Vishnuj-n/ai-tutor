<template>
  <div class="notebook-page">
    <div class="notebook-header">
      <h1>Notebooks</h1>
      <p class="subtitle">Upload and manage your learning materials</p>
    </div>

    <!-- Errors Section -->
    <div v-if="settingsError || notebooksError" class="error-container" style="margin-bottom: 20px">
      <div v-if="settingsError" class="error-message" style="margin-bottom: 10px">
        Failed to load settings: {{ settingsError }}
      </div>
      <div v-if="notebooksError" class="error-message">
        Failed to load notebooks: {{ notebooksError }}
      </div>
    </div>

    <!-- Upload Section -->
    <NotebookUpload
      :upload-progress="uploadProgress"
      :ingestion-status-message="ingestionStatusMessage"
      :indexing-status-message="indexingStatusMessage"
      :upload-error="uploadError"
      :success-message="successMessage"
      @upload-file="uploadFile"
    />

    <!-- Active Lane (prioritized section) -->
    <div v-if="!loading && activeNotebooks.length > 0" class="active-lane-section">
      <h2>Active Lane ({{ activeNotebooks.length }} / 4)</h2>
      <p class="section-hint">Your currently studying textbooks. Maximum 4 active at a time.</p>
      <div class="notebook-grid">
        <NotebookCard
          v-for="notebook in activeNotebooks"
          :key="notebook.id"
          :notebook="notebook"
          :available-topics="availableTopics"
          :rag-enabled="ragEnabled"
          :rag-notebook-chapter="ragNotebookChapter"
          variant="active"
          @edit-syllabus="openSyllabusDraft"
          @update-priority="updatePriority"
          @change-status="setStudyStatus"
          @delete="deleteNotebook"
        />
      </div>
    </div>

    <!-- All Notebooks (dormant by default) -->
    <div class="notebooks-list">
      <h2>{{ activeNotebooks.length > 0 ? 'Dormant Books' : 'Your Notebooks' }}</h2>

      <div v-if="loading" class="loading">Loading notebooks...</div>

      <div
        v-if="!loading && dormantNotebooks.length === 0 && activeNotebooks.length === 0"
        class="empty-state"
      >
        <p>No notebooks yet. Upload your first document above!</p>
      </div>

      <div
        v-if="!loading && dormantNotebooks.length === 0 && activeNotebooks.length > 0"
        class="empty-state"
      >
        <p>All textbooks are active. Add more books above!</p>
      </div>

      <div v-if="!loading && dormantNotebooks.length > 0" class="notebook-grid">
        <NotebookCard
          v-for="notebook in dormantNotebooks"
          :key="notebook.id"
          :notebook="notebook"
          :available-topics="availableTopics"
          :rag-enabled="ragEnabled"
          :rag-notebook-chapter="ragNotebookChapter"
          variant="dormant"
          :active-limit-reached="activeNotebooks.length >= 4"
          @edit-syllabus="openSyllabusDraft"
          @update-priority="updatePriority"
          @change-status="setStudyStatus"
          @delete="deleteNotebook"
        />
      </div>
    </div>

    <!-- Syllabus Modal -->
    <NotebookSyllabusModal
      :show="showSyllabusModal"
      :notebook-title="draftNotebookTitle"
      :notebook-priority="draftNotebookPriority"
      :page-count="draftPageCount"
      :chapters="draftChapters"
      :is-confirming="isConfirmingDraft"
      :is-cleaning="isAICleaning"
      :error="draftError"
      @close="closeSyllabusModal"
      @ai-cleanup="aiCleanupChapters"
      @confirm="handleConfirmSyllabus"
    />

    <div class="toast-stack">
      <transition name="toast-fade">
        <div v-if="showFallbackToast" class="fallback-toast">
          <div class="fallback-toast-inner">
            <span class="fallback-toast-title">Fallback used</span>
            <p>{{ fallbackToastMessage }}</p>
          </div>
        </div>
      </transition>
      <transition name="toast-fade">
        <div v-if="showActionToast" class="action-toast">
          <div class="action-toast-inner">
            <span class="fallback-toast-title">Notice</span>
            <p>{{ actionToastMessage }}</p>
          </div>
        </div>
      </transition>
      <transition name="toast-fade">
        <div v-if="isDraftingSyllabus" class="drafting-toast">
          <div class="drafting-toast-inner">
            <div class="spinner"></div>
            <span class="drafting-title">Preparing chapter draft...</span>
            <p>{{ draftingNotebookTitle }}</p>
          </div>
        </div>
      </transition>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import {
  getAvailableTopics,
  getNotebooks as fetchNotebooks,
  uploadNotebook as apiUploadNotebook,
  uploadNotebookFromPath as apiUploadNotebookFromPath,
  draftNotebookSyllabus as apiDraftNotebookSyllabus,
  aiCleanupNotebookSyllabus as apiAICleanupNotebookSyllabus,
  confirmNotebookSyllabus as apiConfirmNotebookSyllabus,
  updateNotebookTitle as apiUpdateNotebookTitle,
  updateNotebookPriority as apiUpdateNotebookPriority,
  deleteNotebook as apiDeleteNotebook,
  updateNotebookStudyStatus,
  getUserSettings,
} from '../services/appApi'
import {
  CanResolveFilePaths,
  EventsOff,
  EventsOn,
  ResolveFilePaths,
} from '../../wailsjs/runtime/runtime'

import NotebookUpload from '../components/NotebookUpload.vue'
import NotebookCard from '../components/NotebookCard.vue'
import NotebookSyllabusModal from '../components/NotebookSyllabusModal.vue'

const uploadProgress = ref(0)
const uploadError = ref('')
const successMessage = ref('')
const notebooks = ref([])
const availableTopics = ref([])
const loading = ref(false)
const settingsError = ref('')
const notebooksError = ref('')
const ingestionStatusMessage = ref('')
const ingestionNotebookID = ref('')
const indexingStatusMessage = ref('')
const showSyllabusModal = ref(false)
const draftNotebookID = ref('')
const draftNotebookTitle = ref('')
const draftNotebookPriority = ref(5)
const originalDraftTitle = ref('')
const originalDraftPriority = ref(5)

const draftPageCount = ref(1)
const draftChapters = ref([])
const originalDraftChapters = ref([])
const draftError = ref('')
const isConfirmingDraft = ref(false)
const showFallbackToast = ref(false)
const fallbackToastMessage = ref('')
const showActionToast = ref(false)
const actionToastMessage = ref('')
const fallbackToastTimer = ref(null)
const actionToastTimer = ref(null)
const isDraftingSyllabus = ref(false)
const draftingNotebookTitle = ref('')
const isAICleaning = ref(false)
const activeProfileID = ref('')
let loadNotebooksToken = 0
const ragEnabled = ref(false)
const ragNotebookChapter = ref(true)

const activeNotebooks = computed(() => {
  if (!Array.isArray(notebooks.value)) return []
  return notebooks.value.filter(
    (nb) =>
      nb.study_status === 'active' &&
      (!activeProfileID.value || nb.profile_id === activeProfileID.value)
  )
})

const dormantNotebooks = computed(() => {
  if (!Array.isArray(notebooks.value)) return []
  return notebooks.value.filter(
    (nb) =>
      (nb.study_status === 'dormant' || !nb.study_status) &&
      (!activeProfileID.value || nb.profile_id === activeProfileID.value)
  )
})

async function setStudyStatus(notebookID, status) {
  try {
    const res = await updateNotebookStudyStatus(notebookID, status)
    if (res?.error) {
      showToast(`Failed to update study status: ${res.error}`)
      return
    }
    await loadNotebooks()
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    showToast(`Failed to update study status: ${msg}`)
    uploadError.value = `Failed to update study status: ${msg}`
  }
}

onMounted(async () => {
  EventsOn('ingestion-progress', handleIngestionProgress)

  // Load active profile for Smart Shelf profile scoping
  try {
    const settings = await getUserSettings()
    if (settings && !settings.error) {
      activeProfileID.value = settings.active_profile_id || ''
      ragEnabled.value = settings.rag_enabled || false
      if (typeof settings.rag_notebook_chapter !== 'undefined') {
        ragNotebookChapter.value = settings.rag_notebook_chapter
      }
    } else if (settings && settings.error) {
      settingsError.value = settings.error
    }
  } catch (err) {
    settingsError.value = err instanceof Error ? err.message : String(err)
  }

  // Load available topics and notebooks
  await loadTopics()
  await loadNotebooks()
})

onUnmounted(() => {
  EventsOff('ingestion-progress')
  clearFallbackToastTimer()
  clearActionToastTimer()
})

function clearFallbackToastTimer() {
  if (fallbackToastTimer.value) {
    clearTimeout(fallbackToastTimer.value)
    fallbackToastTimer.value = null
  }
}

function clearActionToastTimer() {
  if (actionToastTimer.value) {
    clearTimeout(actionToastTimer.value)
    actionToastTimer.value = null
  }
}

function handleIngestionProgress(payload) {
  if (!payload) {
    return
  }

  // Handle ingestion progress (upload/chunking phase)
  if (!ingestionNotebookID.value && payload.notebook_id) {
    ingestionNotebookID.value = payload.notebook_id
  }

  if (
    ingestionNotebookID.value &&
    payload.notebook_id &&
    payload.notebook_id !== ingestionNotebookID.value
  ) {
    return
  }

  if (typeof payload.percent === 'number') {
    uploadProgress.value = payload.percent
  }

  if (payload.message) {
    ingestionStatusMessage.value = payload.message
  }

  // Handle indexing progress (RAG indexing phase - background)
  if (payload.stage === 'indexing') {
    if (typeof payload.processed_chunks === 'number' && typeof payload.total_chunks === 'number') {
      const percent = Math.round((payload.processed_chunks / payload.total_chunks) * 100)
      indexingStatusMessage.value = `Semantic indexing: ${percent}% (${payload.processed_chunks}/${payload.total_chunks} chunks)`
    }
  }

  const terminalStates = new Set(['failed', 'chunked', 'indexed'])
  if (typeof payload.status === 'string' && terminalStates.has(payload.status)) {
    void loadNotebooks()
  }
}

async function loadTopics() {
  try {
    const topics = await getAvailableTopics()
    const topicList = Array.isArray(topics)
      ? topics
      : Array.isArray(topics?.topics)
        ? topics.topics
        : []
    availableTopics.value = topicList
  } catch (error) {
    console.error('Failed to load topics:', error)
    availableTopics.value = []
  }
}

async function loadNotebooks() {
  const token = ++loadNotebooksToken
  loading.value = true
  notebooksError.value = ''
  try {
    const result = await fetchNotebooks('', activeProfileID.value)
    if (token !== loadNotebooksToken) return
    if (Array.isArray(result) && result.length > 0 && result[0].error) {
      throw new Error(result[0].error)
    }
    notebooks.value = Array.isArray(result) ? result : []
  } catch (error) {
    if (token !== loadNotebooksToken) return
    console.error('Failed to load notebooks:', error)
    notebooksError.value = error instanceof Error ? error.message : String(error)
    notebooks.value = []
  } finally {
    if (token === loadNotebooksToken) {
      loading.value = false
    }
  }
}

async function uploadFile(file) {
  uploadError.value = ''
  successMessage.value = ''
  ingestionStatusMessage.value = ''
  ingestionNotebookID.value = ''
  draftError.value = ''
  uploadProgress.value = 10

  // Validate file type
  const validTypes = ['application/pdf', 'text/plain', 'text/markdown']
  if (!validTypes.includes(file.type) && !file.name.endsWith('.md')) {
    uploadError.value = 'Invalid file type. Please upload PDF, TXT, or MD files.'
    return
  }

  // Validate file size (50MB max)
  const maxSize = 50 * 1024 * 1024
  if (file.size > maxSize) {
    uploadError.value = 'File too large. Maximum size is 50MB.'
    return
  }

  try {
    const localPath = await resolveLocalFilePath(file)
    let result
    if (localPath) {
      uploadProgress.value = 40
      result = await apiUploadNotebookFromPath(localPath)
    } else {
      const arrayBuffer = await file.arrayBuffer()
      const bytes = new Uint8Array(arrayBuffer)
      uploadProgress.value = 50
      result = await apiUploadNotebook(Array.from(bytes), file.name)
    }

    if (result?.error) {
      throw new Error(result.error)
    }

    if (result?.status === 'chunked') {
      ingestionStatusMessage.value = 'Chunking complete'
    } else {
      ingestionStatusMessage.value = 'Uploaded. Drafting syllabus for review...'
    }

    successMessage.value = `Upload successful${result?.file_name ? `: ${result.file_name}` : ''}`

    if (result?.id) {
      await openSyllabusDraft(result.id, result?.file_name || '')
    }

    uploadProgress.value = 100
    setTimeout(() => {
      uploadProgress.value = 0
      successMessage.value = ''
      ingestionStatusMessage.value = ''
      ingestionNotebookID.value = ''
      void loadNotebooks()
    }, 2000)
  } catch (error) {
    successMessage.value = ''
    uploadError.value = `Upload failed: ${error.message}`
    uploadProgress.value = 0
  }
}

async function resolveLocalFilePath(file) {
  if (typeof file?.path === 'string' && file.path.trim() !== '') {
    return file.path
  }

  try {
    if (CanResolveFilePaths()) {
      await Promise.resolve(ResolveFilePaths([file]))
      if (typeof file?.path === 'string' && file.path.trim() !== '') {
        return file.path
      }
    }
  } catch (error) {
    console.warn('Could not resolve local file path via Wails runtime:', error)
  }

  return ''
}

async function openSyllabusDraft(notebookID, notebookTitle = '') {
  draftNotebookID.value = notebookID
  draftNotebookTitle.value = String(notebookTitle || '').trim()
  draftNotebookPriority.value = 5 // Default priority
  draftError.value = ''

  // Set loading state immediately for UI responsiveness
  isDraftingSyllabus.value = true
  draftingNotebookTitle.value = String(notebookTitle || '').trim()

  try {
    const draft = await apiDraftNotebookSyllabus(notebookID, false) // Load from DB, don't regenerate
    if (draft?.error) {
      throw new Error(draft.error)
    }

    const chapters = Array.isArray(draft?.chapters) ? draft.chapters : []
    draftPageCount.value = Number(draft?.page_count) > 0 ? Number(draft.page_count) : 1
    draftChapters.value =
      chapters.length > 0
        ? chapters.map((ch) => ({
            title: String(ch?.title || 'Untitled Chapter').trim() || 'Untitled Chapter',
            start_page: Number(ch?.start_page) || 1,
            end_page: Number(ch?.end_page) || 1,
          }))
        : [{ title: 'General', start_page: 1, end_page: draftPageCount.value }]

    // Load notebook to get current priority
    const notebook = notebooks.value.find((nb) => nb.id === notebookID)
    if (notebook) {
      if (notebook.priority) {
        draftNotebookPriority.value = notebook.priority
      }
    }

    showSyllabusModal.value = true

    if (draft?.fallback_used) {
      fallbackToastMessage.value = 'PDF bookmark extraction failed, using fallback chapter draft.'
      showFallbackToast.value = true
      clearFallbackToastTimer()
      fallbackToastTimer.value = setTimeout(() => {
        showFallbackToast.value = false
        fallbackToastTimer.value = null
      }, 5000)
    }

    originalDraftTitle.value = draftNotebookTitle.value
    originalDraftPriority.value = draftNotebookPriority.value
    originalDraftChapters.value = draftChapters.value.map((ch) => ({
      title: String(ch.title || '').trim(),
      start_page: Number(ch.start_page) || 1,
      end_page: Number(ch.end_page) || 1,
    }))
    draftError.value = ''
  } catch (error) {
    console.error('[Notebook] openSyllabusDraft error:', error)
    const message = error instanceof Error ? error.message : String(error)
    draftError.value = `Could not draft syllabus: ${message}`
    throw error instanceof Error ? error : new Error(message)
  } finally {
    isDraftingSyllabus.value = false
    draftingNotebookTitle.value = ''
  }
}

function closeSyllabusModal() {
  if (isAICleaning.value) return
  showSyllabusModal.value = false
  isConfirmingDraft.value = false
}

async function aiCleanupChapters() {
  if (!draftNotebookID.value || isAICleaning.value) return

  isAICleaning.value = true
  draftError.value = ''

  try {
    const result = await apiAICleanupNotebookSyllabus(draftNotebookID.value)
    if (result?.error) {
      throw new Error(result.error)
    }

    const chapters = Array.isArray(result?.chapters) ? result.chapters : []
    draftPageCount.value =
      Number(result?.page_count) > 0 ? Number(result.page_count) : draftPageCount.value
    draftChapters.value =
      chapters.length > 0
        ? chapters.map((ch) => ({
            title: String(ch?.title || 'Untitled Chapter').trim() || 'Untitled Chapter',
            start_page: Number(ch?.start_page) || 1,
            end_page: Number(ch?.end_page) || 1,
          }))
        : [{ title: 'General', start_page: 1, end_page: draftPageCount.value }]

    // Update snapshot so "Confirm" only re-ingests if user edits further
    originalDraftChapters.value = draftChapters.value.map((ch) => ({
      title: String(ch.title || '').trim(),
      start_page: Number(ch.start_page) || 1,
      end_page: Number(ch.end_page) || 1,
    }))

    if (result?.fallback_used) {
      showToast('AI unavailable — using bookmark chapters')
    } else {
      showToast('AI cleaned up chapter list')
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    draftError.value = `AI cleanup failed: ${message}`
  } finally {
    isAICleaning.value = false
  }
}

function chaptersEqual(a, b) {
  if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) {
    return false
  }
  return a.every((chapter, index) => {
    const other = b[index]
    return (
      chapter.title === other.title &&
      chapter.start_page === other.start_page &&
      chapter.end_page === other.end_page
    )
  })
}

function showToast(message) {
  actionToastMessage.value = message
  showActionToast.value = true
  clearActionToastTimer()
  actionToastTimer.value = setTimeout(() => {
    showActionToast.value = false
    actionToastTimer.value = null
  }, 5000)
}

async function handleConfirmSyllabus({ title, priority, chapters }) {
  draftNotebookTitle.value = title
  draftNotebookPriority.value = priority
  draftChapters.value = chapters
  await confirmSyllabusDraft()
}

async function confirmSyllabusDraft() {
  if (isAICleaning.value) return
  if (!draftNotebookID.value) {
    draftError.value = 'Notebook id is missing for confirmation.'
    return
  }

  const sanitized = draftChapters.value
    .map((ch) => ({
      title: String(ch?.title || '').trim(),
      start_page: Number(ch?.start_page) || 1,
      end_page: Number(ch?.end_page) || 1,
    }))
    .filter((ch) => ch.title !== '')

  if (sanitized.length === 0) {
    draftError.value = 'Add at least one chapter before confirming.'
    return
  }

  for (const chapter of sanitized) {
    chapter.start_page = Math.max(1, Math.min(chapter.start_page, draftPageCount.value))
    chapter.end_page = Math.max(
      chapter.start_page,
      Math.min(chapter.end_page, draftPageCount.value)
    )
  }

  const trimmedTitle = String(draftNotebookTitle.value || '').trim()
  const titleChanged = trimmedTitle !== String(originalDraftTitle.value || '').trim()
  const priorityChanged = draftNotebookPriority.value !== originalDraftPriority.value
  const chaptersChanged = !chaptersEqual(sanitized, originalDraftChapters.value)

  draftError.value = ''
  isConfirmingDraft.value = true

  try {
    const notebook = notebooks.value.find((nb) => nb.id === draftNotebookID.value)

    if (titleChanged) {
      const titleResult = await apiUpdateNotebookTitle(draftNotebookID.value, trimmedTitle)
      if (titleResult?.error) {
        throw new Error(titleResult.error)
      }
      if (notebook) {
        notebook.title = trimmedTitle
      }
    }

    if (priorityChanged) {
      const priorityResult = await apiUpdateNotebookPriority(
        draftNotebookID.value,
        draftNotebookPriority.value
      )
      if (priorityResult?.error) {
        throw new Error(priorityResult.error)
      }
      if (notebook) {
        notebook.priority = draftNotebookPriority.value
      }
    }

    // Only call ConfirmNotebookSyllabus if chapters actually changed OR if notebook is not yet chunked
    // This avoids expensive re-ingestion when only notebook title or priority changed on already-ingested books
    const isNotChunked = !notebook || notebook.status !== 'chunked'
    if (chaptersChanged || isNotChunked) {
      const result = await apiConfirmNotebookSyllabus(draftNotebookID.value, sanitized)
      if (result?.error) {
        throw new Error(result.error)
      }
      await loadTopics()
      await loadNotebooks()
      closeSyllabusModal()
      if (ragEnabled.value) {
        showToast('Notebook ready! Semantic indexing running in background...')
      } else {
        showToast('Notebook ready!')
      }
    } else {
      // Chapters didn't change, just update notebook title/priority if needed
      await loadNotebooks()
      closeSyllabusModal()
      showToast('Notebook metadata updated')
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    draftError.value = `Failed to confirm syllabus: ${message}`
    showToast(`Failed to confirm syllabus: ${message}`)
  } finally {
    isConfirmingDraft.value = false
  }
}

async function deleteNotebook(notebookId) {
  if (!confirm('Are you sure you want to delete this notebook?')) {
    return
  }

  try {
    const result = await apiDeleteNotebook(notebookId)
    if (result?.error) {
      throw new Error(result.error)
    }
    await loadNotebooks()
  } catch (error) {
    console.error('Failed to delete notebook:', error)
    uploadError.value = `Delete failed: ${error.message}`
  }
}

async function updatePriority(notebookId, priority) {
  try {
    const result = await apiUpdateNotebookPriority(notebookId, priority)
    if (result?.error) {
      throw new Error(result.error)
    }
    // Update local state
    const notebook = notebooks.value.find((n) => n.id === notebookId)
    if (notebook) {
      notebook.priority = priority
    }
  } catch (error) {
    console.error('Failed to update priority:', error)
    uploadError.value = `Failed to update priority: ${error.message}`
  }
}
</script>

<style scoped>
.notebook-page {
  padding: 32px;
  max-width: 1200px;
  margin: 0 auto;
}

.notebook-header {
  margin-bottom: 32px;
}

.notebook-header h1 {
  margin: 0;
  font-size: 32px;
  font-weight: 700;
  color: var(--on-surface);
}

.subtitle {
  margin: 8px 0 0;
  font-size: 14px;
  color: var(--muted-text);
}

.error-message {
  margin-top: 12px;
  padding: 12px;
  background: #ffebee;
  color: #c62828;
  border-radius: 6px;
  font-size: 14px;
}

.notebooks-list {
  margin-top: 48px;
}

.notebooks-list h2 {
  margin: 0 0 24px;
  font-size: 24px;
  color: var(--on-surface);
}

.loading {
  text-align: center;
  padding: 32px;
  color: var(--muted-text);
}

.empty-state {
  text-align: center;
  padding: 48px;
  background: var(--surface-container);
  border-radius: 12px;
  color: var(--muted-text);
}

.notebook-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.toast-stack {
  position: fixed;
  right: 20px;
  bottom: 20px;
  z-index: 1300;
  display: flex;
  flex-direction: column-reverse;
  gap: 8px;
  pointer-events: none;
}

.toast-stack > * {
  pointer-events: auto;
}

.action-toast,
.fallback-toast,
.drafting-toast {
  position: relative;
}

.action-toast-inner,
.fallback-toast-inner {
  max-width: 320px;
  padding: 14px 16px;
  background: #1f8b4c;
  color: #fff;
  border-radius: 14px;
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.18);
  border: 1px solid rgba(255, 255, 255, 0.12);
}

.fallback-toast-inner {
  background: #b33939;
}

.drafting-toast-inner {
  max-width: 320px;
  padding: 16px 20px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  border-radius: 14px;
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.18);
  border: 1px solid var(--outline-variant);
  display: flex;
  align-items: center;
  gap: 12px;
}

.drafting-title {
  display: block;
  font-weight: 700;
  font-size: 14px;
  color: var(--on-surface);
  margin-bottom: 2px;
}

.drafting-toast-inner p {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
}

.fallback-toast-title {
  display: block;
  font-weight: 700;
  margin-bottom: 4px;
}

.toast-fade-enter-active,
.toast-fade-leave-active {
  transition:
    opacity 0.25s ease,
    transform 0.25s ease;
}

.toast-fade-enter-from,
.toast-fade-leave-to {
  opacity: 0;
  transform: translateY(12px);
}

@media (max-width: 768px) {
  .notebook-grid {
    grid-template-columns: 1fr;
  }

  .toast-stack {
    left: 20px;
    right: auto;
  }
}

/* ── Active Lane Section ─────────────────────────────── */
.active-lane-section {
  margin-top: 32px;
}

.active-lane-section h2 {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 700;
  color: var(--on-surface);
}

.section-hint {
  margin: 0 0 16px;
  font-size: 13px;
  color: var(--muted-text, #888);
}
</style>
