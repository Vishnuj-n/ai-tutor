<template>
  <section class="page">
    <p class="eyebrow">Written Assessment</p>
    <h1>Short-Answer Practice</h1>

    <article class="panel controls">
      <label class="field">
        <span>Notebook</span>
        <select v-model="selectedNotebookID" @change="onNotebookChange">
          <option disabled value="">Select a notebook</option>
          <option v-for="notebook in notebookTree" :key="notebook.notebook_id" :value="notebook.notebook_id">
            {{ notebook.title }}
          </option>
        </select>
      </label>

      <label class="field">
        <span>Topic</span>
        <select v-model="selectedTopicID" :disabled="availableTopics.length === 0">
          <option disabled value="">
            {{ availableTopics.length === 0 ? 'No topics available yet' : 'Select a topic' }}
          </option>
          <option v-for="topic in availableTopics" :key="topic.topic_id" :value="topic.topic_id">
            {{ topic.title }}
          </option>
        </select>
      </label>

      <button class="primary" :disabled="isGenerating || !canGenerate" @click="onGeneratePrompt">
        {{ isGenerating ? 'Generating...' : 'Generate Prompt' }}
      </button>
    </article>

    <article v-if="errorMessage" class="panel error">{{ errorMessage }}</article>

    <article v-if="isGenerating" class="panel loading-panel">
      <div class="loading-bubble" aria-live="polite" aria-label="Generating short-answer question">
        <span></span>
        <span></span>
        <span></span>
      </div>
      <p>Creating a grounded short-answer prompt...</p>
    </article>

    <article v-if="questionState && !isGenerating" class="panel question-card">
      <header>
        <p class="question-index">Question</p>
        <h2>{{ questionState.prompt }}</h2>
      </header>

      <label class="field answer-field">
        <span>Your answer</span>
        <textarea
          v-model="userAnswer"
          rows="7"
          class="answer-input"
          placeholder="Write a concise answer grounded in your notes..."
          :disabled="isScoring"
        ></textarea>
      </label>

      <div class="actions">
        <button class="ghost" :disabled="isScoring" @click="onClear">Clear</button>
        <button class="primary" :disabled="isScoring || !canSubmitAnswer" @click="onSubmitAnswer">
          {{ isScoring ? 'Scoring...' : 'Submit Answer' }}
        </button>
      </div>

      <section v-if="scoreResult" class="feedback" :class="ratingClass(scoreResult.fsrsRating)">
        <h3>Score {{ scoreResult.score }}/10 · {{ formatRating(scoreResult.fsrsRating) }}</h3>
        <p>{{ scoreResult.feedback }}</p>

        <div class="actions">
          <button class="ghost" :disabled="isGenerating" @click="onClear">Clear</button>
          <button class="primary" :disabled="isGenerating || !canGenerate" @click="onNextPrompt">
            {{ isGenerating ? 'Generating...' : 'Next Prompt' }}
          </button>
        </div>
      </section>
    </article>

    <article v-else-if="!isGenerating" class="panel">
      <h2>Ready to practice</h2>
      <p v-if="notebookTree.length === 0">Upload a notebook to start generating written assessments.</p>
      <p v-else-if="selectedNotebook && availableTopics.length === 0">
        This notebook has no topics yet. Wait for extraction to finish or choose another notebook.
      </p>
      <p v-else>Select a notebook and topic to generate a short-answer prompt.</p>
    </article>
  </section>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { generateShortAnswerPrompt, getNotebookTopicTree, scoreShortAnswer } from '../services/appApi'

const route = useRoute()

const notebookTree = ref([])
const selectedNotebookID = ref('')
const selectedTopicID = ref('')
const questionState = ref(null)
const userAnswer = ref('')
const scoreResult = ref(null)
const errorMessage = ref('')
const isGenerating = ref(false)
const isScoring = ref(false)

const selectedNotebook = computed(() =>
  notebookTree.value.find((notebook) => notebook.notebook_id === selectedNotebookID.value) || null
)
const availableTopics = computed(() => selectedNotebook.value?.topics || [])
const canGenerate = computed(() => selectedTopicID.value !== '')
const canSubmitAnswer = computed(() => userAnswer.value.trim().length > 0 && questionState.value !== null)

watch(selectedTopicID, () => {
  resetAssessmentState()
})

onMounted(async () => {
  await loadNotebookTopicTree()
})

async function loadNotebookTopicTree() {
  try {
    const data = await getNotebookTopicTree()
    notebookTree.value = Array.isArray(data) ? data : []
    applyInitialSelection(getPreferredTopicID())
  } catch (err) {
    errorMessage.value = err?.message || 'Failed to load notebook topics'
    notebookTree.value = []
  }
}

function resetAssessmentState() {
  questionState.value = null
  userAnswer.value = ''
  scoreResult.value = null
  errorMessage.value = ''
}

function onClear() {
  resetAssessmentState()
}

async function onGeneratePrompt() {
  if (!canGenerate.value) {
    return
  }
  isGenerating.value = true
  isScoring.value = false
  errorMessage.value = ''
  questionState.value = null
  scoreResult.value = null
  userAnswer.value = ''

  try {
    const result = await generateShortAnswerPrompt(selectedTopicID.value)
    if (result?.error) {
      errorMessage.value = result.error
      return
    }
    if (!result?.questionID || !result?.prompt) {
      errorMessage.value = 'Question generation returned invalid data.'
      return
    }
    questionState.value = {
      questionID: result.questionID,
      prompt: result.prompt,
      topicID: result.topicID || selectedTopicID.value,
    }
  } catch (err) {
    errorMessage.value = err?.message || 'Failed to generate short-answer prompt'
  } finally {
    isGenerating.value = false
  }
}

async function onSubmitAnswer() {
  if (!questionState.value || !userAnswer.value.trim()) {
    return
  }
  isScoring.value = true
  errorMessage.value = ''
  try {
    const result = await scoreShortAnswer(
      questionState.value.questionID,
      questionState.value.prompt,
      userAnswer.value.trim()
    )
    if (result?.error) {
      errorMessage.value = result.error
      return
    }
    scoreResult.value = {
      score: Number(result.score || 0),
      fsrsRating: String(result.fsrsRating || ''),
      feedback: String(result.feedback || ''),
    }
  } catch (err) {
    errorMessage.value = err?.message || 'Failed to score short answer'
  } finally {
    isScoring.value = false
  }
}

async function onNextPrompt() {
  resetAssessmentState()
  await onGeneratePrompt()
}

function applyInitialSelection(preferredTopicID) {
  if (notebookTree.value.length === 0) {
    selectedNotebookID.value = ''
    selectedTopicID.value = ''
    return
  }

  if (preferredTopicID) {
    for (const notebook of notebookTree.value) {
      const topic = Array.isArray(notebook.topics)
        ? notebook.topics.find((item) => item.topic_id === preferredTopicID)
        : null
      if (topic) {
        selectedNotebookID.value = notebook.notebook_id
        selectedTopicID.value = topic.topic_id
        return
      }
    }
  }

  const firstNotebookWithTopics = notebookTree.value.find(
    (notebook) => Array.isArray(notebook.topics) && notebook.topics.length > 0
  )
  const fallbackNotebook = firstNotebookWithTopics || notebookTree.value[0]
  selectedNotebookID.value = fallbackNotebook?.notebook_id || ''
  selectedTopicID.value = fallbackNotebook?.topics?.[0]?.topic_id || ''
}

function onNotebookChange() {
  const nextTopicID = availableTopics.value[0]?.topic_id || ''
  if (!availableTopics.value.some((topic) => topic.topic_id === selectedTopicID.value)) {
    selectedTopicID.value = nextTopicID
  }
}

function getPreferredTopicID() {
  if (typeof route.query.topic === 'string') {
    return route.query.topic
  }
  return ''
}

function formatRating(raw) {
  const value = String(raw || '').toLowerCase()
  if (value === 'again') return 'Again'
  if (value === 'hard') return 'Hard'
  if (value === 'good') return 'Good'
  if (value === 'easy') return 'Easy'
  return 'Unrated'
}

function ratingClass(raw) {
  const value = String(raw || '').toLowerCase()
  if (value === 'again') return 'bad'
  if (value === 'hard') return 'warn'
  if (value === 'good') return 'good'
  if (value === 'easy') return 'great'
  return ''
}
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
  overflow-x: hidden;
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
  margin: 0;
  font-size: 46px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 24px;
  width: 100%;
  box-sizing: border-box;
}

.controls {
  display: flex;
  align-items: end;
  gap: 14px;
  flex-wrap: wrap;
  width: 100%;
  box-sizing: border-box;
}

.field {
  display: grid;
  gap: 8px;
  flex: 1 1 auto;
  min-width: clamp(200px, 100%, 420px);
}

.field span {
  color: var(--muted-text);
  font-size: 13px;
}

select {
  border: 1px solid color-mix(in srgb, var(--muted-text) 20%, transparent);
  background: white;
  border-radius: 12px;
  width: 100%;
  box-sizing: border-box;
  padding: 10px 12px;
  font-size: 15px;
}

.primary,
.ghost {
  border-radius: 12px;
  padding: 8px 14px;
  font-weight: 600;
  font-size: 14px;
  cursor: pointer;
}

.primary {
  border: 0;
  background: #20222f;
  color: #fff;
}

.ghost {
  border: 1px solid color-mix(in srgb, var(--muted-text) 20%, transparent);
  background: var(--surface-container-highest);
  color: var(--on-surface);
}

.primary:disabled,
.ghost:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

h2 {
  margin: 0 0 8px;
  font-family: 'Manrope', sans-serif;
}

p {
  margin: 0;
  color: var(--muted-text);
}

.question-card {
  display: grid;
  gap: 18px;
}

.question-index {
  margin-bottom: 6px;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.12em;
}

.answer-field {
  min-width: 100%;
}

.answer-input {
  border: 1px solid color-mix(in srgb, var(--muted-text) 20%, transparent);
  background: white;
  border-radius: 12px;
  width: 100%;
  box-sizing: border-box;
  padding: 12px;
  font-size: 15px;
  font-family: inherit;
  resize: vertical;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.feedback {
  border-radius: 12px;
  padding: 14px;
  display: grid;
  gap: 8px;
}

.feedback h3 {
  margin: 0;
}

.feedback.bad {
  background: #fff1ee;
}

.feedback.warn {
  background: #fff6e8;
}

.feedback.good {
  background: #e8f7ee;
}

.feedback.great {
  background: #e5f5ff;
}

.loading-panel {
  display: grid;
  gap: 12px;
  justify-items: center;
}

.loading-bubble {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.loading-bubble span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #596080;
  animation: pulse 1s ease-in-out infinite;
}

.loading-bubble span:nth-child(2) {
  animation-delay: 0.12s;
}

.loading-bubble span:nth-child(3) {
  animation-delay: 0.24s;
}

.error {
  border: 1px solid #f3b5a7;
  background: #fff3ef;
  color: #8a2d16;
}

@keyframes pulse {
  0%,
  80%,
  100% {
    transform: scale(0.8);
    opacity: 0.5;
  }
  40% {
    transform: scale(1);
    opacity: 1;
  }
}
</style>
