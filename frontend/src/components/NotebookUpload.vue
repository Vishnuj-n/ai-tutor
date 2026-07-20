<template>
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
</template>

<script setup>
import { ref } from 'vue'

defineProps({
  uploadProgress: { type: Number, default: 0 },
  ingestionStatusMessage: { type: String, default: '' },
  indexingStatusMessage: { type: String, default: '' },
  uploadError: { type: String, default: '' },
  successMessage: { type: String, default: '' },
})

const emit = defineEmits(['upload-file'])

const fileInput = ref(null)
const isDragging = ref(false)

function triggerFilePicker() {
  fileInput.value?.click()
}

function handleFileSelect(event) {
  const files = event.target.files
  if (files.length > 0) {
    emit('upload-file', files[0])
  }
  // Reset input so the same file can be re-selected
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}

function handleFileDrop(event) {
  isDragging.value = false
  const files = event.dataTransfer.files
  if (files.length > 0) {
    emit('upload-file', files[0])
  }
}
</script>

<style scoped>
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
</style>
