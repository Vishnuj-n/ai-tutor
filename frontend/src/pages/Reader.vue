<template>
  <section class="page">
    <header class="head">
      <p class="eyebrow">Reader</p>
      <h1>{{ topicTitle }}</h1>
      <p class="meta">
        <span>{{ sectionCount }} sections</span>
        <span v-if="notebookTitle">Notebook: {{ notebookTitle }}</span>
      </p>
    </header>

    <article class="panel topic-picker">
      <label class="field">
        <span>Topic</span>
        <select v-model="topicID" :disabled="loadingTopics || loading || availableTopics.length === 0" @change="onTopicChange">
          <option disabled value="">
            {{ loadingTopics ? 'Loading topics...' : availableTopics.length === 0 ? 'No topics available' : 'Select a topic' }}
          </option>
          <option v-for="topic in availableTopics" :key="topic.id" :value="topic.id">
            {{ topic.title }}
          </option>
        </select>
      </label>
    </article>

    <article v-if="loadError" class="panel error">{{ loadError }}</article>

    <div v-else class="workspace">
      <article class="panel stage" :class="{ busy: loading }">
        <div class="stage-toolbar">
          <p>Document Stage</p>
          <p class="page-chip" v-if="viewerReady">Page {{ currentPage }} / {{ totalPages }}</p>
        </div>

        <div v-if="loading" class="loading">Loading reader bundle...</div>
        <div v-else-if="!notebookUrl || fileType !== 'pdf'" class="empty">
          PDF not available for this topic. Section rail and AI actions still work.
        </div>
        <div v-else class="pdf-shell">
          <div class="canvas-wrap">
            <canvas ref="pdfCanvas"></canvas>
          </div>
          <div class="pdf-controls">
            <button class="secondary" :disabled="!viewerReady || currentPage <= 1" @click="goToPage(currentPage - 1)">
              Prev
            </button>
            <button class="secondary" :disabled="!viewerReady || currentPage >= totalPages" @click="goToPage(currentPage + 1)">
              Next
            </button>
          </div>
        </div>
      </article>

      <aside class="panel rail">
        <div class="rail-head">
          <h2>Action Rail</h2>
          <button class="primary" :disabled="learnLoading || !topicID" @click="markLearned">
            {{ learnLoading ? 'Preparing...' : 'Mark as Learned' }}
          </button>
        </div>

        <article class="toast" v-if="toastMessage" role="status" aria-live="polite" aria-atomic="true">
          {{ toastMessage }}
        </article>

        <div class="section-list" v-if="sections.length > 0">
          <button
            v-for="section in sections"
            :key="section.id"
            class="section-item"
            :class="{ active: activeSection?.id === section.id }"
            @click="selectSection(section)"
          >
            <span class="dot" aria-hidden="true"></span>
            <span class="text-wrap">
              <span class="title">{{ section.heading }}</span>
              <span class="detail">{{ section.page_num > 0 ? `Page ${section.page_num}` : 'Page map missing' }}</span>
            </span>
          </button>
        </div>

        <div v-else class="empty">No sections available for this topic.</div>

        <div class="section-body" v-if="activeSection">
          <h3>{{ activeSection.heading }}</h3>
          <p>{{ activeSection.content }}</p>
        </div>

        <label class="field">
          <span>Explain this</span>
          <textarea
            v-model="explainPrompt"
            placeholder="Ask for summary, analogy, or clarification..."
            :disabled="explainLoading || !activeSection"
          ></textarea>
        </label>

        <button class="primary explain" :disabled="explainLoading || !activeSection" @click="explainSection">
          {{ explainLoading ? 'Explaining...' : 'Explain this' }}
        </button>

        <article v-if="explainError" class="error">{{ explainError }}</article>

        <article v-if="explainAnswer" class="response">
          <h3>AI Clarification</h3>
          <p>{{ explainAnswer }}</p>
        </article>
      </aside>
    </div>
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { explainReaderSection, generateFlashcards, getAvailableTopics, getReaderTopicBundle } from '../services/appApi'

const route = useRoute()

const topicID = ref(typeof route.query.topic === 'string' ? route.query.topic : '')
const topicTitle = ref('Reader')
const notebookTitle = ref('')
const notebookUrl = ref('')
const fileType = ref('')
const sections = ref([])
const activeSection = ref(null)

const loadError = ref('')
const loading = ref(true)
const explainPrompt = ref('')
const explainAnswer = ref('')
const explainError = ref('')
const explainLoading = ref(false)
const learnLoading = ref(false)
const toastMessage = ref('')

const pdfCanvas = ref(null)
const pdfDoc = ref(null)
const renderTask = ref(null)
const totalPages = ref(0)
const currentPage = ref(1)

const sectionCount = computed(() => sections.value.length)
const viewerReady = computed(() => !!pdfDoc.value && totalPages.value > 0)

const availableTopics = ref([])
const loadingTopics = ref(true)

let toastTimer = null

onMounted(async () => {
  await loadTopicOptions()
  ensureSelectedTopic()
  await loadBundle()
})

onBeforeUnmount(() => {
  clearToast()
  cancelRender()
  pdfDoc.value = null
})

async function loadBundle() {
  if (!topicID.value) {
    loadError.value = 'Select topic to start Reader.'
    loading.value = false
    return
  }

  loading.value = true
  loadError.value = ''
  explainError.value = ''
  explainAnswer.value = ''

  try {
    const bundle = await getReaderTopicBundle(topicID.value)
    if (bundle?.error) {
      loadError.value = bundle.error
      return
    }

    topicTitle.value = bundle?.topic_title || 'Reader'
    notebookTitle.value = bundle?.notebook_title || ''
    notebookUrl.value = bundle?.notebook_url || ''
    fileType.value = (bundle?.file_type || '').toLowerCase()
    sections.value = Array.isArray(bundle?.sections) ? bundle.sections : []
    activeSection.value = sections.value[0] || null

    if (notebookUrl.value && fileType.value === 'pdf') {
      await openPdf(notebookUrl.value)
      if (activeSection.value) {
        await jumpToSectionPage(activeSection.value)
      }
    }
  } catch (err) {
    loadError.value = err?.message || 'Failed to load reader bundle'
  } finally {
    loading.value = false
  }
}

async function loadTopicOptions() {
  loadingTopics.value = true
  try {
    const topics = await getAvailableTopics()
    availableTopics.value = Array.isArray(topics) ? topics : []
  } catch (err) {
    availableTopics.value = []
  } finally {
    loadingTopics.value = false
  }
}

function ensureSelectedTopic() {
  if (topicID.value) {
    return
  }
  const fallback = availableTopics.value[0]?.id
  if (typeof fallback === 'string') {
    topicID.value = fallback
  }
}

async function onTopicChange() {
  clearToast()
  await loadBundle()
}

async function openPdf(url) {
  cancelRender()

  const pdfjsLib = await import('pdfjs-dist')
  const worker = await import('pdfjs-dist/build/pdf.worker.min.mjs?url')
  pdfjsLib.GlobalWorkerOptions.workerSrc = worker.default

  const loadingTask = pdfjsLib.getDocument(url)
  const doc = await loadingTask.promise
  pdfDoc.value = doc
  totalPages.value = doc.numPages
  currentPage.value = Math.max(1, Math.min(currentPage.value, totalPages.value))

  await renderPage(currentPage.value)
}

async function renderPage(pageNum) {
  if (!pdfDoc.value || !pdfCanvas.value) {
    return
  }

  cancelRender()
  const page = await pdfDoc.value.getPage(pageNum)
  const viewport = page.getViewport({ scale: 1.25 })

  const canvas = pdfCanvas.value
  const context = canvas.getContext('2d')
  canvas.width = viewport.width
  canvas.height = viewport.height

  renderTask.value = page.render({
    canvasContext: context,
    viewport,
  })

  await renderTask.value.promise
  renderTask.value = null
  currentPage.value = pageNum
}

function cancelRender() {
  if (renderTask.value && typeof renderTask.value.cancel === 'function') {
    renderTask.value.cancel()
  }
  renderTask.value = null
}

async function goToPage(pageNum) {
  if (!viewerReady.value) {
    return
  }
  const target = Math.max(1, Math.min(pageNum, totalPages.value))
  await renderPage(target)
}

async function selectSection(section) {
  activeSection.value = section
  explainError.value = ''
  explainAnswer.value = ''
  await jumpToSectionPage(section)
}

async function jumpToSectionPage(section) {
  if (!viewerReady.value) {
    return
  }

  const page = Number(section?.page_num)
  if (!Number.isFinite(page) || page <= 0) {
    showToast('Section page mapping missing. Staying on current page.')
    return
  }

  await goToPage(page)
}

async function explainSection() {
  if (!activeSection.value) {
    return
  }

  explainLoading.value = true
  explainError.value = ''
  explainAnswer.value = ''

  try {
    const result = await explainReaderSection(activeSection.value.id, explainPrompt.value)
    if (result?.error) {
      explainError.value = result.error
      return
    }

    explainAnswer.value = result?.answer || 'No explanation returned.'
  } catch (err) {
    explainError.value = err?.message || 'Failed to explain selected section'
  } finally {
    explainLoading.value = false
  }
}

async function markLearned() {
  if (!topicID.value) {
    return
  }

  learnLoading.value = true
  explainError.value = ''

  try {
    const result = await generateFlashcards(topicID.value)
    if (result?.error) {
      explainError.value = result.error
      return
    }

    const count = Array.isArray(result?.cards) ? result.cards.length : 0
    showToast(`Marked learned. ${count} flashcards ready for review.`)
  } catch (err) {
    explainError.value = err?.message || 'Failed to mark topic as learned'
  } finally {
    learnLoading.value = false
  }
}

function showToast(message) {
  clearToast()
  toastMessage.value = message
  toastTimer = setTimeout(() => {
    toastMessage.value = ''
    toastTimer = null
  }, 2400)
}

function clearToast() {
  if (toastTimer) {
    clearTimeout(toastTimer)
    toastTimer = null
  }
}
</script>

<style scoped>
.page {
  display: grid;
  gap: 16px;
}

.head {
  display: grid;
  gap: 8px;
}

.eyebrow {
  margin: 0;
  font-size: 11px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-weight: 700;
}

h1 {
  margin: 0;
  font-size: 38px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.meta {
  margin: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  color: var(--muted-text);
  font-size: 13px;
}

.workspace {
  display: grid;
  grid-template-columns: 1.75fr 1fr;
  gap: 14px;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 14px;
  border: 1px solid var(--surface-container-low);
  padding: 14px;
  min-height: 0;
}

.topic-picker {
  padding: 12px 14px;
}

.topic-picker .field {
  max-width: 360px;
}

.stage {
  display: grid;
  gap: 10px;
  align-content: start;
}

.stage-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  color: var(--muted-text);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-weight: 700;
}

.page-chip {
  background: var(--surface-container-low);
  border-radius: 999px;
  padding: 4px 10px;
}

.pdf-shell {
  display: grid;
  gap: 10px;
}

.canvas-wrap {
  overflow: auto;
  border-radius: 12px;
  background: #f3f4f6;
  border: 1px solid #e2e4e8;
  min-height: 220px;
  display: grid;
  place-items: start center;
  padding: 10px;
}

canvas {
  max-width: 100%;
  height: auto;
  box-shadow: 0 8px 24px rgba(10, 12, 18, 0.14);
  background: #fff;
}

.pdf-controls {
  display: flex;
  gap: 8px;
}

.rail {
  display: grid;
  gap: 10px;
  align-content: start;
}

.rail-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

h2,
h3 {
  margin: 0;
  font-family: 'Manrope', sans-serif;
}

h2 {
  font-size: 20px;
}

h3 {
  font-size: 17px;
}

.section-list {
  display: grid;
  gap: 6px;
  max-height: 220px;
  overflow: auto;
  padding-right: 4px;
}

.section-item {
  border: 1px solid var(--surface-container-low);
  background: var(--surface-container-lowest);
  color: var(--on-surface);
  border-radius: 10px;
  padding: 10px;
  display: grid;
  grid-template-columns: 14px 1fr;
  gap: 8px;
  text-align: left;
  cursor: pointer;
  transition: transform 120ms ease, border-color 120ms ease, background-color 120ms ease;
}

.section-item:hover {
  border-color: var(--primary-dim);
}

.section-item:active {
  transform: scale(0.95);
}

.section-item.active {
  border-color: var(--primary);
  background: color-mix(in srgb, var(--primary) 12%, var(--surface-container-lowest));
}

.dot {
  width: 8px;
  height: 8px;
  margin-top: 6px;
  border-radius: 999px;
  background: var(--surface-container-low);
  transition: transform 120ms ease, background-color 120ms ease;
}

.section-item.active .dot {
  background: var(--primary);
  transform: translateX(2px);
}

.text-wrap {
  display: grid;
  gap: 2px;
}

.title {
  font-size: 13px;
  font-weight: 700;
}

.detail {
  font-size: 12px;
  color: var(--muted-text);
}

.section-body {
  background: var(--surface-container-low);
  border-radius: 10px;
  padding: 12px;
  display: grid;
  gap: 8px;
}

.section-body p {
  margin: 0;
  font-size: 14px;
  line-height: 1.6;
  max-height: 140px;
  overflow: auto;
}

.field {
  display: grid;
  gap: 6px;
}

.field span {
  font-size: 12px;
  color: var(--muted-text);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-weight: 700;
}

textarea {
  width: 100%;
  min-height: 90px;
  border: 1px solid var(--surface-container-low);
  outline: 0;
  border-radius: 10px;
  background: var(--surface-container-lowest);
  padding: 10px;
  color: var(--on-surface);
  font: inherit;
  resize: vertical;
}

select {
  width: 100%;
  border: 1px solid var(--surface-container-low);
  outline: 0;
  border-radius: 10px;
  background: var(--surface-container-lowest);
  padding: 10px;
  color: var(--on-surface);
  font: inherit;
}

button {
  border: 0;
  border-radius: 10px;
  padding: 10px 14px;
  font-weight: 700;
  cursor: pointer;
  transition: opacity 120ms ease, transform 120ms ease;
}

button:active {
  transform: scale(0.95);
}

button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.primary {
  color: var(--on-primary);
  background: linear-gradient(160deg, var(--primary), var(--primary-dim));
}

.secondary {
  color: var(--on-surface);
  background: var(--surface-container-low);
}

.toast {
  background: color-mix(in srgb, #16a34a 14%, var(--surface-container-lowest));
  border: 1px solid color-mix(in srgb, #16a34a 35%, var(--surface-container-low));
  color: #14532d;
  border-radius: 10px;
  padding: 10px 12px;
  font-size: 13px;
}

.response {
  background: var(--surface-container-low);
  border-radius: 10px;
  padding: 12px;
  display: grid;
  gap: 8px;
}

.response p {
  margin: 0;
  font-size: 14px;
  line-height: 1.6;
}

.error {
  color: #b42318;
  background: color-mix(in srgb, #b42318 12%, var(--surface-container-lowest));
  border: 1px solid color-mix(in srgb, #b42318 30%, var(--surface-container-low));
  border-radius: 10px;
  padding: 10px 12px;
  font-size: 13px;
}

.loading,
.empty {
  color: var(--muted-text);
  background: var(--surface-container-low);
  border-radius: 10px;
  padding: 12px;
  font-size: 14px;
}

@media (max-width: 1080px) {
  .workspace {
    grid-template-columns: 1fr;
  }

  .section-list {
    max-height: 180px;
  }
}
</style>
