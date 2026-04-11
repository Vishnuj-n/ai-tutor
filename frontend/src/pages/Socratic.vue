<template>
  <section class="page">
    <div class="header">
      <p class="eyebrow">Socratic Tutor</p>
      <h1>Guided Thinking</h1>
      <p class="subtitle">Ask questions and develop deeper understanding</p>
    </div>

    <!-- Topic Selector -->
    <article class="panel topic-selector">
      <label for="topic-select" class="label">Select Topic</label>
      <select
        id="topic-select"
        v-model="selectedTopic"
        class="select-input"
        @change="onTopicChange"
      >
        <option value="">-- Choose a topic --</option>
        <option v-for="topic in availableTopics" :key="topic" :value="topic">
          {{ formatTopicName(topic) }}
        </option>
      </select>
      <p v-if="!selectedTopic" class="helper-text">Select a topic to begin asking questions</p>
    </article>

    <!-- Chat Interface -->
    <article v-if="selectedTopic" class="panel chat-container">
      <!-- Messages Thread -->
      <div class="messages-wrapper">
        <div v-if="messages.length === 0" class="empty-state">
          <p class="empty-icon">💭</p>
          <h3>Start a Conversation</h3>
          <p>
            Ask a question about <strong>{{ formatTopicName(selectedTopic) }}</strong> and explore
            the topic through guided discussion.
          </p>
        </div>

        <div v-else class="messages-list">
          <div v-for="(msg, idx) in messages" :key="idx" :class="['message', msg.role]">
            <div class="message-content">
              <p class="message-text">{{ msg.text }}</p>

              <!-- AI Response Metadata -->
              <template v-if="msg.role === 'ai'">
                <div v-if="msg.citations && msg.citations.length > 0" class="citations">
                  <p class="citation-label">📚 Based on:</p>
                  <ul class="citation-list">
                    <li v-for="(citation, cidx) in msg.citations" :key="cidx">
                      {{ citation }}
                    </li>
                  </ul>
                </div>
                <div v-if="msg.error" class="error-badge">⚠️ {{ msg.error }}</div>
              </template>
            </div>
          </div>

          <!-- Loading State -->
          <div v-if="isLoading" class="message ai loading-message">
            <div class="message-content">
              <div class="typing-indicator"><span></span><span></span><span></span></div>
              <p class="typing-text">Thinking...</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Input Area -->
      <div class="input-area">
        <textarea
          v-model="inputQuestion"
          placeholder="Ask a question to deepen your understanding..."
          class="input-field"
          :disabled="isLoading"
          @keydown.enter.ctrl="submitQuestion"
        ></textarea>
        <div class="input-footer">
          <p class="helper-small">💡 Tip: Press Ctrl+Enter to send</p>
          <button
            type="button"
            class="send-btn"
            :disabled="isLoading || !inputQuestion.trim()"
            @click="submitQuestion"
          >
            {{ isLoading ? 'Sending...' : 'Ask' }}
          </button>
        </div>
      </div>
    </article>

    <!-- Error State -->
    <article v-if="globalError" class="panel error-panel">
      <p class="error-icon">⚠️</p>
      <p class="error-message">{{ globalError }}</p>
    </article>
  </section>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import {
  askAI as askAIRequest,
  getAvailableTopics as fetchAvailableTopics,
} from '../services/appApi'

const selectedTopic = ref('')
const inputQuestion = ref('')
const messages = ref([])
const isLoading = ref(false)
const globalError = ref('')
const availableTopics = ref([])

onMounted(async () => {
  await loadTopics()
})

async function loadTopics() {
  try {
    globalError.value = ''
    const result = await fetchAvailableTopics()

    if (result.error) {
      globalError.value = result.error
      availableTopics.value = []
      return
    }

    availableTopics.value = result.topics || []
  } catch (err) {
    globalError.value = `Failed to load topics: ${err.message}`
    availableTopics.value = []
  }
}

function onTopicChange() {
  messages.value = []
  inputQuestion.value = ''
  globalError.value = ''
}

async function submitQuestion() {
  const question = inputQuestion.value.trim()

  if (!question || !selectedTopic.value) return

  try {
    // Add user message to thread
    messages.value.push({
      role: 'user',
      text: question,
      timestamp: new Date(),
    })

    inputQuestion.value = ''
    isLoading.value = true
    globalError.value = ''

    // Call backend
    const result = await askAIRequest(selectedTopic.value, question)

    // Add AI response to thread
    if (result.error) {
      messages.value.push({
        role: 'ai',
        text: 'I encountered an issue while processing your question.',
        error: result.error,
        timestamp: new Date(),
      })
    } else {
      messages.value.push({
        role: 'ai',
        text: result.answer || 'No response generated.',
        citations: result.cited_sections || [],
        chunks_retrieved: result.chunks_retrieved,
        sections_used: result.sections_used,
        timestamp: new Date(),
      })
    }
  } catch (err) {
    globalError.value = `Error: ${err.message}`
    // Remove the last user message on error
    if (messages.value.length > 0 && messages.value[messages.value.length - 1].role === 'user') {
      messages.value.pop()
    }
  } finally {
    isLoading.value = false
  }
}

function formatTopicName(topicId) {
  return topicId
    .split('-')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
}
</script>

<style scoped>
* {
  box-sizing: border-box;
}

.page {
  display: grid;
  gap: 24px;
  height: 100%;
}

.header {
  padding: 0 4px;
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
  margin: 8px 0 0;
  font-size: 46px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.subtitle {
  margin: 12px 0 0;
  font-size: 16px;
  color: var(--muted-text);
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 24px;
}

.topic-selector {
  display: grid;
  gap: 12px;
}

.label {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-text);
  margin: 0;
}

.select-input {
  padding: 12px;
  border: 1px solid var(--surface-container-low);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  font-family: inherit;
  font-size: 14px;
  cursor: pointer;
  outline: none;
  transition: border-color 0.2s;
}

.select-input:focus {
  border-color: var(--primary);
}

.helper-text {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
}

.helper-small {
  margin: 0;
  font-size: 12px;
  color: var(--muted-text);
}

.chat-container {
  display: flex;
  flex-direction: column;
  height: 600px;
  gap: 16px;
}

.messages-wrapper {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
  background: var(--surface-container-low);
  border-radius: 12px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  text-align: center;
  gap: 12px;
  color: var(--muted-text);
}

.empty-icon {
  font-size: 48px;
  margin: 0;
}

.empty-state h3 {
  margin: 0;
  font-size: 18px;
  color: var(--on-surface);
  font-family: 'Manrope', sans-serif;
}

.empty-state p {
  margin: 0;
  font-size: 14px;
  max-width: 300px;
}

.messages-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.message {
  display: flex;
  gap: 12px;
  animation: slideIn 0.3s ease-out;
}

.message.user {
  justify-content: flex-end;
}

.message-content {
  max-width: 70%;
  padding: 12px;
  border-radius: 12px;
  word-break: break-word;
}

.message.user .message-content {
  background: linear-gradient(135deg, var(--primary-dim), var(--primary));
  color: var(--on-primary);
}

.message.ai .message-content {
  background: var(--surface-container-highest);
  color: var(--on-surface);
  border: 1px solid var(--surface-container);
}

.message-text {
  margin: 0;
  font-size: 14px;
  line-height: 1.5;
}

.citations {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--surface-container);
  font-size: 12px;
}

.citation-label {
  margin: 0 0 8px;
  font-weight: 600;
  color: var(--muted-text);
}

.citation-list {
  margin: 0;
  padding-left: 20px;
  list-style: disc;
}

.citation-list li {
  margin: 4px 0;
  font-size: 12px;
  color: var(--on-surface);
  line-height: 1.4;
}

.error-badge {
  margin-top: 8px;
  padding: 8px;
  background: #fff3cd;
  border-radius: 6px;
  font-size: 12px;
  color: #856404;
}

.loading-message .message-content {
  background: var(--surface-container-highest);
  border: 1px solid var(--surface-container);
}

.typing-indicator {
  display: flex;
  gap: 4px;
  height: 12px;
}

.typing-indicator span {
  width: 8px;
  height: 8px;
  background: var(--muted-text);
  border-radius: 50%;
  animation: typing 1.4s infinite;
}

.typing-indicator span:nth-child(2) {
  animation-delay: 0.2s;
}

.typing-indicator span:nth-child(3) {
  animation-delay: 0.4s;
}

.typing-text {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--muted-text);
}

.input-area {
  display: grid;
  gap: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--surface-container-low);
}

.input-field {
  width: 100%;
  min-height: 80px;
  max-height: 120px;
  padding: 12px;
  border: 1px solid var(--surface-container-low);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  font-family: inherit;
  font-size: 14px;
  resize: vertical;
  outline: none;
  transition: border-color 0.2s;
}

.input-field:focus {
  border-color: var(--primary);
}

.input-field:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.input-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.send-btn {
  padding: 10px 20px;
  border: 0;
  border-radius: 12px;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  color: var(--on-primary);
  font-weight: 600;
  font-size: 14px;
  cursor: pointer;
  transition: opacity 0.2s;
}

.send-btn:hover:not(:disabled) {
  opacity: 0.9;
}

.send-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.error-panel {
  background: #fff3cd;
  border: 1px solid #ffeaa7;
  color: #856404;
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.error-icon {
  font-size: 20px;
  margin: 0;
}

.error-message {
  margin: 0;
  font-size: 14px;
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes typing {
  0%,
  60%,
  100% {
    opacity: 0.5;
  }
  30% {
    opacity: 1;
  }
}

/* Scrollbar styling */
.messages-wrapper::-webkit-scrollbar {
  width: 6px;
}

.messages-wrapper::-webkit-scrollbar-track {
  background: transparent;
}

.messages-wrapper::-webkit-scrollbar-thumb {
  background: var(--surface-container);
  border-radius: 3px;
}

.messages-wrapper::-webkit-scrollbar-thumb:hover {
  background: var(--surface-container-high);
}
</style>
