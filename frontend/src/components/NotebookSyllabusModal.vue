<template>
  <div v-if="show" class="modal-backdrop">
    <div class="modal-card">
      <div class="modal-header">
        <h3>Verify Syllabus Chapters</h3>
        <button type="button" class="modal-close" :disabled="isCleaning" @click="close">×</button>
      </div>

      <p class="modal-warning">
        Use absolute PDF page numbers. Page labels shown inside the PDF viewer may differ from file
        page numbers.
      </p>

      <div class="modal-title-edit">
        <label for="notebook-title">Notebook title</label>
        <input
          id="notebook-title"
          v-model="localTitle"
          type="text"
          class="chapter-input"
          placeholder="Notebook name"
          :disabled="isCleaning"
        />
      </div>

      <div class="modal-priority-edit">
        <label for="notebook-priority">Notebook priority (1-10)</label>
        <select
          id="notebook-priority"
          v-model.number="localPriority"
          class="priority-select-modal"
          :disabled="isCleaning"
        >
          <option v-for="n in 10" :key="n" :value="n">
            {{ n }} - {{ n === 1 ? 'Lowest' : n === 10 ? 'Highest' : n === 5 ? 'Default' : '' }}
          </option>
        </select>
        <p class="priority-hint">Higher-priority notebooks appear earlier in your study queue.</p>
      </div>

      <div v-if="error" class="error-message modal-error">{{ error }}</div>

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
            <tr v-for="(chapter, index) in localChapters" :key="`chapter-${index}`">
              <td>
                <input
                  v-model="chapter.title"
                  type="text"
                  class="chapter-input"
                  placeholder="Chapter title"
                  :disabled="isCleaning"
                />
              </td>
              <td>
                <input
                  v-model.number="chapter.start_page"
                  type="number"
                  min="1"
                  :max="pageCount"
                  class="chapter-input chapter-page"
                  :disabled="isCleaning"
                  @change="sanitizeChapterPages(chapter)"
                />
              </td>
              <td>
                <input
                  v-model.number="chapter.end_page"
                  type="number"
                  min="1"
                  :max="pageCount"
                  class="chapter-input chapter-page"
                  :disabled="isCleaning"
                  @change="sanitizeChapterPages(chapter)"
                />
              </td>
              <td>
                <button
                  type="button"
                  class="row-delete"
                  :disabled="isCleaning"
                  @click="removeDraftChapter(index)"
                >
                  Delete
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="modal-actions">
        <button type="button" class="btn-secondary" :disabled="isCleaning" @click="addDraftChapter">
          Add Chapter
        </button>
        <button
          type="button"
          class="btn-ai-cleanup"
          :disabled="isCleaning"
          @click="$emit('ai-cleanup')"
        >
          {{ isCleaning ? 'Cleaning up...' : 'AI Clean Up' }}
        </button>
        <button type="button" class="btn-secondary" :disabled="isCleaning" @click="close">
          Cancel
        </button>
        <button
          type="button"
          class="btn-primary"
          :disabled="isConfirming || isCleaning"
          @click="confirm"
        >
          {{ isConfirming ? 'Confirming...' : 'Confirm and Ingest' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  show: { type: Boolean, default: false },
  notebookTitle: { type: String, default: '' },
  notebookPriority: { type: Number, default: 5 },
  pageCount: { type: Number, default: 1 },
  chapters: { type: Array, default: () => [] },
  isConfirming: { type: Boolean, default: false },
  isCleaning: { type: Boolean, default: false },
  error: { type: String, default: '' },
})

const emit = defineEmits(['close', 'ai-cleanup', 'confirm'])

const localTitle = ref('')
const localPriority = ref(5)
const localChapters = ref([])

watch(
  () => props.notebookTitle,
  (val) => {
    localTitle.value = val
  },
  { immediate: true }
)

watch(
  () => props.notebookPriority,
  (val) => {
    localPriority.value = val
  },
  { immediate: true }
)

watch(
  () => props.chapters,
  (val) => {
    if (Array.isArray(val)) {
      localChapters.value = val.map((ch) => ({
        title: ch.title || '',
        start_page: ch.start_page || 1,
        end_page: ch.end_page || 1,
      }))
    } else {
      localChapters.value = []
    }
  },
  { immediate: true, deep: true }
)

function close() {
  if (props.isCleaning) return
  emit('close')
}

function addDraftChapter() {
  const start =
    localChapters.value.length > 0
      ? Number(localChapters.value[localChapters.value.length - 1].end_page) + 1
      : 1
  localChapters.value.push({
    title: `Chapter ${localChapters.value.length + 1}`,
    start_page: Math.min(start, props.pageCount),
    end_page: props.pageCount,
  })
}

function removeDraftChapter(index) {
  localChapters.value.splice(index, 1)
}

function sanitizeChapterPages(chapter) {
  chapter.start_page = Math.max(1, Math.min(Number(chapter.start_page) || 1, props.pageCount))
  chapter.end_page = Math.max(
    chapter.start_page,
    Math.min(Number(chapter.end_page) || chapter.start_page, props.pageCount)
  )
}

function confirm() {
  emit('confirm', {
    title: localTitle.value,
    priority: localPriority.value,
    chapters: localChapters.value,
  })
}
</script>

<style scoped>
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
  width: 90vw;
  max-width: 1100px;
  max-height: 90vh;
  overflow: auto;
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 14px;
  padding: 24px;
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

.error-message {
  margin-top: 12px;
  padding: 12px;
  background: #ffebee;
  color: #c62828;
  border-radius: 6px;
  font-size: 14px;
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

.chapter-table th:nth-child(1),
.chapter-table td:nth-child(1) {
  width: 50%;
  min-width: 250px;
}

.chapter-table th:nth-child(2),
.chapter-table td:nth-child(2),
.chapter-table th:nth-child(3),
.chapter-table td:nth-child(3) {
  width: 20%;
  min-width: 110px;
}

.chapter-table th:nth-child(4),
.chapter-table td:nth-child(4) {
  width: 10%;
  min-width: 80px;
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

.btn-ai-cleanup {
  border: 0;
  border-radius: 10px;
  padding: 10px 14px;
  font-weight: 700;
  cursor: pointer;
  background: linear-gradient(135deg, #7c3aed, #6d28d9);
  color: white;
  transition: all 0.2s;
}

.btn-ai-cleanup:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(124, 58, 237, 0.3);
}

.btn-ai-cleanup:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
