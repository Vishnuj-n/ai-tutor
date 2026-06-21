<template>
  <aside class="panel chat" :class="{ closed: chat.chatCollapsed.value, 'rag-off': !ragEnabled }">
    <div class="chat-head">
      <h2>AI Chat</h2>
      <button class="ghost" @click="chat.toggleChat">
        {{ chat.chatCollapsed.value ? 'Expand' : 'Collapse' }}
      </button>
    </div>

    <div v-if="!chat.chatCollapsed.value && !ragSettingsLoaded" class="rag-disabled-overlay">
      <h3>Loading settings...</h3>
    </div>
    <div v-else-if="!chat.chatCollapsed.value && ragSettingsError" class="rag-disabled-overlay">
      <div class="lock-icon">⚠️</div>
      <h3>Settings Error</h3>
      <p>{{ ragSettingsError }}</p>
      <button class="primary" @click="$emit('retry-settings')">Retry</button>
    </div>
    <template v-else-if="!chat.chatCollapsed.value && ragEnabled">
      <p class="chat-context">
        Using topic <strong>{{ selectedTopicTitle || 'None' }}</strong>
        <span v-if="selectedNotebookTitle">from {{ selectedNotebookTitle }}</span>
      </p>

      <div class="scope-bar">
        <div class="scope-main">
          <span class="scope-label">Retrieval Scope</span>
          <select v-model="chat.chatScope.value" class="scope-select">
            <option value="entire_notebook">Entire Notebook</option>
            <option value="current_chapter">Current Chapter</option>
            <option value="current_page">Current Page</option>
          </select>
        </div>
        <p class="scope-helper">Broader scopes search more of your notebook.</p>
      </div>

      <div ref="messagesPaneRef" class="messages">
        <!-- eslint-disable-next-line vue/no-v-html -->
        <article
          v-for="(msg, idx) in chat.chatMessages.value"
          :key="msg.id || idx"
          class="msg"
          :class="msg.role"
        >
          <p class="role">{{ msg.role === 'user' ? 'You' : 'Tutor' }}</p>
          <p v-if="msg.role === 'user'">{{ msg.text }}</p>
          <div v-else class="markdown-body" v-html="chat.renderMarkdown(msg.text)"></div>
        </article>
      </div>

      <article v-if="chat.chatError.value" class="error">{{ chat.chatError.value }}</article>

      <label class="field">
        <span>Ask AI</span>
        <textarea
          v-model="chat.chatInput.value"
          :disabled="chat.chatLoading.value || !selectedTopicID"
          placeholder="Ask about what you’re reading right now..."
        ></textarea>
      </label>

      <button
        class="primary"
        :disabled="chat.chatLoading.value || !chat.chatInput.value.trim() || !selectedTopicID"
        @click="sendChat"
      >
        {{ chat.chatLoading.value ? 'Thinking...' : 'Send' }}
      </button>
    </template>

    <div
      v-if="!chat.chatCollapsed.value && ragSettingsLoaded && !ragEnabled && !ragSettingsError"
      class="rag-disabled-overlay"
    >
      <div class="lock-icon">🔒</div>
      <h3>Local AI Retrieval Offline</h3>
      <p>Local semantic search and Q&A is currently disabled to save memory and CPU.</p>
      <router-link to="/settings" class="enable-rag-btn">Enable in Settings</router-link>
    </div>
  </aside>
</template>

<script setup>
import { inject, ref, watch, onUnmounted } from 'vue'
import { logFrontendEvent } from '../services/appApi'

const props = defineProps({
  selectedTopicID: {
    type: String,
    default: '',
  },
  selectedTopicTitle: {
    type: String,
    default: '',
  },
  selectedNotebookID: {
    type: String,
    default: '',
  },
  selectedNotebookTitle: {
    type: String,
    default: '',
  },
  currentPage: {
    type: Number,
    required: true,
  },
  topicStartPage: {
    type: Number,
    required: true,
  },
  topicEndPage: {
    type: Number,
    required: true,
  },
  ragEnabled: {
    type: Boolean,
    required: true,
  },
  ragSettingsLoaded: {
    type: Boolean,
    required: true,
  },
  ragSettingsError: {
    type: String,
    default: null,
  },
})

defineEmits(['retry-settings'])

const chat = inject('chat')
const messagesPaneRef = ref(null)

// Watch settings errors and RAG toggle status
watch(() => props.ragSettingsError, (newVal) => {
  if (newVal) {
    logFrontendEvent('error', 'ReaderChat', 'rag_settings_error', { error: newVal })
  }
})

watch(() => props.ragEnabled, (newVal) => {
  logFrontendEvent('info', 'ReaderChat', 'rag_status_changed', { enabled: newVal })
}, { immediate: true })

// Synchronize messages pane element reference with parent's chat state
let activePaneEl = null

watch(messagesPaneRef, (el) => {
  chat.messagesPane.value = el
  if (el) {
    activePaneEl = el
  }
})

onUnmounted(() => {
  if (chat.messagesPane.value && chat.messagesPane.value === activePaneEl) {
    chat.messagesPane.value = null
  }
})

async function sendChat() {
  await chat.sendMessage({
    topicID: props.selectedTopicID,
    notebookID: props.selectedNotebookID,
    currentPage: props.currentPage,
    chapterStartPage: props.topicStartPage,
    chapterEndPage: props.topicEndPage,
  })
}
</script>

<style scoped>
.panel {
  background: var(--surface-container-lowest);
  border: 1px solid var(--surface-container-low);
  border-radius: 14px;
  padding: 12px;
}

.chat {
  display: grid;
  gap: 10px;
  align-content: start;
}

.chat.closed {
  padding: 10px 8px;
  gap: 0;
}

.chat.closed .chat-head {
  flex-direction: column;
}

.chat.closed h2 {
  writing-mode: vertical-rl;
  text-orientation: mixed;
  transform: rotate(180deg);
  font-size: 14px;
  word-break: break-word;
}

.chat.closed .chat-head button.ghost {
  width: 100%;
  padding: 8px 4px;
  font-size: 11px;
  white-space: normal;
  word-break: break-word;
}

.chat-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

h2 {
  margin: 0;
  font-size: 28px;
  font-family: 'Manrope', sans-serif;
}

h3 {
  margin: 0;
  font-size: 18px;
  font-family: 'Manrope', sans-serif;
}

.chat-context {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
}

.scope-bar {
  display: grid;
  gap: 6px;
  padding: 10px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--surface-container-low) 86%, transparent);
}

.scope-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.scope-helper {
  margin: 0;
  font-size: 11px;
  color: var(--muted-text);
  line-height: 1.3;
}

.scope-label {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--muted-text);
}

.scope-select {
  width: auto;
  min-width: 160px;
  padding: 8px 10px;
  font-size: 13px;
  border-radius: 10px;
}

.messages {
  max-height: 320px;
  overflow: auto;
  display: grid;
  gap: 8px;
  padding-right: 3px;
}

.msg {
  border-radius: 10px;
  padding: 9px 10px;
  display: grid;
  gap: 4px;
}

.msg.user {
  background: color-mix(in srgb, var(--primary) 14%, var(--surface-container-lowest));
}

.msg.assistant {
  background: var(--surface-container-low);
}

.msg .role {
  margin: 0;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--muted-text);
  font-weight: 700;
}

.msg p {
  margin: 0;
  font-size: 14px;
  line-height: 1.5;
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
  background: var(--surface-container-low);
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

.field {
  display: grid;
  gap: 5px;
}

.field span {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--muted-text);
}

select,
textarea {
  width: 100%;
  border: 1px solid var(--surface-container-low);
  background: var(--surface-container-lowest);
  color: var(--on-surface);
  border-radius: 10px;
  font: inherit;
  padding: 10px;
  outline: 0;
}

textarea {
  min-height: 110px;
  resize: vertical;
}

button {
  border: 0;
  border-radius: 10px;
  padding: 9px 12px;
  font-weight: 700;
  cursor: pointer;
}

button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.primary {
  color: var(--on-primary);
  background: linear-gradient(160deg, var(--primary), var(--primary-dim));
}

.secondary,
.ghost {
  color: var(--on-surface);
  background: var(--surface-container-low);
}

.error {
  color: #b42318;
  background: color-mix(in srgb, #b42318 12%, var(--surface-container-lowest));
  border: 1px solid color-mix(in srgb, #b42318 30%, var(--surface-container-low));
  border-radius: 10px;
  padding: 10px;
  font-size: 13px;
}

/* RAG Disabled styles in Reader */
.panel.chat.rag-off {
  background: color-mix(in srgb, var(--surface-container-low) 90%, #000000);
  opacity: 0.85;
}

.rag-disabled-overlay {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  text-align: center;
  height: calc(100% - 60px);
}

.rag-disabled-overlay .lock-icon {
  font-size: 32px;
  margin-bottom: 16px;
  opacity: 0.7;
}

.rag-disabled-overlay h3 {
  font-size: 16px;
  font-weight: 700;
  margin-bottom: 12px;
  color: var(--on-surface);
}

.rag-disabled-overlay p {
  font-size: 13px;
  line-height: 1.5;
  color: var(--on-surface-variant);
  margin-bottom: 24px;
}

.enable-rag-btn {
  display: inline-block;
  padding: 8px 16px;
  background: var(--primary);
  color: var(--on-primary);
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  transition: background 0.2s ease;
}

.enable-rag-btn:hover {
  background: color-mix(in srgb, var(--primary) 85%, #000000);
}
</style>
