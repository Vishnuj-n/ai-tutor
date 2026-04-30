<template>
  <div class="quiz-page">
    <header class="page-header">
      <h1 class="page-title">Quiz</h1>
      <select v-if="!isContextLocked" v-model="selectedNotebookID" class="notebook-select" :disabled="loading">
        <option value="">— Select Notebook —</option>
        <option v-for="nb in notebooks" :key="nb.id" :value="nb.id">{{ nb.title }}</option>
      </select>
    </header>

    <nav v-if="!isContextLocked" class="tabs">
      <button :class="['tab-btn', { active: activeTab === 'comprehensive' }]" :disabled="loading" @click="activeTab = 'comprehensive'">Comprehensive Extraction</button>
      <button :class="['tab-btn', { active: activeTab === 'explorer' }]" :disabled="loading" @click="activeTab = 'explorer'">Semantic Discovery</button>
    </nav>

    <section v-if="activeTab === 'comprehensive'" class="content">
      <div v-if="!isContextLocked && !questions.length" class="controls">
        <div class="input-group">
          <label>Start Page</label>
          <input v-model.number="startPage" type="number" min="1" :disabled="loading" />
        </div>
        <div class="input-group">
          <label>End Page</label>
          <input v-model.number="endPage" type="number" min="1" :disabled="loading" />
        </div>
        <BaseButton :disabled="!canGenerate" :loading="loading" @click="generate">Generate Quiz</BaseButton>
      </div>
      <ErrorMessage :message="error" />

      <div v-if="questions.length && !sessionComplete" class="questions">
        <div v-for="(q, qi) in questions" :key="q.id" class="question" :class="{ answered: answers[q.id] !== undefined }">
          <p class="prompt"><span class="num">{{ qi + 1 }}.</span> {{ q.prompt }}</p>
          <ul class="options">
            <li v-for="opt in q.options" :key="opt" class="option-item">
              <button
                type="button"
                :class="['option', optionClass(q, opt)]"
                @click="submitAnswer(q, opt)">
                {{ opt }}
              </button>
            </li>
          </ul>
          <div v-if="answers[q.id]" class="result" :class="answers[q.id].correct ? 'correct' : 'wrong'">
            <span class="label">{{ answers[q.id].correct ? '✓ Correct' : '✗ Incorrect' }}</span>
            <p v-if="answers[q.id].feedback" class="feedback">{{ answers[q.id].feedback }}</p>
          </div>
        </div>
        <div class="score">Score: {{ correctCount }} / {{ answeredCount }}</div>
      </div>

      <div v-if="sessionComplete" class="session-complete">
        <h2>Session Complete</h2>
        <p>Final Score: {{ correctCount }} / {{ questions.length }}</p>
        <BaseButton @click="returnToDashboard">Return to Dashboard</BaseButton>
      </div>
    </section>

    <section v-else class="content stub">
      <p>Semantic Discovery — coming in Phase 2.</p>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getDailyAgenda, getNotebooks, generateMarathonQuiz, generateTopicQuiz, scoreAnswer } from '../services/appApi.js'
import BaseButton from '../components/BaseButton.vue'
import ErrorMessage from '../components/ErrorMessage.vue'

const route = useRoute()
const router = useRouter()

// Phase 3: Context-locked mode
const isContextLocked = computed(() => Boolean(route.query.topicId))

const notebooks     = ref([])
const selectedNotebookID = ref('')
const activeTab     = ref('comprehensive')
const startPage     = ref(1)
const endPage       = ref(10)
const loading       = ref(false)
const error         = ref('')
const questions     = ref([])
const answers       = ref({})
const pendingAnswers = new Set()
const sessionComplete = ref(false)
const navigatingToDashboard = ref(false)

const canGenerate = computed(() =>
  selectedNotebookID.value && startPage.value > 0 && endPage.value >= startPage.value && !loading.value
)

const answeredCount = computed(() => Object.keys(answers.value).length)
const correctCount  = computed(() => Object.values(answers.value).filter(a => a.correct).length)

onMounted(async () => {
  if (isContextLocked.value) {
    // Auto-generate quiz from route context
    await loadContextLockedQuiz()
  } else {
    try {
      const res = await getNotebooks()
      notebooks.value = Array.isArray(res) ? res.filter(n => !n.error) : []
    } catch (e) {
      error.value = 'Failed to load notebooks.'
    }
  }
})

async function loadContextLockedQuiz() {
  const topicId = route.query.topicId
  const start = Number(route.query.startPage) || 1
  const end = Number(route.query.endPage) || 10
  
  if (!topicId) {
    error.value = 'Missing topic ID in route'
    return
  }

  loading.value = true
  error.value = ''
  try {
    const res = await generateTopicQuiz(topicId, start, end)
    if (res.error) {
      error.value = res.error
      return
    }
    questions.value = res.questions ?? []
  } catch (e) {
    error.value = e?.message ?? 'Quiz generation failed.'
  } finally {
    loading.value = false
  }
}

async function generate() {
  error.value = ''
  questions.value = []
  answers.value = {}
  sessionComplete.value = false
  loading.value = true
  try {
    const res = await generateMarathonQuiz(selectedNotebookID.value, startPage.value, endPage.value)
    if (res.error) { error.value = res.error; return }
    questions.value = res.questions ?? []
  } catch (e) {
    error.value = e?.message ?? 'Quiz generation failed.'
  } finally {
    loading.value = false
  }
}

async function submitAnswer(q, opt) {
  if (answers.value[q.id] !== undefined) return
  if (pendingAnswers.has(q.id)) return
  
  pendingAnswers.add(q.id)
  try {
    const res = await scoreAnswer(q.id, opt)
    if (res.error) {
      // Don't mark as answered if there's an error, allow retry
      return
    }
    answers.value[q.id] = { selected: opt, correct: res.correct, feedback: res.feedback }
    
    // Check if session is complete (all questions answered)
    if (answeredCount.value === questions.value.length) {
      sessionComplete.value = true
      // Phase 3: Refresh agenda and auto-return to dashboard in context-locked mode
      if (isContextLocked.value) {
        setTimeout(async () => {
          await getDailyAgenda() // Refresh agenda
          returnToDashboard()
        }, 1500)
      }
    }
  } catch (e) {
    // Don't mark as answered on network error, allow retry
  } finally {
    pendingAnswers.delete(q.id)
  }
}

function returnToDashboard() {
  navigatingToDashboard.value = true
  router.push('/dashboard')
}

function optionClass(q, opt) {
  const a = answers.value[q.id]
  if (!a) return ''
  if (opt === q.correct_answer) return 'correct-opt'
  return a.selected === opt && !a.correct ? 'wrong-opt' : ''
}
</script>

<style scoped>
.quiz-page {
  padding: 1.5rem;
  max-width: 1000px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  align-items: center;
  gap: 1.5rem;
  margin-bottom: 1rem;
}

.page-title {
  font-size: 1.75rem;
  font-weight: 700;
  margin: 0;
  color: var(--on-surface);
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

.tabs {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
  border-bottom: 1px solid #e0e0e0;
  padding-bottom: 0.5rem;
}

.tab-btn {
  padding: 0.5rem 1rem;
  border: none;
  background: transparent;
  color: var(--on-surface);
  cursor: pointer;
  font-size: 0.9rem;
  border-radius: 4px;
}

.tab-btn:hover {
  background: #f5f5f5;
}

.tab-btn.active {
  background: var(--primary);
  color: white;
  font-weight: 600;
}

.tab-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.content {
  animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.controls {
  display: flex;
  gap: 1rem;
  align-items: flex-end;
  padding: 1rem;
  background: #f5f5f5;
  border-radius: 4px;
  margin-bottom: 1rem;
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

.questions {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.question {
  padding: 1rem;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
  background: white;
}

.question.answered {
  background: #fafafa;
}

.prompt {
  font-size: 1rem;
  font-weight: 600;
  margin: 0 0 0.75rem 0;
  color: var(--on-surface);
}

.num {
  color: var(--primary);
  font-weight: 700;
}

.options {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.option-item {
  padding: 0;
  margin: 0;
}

.option {
  width: 100%;
  padding: 0.6rem 0.75rem;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  background: white;
  transition: background 0.1s;
  text-align: left;
}

.option:hover:not(.correct-opt):not(.wrong-opt) {
  background: #f5f5f5;
  border-color: var(--primary);
}

.correct-opt {
  background: #dcfce7;
  border-color: #22c55e;
  color: #16a34a;
  font-weight: 600;
}

.wrong-opt {
  background: #fee2e2;
  border-color: #ef4444;
  color: #dc2626;
}

.result {
  margin-top: 0.75rem;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
}

.result.correct {
  background: #dcfce7;
}

.result.wrong {
  background: #fee2e2;
}

.label {
  font-weight: 700;
  font-size: 0.85rem;
  display: block;
  margin-bottom: 0.25rem;
}

.result.correct .label {
  color: #16a34a;
}

.result.wrong .label {
  color: #dc2626;
}

.feedback {
  font-size: 0.85rem;
  color: #666;
  margin: 0;
}

.score {
  text-align: center;
  padding: 0.75rem;
  background: #f5f5f5;
  border-radius: 4px;
  font-weight: 600;
  font-size: 0.95rem;
  margin-top: 0.5rem;
}

.session-complete {
  text-align: center;
  padding: 3rem;
  background: #f5f5f5;
  border-radius: 8px;
}

.session-complete h2 {
  font-size: 1.5rem;
  font-weight: 700;
  margin: 0 0 0.5rem 0;
  color: var(--on-surface);
}

.session-complete p {
  font-size: 0.95rem;
  color: #666;
  margin: 0 0 1rem 0;
}

.stub {
  text-align: center;
  padding: 3rem;
  color: #666;
}
</style>
