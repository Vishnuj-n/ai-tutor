<template>
  <div class="notebook-page">
    <div class="notebook-header">
      <h1>Notebooks</h1>
      <p class="subtitle">Upload and manage your learning materials</p>
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
        </div>

        <div v-if="uploadError" class="error-message">
          {{ uploadError }}
        </div>

        <div v-if="uploadSuccess" class="success-message">✓ Upload successful!</div>
      </div>

      <!-- Topic Selection -->
      <div class="topic-selection">
        <label>Link to Topic (Optional)</label>
        <select v-model="selectedTopic">
          <option value="">No topic</option>
          <option v-for="topic in availableTopics" :key="topic.id" :value="topic.id">
            {{ topic.title }}
          </option>
        </select>
        <p class="help-text">Linking creates chunks and adds to RAG for Q&A</p>
      </div>
    </div>

    <!-- Notebooks List -->
    <div class="notebooks-list">
      <h2>Your Notebooks</h2>

      <div v-if="loading" class="loading">Loading notebooks...</div>

      <div v-if="!loading && notebooks.length === 0" class="empty-state">
        <p>No notebooks yet. Upload your first document above!</p>
      </div>

      <div v-if="!loading && notebooks.length > 0" class="notebook-grid">
        <div v-for="notebook in notebooks" :key="notebook.id" class="notebook-card">
          <div class="notebook-header-card">
            <div class="file-icon">{{ getFileIcon(notebook.file_type) }}</div>
            <div class="notebook-info">
              <h3>{{ notebook.title }}</h3>
              <p class="meta">{{ notebook.file_type.toUpperCase() }}</p>
              <p v-if="notebook.page_count > 0" class="meta">{{ notebook.page_count }} pages</p>
              <p class="meta">{{ notebook.chunk_count }} chunks</p>
            </div>
          </div>

          <div v-if="notebook.topic_id" class="notebook-topic">
            <span class="badge">{{ getTopicTitle(notebook.topic_id) }}</span>
          </div>

          <div class="notebook-date">Uploaded: {{ formatDate(notebook.uploaded_at) }}</div>

          <div class="notebook-actions">
            <button class="btn-view" @click="viewNotebook(notebook.id)">View</button>
            <button class="btn-download" @click="downloadNotebook(notebook.id)">Download</button>
            <button class="btn-delete" @click="deleteNotebook(notebook.id)">Delete</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import {
  getAvailableTopics,
  getNotebooks as fetchNotebooks,
  uploadNotebook as apiUploadNotebook,
  deleteNotebook as apiDeleteNotebook,
} from '../services/appApi'

const fileInput = ref(null)
const isDragging = ref(false)
const uploadProgress = ref(0)
const uploadError = ref('')
const uploadSuccess = ref(false)
const selectedTopic = ref('')
const notebooks = ref([])
const availableTopics = ref([])
const loading = ref(false)

onMounted(async () => {
  // Load available topics and notebooks
  await loadTopics()
  await loadNotebooks()
})

async function loadTopics() {
  try {
    const topics = await getAvailableTopics()
    availableTopics.value = Array.isArray(topics) ? topics : []
  } catch (error) {
    console.error('Failed to load topics:', error)
    availableTopics.value = []
  }
}

async function loadNotebooks() {
  loading.value = true
  try {
    const result = await fetchNotebooks('')
    if (Array.isArray(result) && result.length > 0 && result[0].error) {
      throw new Error(result[0].error)
    }
    notebooks.value = Array.isArray(result) ? result : []
  } catch (error) {
    console.error('Failed to load notebooks:', error)
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
    // Read file as Buffer/bytes
    const arrayBuffer = await file.arrayBuffer()
    const bytes = new Uint8Array(arrayBuffer)
    uploadProgress.value = 50

    const result = await apiUploadNotebook(Array.from(bytes), file.name, selectedTopic.value)
    if (result?.error) {
      throw new Error(result.error)
    }

    uploadProgress.value = 100
    uploadSuccess.value = true
    selectedTopic.value = ''

    // Reset after 2 seconds
    setTimeout(() => {
      uploadProgress.value = 0
      uploadSuccess.value = false
      fileInput.value.value = ''
      void loadNotebooks()
    }, 2000)
  } catch (error) {
    uploadError.value = `Upload failed: ${error.message}`
    uploadProgress.value = 0
  }
}

function viewNotebook(notebookId) {
  // TODO: Navigate to notebook viewer or open preview modal
  console.log('View notebook:', notebookId)
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
  return topic ? topic.title : 'Unknown'
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
  color: var(--on-surface-variant);
}

.upload-section {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 24px;
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
  border: 1px solid rgba(45, 51, 56, 0.2);
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
  color: var(--on-surface-variant);
  margin-top: 8px;
  text-align: center;
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

.topic-selection {
  background: var(--surface-container);
  border-radius: 12px;
  padding: 24px;
  border: 2px solid var(--outline-variant);
}

.topic-selection label {
  display: block;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--on-surface);
}

.topic-selection select {
  width: 100%;
  padding: 10px;
  border: 1px solid var(--outline);
  border-radius: 6px;
  background: var(--surface);
  color: var(--on-surface);
  font-size: 14px;
  margin-bottom: 12px;
}

.help-text {
  margin: 0;
  font-size: 12px;
  color: var(--on-surface-variant);
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
  color: var(--on-surface-variant);
}

.empty-state {
  text-align: center;
  padding: 48px;
  background: var(--surface-container);
  border-radius: 12px;
  color: var(--on-surface-variant);
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
}

.notebook-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.notebook-header-card {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
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
  color: var(--on-surface-variant);
}

.notebook-topic {
  margin-bottom: 12px;
}

.badge {
  display: inline-block;
  background: var(--primary-dim);
  color: var(--primary);
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
}

.notebook-date {
  font-size: 12px;
  color: var(--on-surface-variant);
  margin-bottom: 12px;
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
  background: var(--secondary);
  color: var(--on-secondary);
}

.btn-download:hover {
  opacity: 0.9;
}

.btn-delete {
  background: var(--error-dim);
  color: var(--error);
}

.btn-delete:hover {
  opacity: 0.9;
}

@media (max-width: 768px) {
  .upload-section {
    grid-template-columns: 1fr;
  }

  .notebook-grid {
    grid-template-columns: 1fr;
  }
}
</style>
