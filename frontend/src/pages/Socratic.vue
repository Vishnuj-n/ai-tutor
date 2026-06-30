<template>
  <section class="socratic-page">
    <header class="page-header">
      <p class="eyebrow">Tutor</p>
      <h1>Guided Thinking</h1>
    </header>

    <article class="chat-shell">
      <!-- Socratic Rescue Active Banner -->
      <div v-if="isRescueMode" class="rescue-alert-banner">
        <div class="rescue-alert-content">
          <span class="rescue-alert-icon">🛡️</span>
          <div class="rescue-alert-text">
            <strong>Concept Rescue Active</strong>
            <p>
              Chat with the Socratic tutor about this topic. When you feel ready, click the button
              to retry your quiz.
            </p>
          </div>
        </div>
        <button
          type="button"
          class="rescue-complete-btn"
          :disabled="completingRescue"
          @click="finishRescue"
        >
          {{ completingRescue ? 'Completing...' : 'Complete Session & Retry Quiz' }}
        </button>
      </div>

      <div class="chat-header-row">
        <div class="selector-pills">
          <div class="selector-pill">
            <span class="pill-icon">📖</span>
            <select
              id="notebook-select"
              v-model="selectedNotebookID"
              :disabled="isRescueMode"
              @change="handleNotebookChange"
            >
              <option value="" disabled>Choose notebook</option>
              <option v-for="notebook in notebooks" :key="notebook.id" :value="notebook.id">
                {{ formatNotebookLabel(notebook) }}
              </option>
            </select>
          </div>

          <div class="selector-pill">
            <span class="pill-icon">🎯</span>
            <select
              id="topic-select"
              v-model="selectedTopicID"
              :disabled="isRescueMode"
              @change="handleTopicChange"
            >
              <option v-if="ragEntireNotebookEnabled" value="">
                Entire book (No topic filter)
              </option>
              <option v-else value="" disabled>Choose topic</option>
              <option v-for="topic in availableTopics" :key="topic.id" :value="topic.id">
                {{ topic.title }}
              </option>
            </select>
          </div>
        </div>

        <button
          type="button"
          class="clear-btn-slim"
          :disabled="isRescueMode"
          title="Clear chat history"
          @click="clearConversation"
        >
          <span class="clear-icon">🧹</span> Clear Chat
        </button>
      </div>

      <div ref="threadRef" class="chat-thread">
        <div v-if="messages.length === 0" class="empty-state">
          <div class="welcome-card">
            <div class="welcome-icon">🧠</div>
            <h3 v-if="isRescueMode">Concept Rescue Session</h3>
            <h3 v-else>Socratic Tutor</h3>

            <p v-if="isRescueMode" class="welcome-desc">
              Let's clarify your understanding before retaking the quiz. Ask questions or start a
              guided session with the tutor.
            </p>
            <p v-else class="welcome-desc">
              Select a notebook and topic above, then start a guided session or type a specific
              question below to begin.
            </p>

            <p v-if="selectionHint" class="selection-status-hint">
              {{ selectionHint }}
            </p>

            <button
              type="button"
              class="start-session-btn"
              :disabled="!canStart || isLoading"
              @click="initiateSocraticSession"
            >
              <span v-if="isLoading" class="spinner"></span>
              <span v-else class="start-icon">▶</span>
              {{
                isLoading
                  ? 'Initializing...'
                  : isRescueMode
                    ? 'Start Concept Rescue'
                    : 'Start Socratic Session'
              }}
            </button>
          </div>
        </div>

        <div v-for="(message, idx) in messages" :key="idx" :class="['bubble-row', message.role]">
          <article class="bubble">
            <p v-if="message.role === 'user'" class="message-text">{{ message.text }}</p>
            <div v-else class="markdown-body" v-html="renderMarkdown(message.text)"></div>

            <div v-if="message.role === 'assistant' && message.error" class="message-error">
              <p class="error-text">{{ message.error }}</p>
              <button
                v-if="message.isNetworkError && message.promptText"
                class="retry-msg-btn"
                @click="retrySocraticMessage(idx)"
              >
                {{ retryingMessageId === idx ? 'Retrying...' : 'Retry' }}
              </button>
              <button
                v-if="message.isNetworkError && message.promptText && message.isRescueContext"
                class="copy-prompt-btn"
                @click="copyPromptToClipboard(message.promptText)"
              >
                {{ copiedMessageId === idx ? 'Copied!' : 'Copy Prompt' }}
              </button>
            </div>

            <div
              v-if="
                message.role === 'assistant' && message.citations && message.citations.length > 0
              "
              class="citations"
            >
              <button
                type="button"
                class="citation-info-btn"
                @click.stop="toggleCitationPopover(idx)"
                @mouseenter="showCitationPopover = idx"
                @mouseleave="scheduleHideCitationPopover"
              >
                ℹ
              </button>
              <div
                v-if="showCitationPopover === idx"
                class="citation-popover"
                @mouseenter="cancelHideCitationPopover"
                @mouseleave="hideCitationPopover"
              >
                <p class="citation-popover-title">Source Chunks</p>
                <div
                  v-for="(chunk, cIdx) in (message.chunkTexts || [])"
                  :key="cIdx"
                  class="citation-chunk"
                >
                  <span class="citation-chunk-label">{{ message.citations[cIdx] || '' }}</span>
                  <p class="citation-chunk-text">{{ chunk }}</p>
                </div>
                <div v-if="!message.chunkTexts || message.chunkTexts.length === 0">
                  <p
                    v-for="(citation, cIdx) in message.citations"
                    :key="cIdx"
                    class="citation-chunk-label"
                  >
                    {{ citation }}
                  </p>
                </div>
              </div>
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
        <div class="composer-box">
          <textarea
            v-model="inputQuestion"
            class="composer-input"
            aria-label="Question"
            placeholder="Ask a grounded question about your material, and the tutor will guide you..."
            :disabled="isLoading"
            @keydown="handleComposerKeydown"
          ></textarea>

          <button
            type="submit"
            class="composer-send-btn"
            :disabled="!canSend || isLoading"
            title="Send message"
          >
            <svg
              v-if="!isLoading"
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="currentColor"
              class="send-svg"
            >
              <path
                d="M3.478 2.404a.75.75 0 0 0-.926.941l2.432 7.905H13.5a.75.75 0 0 1 0 1.5H4.984l-2.432 7.905a.75.75 0 0 0 .926.94 60.519 60.519 0 0 0 18.445-8.986.75.75 0 0 0 0-1.218A60.517 60.517 0 0 0 3.478 2.404Z"
              />
            </svg>
            <span v-else class="thinking-dot-loader">
              <span></span><span></span><span></span>
            </span>
          </button>
        </div>
        <div class="composer-hint-row">
          <span>Enter to send, Shift+Enter for new line</span>
        </div>
      </form>
    </article>

    <p v-if="globalError" class="global-error">{{ globalError }}</p>
  </section>
</template>

<script setup>
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  askSocratic,
  getAvailableTopics as fetchAvailableTopics,
  getNotebooks as fetchNotebooks,
  getUserSettings,
  completeSocraticRescue,
  activateTask,
  GetTaskContext,
} from '../services/appApi'
import { renderMarkdown } from '../services/markdown'

const route = useRoute()
const router = useRouter()
const ragEntireNotebookEnabled = ref(true)

const availableTopics = ref([])
const notebooks = ref([])
const selectedTopicID = ref('')
const selectedNotebookID = ref('')
const inputQuestion = ref('')
const messages = ref([])
const isLoading = ref(false)
const globalError = ref('')
const threadRef = ref(null)

const taskId = ref(route.query.taskId || route.query.task_id || '')
const isRescueMode = computed(() => !!taskId.value)
const completingRescue = ref(false)
const copiedMessageId = ref(null)
const retryingMessageId = ref(null)
const showCitationPopover = ref(null)
let hideCitationPopoverTimer = null

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
  const hasQuestion = inputQuestion.value.trim().length > 0
  if (isLoading.value || !hasQuestion) {
    return false
  }
  if (!selectedNotebookID.value) {
    return false
  }
  if (isRescueMode.value) {
    return true
  }
  if (ragEntireNotebookEnabled.value) {
    return true
  }
  return effectiveTopicID.value !== ''
})

const canStart = computed(() => {
  if (!selectedNotebookID.value) {
    return false
  }
  if (isRescueMode.value) {
    return true
  }
  if (ragEntireNotebookEnabled.value) {
    return true
  }
  return effectiveTopicID.value !== ''
})

const selectionHint = computed(() => {
  if (!selectedNotebookID.value) {
    return 'Select a notebook to start the Tutor session.'
  }

  if (
    selectedNotebook.value &&
    !selectedNotebook.value.topic_id &&
    !selectedTopicID.value &&
    !ragEntireNotebookEnabled.value
  ) {
    return 'Selected notebook has no linked topic yet. Choose a topic to run RAG.'
  }

  if (!effectiveTopicID.value) {
    if (ragEntireNotebookEnabled.value) {
      return `Current retrieval scope: Entire Book - ${selectedNotebook.value?.title || ''}`
    }
    return 'Choose a topic to run RAG.'
  }

  const topic = availableTopics.value.find((item) => item.id === effectiveTopicID.value)
  return topic ? `Current retrieval scope: ${topic.title}` : ''
})

onMounted(async () => {
  if (taskId.value) {
    try {
      const activate = await activateTask(taskId.value)
      if (activate?.error && activate.error !== 'ErrTaskNotPending') {
        globalError.value = activate.error
        return
      }
      const context = await GetTaskContext(taskId.value)
      if (context?.error) {
        globalError.value = context.error
        return
      }
      if (context?.notebook?.id) {
        selectedNotebookID.value = context.notebook.id
      }
      if (context?.topic?.id) {
        selectedTopicID.value = context.topic.id
      }
    } catch (err) {
      globalError.value = `Failed to initialize Socratic task: ${err.message || err}`
      return
    }
  }

  try {
    const res = await getUserSettings()
    if (res && typeof res.rag_entire_notebook !== 'undefined') {
      ragEntireNotebookEnabled.value = res.rag_entire_notebook
    }
  } catch (err) {
    console.error('Failed to load user settings in Tutor:', err)
    globalError.value = `Failed to load settings: ${err.message}`
  }

  await Promise.all([loadTopics(), loadNotebooks()])

  const nbParam = route.query.notebook_id || route.query.notebookId || ''
  const topicParam = route.query.topic_id || route.query.topicId || ''

  if (topicParam) {
    selectedTopicID.value = topicParam
  }
  if (nbParam) {
    selectedNotebookID.value = nbParam
  }
})

async function loadTopics() {
  try {
    const result = await fetchAvailableTopics()
    const list = Array.isArray(result) ? result : Array.isArray(result?.topics) ? result.topics : []
    availableTopics.value = list

    if (
      !selectedTopicID.value &&
      availableTopics.value.length > 0 &&
      !ragEntireNotebookEnabled.value
    ) {
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
  showCitationPopover.value = null
}

function toggleCitationPopover(idx) {
  showCitationPopover.value = showCitationPopover.value === idx ? null : idx
}

function scheduleHideCitationPopover() {
  hideCitationPopoverTimer = setTimeout(() => {
    showCitationPopover.value = null
  }, 300)
}

function cancelHideCitationPopover() {
  if (hideCitationPopoverTimer) {
    clearTimeout(hideCitationPopoverTimer)
    hideCitationPopoverTimer = null
  }
}

function hideCitationPopover() {
  cancelHideCitationPopover()
  showCitationPopover.value = null
}

async function submitQuestion() {
  if (!canSend.value) {
    return
  }

  const question = inputQuestion.value.trim()
  const topicID = effectiveTopicID.value

  const conversationHistory = messages.value
    .filter((m) => !m.error)
    .map((m) => ({ role: m.role, content: m.text }))

  messages.value.push({
    role: 'user',
    text: question,
  })

  inputQuestion.value = ''
  isLoading.value = true
  await scrollToBottom()

  try {
    const result = await askSocratic(selectedNotebookID.value, topicID, question, conversationHistory)

    if (result.error) {
      const isNetworkError =
        result.error.includes('network') ||
        result.error.includes('fetch') ||
        result.error.includes('Failed to fetch') ||
        result.error.includes('NetworkError')
      messages.value.push({
        role: 'assistant',
        text: 'Unable to answer this query right now.',
        error: result.error,
        isNetworkError,
        promptText: question,
        isRescueContext: isRescueMode.value,
      })
    } else {
      messages.value.push({
        role: 'assistant',
        text: result.answer || 'No response generated.',
        citations: result.cited_sections || [],
        chunkTexts: result.chunk_texts || [],
      })
    }
  } catch (err) {
    globalError.value = `Chat request failed: ${err.message}`
  } finally {
    isLoading.value = false
    await scrollToBottom()
  }
}

async function initiateSocraticSession() {
  if (!canStart.value || isLoading.value) {
    return
  }

  isLoading.value = true
  globalError.value = ''

  const startPrompt = isRescueMode.value
    ? "Let's start the Concept Rescue session. Please ask me a guiding question about the topic to help me clear up my confusion."
    : "Let's start our Socratic discussion. Please introduce yourself and ask me a starting question to check my understanding of this material."

  try {
    const topicID = effectiveTopicID.value
    const result = await askSocratic(selectedNotebookID.value, topicID, startPrompt)

    if (result.error) {
      const isNetworkError =
        result.error.includes('network') ||
        result.error.includes('fetch') ||
        result.error.includes('Failed to fetch') ||
        result.error.includes('NetworkError')
      messages.value.push({
        role: 'assistant',
        text: 'Unable to answer this query right now.',
        error: result.error,
        isNetworkError,
        promptText: startPrompt,
        isRescueContext: isRescueMode.value,
      })
    } else {
      messages.value.push({
        role: 'assistant',
        text: result.answer || 'No response generated.',
        citations: result.cited_sections || [],
        chunkTexts: result.chunk_texts || [],
      })
    }
  } catch (err) {
    globalError.value = `Failed to start session: ${err.message}`
  } finally {
    isLoading.value = false
    await scrollToBottom()
  }
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

async function copyPromptToClipboard(text) {
  try {
    await navigator.clipboard.writeText(text)
    const msgIndex = messages.value.findIndex((m) => m.promptText === text)
    if (msgIndex !== -1) {
      copiedMessageId.value = msgIndex
      setTimeout(() => {
        copiedMessageId.value = null
      }, 2000)
    }
  } catch (err) {
    console.error('Failed to copy prompt:', err)
    const textarea = document.createElement('textarea')
    textarea.value = text
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
    const msgIndex = messages.value.findIndex((m) => m.promptText === text)
    if (msgIndex !== -1) {
      copiedMessageId.value = msgIndex
      setTimeout(() => {
        copiedMessageId.value = null
      }, 2000)
    }
  }
}

async function retrySocraticMessage(messageIdx) {
  const failedMsg = messages.value[messageIdx]
  if (!failedMsg || retryingMessageId.value !== null) return

  retryingMessageId.value = messageIdx
  const prompt = failedMsg.promptText

  messages.value.splice(messageIdx, 1)

  isLoading.value = true
  await scrollToBottom()

  const conversationHistory = messages.value
    .filter((m, idx) => !m.error && idx !== messageIdx - 1)
    .map((m) => ({ role: m.role, content: m.text }))

  try {
    const topicID = effectiveTopicID.value
    const result = await askSocratic(selectedNotebookID.value, topicID, prompt, conversationHistory)

    if (result.error) {
      const isNetworkError =
        result.error.includes('network') ||
        result.error.includes('fetch') ||
        result.error.includes('Failed to fetch') ||
        result.error.includes('NetworkError')
      messages.value.push({
        role: 'assistant',
        text: 'Unable to answer this query right now.',
        error: result.error,
        isNetworkError,
        promptText: prompt,
        isRescueContext: failedMsg.isRescueContext,
      })
    } else {
      messages.value.push({
        role: 'assistant',
        text: result.answer || 'No response generated.',
        citations: result.cited_sections || [],
        chunkTexts: result.chunk_texts || [],
      })
    }
  } catch (err) {
    globalError.value = `Retry failed: ${err.message}`
  } finally {
    retryingMessageId.value = null
    isLoading.value = false
    await scrollToBottom()
  }
}

async function scrollToBottom() {
  await nextTick()
  if (!threadRef.value) {
    return
  }
  threadRef.value.scrollTop = threadRef.value.scrollHeight
}

async function finishRescue() {
  if (completingRescue.value) return
  completingRescue.value = true
  globalError.value = ''
  try {
    const res = await completeSocraticRescue(taskId.value)
    if (res && res.error) {
      globalError.value = res.error
      completingRescue.value = false
      return
    }
    const nextRoute = res?.quiz_task_id ? `/quiz?taskId=${res.quiz_task_id}` : '/dashboard'
    router.push(nextRoute)
  } catch (err) {
    globalError.value = 'Failed to complete session: ' + (err.message || err)
    completingRescue.value = false
  }
}
</script>

<style scoped>
.socratic-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  height: calc(100vh - 32px);
  overflow: hidden;
}

.page-header {
  padding: 0;
  flex-shrink: 0;
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
  margin: 4px 0 0;
  font-size: 36px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.03em;
  color: var(--on-surface);
}

.chat-shell {
  display: flex;
  flex-direction: column;
  gap: 12px;
  flex: 1;
  min-height: 0;
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 16px 20px 20px;
  border: 1px solid var(--outline-variant);
}

.chat-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding-bottom: 4px;
  flex-shrink: 0;
}

.selector-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.selector-pill {
  display: inline-flex;
  align-items: center;
  background: var(--surface-container-low);
  border: 1px solid var(--outline-variant);
  border-radius: 20px;
  padding: 4px 12px 4px 10px;
  font-family: 'Inter', sans-serif;
  font-size: 13px;
  transition: all 0.2s ease;
}

.selector-pill:focus-within {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px rgba(0, 91, 193, 0.1);
}

.pill-icon {
  font-size: 14px;
  margin-right: 6px;
}

.selector-pill select {
  border: none;
  background: transparent;
  color: var(--on-surface);
  font-family: 'Inter', sans-serif;
  font-size: 13px;
  font-weight: 500;
  outline: none;
  padding: 2px 4px;
  cursor: pointer;
  max-width: 260px;
  text-overflow: ellipsis;
}

.clear-btn-slim {
  border: 1px solid var(--outline-variant);
  border-radius: 20px;
  padding: 6px 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  font-family: 'Inter', sans-serif;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  transition: all 0.2s ease;
  flex-shrink: 0;
}

.clear-btn-slim:hover:not(:disabled) {
  background: var(--surface-container-highest);
  border-color: var(--outline);
}

.clear-btn-slim:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.chat-thread {
  overflow-y: auto;
  padding: 20px;
  background: var(--surface-container-low);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
}

.empty-state {
  margin: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  max-width: 480px;
  padding: 16px;
}

.welcome-card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 16px;
  padding: 32px 24px;
  text-align: center;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
}

.welcome-icon {
  font-size: 40px;
  margin-bottom: 12px;
  animation: pulse-slow 3s infinite ease-in-out;
}

.welcome-card h3 {
  margin: 0 0 8px;
  font-size: 22px;
  font-family: 'Manrope', sans-serif;
  font-weight: 700;
  color: var(--on-surface);
}

.welcome-desc {
  margin: 0 0 16px;
  font-size: 14px;
  line-height: 1.5;
  color: var(--muted-text);
}

.selection-status-hint {
  margin: 0 0 20px;
  padding: 8px 12px;
  border-radius: 8px;
  background: var(--surface-container-low);
  color: var(--muted-text);
  font-size: 12px;
  font-family: 'Inter', sans-serif;
  border: 1px dashed var(--outline-variant);
  width: 100%;
  box-sizing: border-box;
}

.start-session-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: none;
  border-radius: 20px;
  padding: 10px 24px;
  background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dim) 100%);
  color: var(--on-primary);
  font-family: 'Inter', sans-serif;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 4px 12px rgba(0, 91, 193, 0.15);
  width: 100%;
}

.start-session-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(0, 91, 193, 0.25);
}

.start-session-btn:active:not(:disabled) {
  transform: scale(0.98);
}

.start-session-btn:disabled {
  background: var(--surface-container-highest);
  color: var(--muted-text);
  border: 1px solid var(--outline-variant);
  box-shadow: none;
  cursor: not-allowed;
}

.start-icon {
  font-size: 12px;
}

.spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
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
  padding: 10px 14px;
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
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.citations {
  margin-top: 9px;
  border-top: 1px solid rgba(45, 51, 56, 0.12);
  padding-top: 8px;
  position: relative;
}

.citation-info-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 1px solid var(--outline-variant);
  background: var(--surface-container-low);
  color: var(--muted-text);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.15s ease;
  padding: 0;
  line-height: 1;
}

.citation-info-btn:hover {
  background: var(--primary);
  color: var(--on-primary);
  border-color: var(--primary);
}

.citation-popover {
  position: absolute;
  bottom: calc(100% + 8px);
  left: 0;
  z-index: 10;
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 10px;
  padding: 10px 12px;
  width: 320px;
  max-height: 260px;
  overflow-y: auto;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
}

.citation-popover-title {
  margin: 0 0 8px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--muted-text);
}

.citation-chunk {
  margin-bottom: 8px;
}

.citation-chunk:last-child {
  margin-bottom: 0;
}

.citation-chunk-label {
  display: inline-block;
  font-size: 10px;
  font-weight: 700;
  color: var(--primary);
  background: rgba(0, 91, 193, 0.08);
  padding: 1px 6px;
  border-radius: 4px;
  margin-bottom: 2px;
}

.citation-chunk-text {
  margin: 2px 0 0;
  font-size: 11px;
  line-height: 1.45;
  color: var(--on-surface);
  opacity: 0.85;
  display: -webkit-box;
  -webkit-line-clamp: 4;
  line-clamp: 4;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.composer {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex-shrink: 0;
}

.composer-box {
  display: flex;
  align-items: flex-end;
  background: var(--surface-container-low);
  border: 1px solid var(--outline-variant);
  border-radius: 20px;
  padding: 8px 12px;
  transition: all 0.2s ease;
  position: relative;
}

.composer-box:focus-within {
  border-color: var(--primary);
  background: var(--surface-container-lowest);
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
}

.composer-input {
  flex: 1;
  border: none;
  background: transparent;
  padding: 4px 40px 4px 4px;
  color: var(--on-surface);
  font-family: 'Inter', sans-serif;
  font-size: 14px;
  line-height: 1.5;
  outline: none;
  resize: none;
  min-height: 24px;
  max-height: 120px;
}

.composer-send-btn {
  position: absolute;
  right: 8px;
  bottom: 8px;
  border: none;
  border-radius: 50%;
  width: 28px;
  height: 28px;
  background: var(--primary);
  color: var(--on-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s ease;
}

.composer-send-btn:hover:not(:disabled) {
  transform: scale(1.05);
  background: var(--primary-dim);
}

.composer-send-btn:active:not(:disabled) {
  transform: scale(0.95);
}

.composer-send-btn:disabled {
  background: var(--surface-container-highest);
  color: var(--muted-text);
  cursor: not-allowed;
}

.send-svg {
  width: 14px;
  height: 14px;
}

.composer-hint-row {
  display: flex;
  justify-content: flex-end;
  padding: 0 4px;
}

.composer-hint-row span {
  font-size: 10px;
  color: var(--muted-text);
}

.global-error {
  margin: 0;
  padding: 10px 12px;
  border-radius: 12px;
  background: #fff0f0;
  color: #b43131;
  font-size: 13px;
}

.thinking-dot-loader {
  display: flex;
  gap: 2px;
  align-items: center;
}

.thinking-dot-loader span {
  width: 4px;
  height: 4px;
  background-color: var(--on-primary);
  border-radius: 50%;
  display: inline-block;
  animation: pulse 1.1s infinite ease-in-out;
}

.thinking-dot-loader span:nth-child(2) {
  animation-delay: 0.12s;
}
.thinking-dot-loader span:nth-child(3) {
  animation-delay: 0.24s;
}

@keyframes pulse-slow {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.9;
  }
  50% {
    transform: scale(1.04);
    opacity: 1;
  }
}

@keyframes pulse {
  0%,
  80%,
  100% {
    opacity: 0.32;
  }
  40% {
    opacity: 1;
  }
}

@media (max-width: 980px) {
  .socratic-page {
    height: auto;
    overflow: visible;
  }
  h1 {
    font-size: 30px;
  }
  .chat-header-row {
    flex-direction: column;
    align-items: stretch;
  }
  .clear-btn-slim {
    width: 100%;
    justify-content: center;
  }
  .bubble {
    max-width: 94%;
  }
}

.rescue-alert-banner {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  background: linear-gradient(135deg, rgba(211, 84, 0, 0.1) 0%, rgba(230, 126, 34, 0.15) 100%);
  border: 1px solid rgba(211, 84, 0, 0.25);
  border-radius: 12px;
  padding: 12px 16px;
  margin-bottom: 4px;
  flex-shrink: 0;
}

.rescue-alert-content {
  display: flex;
  gap: 10px;
  align-items: center;
  text-align: left;
}

.rescue-alert-icon {
  font-size: 20px;
  line-height: 1;
}

.rescue-alert-text strong {
  display: block;
  font-size: 14px;
  color: #d35400;
  margin-bottom: 1px;
}

.rescue-alert-text p {
  margin: 0;
  font-size: 12px;
  color: var(--on-surface);
  opacity: 0.9;
}

.rescue-complete-btn {
  background: linear-gradient(135deg, #d35400, #e67e22);
  color: white;
  border: none;
  border-radius: 8px;
  padding: 8px 14px;
  font-size: 12.5px;
  font-weight: 700;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.2s ease;
  box-shadow: 0 4px 10px rgba(211, 84, 0, 0.15);
}

.rescue-complete-btn:hover:not(:disabled) {
  opacity: 0.95;
  transform: translateY(-1px);
}

.rescue-complete-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.error-text {
  margin: 0;
}

.copy-prompt-btn {
  align-self: flex-start;
  background: transparent;
  border: 1px solid var(--primary);
  border-radius: 6px;
  padding: 4px 10px;
  font-size: 11px;
  font-weight: 600;
  color: var(--primary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.copy-prompt-btn:hover {
  background: var(--primary);
  color: var(--on-primary);
}

.retry-msg-btn {
  align-self: flex-start;
  background: transparent;
  border: 1px solid var(--outline-variant);
  border-radius: 6px;
  padding: 4px 10px;
  font-size: 11px;
  font-weight: 600;
  color: var(--on-surface);
  cursor: pointer;
  transition: all 0.15s ease;
}

.retry-msg-btn:hover {
  background: var(--surface-container-low);
  border-color: var(--primary);
  color: var(--primary);
}
</style>
