<template>
  <section class="page">
    <p class="eyebrow">Quiz</p>
    <h1>Topic Quiz</h1>

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

      <button class="primary" :disabled="isGenerating || !canGenerateQuiz" @click="onGenerateQuiz">
        {{ isGenerating ? 'Generating...' : 'Generate Quiz' }}
      </button>
    </article>

    <article v-if="errorMessage" class="panel error">{{ errorMessage }}</article>

    <article v-if="currentQuestion" class="panel question-card">
      <header>
        <p class="question-index">Question {{ currentIndex + 1 }} / {{ questions.length }}</p>
        <h2>{{ currentQuestion.prompt }}</h2>
      </header>

      <fieldset v-if="!feedbackVisible" class="options">
        <legend>Answer choices for question {{ currentIndex + 1 }}</legend>
        <label v-for="(opt, idx) in currentQuestion.options" :key="opt + idx" class="option">
          <input
            v-model="selectedAnswer"
            type="radio"
            :name="`quiz-question-${currentQuestion.id}`"
            :value="opt"
          />
          <span>{{ String.fromCharCode(65 + idx) }}. {{ opt }}</span>
        </label>
      </fieldset>

      <div v-if="feedbackVisible && scoreResult" class="feedback" :class="scoreResult.correct ? 'good' : 'bad'">
        <h3>{{ scoreResult.correct ? 'Correct' : 'Not quite' }} · Score {{ scoreResult.score }}</h3>
        <p>{{ scoreResult.feedback }}</p>
        <p class="hint">Hint: {{ scoreResult.hint }}</p>
        <p v-if="!scoreResult.correct" class="expected">Expected: {{ scoreResult.expected }}</p>
      </div>

      <footer class="actions">
        <button
          v-if="!feedbackVisible"
          class="primary"
          :disabled="isScoring || !selectedAnswer"
          @click="onSubmitAnswer"
        >
          {{ isScoring ? 'Scoring...' : 'Submit Answer' }}
        </button>

        <button
          v-if="feedbackVisible"
          class="primary"
          :disabled="currentIndex >= questions.length - 1"
          @click="onNext"
        >
          {{ currentIndex >= questions.length - 1 ? 'Quiz Complete' : 'Next Question' }}
        </button>
      </footer>
    </article>

    <article v-else-if="questions.length === 0" class="panel">
      <h2>Ready to generate</h2>
      <p v-if="notebookTree.length === 0">Upload a notebook to start generating quizzes.</p>
      <p v-else-if="selectedNotebook && availableTopics.length === 0">
        This notebook has no topics yet. Wait for extraction to finish or choose another notebook.
      </p>
      <p v-else>Select a notebook and topic to generate a quiz.</p>
    </article>
  </section>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { generateQuiz, getNotebookTopicTree, scoreAnswer } from '../services/appApi'

const route = useRoute()

const notebookTree = ref([])
const selectedNotebookID = ref('')
const selectedTopicID = ref('')
const questions = ref([])
const currentIndex = ref(0)
const selectedAnswer = ref('')
const scoreResult = ref(null)
const feedbackVisible = ref(false)
const errorMessage = ref('')
const isGenerating = ref(false)
const isScoring = ref(false)

const currentQuestion = computed(() => questions.value[currentIndex.value] || null)
const selectedNotebook = computed(() =>
  notebookTree.value.find((notebook) => notebook.notebook_id === selectedNotebookID.value) || null
)
const availableTopics = computed(() => selectedNotebook.value?.topics || [])
const canGenerateQuiz = computed(() => selectedTopicID.value !== '')

// Helper to clear quiz state
function resetQuizState() {
  questions.value = []
  currentIndex.value = 0
  selectedAnswer.value = ''
  scoreResult.value = null
  feedbackVisible.value = false
}

// Clear quiz state when topic selection changes
watch(selectedTopicID, () => {
  resetQuizState()
})

onMounted(async () => {
  try {
    const data = await getNotebookTopicTree()
    notebookTree.value = Array.isArray(data) ? data : []
    applyInitialSelection(getPreferredTopicID())
  } catch (err) {
    errorMessage.value = err?.message || 'Failed to load notebook topics'
  }
})

async function onGenerateQuiz() {
  if (!selectedTopicID.value) {
    return
  }
  // Clear any stale quiz state before starting new generation
  resetQuizState()
  isGenerating.value = true
  errorMessage.value = ''
  try {
    const result = await generateQuiz(selectedTopicID.value)
    if (result?.error) {
      errorMessage.value = result.error
      return
    }
    questions.value = Array.isArray(result?.questions) ? result.questions : []
    if (questions.value.length === 0) {
      errorMessage.value = 'No quiz questions were generated for this topic.'
    }
  } catch (err) {
    errorMessage.value = err?.message || 'Quiz generation failed'
  } finally {
    isGenerating.value = false
  }
}

async function onSubmitAnswer() {
  if (!currentQuestion.value || !selectedAnswer.value) {
    return
  }
  isScoring.value = true
  errorMessage.value = ''
  try {
    const result = await scoreAnswer(currentQuestion.value.id, selectedAnswer.value)
    if (result?.error) {
      errorMessage.value = result.error
      return
    }
    scoreResult.value = result
    feedbackVisible.value = true
  } catch (err) {
    errorMessage.value = err?.message || 'Failed to score answer'
  } finally {
    isScoring.value = false
  }
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

function onNext() {
  if (currentIndex.value < questions.value.length - 1) {
    currentIndex.value += 1
    selectedAnswer.value = ''
    scoreResult.value = null
    feedbackVisible.value = false
  }
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

.primary {
  border: 0;
  border-radius: 12px;
  padding: 8px 14px;
  background: #20222f;
  color: #fff;
  font-weight: 600;
  font-size: 14px;
  cursor: pointer;
}

.primary:disabled {
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
  width: 100%;
  box-sizing: border-box;
}

.question-index {
  margin-bottom: 6px;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.12em;
}

.options {
  display: grid;
  gap: 10px;
  width: 100%;
  box-sizing: border-box;
}

.option {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  border: 1px solid color-mix(in srgb, var(--muted-text) 20%, transparent);
  border-radius: 12px;
  padding: 10px 12px;
  width: 100%;
  box-sizing: border-box;
  word-wrap: break-word;
  overflow-wrap: break-word;
}

.feedback {
  border-radius: 12px;
  padding: 14px;
  display: grid;
  gap: 8px;
}

.feedback.good {
  background: #e8f7ee;
}

.feedback.bad {
  background: #fff1ee;
}

.feedback h3 {
  margin: 0;
}

.hint,
.expected {
  color: #2f334a;
}

.actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 8px;
}

.error {
  border: 1px solid color-mix(in srgb, #b42318 30%, var(--surface-container-lowest));
  background: color-mix(in srgb, #b42318 10%, var(--surface-container-lowest));
  color: #8a2d16;
}
</style>
