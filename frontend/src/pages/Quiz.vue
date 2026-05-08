<template>
  <section class="quiz-page">
    <header class="page-header">
      <h1 class="page-title">Quiz</h1>
      <p v-if="taskMeta" class="meta">Pages {{ taskMeta.start_page }}-{{ taskMeta.end_page }}</p>
      <select v-else-if="!taskID" v-model="selectedNotebookID" class="notebook-select" :disabled="loading || generating">
        <option value="">— Select Notebook —</option>
        <option v-for="nb in notebooks" :key="nb.id" :value="nb.id">{{ nb.title }}</option>
      </select>
    </header>

    <article v-if="loading" class="state-card">
      <p>Loading quiz...</p>
    </article>

    <article v-else-if="error" class="state-card error-card">
      <p>{{ error }}</p>
    </article>

    <div v-else-if="!taskID && !generating && questions.length === 0" class="manual-controls">
      <div class="input-group">
        <label>Start Page</label>
        <input v-model.number="startPage" type="number" min="1" :disabled="loading" />
      </div>
      <div class="input-group">
        <label>End Page</label>
        <input v-model.number="endPage" type="number" min="1" :disabled="loading" />
      </div>
      <button class="primary-btn" :disabled="!canGenerateManual" :loading="generating" @click="generateManualQuiz">
        {{ generating ? 'Generating...' : 'Generate Quiz' }}
      </button>
    </div>

    <article v-else-if="submitted && result" class="result-card">
      <h2>{{ result.passed ? 'Passed' : 'Needs Reread' }}</h2>
      <p>Score: {{ result.score }}% (threshold {{ result.passing_score }}%)</p>
      <p>{{ result.feedback }}</p>
    </article>

    <article v-else-if="questions.length === 0 && !generating" class="state-card">
      <p>No quiz questions found for this task.</p>
    </article>

    <form v-else class="quiz-form" @submit.prevent="submitQuiz">
      <article v-for="(q, index) in questions" :key="q.id" class="question-card">
        <p class="prompt">{{ index + 1 }}. {{ q.prompt }}</p>
        <label v-for="option in q.options" :key="option" class="option-row">
          <input v-model="answers[q.id]" type="radio" :name="q.id" :value="option" :disabled="submitting" />
          <span>{{ option }}</span>
        </label>
      </article>

      <button class="primary-btn" type="submit" :disabled="submitting || !allAnswered">
        {{ submitting ? 'Scoring...' : 'Submit Quiz' }}
      </button>
    </form>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { activateTask, getTask, submitQuizAttempt, getNotebooks, generateQuizForPageRange } from '../services/appApi'

const route = useRoute()

const loading = ref(true)
const submitting = ref(false)
const submitted = ref(false)
const error = ref('')
const taskMeta = ref(null)
const questions = ref([])
const answers = ref({})
const result = ref(null)

// Manual generation state
const notebooks = ref([])
const selectedNotebookID = ref('')
const startPage = ref(1)
const endPage = ref(10)
const generating = ref(false)

const taskID = computed(() => {
  if (typeof route.query.taskId === 'string' && route.query.taskId.trim() !== '') {
    return route.query.taskId.trim()
  }
  if (typeof route.query.task_id === 'string' && route.query.task_id.trim() !== '') {
    return route.query.task_id.trim()
  }
  return ''
})

const allAnswered = computed(() => {
  if (questions.value.length === 0) {
    return false
  }
  return questions.value.every((q) => typeof answers.value[q.id] === 'string' && answers.value[q.id].trim() !== '')
})

const canGenerateManual = computed(() =>
  selectedNotebookID.value && startPage.value > 0 && endPage.value >= startPage.value && !generating.value
)

onMounted(async () => {
  await loadNotebooks()
  if (taskID.value) {
    await loadQuizTask()
  } else {
    loading.value = false
  }
})

async function loadNotebooks() {
  try {
    const res = await getNotebooks()
    notebooks.value = Array.isArray(res) ? res.filter(n => !n.error) : []
  } catch { error.value = 'Failed to load notebooks.' }
}

async function loadQuizTask() {
  loading.value = true
  error.value = ''
  try {
    const activate = await activateTask(taskID.value)
    if (activate?.error && activate.error !== 'ErrTaskNotPending') {
      error.value = activate.error
      return
    }

    const response = await getTask(taskID.value)
    if (response?.error) {
      error.value = response.error
      return
    }

    const task = response?.task
    if (!task || task.task_type !== 'QUIZ') {
      error.value = 'Task is not a quiz task.'
      return
    }

    taskMeta.value = task
    const payload = typeof task.payload_json === 'string' && task.payload_json.trim() !== ''
      ? JSON.parse(task.payload_json)
      : null

    questions.value = Array.isArray(payload?.questions) ? payload.questions : []
    answers.value = {}
    submitted.value = false
    result.value = null
  } catch (err) {
    error.value = err?.message || 'Failed to load quiz task.'
  } finally {
    loading.value = false
  }
}

async function generateManualQuiz() {
  error.value = ''
  questions.value = []
  answers.value = {}
  submitted.value = false
  result.value = null
  generating.value = true
  try {
    const res = await generateQuizForPageRange(selectedNotebookID.value, startPage.value, endPage.value)
    if (res.error) {
      error.value = res.error
      return
    }
    questions.value = Array.isArray(res.questions) ? res.questions : []
    if (questions.value.length === 0) {
      error.value = 'No questions generated. Try a different page range.'
    }
  } catch (e) {
    error.value = e?.message ?? 'Quiz generation failed.'
  } finally {
    generating.value = false
  }
}

async function submitQuiz() {
  if (!allAnswered.value) {
    return
  }
  submitting.value = true
  error.value = ''
  try {
    const payload = questions.value.map((q) => ({
      question_id: q.id,
      selected: answers.value[q.id],
    }))
    const response = await submitQuizAttempt(taskID.value, payload)
    if (response?.error) {
      error.value = response.error
      return
    }
    result.value = response?.result || null
    submitted.value = true
  } catch (err) {
    error.value = err?.message || 'Failed to submit quiz.'
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.quiz-page {
  display: grid;
  gap: 14px;
}

.page-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.page-title {
  margin: 0;
  font-size: 36px;
  font-family: 'Manrope', sans-serif;
}

.meta {
  margin: 0;
  color: var(--muted-text);
  font-size: 13px;
}

.notebook-select {
  width: 300px;
  padding: 0.5rem 0.75rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  background: var(--background);
  color: var(--on-surface);
  font-size: 0.9rem;
}

.manual-controls {
  display: flex;
  gap: 1rem;
  align-items: flex-end;
  padding: 1rem;
  background: #f5f5f5;
  border-radius: 4px;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.input-group label {
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--on-surface);
}

.input-group input {
  width: 100px;
  padding: 0.5rem 0.75rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 0.9rem;
}

.input-group input:focus {
  outline: none;
  border-color: var(--primary);
}

.state-card,
.result-card,
.question-card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--surface-container-low);
  border-radius: 12px;
  padding: 12px;
}

.error-card {
  color: #b42318;
}

.quiz-form {
  display: grid;
  gap: 10px;
}

.prompt {
  margin: 0 0 8px;
  font-weight: 700;
}

.option-row {
  display: flex;
  gap: 8px;
  padding: 4px 0;
}

.primary-btn {
  border: 0;
  border-radius: 12px;
  padding: 10px 16px;
  color: var(--on-primary);
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  justify-self: start;
}

.primary-btn:disabled {
  opacity: 0.6;
}
</style>
