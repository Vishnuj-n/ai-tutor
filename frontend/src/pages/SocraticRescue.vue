<template>
  <section class="rescue-page">
    <header class="page-header">
      <p class="eyebrow">Remediation</p>
      <h1>Concept Rescue</h1>
      <p class="subtitle">Failed quiz twice. Complete the Socratic session below to retry.</p>
    </header>

    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <p>Retrieving source content...</p>
    </div>

    <div v-else-if="error" class="error-state">
      <p class="error-msg">{{ error }}</p>
      <button type="button" class="action-btn" @click="goBack">Back to Dashboard</button>
    </div>

    <div v-else class="split-layout">
      <!-- Option A: In-App Socratic Tutor -->
      <section class="lane left-lane card in-app-lane">
        <header class="lane-header">
          <h2>Option A: Chat In-App</h2>
          <span class="lane-badge badge-primary">Interactive Tutor</span>
        </header>

        <div class="lane-content">
          <p class="option-desc">
            Resolve this concept rescue directly within our interactive learning environment. The
            in-app Socratic tutor will guide you through leading questions to help you master the
            material.
          </p>

          <div class="features-list">
            <div class="feature-item">
              <span class="feature-icon">💬</span>
              <div>
                <strong>Interactive Dialogue</strong>
                <p class="feature-sub">
                  Engage in a live, guided conversation grounded in your material.
                </p>
              </div>
            </div>
            <div class="feature-item">
              <span class="feature-icon">📖</span>
              <div>
                <strong>Context Grounded</strong>
                <p class="feature-sub">
                  The tutor retrieves relevant sections dynamically from this notebook.
                </p>
              </div>
            </div>
          </div>

          <div class="action-box">
            <button type="button" class="tutor-btn" @click="startInAppTutor">
              Start Socratic Chat In-App ➔
            </button>
          </div>
        </div>
      </section>

      <!-- Option B: External AI Prompt -->
      <section class="lane right-lane card external-lane">
        <header class="lane-header">
          <h2>Option B: Use External AI</h2>
          <span class="lane-badge badge-secondary">Copy Prompt</span>
        </header>

        <div class="lane-content">
          <p class="option-desc">
            Prefer using a premium model (like ChatGPT, Claude, or Gemini)? Copy the pre-engineered
            prompt containing the topic's source material below.
          </p>

          <div class="source-preview">
            <h3>Source Material</h3>
            <div class="source-text">{{ sourceText }}</div>
          </div>

          <div class="prompt-container">
            <textarea
              ref="promptTextarea"
              class="prompt-textarea"
              readonly
              :value="fullPrompt"
            ></textarea>

            <button
              type="button"
              class="copy-btn"
              :class="{ copied: copied }"
              @click="copyPromptToClipboard"
            >
              <span v-if="copied" class="copy-icon">✓</span>
              <span v-else class="copy-icon">📋</span>
              {{ copied ? 'Copied!' : 'Copy to Clipboard' }}
            </button>
          </div>

          <div class="completion-box">
            <p class="completion-instruction">
              Once you have finished the Socratic session externally and feel confident with the
              material, mark this task complete.
            </p>

            <button
              type="button"
              class="complete-btn"
              :disabled="completing"
              @click="finishRescueSession"
            >
              {{ completing ? 'Completing...' : "I've Completed the External Session" }}
            </button>
          </div>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getTopicSectionsContent, completeSocraticRescue, GetTaskContext } from '../services/appApi'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const error = ref('')
const completing = ref(false)
const copied = ref(false)

const topicID = ref('')
const notebookID = ref('')
const taskID = ref('')
const startPage = ref(1)
const endPage = ref(10)
const sourceText = ref('')
const notebookTitle = ref('')

const fullPrompt = computed(() => {
  const bookContext = notebookTitle.value ? `Book: ${notebookTitle.value}\n` : ''
  return `I'm studying the following text from ${notebookTitle.value || 'my material'} for preparation. I've failed to understand it twice. Please act as a Socratic tutor — don't give me summaries or answers. Instead, ask me leading questions that guide me to discover the key concepts myself. Start with the most fundamental question.

${bookContext}---
${sourceText.value}
---`
})

onMounted(async () => {
  taskID.value = route.query.taskId || route.query.task_id || ''

  if (!taskID.value) {
    error.value = 'Missing required route context (taskId).'
    loading.value = false
    return
  }

  try {
    loading.value = true
    const contextRes = await GetTaskContext(taskID.value)
    if (contextRes?.error) {
      error.value = `Failed to load task context: ${contextRes.error}`
      loading.value = false
      return
    }
    const task = contextRes?.task
    if (!task) {
      error.value = 'Task not found.'
      loading.value = false
      return
    }
    topicID.value = task.topic_id || ''
    notebookID.value = task.notebook_id || ''
    startPage.value = parseInt(task.start_page, 10) || 1
    endPage.value = parseInt(task.end_page, 10) || 10
  } catch (err) {
    error.value = `Failed to fetch task context: ${err.message || err}`
    loading.value = false
    return
  } finally {
    loading.value = false
  }

  if (!topicID.value) {
    error.value = 'Task does not specify a topic.'
    return
  }

  await loadSourceContent()
})

async function loadSourceContent() {
  loading.value = true
  error.value = ''
  try {
    const res = await getTopicSectionsContent(topicID.value, notebookID.value)
    if (res.error) {
      error.value = res.error
      return
    }

    notebookTitle.value = res.notebook_title || ''
    sourceText.value = res.content || ''
  } catch (err) {
    error.value = 'Failed to fetch topic source: ' + (err.message || err)
  } finally {
    loading.value = false
  }
}

async function copyPromptToClipboard() {
  try {
    await navigator.clipboard.writeText(fullPrompt.value)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 3000)
  } catch (err) {
    console.error('Failed to copy to clipboard', err)
  }
}

function startInAppTutor() {
  router.push({
    path: '/tutor',
    query: {
      notebook_id: notebookID.value,
      topic_id: topicID.value,
      taskId: taskID.value,
    },
  })
}

async function finishRescueSession() {
  if (completing.value) return
  completing.value = true
  error.value = ''
  try {
    const res = await completeSocraticRescue(taskID.value)
    if (res && res.error) {
      error.value = res.error
      completing.value = false
      return
    }
    // Successfully completed! Route back to dashboard where a new QUIZ task awaits.
    router.push('/dashboard')
  } catch (err) {
    error.value = 'Failed to complete session: ' + (err.message || err)
    completing.value = false
  }
}

function goBack() {
  router.push('/dashboard')
}
</script>

<style scoped>
.rescue-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
  min-height: calc(100vh - 64px);
  padding: 16px 8px;
  font-family: 'Inter', sans-serif;
  color: var(--on-surface);
}

.page-header {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: #d35400;
}

h1 {
  margin: 0;
  font-size: 40px;
  font-family: 'Manrope', sans-serif;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: var(--on-surface);
  line-height: 1.1;
}

.subtitle {
  margin: 0;
  font-size: 14px;
  color: var(--muted-text);
}

.loading-state,
.error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  flex: 1;
  padding: 48px;
  background: var(--surface-container-low);
  border-radius: 16px;
  border: 1px solid var(--outline-variant);
}

.spinner {
  width: 40px;
  height: 40px;
  border: 3.5px solid var(--outline-variant);
  border-top-color: #d35400;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.error-msg {
  color: #eb5e55;
  font-weight: 600;
  font-size: 15px;
  text-align: center;
}

.split-layout {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
  flex: 1;
}

.lane {
  display: flex;
  flex-direction: column;
  min-height: 500px;
}

.card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 20px;
  padding: 24px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.02);
  transition: border-color 0.25s ease;
}

.card:hover {
  border-color: rgba(211, 84, 0, 0.25);
}

.lane-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--outline-variant);
  padding-bottom: 16px;
  margin-bottom: 20px;
}

.lane-header h2 {
  margin: 0;
  font-size: 20px;
  font-family: 'Manrope', sans-serif;
  font-weight: 700;
  color: var(--on-surface);
}

.page-range {
  font-size: 12px;
  font-weight: 600;
  background: var(--surface-container-low);
  padding: 4px 10px;
  border-radius: 8px;
  color: var(--muted-text);
}

.lane-badge {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  background: rgba(211, 84, 0, 0.1);
  color: #d35400;
  padding: 4px 10px;
  border-radius: 8px;
}

.badge-primary {
  background: rgba(0, 91, 193, 0.1);
  color: #005bc1;
}

.badge-secondary {
  background: rgba(211, 84, 0, 0.1);
  color: #d35400;
}

.lane-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  flex: 1;
}

.option-desc {
  margin: 0;
  font-size: 14px;
  line-height: 1.6;
  color: var(--muted-text);
}

.features-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  background: var(--surface-container-low);
  border-radius: 14px;
  padding: 20px;
  border: 1px solid var(--outline-variant);
  flex: 1;
}

.feature-item {
  display: flex;
  gap: 14px;
  align-items: flex-start;
  text-align: left;
}

.feature-icon {
  font-size: 20px;
  line-height: 1;
}

.feature-item strong {
  display: block;
  font-size: 14px;
  font-weight: 700;
  color: var(--on-surface);
  margin-bottom: 2px;
}

.feature-sub {
  margin: 0;
  font-size: 12.5px;
  color: var(--muted-text);
  line-height: 1.4;
}

.action-box {
  margin-top: auto;
  border-top: 1px solid var(--outline-variant);
  padding-top: 20px;
}

.tutor-btn {
  width: 100%;
  background: linear-gradient(135deg, #005bc1, #0077ff);
  color: white;
  border: none;
  border-radius: 10px;
  padding: 12px;
  font-weight: 700;
  cursor: pointer;
  transition:
    opacity 0.2s,
    transform 0.15s;
  box-shadow: 0 4px 12px rgba(0, 91, 193, 0.2);
}

.tutor-btn:hover {
  opacity: 0.95;
  transform: translateY(-1px);
}

.prompt-instruction,
.completion-instruction {
  margin: 0;
  font-size: 13.5px;
  line-height: 1.5;
  color: var(--muted-text);
}

.source-preview {
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: var(--surface-container-low);
  border-radius: 12px;
  padding: 16px;
  border: 1px solid var(--outline-variant);
  text-align: left;
}

.source-preview h3 {
  margin: 0;
  font-size: 13px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-text);
}

.source-text {
  font-size: 13px;
  line-height: 1.5;
  color: var(--on-surface);
  max-height: 120px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.prompt-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: var(--surface-container-low);
  border-radius: 12px;
  padding: 16px;
  border: 1px solid var(--outline-variant);
}

.prompt-textarea {
  width: 100%;
  height: 140px;
  border: none;
  background: transparent;
  resize: none;
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--on-surface);
  outline: none;
}

.copy-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  color: var(--on-surface);
  padding: 10px 16px;
  border-radius: 8px;
  font-size: 13.5px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.copy-btn:hover {
  background: var(--outline-variant);
}

.copy-btn.copied {
  background: rgba(46, 204, 113, 0.1);
  border-color: rgba(46, 204, 113, 0.2);
  color: #2ecc71;
}

.copy-icon {
  font-size: 15px;
}

.completion-box {
  display: flex;
  flex-direction: column;
  gap: 12px;
  border-top: 1px solid var(--outline-variant);
  padding-top: 20px;
  margin-top: 20px;
}

.complete-btn {
  background: linear-gradient(135deg, #d35400, #e67e22);
  color: white;
  border: none;
  border-radius: 10px;
  padding: 12px;
  font-weight: 700;
  cursor: pointer;
  transition:
    opacity 0.2s,
    transform 0.15s;
  box-shadow: 0 4px 12px rgba(211, 84, 0, 0.2);
}

.complete-btn:hover:not(:disabled) {
  opacity: 0.95;
  transform: translateY(-1px);
}

.complete-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.action-btn {
  background: var(--primary);
  color: var(--on-primary);
  border: none;
  border-radius: 8px;
  padding: 10px 20px;
  font-weight: 600;
  cursor: pointer;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 900px) {
  .split-layout {
    grid-template-columns: 1fr;
    gap: 16px;
  }
}
</style>
