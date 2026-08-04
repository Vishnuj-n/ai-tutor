<template>
  <StudyPageLayout
    eyebrow="Assessment"
    :title="taskMeta?.task_type === 'MILESTONE_EXAM' ? 'Milestone Exam' : 'Quiz'"
    :subtitle="
      taskMeta
        ? taskMeta.task_type === 'MILESTONE_EXAM'
          ? 'Cumulative Notebook Assessment'
          : taskMeta.start_page || taskMeta.end_page
            ? `Pages ${taskMeta.start_page}–${taskMeta.end_page}`
            : 'Pages N/A'
        : ''
    "
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
      <button
        v-if="canRetryGeneration"
        class="primary-btn retry-generation-btn"
        @click="generateManualQuiz"
      >
        Retry
      </button>
    </article>

    <!-- Manual controls: page range + generate -->
    <div v-else-if="!taskID && !generating && questions.length === 0" class="config-panel">
      <p class="config-panel__hint">Enter the page range to generate quiz questions from.</p>
      <div class="config-panel__row">
        <div class="number-field">
          <label class="field-label" for="quiz-start">Start Page</label>
          <input
            id="quiz-start"
            v-model.number="startPage"
            class="ghost-input"
            type="number"
            min="1"
            :disabled="loading"
          />
        </div>
        <div class="number-field">
          <label class="field-label" for="quiz-end">End Page</label>
          <input
            id="quiz-end"
            v-model.number="endPage"
            class="ghost-input"
            type="number"
            min="1"
            :disabled="loading"
          />
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

      <!-- Flashcard generation pending message (only when passed and flashcards pending) -->
      <div
        v-if="result.passed && result.flashcards_pending"
        class="result-panel__flashcard-success"
      >
        <p class="flashcard-success-title">Flashcards scheduled for generation.</p>
        <p class="flashcard-success-detail">
          Click Continue to generate flashcards for spaced repetition.
        </p>
        <p class="flashcard-success-hint">
          These cards will appear in your dashboard when they're due for review.
        </p>
      </div>

      <!-- Flashcard generation success message (only when generated during Continue) -->
      <div
        v-if="result.passed && !result.flashcards_pending && result.flashcards_generated > 0"
        class="result-panel__flashcard-success"
      >
        <p class="flashcard-success-title">Flashcards generated successfully.</p>
        <p class="flashcard-success-detail">
          {{ result.flashcards_generated }} review cards scheduled for spaced repetition.
        </p>
        <p class="flashcard-success-hint">
          These cards will appear in your dashboard when they're due for review.
        </p>
      </div>

      <!-- Flashcard generation/sync error -->
      <div
        v-if="result.passed && result.flashcards_generation_error"
        class="result-panel__network-warning"
      >
        <span class="warning-icon">⚠</span>
        <div class="warning-text">
          <p class="warning-title">Generation Warning / Network Error</p>
          <p class="warning-detail">
            Flashcard generation could not be completed automatically. A retry task has been added
            to your queue. You can also try retrying the generation now.
          </p>
          <p class="warning-message">
            Reason: {{ result.flashcards_generation_message || 'Unknown error' }}
          </p>
        </div>
      </div>

      <!-- Reread message (when failed) -->
      <p v-if="!result.passed && result.reread_task_id" class="result-panel__reread-message">
        Reread session added to your learning queue.
      </p>
      <p v-if="!result.passed && result.reread_task_id" class="result-panel__attempts">
        Attempt {{ result.reread_attempt_count }} of {{ result.max_reread_attempts }}
      </p>

      <!-- External help notice (when failed re-quiz) -->
      <div
        v-if="!result.passed && !result.reread_task_id && result.manual_review_recommended"
        class="result-panel__external-help-notice"
      >
        <p class="external-help-title">External Review Required</p>
        <p class="external-help-desc">
          This concept requires external review. Your next reading task has been unlocked so you
          don't fall behind. Please consult your notes or instructor for this page range.
        </p>
      </div>

      <!-- Questions Breakdown (Correct / Incorrect review) -->
      <section v-if="questions.length > 0" class="result-breakdown">
        <h3 class="breakdown-title">Question Breakdown</h3>
        <article
          v-for="(q, index) in questions"
          :key="q.id"
          class="breakdown-card"
          :class="
            (answers[q.id] || '').trim().toLowerCase() === (q.correct_answer || '').trim().toLowerCase()
              ? 'breakdown-card--correct'
              : 'breakdown-card--incorrect'
          "
        >
          <div class="breakdown-header">
            <span class="question-num">{{ index + 1 }}</span>
            <span class="breakdown-status">
              {{
                (answers[q.id] || '').trim().toLowerCase() === (q.correct_answer || '').trim().toLowerCase()
                  ? '✓ Correct'
                  : '✗ Incorrect'
              }}
            </span>
          </div>
          <p class="question-prompt">{{ q.prompt }}</p>
          <div class="breakdown-answers">
            <p class="answer-item">
              <span class="answer-label">Your Answer:</span>
              <span
                class="answer-val"
                :class="
                  (answers[q.id] || '').trim().toLowerCase() === (q.correct_answer || '').trim().toLowerCase()
                    ? 'val--correct'
                    : 'val--incorrect'
                "
              >
                {{ answers[q.id] || 'None' }}
              </span>
            </p>
            <p
              v-if="(answers[q.id] || '').trim().toLowerCase() !== (q.correct_answer || '').trim().toLowerCase()"
              class="answer-item"
            >
              <span class="answer-label">Correct Answer:</span>
              <span class="answer-val val--correct">{{ q.correct_answer }}</span>
            </p>
          </div>
        </article>
      </section>

      <div class="result-panel__actions">
        <!-- Retry Sync Button -->
        <button
          v-if="result.passed && result.flashcards_generation_error"
          class="primary-btn retry-btn"
          :disabled="generatingFlashcards"
          @click="retryFlashcardGeneration"
        >
          {{ generatingFlashcards ? 'Retrying...' : 'Retry Generation' }}
        </button>

        <button
          class="primary-btn continue-btn"
          :disabled="generatingFlashcards"
          @click="handleContinue"
        >
          <span v-if="generatingFlashcards">Generating flashcards for spaced repetition...</span>
          <span v-else>Continue</span>
        </button>
      </div>
    </article>

    <!-- Empty state: no questions -->
    <article v-else-if="questions.length === 0 && !generating" class="state-panel">
      <p class="state-text">No quiz questions found for this task.</p>
      <button
        v-if="isMilestoneExam"
        class="primary-btn retry-generation-btn"
        :disabled="submitting"
        @click="submitMilestone"
      >
        {{ submitting ? 'Completing...' : 'Complete Milestone Exam' }}
      </button>
    </article>

    <!-- Quiz form -->
    <form v-else class="quiz-form" @submit.prevent="submitQuiz">
      <article v-for="(q, index) in questions" :key="q.id" class="question-card">
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
import { useRoute, useRouter } from 'vue-router'
import {
  activateTask,
  getTask,
  submitQuizAttempt,
  getNotebooks,
  generateQuizForPageRange,
  generateFlashcardsForQuizTask,
  completeMilestoneExam,
  trackAnalyticsEvent,
  getUserSettings,
} from '../services/appApi'
import StudyPageLayout from '../components/StudyPageLayout.vue'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const submitting = ref(false)
const submitted = ref(false)
const error = ref('')
const taskMeta = ref(null)
const questions = ref([])
const answers = ref({})
const analyticsEnabled = ref(false)
const anonymousUserID = ref('')
const result = ref(null)
const generatingFlashcards = ref(false)

const isMilestoneExam = ref(false)
const milestonePayload = ref(null)

// Manual generation state
const notebooks = ref([])
const selectedNotebookID = ref('')
const startPage = ref(1)
const endPage = ref(10)
const generating = ref(false)
const manualPassingScore = ref(70)

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
  return questions.value.every(
    (q) => typeof answers.value[q.id] === 'string' && answers.value[q.id].trim() !== ''
  )
})

const canGenerateManual = computed(
  () =>
    selectedNotebookID.value &&
    startPage.value > 0 &&
    endPage.value >= startPage.value &&
    !generating.value
)

const canRetryGeneration = computed(() => {
  return (
    !taskID.value &&
    selectedNotebookID.value &&
    startPage.value > 0 &&
    endPage.value >= startPage.value
  )
})

onMounted(async () => {
  await loadNotebooks()
  try {
    const settings = await getUserSettings()
    analyticsEnabled.value = settings?.analytics_enabled ?? false
    anonymousUserID.value = settings?.anonymous_user_id ?? ''
  } catch (err) {
    console.error('Failed to load user settings in Quiz:', err)
  }
  if (taskID.value) {
    await loadQuizTask()
  } else {
    loading.value = false
  }
})

async function loadNotebooks() {
  try {
    const res = await getNotebooks()
    notebooks.value = Array.isArray(res) ? res.filter((n) => !n.error) : []
  } catch {
    error.value = 'Failed to load notebooks.'
  }
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
    if (!task || (task.task_type !== 'QUIZ' && task.task_type !== 'MILESTONE_EXAM')) {
      error.value = 'Task is not a quiz or milestone exam task.'
      return
    }

    taskMeta.value = task
    const payload =
      typeof task.payload_json === 'string' && task.payload_json.trim() !== ''
        ? JSON.parse(task.payload_json)
        : null

    questions.value = Array.isArray(payload?.questions) ? payload.questions : []
    if (task.task_type === 'MILESTONE_EXAM') {
      isMilestoneExam.value = true
      milestonePayload.value = payload
    } else {
      isMilestoneExam.value = false
      milestonePayload.value = null
    }
    answers.value = {}
    submitted.value = false
    result.value = null
  } catch (err) {
    error.value = err?.message || 'Failed to load quiz task.'
  } finally {
    loading.value = false
  }
}

async function submitMilestone() {
  submitting.value = true
  error.value = ''
  try {
    const res = await completeMilestoneExam(taskID.value)
    if (res && res.error) {
      error.value = res.error
    } else {
      router.push('/dashboard')
    }
  } catch (err) {
    error.value = err?.message || 'Failed to complete milestone exam.'
  } finally {
    submitting.value = false
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
    const res = await generateQuizForPageRange(
      selectedNotebookID.value,
      startPage.value,
      endPage.value
    )
    if (res.error) {
      error.value = res.error
      return
    }
    questions.value = Array.isArray(res.questions) ? res.questions : []
    // Respect backend-supplied passing score when available; default to 70
    if (
      typeof res.passing_score === 'number' &&
      !isNaN(res.passing_score) &&
      res.passing_score > 0
    ) {
      manualPassingScore.value = Math.round(res.passing_score)
    } else {
      manualPassingScore.value = 70
    }
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

  if (!taskID.value) {
    // Stateless frontend scoring for manual quizzes (so they do not touch the study queue)
    try {
      let correctCount = 0
      questions.value.forEach((q) => {
        if (
          typeof answers.value[q.id] === 'string' &&
          typeof q.correct_answer === 'string' &&
          answers.value[q.id].trim().toLowerCase() === q.correct_answer.trim().toLowerCase()
        ) {
          correctCount++
        }
      })
      const score = Math.round((correctCount / questions.value.length) * 100)
      result.value = {
        score,
        passed: score >= manualPassingScore.value,
        passing_score: manualPassingScore.value,
        correct_count: correctCount,
        total_count: questions.value.length,
        feedback:
          score >= manualPassingScore.value
            ? 'Strong work. You can move forward.'
            : 'Review the missed concepts and retry the material.',
      }
      submitted.value = true
    } catch (err) {
      error.value = err?.message || 'Failed to grade manual quiz.'
    } finally {
      submitting.value = false
    }
    return
  }

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

    if (analyticsEnabled.value) {
      const notebook = notebooks.value.find((n) => n.id === taskMeta.value?.notebook_id)
      const fileHash = notebook?.file_hash || ''
      const currentQuizResult = response?.result
      trackAnalyticsEvent('quiz_complete', fileHash, taskMeta.value?.start_page || 0, {
        task_id: taskID.value,
        score: currentQuizResult?.score || 0,
        passed: currentQuizResult?.passed || false,
        correct_count: currentQuizResult?.correct_count || 0,
        total_count: currentQuizResult?.total_count || 0,
        anonymous_user_id: anonymousUserID.value,
      })
    }
  } catch (err) {
    error.value = err?.message || 'Failed to submit quiz.'
  } finally {
    submitting.value = false
  }
}

async function retryFlashcardGeneration() {
  if (result.value) {
    result.value.flashcards_generation_error = false
    result.value.flashcards_generation_message = ''
    result.value.flashcards_pending = true
    await handleContinue()
  }
}

async function handleContinue() {
  // If quiz passed with flashcards pending, generate flashcards first
  if (result.value?.passed && result.value?.flashcards_pending) {
    generatingFlashcards.value = true
    error.value = ''
    result.value.flashcards_generation_error = false
    result.value.flashcards_generation_message = ''
    try {
      const genResult = await generateFlashcardsForQuizTask(result.value.task_id)
      if (genResult?.error) {
        console.warn('[FLASHCARD_PIPELINE] Flashcard generation failed:', genResult.error)
        result.value.flashcards_generation_error = true
        result.value.flashcards_generation_message = genResult.error
        result.value.flashcards_pending = false
        return
      } else {
        // Update result with generated counts for dashboard banner
        result.value.flashcards_generated = genResult?.cards_scheduled || 0
        result.value.flashcards_pending = false
      }
    } catch (err) {
      console.warn('[FLASHCARD_PIPELINE] Flashcard generation error:', err)
      result.value.flashcards_generation_error = true
      result.value.flashcards_generation_message = err?.message || 'Flashcard generation failed.'
      result.value.flashcards_pending = false
      return
    } finally {
      generatingFlashcards.value = false
    }
  }

  // Route to dashboard after quiz completion
  const query = {}
  if (result.value?.passed && result.value?.flashcards_generated > 0) {
    query.flashcardsCreated = result.value.flashcards_generated
  } else if (!result.value?.passed && result.value?.reread_task_id) {
    query.highlight = result.value.reread_task_id
  }
  router.push({ path: '/dashboard', query })
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

/* Flashcard success message styles */
.result-panel__flashcard-success {
  background: color-mix(in srgb, #16a34a 8%, var(--surface-container-low));
  border: 1px solid color-mix(in srgb, #16a34a 20%, transparent);
  border-radius: 12px;
  padding: 16px 20px;
  margin: 8px 0;
}

.flashcard-success-title {
  margin: 0 0 6px;
  font-size: 15px;
  font-weight: 600;
  color: #16a34a;
}

.flashcard-success-detail {
  margin: 0 0 4px;
  font-size: 14px;
  color: var(--on-surface);
}

.flashcard-success-hint {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
  font-style: italic;
}

.result-panel__reread-message {
  margin: 8px 0 0;
  font-size: 14px;
  color: var(--on-surface);
  font-weight: 500;
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
  align-items: center;
  gap: 12px;
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
  transition:
    transform 0.14s ease,
    filter 0.14s ease;
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

.result-panel__network-warning {
  display: flex;
  gap: 12px;
  background: rgba(230, 126, 34, 0.1);
  border: 1px solid rgba(230, 126, 34, 0.2);
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 8px;
  width: 100%;
}

.warning-icon {
  font-size: 20px;
  color: #d35400;
  flex-shrink: 0;
}

.warning-title {
  margin: 0 0 4px;
  font-size: 14px;
  font-weight: 700;
  color: #d35400;
}

.warning-detail,
.warning-message {
  margin: 0;
  font-size: 13px;
  line-height: 1.4;
  color: var(--on-surface);
}

.warning-message {
  margin-top: 6px;
  font-family: monospace;
  font-size: 11.5px;
  background: var(--surface-container-low);
  padding: 4px 8px;
  border-radius: 6px;
}

.result-panel__external-help-notice {
  background: rgba(192, 41, 43, 0.1);
  border: 1px solid rgba(192, 41, 43, 0.2);
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 8px;
  width: 100%;
}

.external-help-title {
  margin: 0 0 4px;
  font-size: 14px;
  font-weight: 700;
  color: #c0392b;
}

.external-help-desc {
  margin: 0;
  font-size: 13px;
  line-height: 1.4;
  color: var(--on-surface);
}

.retry-btn {
  background: var(--surface-container-low);
  color: var(--on-surface);
  border: 1px solid var(--outline-variant);
  margin-right: 12px;
}

.retry-btn:hover {
  background: var(--outline-variant);
}

.retry-generation-btn {
  margin-top: 12px;
  min-width: 120px;
}

/* ── Milestone Exam ───────────────────────────── */
.milestone-panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
  background: var(--surface-container-low);
  border: 1px solid var(--outline-variant);
  border-radius: 16px;
  padding: 24px;
}

.milestone-header {
  border-bottom: 1px solid var(--outline-variant);
  padding-bottom: 16px;
}

.milestone-title {
  margin: 0 0 8px 0;
  font-size: 18px;
  font-weight: 700;
  color: var(--on-surface);
}

.milestone-desc {
  margin: 0;
  font-size: 14px;
  color: var(--muted-text);
  line-height: 1.5;
}

.milestone-attempts-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.milestone-attempt-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--surface-container-high);
  border-radius: 12px;
  padding: 14px 18px;
  border: 1px solid var(--outline-variant);
}

.attempt-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.attempt-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--on-surface);
}

.attempt-score {
  font-size: 12px;
  color: var(--muted-text);
}

.attempt-flags {
  display: flex;
  gap: 6px;
}

.flag-dot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  font-size: 11px;
  font-weight: 800;
}

.flag-dot--correct {
  background: rgba(46, 204, 113, 0.15);
  color: #2ecc71;
  border: 1px solid rgba(46, 204, 113, 0.3);
}

.flag-dot--incorrect {
  background: rgba(231, 76, 60, 0.15);
  color: #e74c3c;
  border: 1px solid rgba(231, 76, 60, 0.3);
}

.milestone-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

/* ── Result Breakdown ─────────────────────────── */
.result-breakdown {
  display: grid;
  gap: 16px;
  margin-top: 24px;
  border-top: 1px solid var(--outline-variant);
  padding-top: 20px;
  text-align: left;
}

.breakdown-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--on-surface);
}

.breakdown-card {
  background: var(--surface-container-high);
  border-radius: 12px;
  padding: 16px;
  border: 1px solid var(--outline-variant);
  display: grid;
  gap: 10px;
}

.breakdown-card--correct {
  border-left: 4px solid #2ecc71;
}

.breakdown-card--incorrect {
  border-left: 4px solid #e74c3c;
}

.breakdown-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.breakdown-status {
  font-size: 12px;
  font-weight: 700;
}

.breakdown-card--correct .breakdown-status {
  color: #2ecc71;
}

.breakdown-card--incorrect .breakdown-status {
  color: #e74c3c;
}

.breakdown-answers {
  display: grid;
  gap: 6px;
  font-size: 13px;
}

.answer-item {
  margin: 0;
  display: flex;
  gap: 8px;
  align-items: baseline;
}

.answer-label {
  font-weight: 600;
  color: var(--muted-text);
}

.answer-val {
  font-weight: 500;
}

.val--correct {
  color: #2ecc71;
}

.val--incorrect {
  color: #e74c3c;
}
</style>
