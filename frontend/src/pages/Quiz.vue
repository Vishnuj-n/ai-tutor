<template>
  <StudyPageLayout
    eyebrow="Assessment"
    :title="taskMeta ? 'Quiz' : 'Quiz'"
    :subtitle="taskMeta ? `Pages ${taskMeta.start_page}–${taskMeta.end_page}` : ''"
  >
    <!-- Toolbar: notebook selector (manual mode only) -->
    <template v-if="!taskID && !generating && questions.length === 0" #toolbar>
      <div class="toolbar-field">
        <label class="field-label" for="quiz-notebook-select">Notebook</label>
        <select
          id="quiz-notebook-select"
          v-model="selectedNotebookID"
          class="ghost-select"
          :disabled="loading || generating"
        >
          <option value="">— Select Notebook —</option>
          <option v-for="nb in notebooks" :key="nb.id" :value="nb.id">{{ nb.title }}</option>
        </select>
      </div>
    </template>

    <!-- Loading -->
    <article v-if="loading" class="state-panel">
      <p class="state-text">Loading quiz…</p>
    </article>

    <!-- Error -->
    <article v-else-if="error" class="state-panel state-panel--error">
      <p class="state-text">{{ error }}</p>
    </article>

    <!-- Manual controls: page range + generate -->
    <div
      v-else-if="!taskID && !generating && questions.length === 0"
      class="config-panel"
    >
      <p class="config-panel__hint">Enter the page range to generate quiz questions from.</p>
      <div class="config-panel__row">
        <div class="number-field">
          <label class="field-label" for="quiz-start">Start Page</label>
          <input id="quiz-start" v-model.number="startPage" class="ghost-input" type="number" min="1" :disabled="loading" />
        </div>
        <div class="number-field">
          <label class="field-label" for="quiz-end">End Page</label>
          <input id="quiz-end" v-model.number="endPage" class="ghost-input" type="number" min="1" :disabled="loading" />
        </div>
        <button
          id="quiz-generate-btn"
          class="primary-btn"
          :disabled="!canGenerateManual"
          @click="generateManualQuiz"
        >
          {{ generating ? 'Generating…' : 'Generate Quiz' }}
        </button>
      </div>
    </div>

    <!-- Generating state -->
    <article v-else-if="generating" class="state-panel">
      <p class="state-text">Generating questions…</p>
    </article>

    <!-- Result card -->
    <article v-else-if="submitted && result" class="result-panel">
      <div class="result-panel__badge" :class="result.passed ? 'badge--pass' : 'badge--fail'">
        {{ result.passed ? 'Passed' : 'Needs Reread' }}
      </div>
      <p class="result-panel__score">
        <span class="score-value">{{ result.score }}%</span>
        <span class="score-threshold">threshold {{ result.passing_score }}%</span>
      </p>
      <p v-if="result.feedback" class="result-panel__feedback">{{ result.feedback }}</p>
      <p v-if="!result.passed" class="result-panel__attempts">
        Attempt {{ result.reread_attempt_count }} of {{ result.max_reread_attempts }}
      </p>
    </article>

    <!-- Empty state: no questions -->
    <article v-else-if="questions.length === 0 && !generating" class="state-panel">
      <p class="state-text">No quiz questions found for this task.</p>
    </article>

    <!-- Quiz form -->
    <form v-else class="quiz-form" @submit.prevent="submitQuiz">
      <article
        v-for="(q, index) in questions"
        :key="q.id"
        class="question-card"
      >
        <p class="question-prompt">
          <span class="question-num">{{ index + 1 }}</span>
          {{ q.prompt }}
        </p>
        <div class="options-grid">
          <label
            v-for="option in q.options"
            :key="option"
            class="option-row"
            :class="{ 'option-row--selected': answers[q.id] === option }"
          >
            <input
              v-model="answers[q.id]"
              class="option-radio"
              type="radio"
              :name="q.id"
              :value="option"
              :disabled="submitting"
            />
            <span class="option-text">{{ option }}</span>
          </label>
        </div>
      </article>

      <div class="form-footer">
        <p v-if="!allAnswered" class="footer-hint">Answer all questions to submit.</p>
        <button
          id="quiz-submit-btn"
          class="primary-btn"
          type="submit"
          :disabled="submitting || !allAnswered"
        >
          {{ submitting ? 'Scoring…' : 'Submit Quiz' }}
        </button>
      </div>
    </form>
  </StudyPageLayout>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { activateTask, getTask, submitQuizAttempt, getNotebooks, generateQuizForPageRange } from '../services/appApi'
import StudyPageLayout from '../components/StudyPageLayout.vue'

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
/* ── Toolbar controls ─────────────────────────── */
.toolbar-field {
  display: grid;
  gap: 4px;
}

.field-label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--muted-text);
}

/* Ghost select: suggestion of a border, no hard box */
.ghost-select {
  appearance: none;
  width: 100%;
  padding: 8px 32px 8px 12px;
  background: var(--surface-container-lowest)
    url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6' fill='none'%3E%3Cpath d='M1 1l4 4 4-4' stroke='%2364707d' stroke-width='1.5' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E")
    no-repeat right 12px center;
  border: 1px solid var(--outline-variant);
  border-radius: 10px;
  font: inherit;
  font-size: 14px;
  color: var(--on-surface);
  cursor: pointer;
  transition: border-color 0.15s ease;
  max-width: 220px;
}

.ghost-select:focus {
  outline: none;
  border-color: var(--primary);
}

.ghost-select:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* ── Config panel ─────────────────────────────── */
.config-panel {
  background: var(--surface-container-low);
  border-radius: 16px;
  padding: 24px;
  display: grid;
  gap: 16px;
}

.config-panel__hint {
  margin: 0;
  font-size: 14px;
  color: var(--muted-text);
  line-height: 1.5;
}

.config-panel__row {
  display: flex;
  gap: 12px;
  align-items: flex-end;
  flex-wrap: wrap;
}

.number-field {
  display: grid;
  gap: 4px;
}

/* Ghost input: 1px outline-variant hint, no heavy box */
.ghost-input {
  width: 96px;
  padding: 8px 12px;
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 10px;
  font: inherit;
  font-size: 14px;
  color: var(--on-surface);
  transition: border-color 0.15s ease;
}

.ghost-input:focus {
  outline: none;
  border-color: var(--primary);
}

.ghost-input:disabled {
  opacity: 0.5;
}

/* ── State panels ─────────────────────────────── */
.state-panel {
  background: var(--surface-container-low);
  border-radius: 16px;
  padding: 48px 24px;
  text-align: center;
}

.state-panel--error .state-text {
  color: #b42318;
}

.state-text {
  margin: 0;
  font-size: 15px;
  color: var(--muted-text);
}

/* ── Result panel ─────────────────────────────── */
.result-panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 32px 24px;
  display: grid;
  gap: 12px;
  justify-items: start;
}

.result-panel__badge {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  padding: 4px 12px;
  border-radius: 999px;
}

.badge--pass {
  background: color-mix(in srgb, #16a34a 12%, var(--surface-container-low));
  color: #16a34a;
}

.badge--fail {
  background: color-mix(in srgb, #b42318 12%, var(--surface-container-low));
  color: #b42318;
}

.result-panel__score {
  margin: 0;
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.score-value {
  font-family: 'Manrope', sans-serif;
  font-size: 40px;
  font-weight: 700;
  letter-spacing: -0.03em;
  color: var(--on-surface);
  line-height: 1;
}

.score-threshold {
  font-size: 13px;
  color: var(--muted-text);
}

.result-panel__feedback {
  margin: 0;
  font-size: 15px;
  color: var(--on-surface);
  line-height: 1.6;
  max-width: 60ch;
}

.result-panel__attempts {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
}

/* ── Quiz form ────────────────────────────────── */
.quiz-form {
  display: grid;
  gap: 12px;
}

.question-card {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 24px;
  display: grid;
  gap: 16px;
  transition: background 0.15s ease;
}

.question-prompt {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--on-surface);
  line-height: 1.5;
  display: flex;
  gap: 10px;
}

.question-num {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted-text);
  padding-top: 3px;
  min-width: 1.5ch;
}

.options-grid {
  display: grid;
  gap: 8px;
}

.option-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--surface-container-low);
  cursor: pointer;
  transition: background 0.12s ease;
}

.option-row:hover {
  background: color-mix(in srgb, var(--primary) 6%, var(--surface-container-low));
}

.option-row--selected {
  background: color-mix(in srgb, var(--primary) 10%, var(--surface-container-lowest));
}

.option-radio {
  margin-top: 2px;
  flex-shrink: 0;
  accent-color: var(--primary);
}

.option-text {
  font-size: 14px;
  color: var(--on-surface);
  line-height: 1.5;
}

/* ── Form footer ──────────────────────────────── */
.form-footer {
  display: flex;
  align-items: center;
  gap: 16px;
  padding-top: 8px;
  flex-wrap: wrap;
}

.footer-hint {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
}

/* ── Primary CTA ──────────────────────────────── */
.primary-btn {
  border: 0;
  border-radius: 12px;
  padding: 11px 24px;
  color: var(--on-primary);
  font-family: inherit;
  font-size: 14px;
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  cursor: pointer;
  transition: transform 0.14s ease, filter 0.14s ease;
  white-space: nowrap;
}

.primary-btn:hover:not(:disabled) {
  filter: brightness(1.08);
}

.primary-btn:active:not(:disabled) {
  transform: scale(0.96);
}

.primary-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
