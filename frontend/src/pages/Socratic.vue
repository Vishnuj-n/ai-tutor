<template>
  <section class="socratic-page">
    <header class="page-header">
      <p class="eyebrow">Socratic Tutor</p>
      <h1>Guided Thinking</h1>
    </header>

    <article class="chat-shell">
      <div class="chat-toolbar">
        <div class="control-group">
          <label for="topic-select">Topic</label>
          <select id="topic-select" v-model="selectedTopicID" @change="handleTopicChange">
            <option value="">Choose topic</option>
            <option v-for="topic in availableTopics" :key="topic.id" :value="topic.id">
              {{ topic.title }}
            </option>
          </select>
        </div>

        <div class="control-group">
          <label for="notebook-select">Notebook</label>
          <select id="notebook-select" v-model="selectedNotebookID" @change="handleNotebookChange">
            <option value="">All notebooks</option>
            <option v-for="notebook in notebooks" :key="notebook.id" :value="notebook.id">
              {{ formatNotebookLabel(notebook) }}
            </option>
          </select>
        </div>

        <button type="button" class="clear-btn" @click="clearConversation">Clear Chat</button>
      </div>

      <p v-if="selectionHint" class="selection-hint">{{ selectionHint }}</p>

      <!-- Conversation Analytics Panel -->
      <div class="conversation-analytics">
        <div class="analytics-card">
          <h3 class="analytics-title">Session Metrics</h3>
          <div class="metric-row">
            <span class="metric-label">Messages Exchanged</span>
            <span class="metric-value">{{ messages.length }}</span>
          </div>
          <div class="metric-row">
            <span class="metric-label">Session Duration</span>
            <span class="metric-value">{{ Math.floor(messages.length * 2) }} min</span>
          </div>
          <div class="metric-row">
            <span class="metric-label">Topics Covered</span>
            <span class="metric-value">{{ availableTopics.length }}</span>
          </div>
        </div>
        <div class="analytics-card">
          <h3 class="analytics-title">Learning Progress</h3>
          <div class="metric-row">
            <span class="metric-label">Questions Asked</span>
            <span class="metric-value">{{ messages.filter(m => m.role === 'user').length }}</span>
          </div>
          <div class="metric-row">
            <span class="metric-label">Hints Received</span>
            <span class="metric-value">{{ messages.filter(m => m.role === 'assistant').length }}</span>
          </div>
          <div class="metric-row">
            <span class="metric-label">Citations Provided</span>
            <span class="metric-value">{{ messages.reduce((count, m) => count + (m.citations?.length || 0), 0) }}</span>
          </div>
        </div>
      </div>

      <div ref="threadRef" class="chat-thread">
        <div v-if="messages.length === 0" class="empty-state">
          <h3>Start the Socratic conversation</h3>
          <p>
            Select a topic or notebook, then ask a grounded question. The tutor will respond with a
            guiding question and a hint based on the selected material.
          </p>
        </div>

        <div v-for="(message, idx) in messages" :key="idx" :class="['bubble-row', message.role]">
          <article class="bubble">
            <p v-if="message.role === 'user'" class="message-text">{{ message.text }}</p>
            <div v-else class="markdown-body" v-html="renderMarkdown(message.text)"></div>

            <div v-if="message.role === 'assistant' && message.error" class="message-error">
              {{ message.error }}
            </div>

            <div
              v-if="message.role === 'assistant' && message.citations && message.citations.length > 0"
              class="citations"
            >
              <p class="citation-label">Citations</p>
              <ul>
                <li v-for="(citation, citationIdx) in message.citations" :key="citationIdx">
                  {{ citation }}
                </li>
              </ul>
            </div>
          </article>
        </div>

        <div v-if="isLoading" class="bubble-row assistant">
          <article class="bubble loading-bubble">
            <span></span>
            <span></span>
            <span></span>
          </article>
        </div>
      </div>

      <form class="composer" @submit.prevent="submitQuestion">
        <textarea
          v-model="inputQuestion"
          class="composer-input"
          aria-label="Question"
          placeholder="Ask a grounded question about your material, and the tutor will guide you with a Socratic hint."
          :disabled="isLoading"
          @keydown="handleComposerKeydown"
        ></textarea>

        <div class="composer-footer">
          <p>Enter to send, Shift+Enter for new line</p>
          <button type="submit" class="send-btn" :disabled="!canSend">
            {{ isLoading ? 'Thinking...' : 'Send' }}
          </button>
        </div>
      </form>
    </article>

    <p v-if="globalError" class="global-error">{{ globalError }}</p>
  </section>
</template>

<script setup>
import { computed, nextTick, onMounted, ref } from 'vue'
import {
  askAI as askAIRequest,
  getAvailableTopics as fetchAvailableTopics,
  getNotebooks as fetchNotebooks,
} from '../services/appApi'
import { renderMarkdown } from '../services/markdown'

const availableTopics = ref([])
const notebooks = ref([])
const selectedTopicID = ref('')
const selectedNotebookID = ref('')
const inputQuestion = ref('')
const messages = ref([])
const isLoading = ref(false)
const globalError = ref('')
const threadRef = ref(null)

const selectedNotebook = computed(() =>
  notebooks.value.find((notebook) => notebook.id === selectedNotebookID.value)
)

const effectiveTopicID = computed(() => {
  if (selectedTopicID.value) {
    return selectedTopicID.value
  }

  if (selectedNotebook.value && selectedNotebook.value.topic_id) {
    return selectedNotebook.value.topic_id
  }

  return ''
})

const canSend = computed(() => {
  return !isLoading.value && inputQuestion.value.trim().length > 0 && effectiveTopicID.value !== ''
})

const selectionHint = computed(() => {
  if (selectedNotebook.value && !selectedNotebook.value.topic_id && !selectedTopicID.value) {
    return 'Selected notebook has no linked topic yet. Choose a topic to run RAG.'
  }

  if (!effectiveTopicID.value) {
    return 'Choose a topic or select a notebook that is linked to a topic.'
  }

  const topic = availableTopics.value.find((item) => item.id === effectiveTopicID.value)
  return topic ? `Current retrieval scope: ${topic.title}` : ''
})

onMounted(async () => {
  await Promise.all([loadTopics(), loadNotebooks()])
})

async function loadTopics() {
  try {
    const result = await fetchAvailableTopics()
    const list = Array.isArray(result) ? result : Array.isArray(result?.topics) ? result.topics : []
    availableTopics.value = list

    if (!selectedTopicID.value && availableTopics.value.length > 0) {
      selectedTopicID.value = availableTopics.value[0].id
    }
  } catch (err) {
    globalError.value = `Failed to load topics: ${err.message}`
    availableTopics.value = []
  }
}

async function loadNotebooks() {
  try {
    const result = await fetchNotebooks('')
    notebooks.value = Array.isArray(result) ? result.filter((item) => !item.error) : []
  } catch (err) {
    globalError.value = `Failed to load notebooks: ${err.message}`
    notebooks.value = []
  }
}

function handleTopicChange() {
  globalError.value = ''
}

function handleNotebookChange() {
  globalError.value = ''
  const notebook = selectedNotebook.value
  if (notebook && notebook.topic_id) {
    selectedTopicID.value = notebook.topic_id
  }
}

function clearConversation() {
  messages.value = []
  inputQuestion.value = ''
  globalError.value = ''
}

async function submitQuestion() {
  if (!canSend.value) {
    return
  }

  const question = inputQuestion.value.trim()
  const topicID = effectiveTopicID.value

  messages.value.push({
    role: 'user',
    text: question,
  })

  inputQuestion.value = ''
  isLoading.value = true
  await scrollToBottom()

  try {
    const result = await askAIRequest(topicID, buildSocraticQuestion(question))

    if (result.error) {
      messages.value.push({
        role: 'assistant',
        text: 'Unable to answer this query right now.',
        error: result.error,
      })
    } else {
      messages.value.push({
        role: 'assistant',
        text: result.answer || 'No response generated.',
        citations: result.cited_sections || [],
      })
    }
  } catch (err) {
    globalError.value = `Chat request failed: ${err.message}`
  } finally {
    isLoading.value = false
    await scrollToBottom()
  }
}

const SOCRATIC_INSTRUCTIONS = [
  'You are a Socratic tutor.',
  '- Begin with a short, probing question that helps the student analyze the topic.',
  '- Follow with a concise hint that is grounded only in the selected material and retrieval scope.',
  '- Do not provide the final answer unless the student explicitly requests it.',
  '- Keep responses clear, calm, and focused on guiding thinking rather than giving solutions.',
  '',
]

function buildSocraticQuestion(question) {
  return [...SOCRATIC_INSTRUCTIONS, `Student question: ${question}`].join('\n')
}

function handleComposerKeydown(event) {
  if (event.key !== 'Enter') {
    return
  }

  if (event.shiftKey || event.isComposing) {
    return
  }

  event.preventDefault()
  void submitQuestion()
}

function formatNotebookLabel(notebook) {
  if (notebook.topic_id) {
    const topic = availableTopics.value.find((item) => item.id === notebook.topic_id)
    if (topic) {
      return `${notebook.title} (${topic.title})`
    }
  }
  return notebook.title
}

async function scrollToBottom() {
  await nextTick()
  if (!threadRef.value) {
    return
  }
  threadRef.value.scrollTop = threadRef.value.scrollHeight
}
</script>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Manrope:wght@400;500;600;700&family=Inter:wght@400;500;600;700&display=swap');

.socratic-page {
  display: grid;
  gap: 12px;
  min-height: calc(100vh - 48px);
}

.page-header {
  padding: 2px 2px 0;
}

.eyebrow {
  margin: 0;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-family: 'Inter', sans-serif;
}

h1 {
  margin: 8px 0 0;
  font-size: 42px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.03em;
  color: var(--on-surface);
}

.chat-shell {
  display: grid;
  grid-template-rows: auto auto 1fr auto;
  gap: 10px;
  min-height: 620px;
  background: var(--surface-container-lowest);
  border-radius: 18px;
  padding: 14px;
}

.chat-toolbar {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 10px;
  align-items: end;
}

.control-group {
  display: grid;
  gap: 6px;
}

.control-group label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-family: 'Inter', sans-serif;
}

.control-group select {
  width: 100%;
  border: 1px solid rgba(45, 51, 56, 0.2);
  border-radius: 10px;
  padding: 10px;
  background: var(--surface-container-highest);
  color: var(--on-surface);
  font-family: 'Inter', sans-serif;
  font-size: 14px;
  outline: none;
  transition: all 0.2s ease;
}

.control-group select:focus {
  border-color: var(--primary);
}

.clear-btn {
  border: none;
  border-radius: 10px;
  padding: 10px 14px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  font-family: 'Inter', sans-serif;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.clear-btn:hover {
  background: var(--surface-container-lowest);
  transform: translateY(-1px);
}

.selection-hint {
  margin: 0;
  padding: 8px 10px;
  border-radius: 10px;
  background: var(--surface-container-low);
  color: var(--muted-text);
  font-size: 13px;
  font-family: 'Inter', sans-serif;
}

/* Conversation Analytics Panel */
.conversation-analytics {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1.5rem;
  margin-top: 0.5rem;
}

.analytics-card {
  background: var(--surface-container-low);
  border-radius: 0.75rem;
  padding: 1.5rem;
  transition: all 0.3s ease;
}

.analytics-card:hover {
  background: var(--surface-container-lowest);
  box-shadow: 0 20px 40px rgba(45, 51, 56, 0.06);
  transform: translateY(-2px);
}

.analytics-title {
  font-family: 'Manrope', sans-serif;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--on-surface);
  margin: 0 0 1rem 0;
  letter-spacing: -0.2%;
}

.metric-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem 0;
  border-bottom: 1px solid var(--outline-variant);
}

.metric-row:last-child {
  border-bottom: none;
}

.metric-label {
  font-family: 'Inter', sans-serif;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--muted-text);
}

.metric-value {
  font-family: 'Inter', sans-serif;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--on-surface);
}

.chat-thread {
  overflow-y: auto;
  padding: 12px;
  background: var(--surface-container-highest);
  border-radius: 14px;
  display: grid;
  gap: 10px;
  min-height: 420px;
}

.empty-state {
  margin: auto;
  text-align: center;
  max-width: 440px;
  padding: 24px;
}

.empty-state h3 {
  margin: 0;
  font-size: 24px;
  font-family: 'Manrope', sans-serif;
}

.empty-state p {
  margin: 10px 0 0;
  color: var(--muted-text);
  line-height: 1.5;
}

.bubble-row {
  display: flex;
}

.bubble-row.user {
  justify-content: flex-end;
}

.bubble {
  max-width: 78%;
  border-radius: 14px;
  padding: 10px 12px;
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
}

.bubble-row.user .bubble {
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  border: none;
  color: var(--on-primary);
}

.message-text {
  margin: 0;
  font-size: 14px;
  line-height: 1.55;
  white-space: pre-wrap;
}

.markdown-body {
  font-size: 14px;
  line-height: 1.6;
}

.markdown-body :first-child {
  margin-top: 0;
}

.markdown-body :last-child {
  margin-bottom: 0;
}

.markdown-body p,
.markdown-body ul,
.markdown-body ol,
.markdown-body pre,
.markdown-body blockquote {
  margin: 0 0 8px;
}

.markdown-body code {
  background: rgba(45, 51, 56, 0.08);
  border-radius: 6px;
  padding: 1px 5px;
  font-size: 12px;
}

.markdown-body pre {
  background: var(--surface-container-low);
  border-radius: 8px;
  padding: 8px;
  overflow-x: auto;
}

.markdown-body pre code {
  background: transparent;
  padding: 0;
}

.message-error {
  margin-top: 8px;
  padding: 8px;
  background: #fff0f0;
  color: #b43131;
  border-radius: 10px;
  font-size: 12px;
}

.citations {
  margin-top: 9px;
  border-top: 1px solid rgba(45, 51, 56, 0.12);
  padding-top: 8px;
}

.citation-label {
  margin: 0;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--muted-text);
}

.citations ul {
  margin: 6px 0 0;
  padding-left: 18px;
}

.citations li {
  margin: 4px 0;
  font-size: 12px;
  line-height: 1.4;
}

.loading-bubble {
  width: 58px;
  display: flex;
  justify-content: space-between;
}

.loading-bubble span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--muted-text);
  animation: pulse 1.1s infinite ease-in-out;
}

.loading-bubble span:nth-child(2) {
  animation-delay: 0.12s;
}

.loading-bubble span:nth-child(3) {
  animation-delay: 0.24s;
}

.composer {
  display: grid;
  gap: 8px;
}

.composer-input {
  width: 100%;
  min-height: 88px;
  max-height: 160px;
  resize: vertical;
  border: 1px solid rgba(45, 51, 56, 0.2);
  border-radius: 0.75rem;
  background: var(--surface-container-lowest);
  padding: 1rem;
  color: var(--on-surface);
  font-family: 'Inter', sans-serif;
  font-size: 0.9rem;
  line-height: 1.5;
  outline: none;
  transition: all 0.2s ease;
}

.composer-input:focus {
  border-color: var(--primary);
}

.composer-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.composer-footer p {
  margin: 0;
  color: var(--muted-text);
  font-size: 12px;
}

.send-btn {
  border: 0;
  border-radius: 0.75rem;
  padding: 0.75rem 1.5rem;
  background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dim) 100%);
  color: var(--on-primary);
  font-family: 'Inter', sans-serif;
  font-size: 0.9rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 4px 12px rgba(0, 91, 193, 0.15);
}

.send-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(0, 91, 193, 0.25);
}

.send-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  transform: none;
  box-shadow: none;
}

.global-error {
  margin: 0;
  padding: 10px 12px;
  border-radius: 12px;
  background: #fff0f0;
  color: #b43131;
  font-size: 13px;
}

@keyframes pulse {
  0%,
  80%,
  100% {
    opacity: 0.32;
    transform: translateY(0);
  }
  40% {
    opacity: 1;
    transform: translateY(-2px);
  }
}

@media (max-width: 980px) {
  .socratic-page {
    min-height: auto;
  }

  h1 {
    font-size: 34px;
  }

  .chat-toolbar {
    grid-template-columns: 1fr;
  }

  .clear-btn {
    width: 100%;
  }

  .bubble {
    max-width: 94%;
  }
}
</style>

