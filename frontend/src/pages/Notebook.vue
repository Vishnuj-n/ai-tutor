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
    <div class="upload-section">
      <div class="upload-card">
        <div class="upload-icon">📄</div>
        <h3>Upload Document</h3>
        <p>Drag and drop or click to select PDF, TXT, or MD files</p>

        <input
          ref="fileInput"
          type="file"
          accept=".pdf,.txt,.md"
          style="display: none"
          @change="handleFileSelect"
        />

        <div
          class="drop-zone"
          :class="{ dragging: isDragging }"
          @click="triggerFilePicker"
          @dragover.prevent="isDragging = true"
          @dragleave.prevent="isDragging = false"
          @drop.prevent="handleFileDrop"
        >
          <p class="drop-title">Drop files here</p>
          <button type="button" class="upload-cta">Choose File</button>
          <p class="drop-hint">or drag and drop PDF, TXT, MD up to 50 MB</p>
        </div>

        <div v-if="uploadProgress > 0 && uploadProgress < 100" class="progress">
          <div class="progress-bar" :style="{ width: uploadProgress + '%' }"></div>
          <span>{{ uploadProgress }}%</span>
          <p v-if="ingestionStatusMessage" class="progress-label">{{ ingestionStatusMessage }}</p>
        </div>

        <div v-if="indexingStatusMessage" class="progress indexing-progress">
          <p class="progress-label">{{ indexingStatusMessage }}</p>
        </div>

        <div v-if="uploadError" class="error-message">
          {{ uploadError }}
        </div>

        <div v-if="successMessage" class="success-message">{{ successMessage }}</div>
      </div>
    </div>

    <!-- Active Lane (prioritized section) -->
    <div v-if="!loading && activeNotebooks.length > 0" class="active-lane-section">
      <h2>Active Lane ({{ activeNotebooks.length }} / 4)</h2>
      <p class="section-hint">Your currently studying textbooks. Maximum 4 active at a time.</p>
      <div class="notebook-grid">
        <div
          v-for="notebook in activeNotebooks"
          :key="notebook.id"
          class="notebook-card active-notebook-card"
        >
          <button
            class="btn-edit-pen"
            title="Edit notebook and chapters"
            @click="openSyllabusDraft(notebook.id, notebook.title)"
          >
            ✎
          </button>
          <div class="notebook-header-card">
            <div class="file-icon active-icon">{{ getFileIcon(notebook.file_type) }}</div>
            <div class="notebook-info">
              <h3>{{ notebook.title }}</h3>
              <p class="meta">{{ notebook.file_type.toUpperCase() }}</p>
              <p v-if="notebook.page_count > 0" class="meta">{{ notebook.page_count }} pages</p>
              <p class="meta">{{ notebook.chunk_count }} chunks</p>
            </div>
          </div>

          <div
            v-if="notebook.topic_id"
            class="notebook-topic"
            style="display: flex; align-items: center; gap: 8px"
          >
            <span class="badge">{{ getTopicTitle(notebook.topic_id) }}</span>
            <RouterLink
              v-if="ragEnabled && ragNotebookChapter"
              :to="`/tutor?topic_id=${notebook.topic_id}&notebook_id=${notebook.id}`"
              class="tutor-link-btn"
              title="Ask Tutor (RAG)"
              style="text-decoration: none; font-size: 13px"
            >
              ◎ Ask Tutor
            </RouterLink>
          </div>

          <div class="notebook-priority">
            <label class="priority-label">Priority:</label>
            <select
              :value="notebook.priority || 5"
              class="priority-select"
              @change="(e) => updatePriority(notebook.id, parseInt(e.target.value))"
            >
              <option v-for="n in 10" :key="n" :value="n">{{ n }}</option>
            </select>
          </div>

          <div class="notebook-date">Uploaded: {{ formatDate(notebook.uploaded_at) }}</div>

          <div class="notebook-actions">
            <button class="btn-sleep" @click="setStudyStatus(notebook.id, 'dormant')">Sleep</button>
            <button class="btn-download" @click="downloadNotebook(notebook.id)">Download</button>
            <button class="btn-delete" @click="deleteNotebook(notebook.id)">Delete</button>
          </div>
        </div>
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
        <div
          v-for="notebook in dormantNotebooks"
          :key="notebook.id"
          class="notebook-card dormant-notebook-card"
        >
          <button
            class="btn-edit-pen"
            title="Edit notebook and chapters"
            @click="openSyllabusDraft(notebook.id, notebook.title)"
          >
            ✎
          </button>
          <div class="notebook-header-card">
            <div class="file-icon">{{ getFileIcon(notebook.file_type) }}</div>
            <div class="notebook-info">
              <h3>{{ notebook.title }}</h3>
              <p class="meta">{{ notebook.file_type.toUpperCase() }}</p>
              <p v-if="notebook.page_count > 0" class="meta">{{ notebook.page_count }} pages</p>
              <p class="meta">{{ notebook.chunk_count }} chunks</p>
              <p class="meta">Status: {{ formatStatus(notebook.status) }}</p>
            </div>
          </div>

          <div
            v-if="notebook.topic_id"
            class="notebook-topic"
            style="display: flex; align-items: center; gap: 8px"
          >
            <span class="badge">{{ getTopicTitle(notebook.topic_id) }}</span>
            <RouterLink
              v-if="ragEnabled && ragNotebookChapter"
              :to="`/tutor?topic_id=${notebook.topic_id}&notebook_id=${notebook.id}`"
              class="tutor-link-btn"
              title="Ask Tutor (RAG)"
              style="text-decoration: none; font-size: 13px"
            >
              ◎ Ask Tutor
            </RouterLink>
          </div>

          <div v-else class="notebook-topic">
            <span class="badge muted">No topic linked</span>
          </div>

          <div class="notebook-priority">
            <label class="priority-label">Priority:</label>
            <select
              :value="notebook.priority || 5"
              class="priority-select"
              @change="(e) => updatePriority(notebook.id, parseInt(e.target.value))"
            >
              <option v-for="n in 10" :key="n" :value="n">{{ n }}</option>
            </select>
          </div>

          <div class="notebook-date">Uploaded: {{ formatDate(notebook.uploaded_at) }}</div>

          <div class="notebook-actions">
            <button
              class="btn-activate"
              :disabled="activeNotebooks.length >= 4"
              @click="setStudyStatus(notebook.id, 'active')"
            >
              Activate
            </button>
            <button class="btn-download" @click="downloadNotebook(notebook.id)">Download</button>
            <button class="btn-delete" @click="deleteNotebook(notebook.id)">Delete</button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showSyllabusModal" class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <h3>Verify Syllabus Chapters</h3>
          <button type="button" class="modal-close" @click="closeSyllabusModal">×</button>
        </div>

        <p class="modal-warning">
          Use absolute PDF page numbers. Page labels shown inside the PDF viewer may differ from
          file page numbers.
        </p>

        <div class="modal-title-edit">
          <label for="notebook-title">Notebook title</label>
          <input
            id="notebook-title"
            v-model="draftNotebookTitle"
            type="text"
            class="chapter-input"
            placeholder="Notebook name"
          />
        </div>

        <div class="modal-priority-edit">
          <label for="notebook-priority">Notebook priority (1-10)</label>
          <select
            id="notebook-priority"
            v-model.number="draftNotebookPriority"
            class="priority-select-modal"
          >
            <option v-for="n in 10" :key="n" :value="n">
              {{ n }} - {{ n === 1 ? 'Lowest' : n === 10 ? 'Highest' : n === 5 ? 'Default' : '' }}
            </option>
          </select>
          <p class="priority-hint">Higher-priority notebooks appear earlier in your study queue.</p>
        </div>

        <div v-if="draftError" class="error-message modal-error">{{ draftError }}</div>

        <div class="chapter-table-wrap">
          <table class="chapter-table">
            <thead>
              <tr>
                <th>Title</th>
                <th>Start Page</th>
                <th>End Page</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(chapter, index) in draftChapters" :key="`chapter-${index}`">
                <td>
                  <input
                    v-model="chapter.title"
                    type="text"
                    class="chapter-input"
                    placeholder="Chapter title"
                  />
                </td>
                <td>
                  <input
                    v-model.number="chapter.start_page"
                    type="number"
                    min="1"
                    :max="draftPageCount"
                    class="chapter-input chapter-page"
                    @change="sanitizeChapterPages(chapter)"
                  />
                </td>
                <td>
                  <input
                    v-model.number="chapter.end_page"
                    type="number"
                    min="1"
                    :max="draftPageCount"
                    class="chapter-input chapter-page"
                    @change="sanitizeChapterPages(chapter)"
                  />
                </td>
                <td>
                  <button type="button" class="row-delete" @click="removeDraftChapter(index)">
                    Delete
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="modal-actions">
          <button type="button" class="btn-secondary" @click="addDraftChapter">Add Chapter</button>
          <button type="button" class="btn-secondary" @click="closeSyllabusModal">Cancel</button>
          <button
            type="button"
            class="btn-primary"
            :disabled="isConfirmingDraft"
            @click="confirmSyllabusDraft"
          >
            {{ isConfirmingDraft ? 'Confirming...' : 'Confirm and Ingest' }}
          </button>
        </div>
      </div>
    </div>
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
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import {
  getAvailableTopics,
  getNotebooks as fetchNotebooks,
  uploadNotebook as apiUploadNotebook,
  uploadNotebookFromPath as apiUploadNotebookFromPath,
  draftNotebookSyllabus as apiDraftNotebookSyllabus,
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

const fileInput = ref(null)
const isDragging = ref(false)
const uploadProgress = ref(0)
const uploadError = ref('')
const uploadSuccess = ref(false)
const successMessage = ref('')
const notebooks = ref([])
const availableTopics = ref([])
const loading = ref(false)
const settingsError = ref('')
const notebooksError = ref('')
const ingestionStatusMessage = ref('')
const ingestionNotebookID = ref('')
const indexingProgress = ref(0)
const indexingStatusMessage = ref('')
const indexingNotebookID = ref('')
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
const activeProfileID = ref('')
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
  loading.value = true
  notebooksError.value = ''
  try {
    // Pass activeProfileID to filter notebooks by profile (for profile isolation)
    const result = await fetchNotebooks('', activeProfileID.value)
    if (Array.isArray(result) && result.length > 0 && result[0].error) {
      throw new Error(result[0].error)
    }
    notebooks.value = Array.isArray(result) ? result : []
  } catch (error) {
    console.error('Failed to load notebooks:', error)
    notebooksError.value = error instanceof Error ? error.message : String(error)
    notebooks.value = []
  } finally {
    loading.value = false
  }
}

function triggerFilePicker() {
  fileInput.value?.click()
}

function handleFileSelect(event) {
  const files = event.target.files
  if (files.length > 0) {
    uploadFile(files[0])
  }
}

function handleFileDrop(event) {
  isDragging.value = false
  const files = event.dataTransfer.files
  if (files.length > 0) {
    uploadFile(files[0])
  }
}

async function uploadFile(file) {
  uploadError.value = ''
  uploadSuccess.value = false
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
    uploadSuccess.value = true
    setTimeout(() => {
      uploadProgress.value = 0
      uploadSuccess.value = false
      successMessage.value = ''
      ingestionStatusMessage.value = ''
      ingestionNotebookID.value = ''
      if (fileInput.value) {
        fileInput.value.value = ''
      }
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
  showSyllabusModal.value = false
  isConfirmingDraft.value = false
}

function addDraftChapter() {
  const start =
    draftChapters.value.length > 0
      ? Number(draftChapters.value[draftChapters.value.length - 1].end_page) + 1
      : 1
  draftChapters.value.push({
    title: `Chapter ${draftChapters.value.length + 1}`,
    start_page: Math.min(start, draftPageCount.value),
    end_page: draftPageCount.value,
  })
}

function removeDraftChapter(index) {
  draftChapters.value.splice(index, 1)
}

function sanitizeChapterPages(chapter) {
  chapter.start_page = Math.max(1, Math.min(Number(chapter.start_page) || 1, draftPageCount.value))
  chapter.end_page = Math.max(
    chapter.start_page,
    Math.min(Number(chapter.end_page) || chapter.start_page, draftPageCount.value)
  )
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

async function confirmSyllabusDraft() {
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
    if (titleChanged) {
      const titleResult = await apiUpdateNotebookTitle(draftNotebookID.value, trimmedTitle)
      if (titleResult?.error) {
        throw new Error(titleResult.error)
      }
      const notebook = notebooks.value.find((nb) => nb.id === draftNotebookID.value)
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
      const notebook = notebooks.value.find((nb) => nb.id === draftNotebookID.value)
      if (notebook) {
        notebook.priority = draftNotebookPriority.value
      }
    }

    // Only call ConfirmNotebookSyllabus if chapters actually changed
    // This avoids expensive re-ingestion when only notebook title or priority changed
    if (chaptersChanged) {
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
    draftError.value = `Failed to confirm syllabus: ${error.message}`
    uploadError.value = `Failed to confirm syllabus: ${error.message}`
    showToast(`Failed to confirm syllabus: ${error.message}`)
  } finally {
    isConfirmingDraft.value = false
  }
}

function downloadNotebook(notebookId) {
  // TODO: Trigger download from backend
  console.log('Download notebook:', notebookId)
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

function getFileIcon(fileType) {
  const icons = {
    pdf: '📕',
    txt: '📄',
    md: '📝',
  }
  return icons[fileType] || '📄'
}

function getTopicTitle(topicId) {
  const topic = availableTopics.value.find((t) => t.id === topicId)
  return topic ? topic.title : 'No topic'
}

function formatStatus(status) {
  if (!status) {
    return 'uploaded'
  }
  return status.replaceAll('_', ' ')
}

function formatDate(dateString) {
  return new Date(dateString).toLocaleDateString()
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

.upload-section {
  display: block;
  margin-bottom: 48px;
}

.upload-card {
  background: var(--surface-container-low);
  border-radius: 16px;
  padding: 24px;
}

.upload-icon {
  font-size: 48px;
  text-align: center;
  margin-bottom: 16px;
}

.upload-card h3 {
  margin: 0 0 8px;
  font-size: 18px;
  color: var(--on-surface);
}

.upload-card p {
  margin: 0 0 16px;
  font-size: 14px;
  color: var(--muted-text);
}

.drop-zone {
  border: 1px solid var(--outline-variant);
  border-radius: 14px;
  padding: 28px;
  text-align: center;
  cursor: pointer;
  transition: all 0.2s ease;
  background: var(--surface-container-lowest);
  min-height: 170px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.drop-zone:hover,
.drop-zone.dragging {
  background: rgba(0, 91, 193, 0.06);
  border-color: var(--primary);
}

.drop-title {
  margin: 0;
  font-size: 18px;
  font-family: 'Manrope', sans-serif;
  font-weight: 700;
  color: var(--on-surface);
}

.upload-cta {
  border: none;
  border-radius: 12px;
  padding: 12px 20px;
  font-size: 14px;
  font-family: 'Manrope', sans-serif;
  font-weight: 700;
  letter-spacing: 0.01em;
  color: var(--on-primary);
  background: linear-gradient(15deg, var(--primary), var(--primary-dim));
  cursor: pointer;
}

.drop-hint {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
}

.progress {
  margin-top: 16px;
  position: relative;
}

.progress-bar {
  height: 4px;
  background: var(--primary);
  border-radius: 2px;
  transition: width 0.3s;
}

.progress span {
  display: block;
  font-size: 12px;
  color: var(--muted-text);
  margin-top: 8px;
  text-align: center;
}

.progress-label {
  margin: 8px 0 0;
  text-align: center;
  font-size: 12px;
  color: var(--muted-text);
}

.indexing-progress {
  margin-top: 12px;
  border: 1px solid var(--outline-variant);
  border-radius: 8px;
  padding: 12px;
  background: var(--surface-container-low);
}

.indexing-progress .progress-bar {
  background: linear-gradient(15deg, #2e7d32, #4caf50);
}

.error-message {
  margin-top: 12px;
  padding: 12px;
  background: #ffebee;
  color: #c62828;
  border-radius: 6px;
  font-size: 14px;
}

.success-message {
  margin-top: 12px;
  padding: 12px;
  background: #e8f5e9;
  color: #2e7d32;
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

.notebook-card {
  background: var(--surface-container);
  border-radius: 12px;
  padding: 16px;
  border: 1px solid var(--outline-variant);
  transition: all 0.2s;
  position: relative;
}

.notebook-card:hover {
  box-shadow: 0 2px 8px rgba(45, 51, 56, 0.06);
}

.notebook-header-card {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.btn-edit-pen {
  position: absolute;
  top: 10px;
  right: 10px;
  border: 0;
  border-radius: 8px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  width: 30px;
  height: 30px;
  font-size: 15px;
  cursor: pointer;
}

.btn-edit-pen:hover {
  background: var(--surface-container-high, #e6e9ef);
}

.file-icon {
  font-size: 28px;
  flex-shrink: 0;
}

.notebook-info h3 {
  margin: 0;
  font-size: 16px;
  color: var(--on-surface);
  word-break: break-word;
}

.meta {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted-text);
}

.pending-index {
  color: #b1532a;
}

.indexing-status {
  color: #1976d2;
  font-weight: 600;
}

.ready-status {
  color: #2e7d32;
  font-weight: 600;
}

.failed-status {
  color: #c62828;
  font-weight: 600;
}

.notebook-topic {
  margin-bottom: 12px;
}

.badge {
  display: inline-block;
  background: var(--surface-container-low);
  color: var(--primary);
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
}

.badge.muted {
  color: var(--muted-text);
}

.notebook-date {
  font-size: 12px;
  color: var(--muted-text);
  margin-bottom: 12px;
}

.notebook-priority {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.priority-label {
  font-size: 12px;
  color: var(--muted-text);
}

.priority-select {
  padding: 4px 8px;
  border: 1px solid var(--outline-variant);
  border-radius: 4px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  font-size: 12px;
  cursor: pointer;
}

.priority-select:hover {
  border-color: var(--primary);
}

.notebook-actions {
  display: flex;
  gap: 8px;
}

.btn-view,
.btn-download,
.btn-delete {
  flex: 1;
  padding: 8px 12px;
  border: none;
  border-radius: 6px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  font-weight: 600;
}

.btn-view {
  background: var(--primary);
  color: var(--on-primary);
}

.btn-view:hover {
  opacity: 0.9;
}

.btn-download {
  background: var(--surface-container-low);
  color: var(--on-surface);
}

.btn-download:hover {
  opacity: 0.9;
}

.btn-delete {
  background: #ffe9e8;
  color: #b5423d;
}

.btn-delete:hover {
  opacity: 0.9;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(18, 22, 28, 0.58);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  z-index: 1200;
}

.modal-card {
  width: min(920px, 100%);
  max-height: 88vh;
  overflow: auto;
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 14px;
  padding: 18px;
  z-index: 1300;
  position: relative;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  color: var(--on-surface);
}

.modal-close {
  border: 0;
  background: transparent;
  color: var(--muted-text);
  font-size: 24px;
  line-height: 1;
  cursor: pointer;
}

.modal-warning {
  margin: 0 0 12px;
  padding: 10px 12px;
  border-radius: 8px;
  background: #fff8e6;
  color: #8c6700;
  font-size: 13px;
}

.modal-title-edit {
  margin: 0 0 12px;
}

.modal-title-edit label {
  display: block;
  font-size: 12px;
  color: var(--muted-text);
  margin-bottom: 6px;
}

.modal-priority-edit {
  margin: 0 0 12px;
}

.modal-priority-edit label {
  display: block;
  font-size: 12px;
  color: var(--muted-text);
  margin-bottom: 6px;
}

.priority-select-modal {
  width: 100%;
  border: 1px solid var(--outline-variant);
  border-radius: 8px;
  padding: 8px 10px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  cursor: pointer;
}

.priority-select-modal:hover {
  border-color: var(--primary);
}

.priority-hint {
  margin: 6px 0 0;
  font-size: 11px;
  color: var(--muted-text);
}

.modal-topics-preview {
  margin: 0 0 16px;
  padding: 12px;
  background: var(--surface-container);
  border-radius: 8px;
  border: 1px solid var(--outline-variant);
}

.modal-topics-preview h4 {
  margin: 0 0 10px;
  font-size: 13px;
  color: var(--on-surface);
}

.topics-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.topic-preview-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  background: var(--surface-container-low);
  border-radius: 6px;
  font-size: 12px;
}

.topic-number {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  background: var(--primary);
  color: var(--on-primary);
  border-radius: 50%;
  font-size: 10px;
  font-weight: 700;
  flex-shrink: 0;
}

.topic-title {
  flex: 1;
  color: var(--on-surface);
  font-weight: 500;
}

.topic-pages {
  color: var(--muted-text);
  font-size: 11px;
}

.topics-empty {
  padding: 8px;
  text-align: center;
}

.topics-empty p {
  margin: 0;
  font-size: 12px;
  color: var(--muted-text);
}

.modal-error {
  margin-bottom: 10px;
}

.chapter-table-wrap {
  overflow-x: auto;
}

.chapter-table {
  width: 100%;
  border-collapse: collapse;
}

.chapter-table th,
.chapter-table td {
  text-align: left;
  border-bottom: 1px solid var(--outline-variant);
  padding: 8px;
  vertical-align: middle;
}

.chapter-table th {
  font-size: 12px;
  color: var(--muted-text);
}

.chapter-input {
  width: 100%;
  border: 1px solid var(--outline-variant);
  border-radius: 8px;
  padding: 8px 10px;
  background: var(--surface-container-low);
  color: var(--on-surface);
}

.chapter-page {
  min-width: 100px;
}

.row-delete {
  border: 0;
  border-radius: 8px;
  padding: 8px 10px;
  background: #ffe9e8;
  color: #b5423d;
  cursor: pointer;
  font-weight: 600;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 14px;
}

.btn-secondary,
.btn-primary {
  border: 0;
  border-radius: 10px;
  padding: 10px 14px;
  font-weight: 700;
  cursor: pointer;
}

.btn-secondary {
  background: var(--surface-container-low);
  color: var(--on-surface);
}

.btn-primary {
  background: var(--primary);
  color: var(--on-primary);
}

.btn-primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.action-toast,
.fallback-toast,
.drafting-toast {
  position: fixed;
  right: 20px;
  bottom: 20px;
  z-index: 1300;
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

.spinner {
  width: 20px;
  height: 20px;
  border: 2px solid var(--outline-variant);
  border-top-color: var(--primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
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

  .modal-actions {
    flex-wrap: wrap;
  }

  .fallback-toast {
    position: fixed;
    left: 20px;
    bottom: 20px;
    z-index: 1300;
  }

  .fallback-toast-inner {
    max-width: 320px;
    padding: 14px 16px;
    background: #b33939;
    color: #fff;
    border-radius: 14px;
    box-shadow: 0 18px 42px rgba(0, 0, 0, 0.18);
    border: 1px solid rgba(255, 255, 255, 0.12);
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

.active-notebook-card {
  border-color: var(--primary);
  box-shadow:
    0 0 0 1px var(--primary),
    0 4px 12px rgba(108, 92, 231, 0.15);
}

.active-icon {
  color: var(--primary);
}

/* ── New Action Buttons ───────────────────────────────── */
.btn-activate {
  background: linear-gradient(135deg, var(--primary), #7c3aed);
  color: white;
  border: none;
  border-radius: 8px;
  padding: 8px 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-activate:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(108, 92, 231, 0.3);
}

.btn-activate:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-sleep {
  background: linear-gradient(135deg, #f59e0b, #d97706);
  color: white;
  border: none;
  border-radius: 8px;
  padding: 8px 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-sleep:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(245, 158, 11, 0.3);
}

/* ── Smart Shelf (Legacy - to be removed) ─────────────── */
.shelf-section {
  margin-top: 32px;
}

.shelf-section h2 {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 700;
  color: var(--on-surface);
}

.shelf-description {
  margin: 0 0 16px;
  font-size: 13px;
  color: var(--muted-text, #888);
}

.shelf-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.shelf-column {
  background: var(--surface-container-low, #1e1e1e);
  border: 1px solid var(--outline-variant, #333);
  border-radius: 16px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.shelf-column h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: var(--on-surface);
}

.shelf-empty {
  text-align: center;
  padding: 24px 16px;
  color: var(--muted-text, #888);
  font-size: 13px;
  border: 1px dashed var(--outline-variant, #333);
  border-radius: 12px;
}

.shelf-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.shelf-card {
  background: var(--surface-container-lowest, #161616);
  border: 1px solid var(--outline-variant, #333);
  border-radius: 12px;
  padding: 12px 14px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.shelf-card-details h4 {
  margin: 0 0 3px;
  font-size: 13px;
  font-weight: 700;
  color: var(--on-surface);
}

.shelf-card-meta {
  margin: 0;
  font-size: 12px;
  color: var(--muted-text, #888);
}

.shelf-action-btn {
  border: none;
  border-radius: 8px;
  padding: 6px 14px;
  font-weight: 700;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.18s;
  flex-shrink: 0;
  font-family: inherit;
}

.sleep-btn {
  background: rgba(235, 94, 85, 0.12);
  color: #eb5e55;
}

.sleep-btn:hover {
  background: #eb5e55;
  color: #fff;
}

.activate-btn {
  background: rgba(108, 92, 231, 0.12);
  color: var(--primary, #6c5ce7);
}

.activate-btn:hover:not(:disabled) {
  background: var(--primary, #6c5ce7);
  color: #fff;
}

.activate-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

@media (max-width: 640px) {
  .shelf-grid {
    grid-template-columns: 1fr;
  }
}
</style>
