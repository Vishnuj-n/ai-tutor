<template>
  <div class="command-center">
    <header class="cc-header">
      <h1 class="cc-title">Quiz</h1>
      <select id="notebook-select" v-model="selectedNotebookID" class="notebook-select" :disabled="loading">
        <option value="">— Select Notebook —</option>
        <option v-for="nb in notebooks" :key="nb.id" :value="nb.id">{{ nb.title }}</option>
      </select>
    </header>

    <div class="cc-tabs">
      <button id="tab-comprehensive" :class="['tab-btn', { active: activeTab === 'comprehensive' }]" @click="activeTab = 'comprehensive'">Comprehensive Extraction</button>
      <button id="tab-explorer" :class="['tab-btn', { active: activeTab === 'explorer' }]" @click="activeTab = 'explorer'">Key Concept Extraction</button>
    </div>

    <!-- Comprehensive Extraction -->
    <section v-if="activeTab === 'comprehensive'" class="tab-panel">
      <div class="range-row">
        <label class="range-label">Start Page</label>
        <input id="start-page" v-model.number="startPage" type="number" min="1" class="page-input" :disabled="loading" />
        <label class="range-label">End Page</label>
        <input id="end-page" v-model.number="endPage" type="number" min="1" class="page-input" :disabled="loading" />
        <button id="btn-generate" class="generate-btn" :disabled="!canGenerate" @click="generate">
          <span v-if="!loading">Generate Quiz →</span>
          <span v-else class="spinner" />
        </button>
      </div>
      <p v-if="error" class="error-msg">{{ error }}</p>


      <!-- Questions -->
      <div v-if="questions.length" class="questions-list">
        <div v-for="(q, qi) in questions" :key="q.id" class="question-card" :class="{ answered: answers[q.id] !== undefined }">
          <p class="q-prompt"><span class="q-num">{{ qi + 1 }}.</span> {{ q.prompt }}</p>
          <ul class="options">
            <li v-for="(opt, oi) in q.options" :key="oi"
              :class="['option', optionClass(q, opt)]"
              tabindex="0"
              role="button"
              @click="submitAnswer(q, opt)"
              @keydown.enter.prevent="submitAnswer(q, opt)"
              @keydown.space.prevent="submitAnswer(q, opt)">
              {{ opt }}
            </li>
          </ul>
          <div v-if="answers[q.id]" class="answer-result" :class="answers[q.id].correct ? 'correct' : 'wrong'">
            <p class="result-label">{{ answers[q.id].correct ? '✓ Correct' : '✗ Incorrect' }}</p>
            <p v-if="answers[q.id].feedback" class="result-feedback">{{ answers[q.id].feedback }}</p>
          </div>
        </div>
        <div class="score-bar">
          Score: {{ correctCount }} / {{ answeredCount }} answered
        </div>
      </div>
    </section>

    <!-- Explorer Mode stub -->
    <section v-else class="tab-panel explorer-stub">
      <div class="stub-icon">🔭</div>
      <h2 class="stub-title">Explorer Mode</h2>
      <p class="stub-desc">Search by topic or concept — coming in Phase 2.</p>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getNotebooks, generateMarathonQuiz, scoreAnswer } from '../services/appApi.js'

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

const canGenerate = computed(() =>
  selectedNotebookID.value && startPage.value > 0 && endPage.value >= startPage.value && !loading.value
)

const answeredCount = computed(() => Object.keys(answers.value).length)
const correctCount  = computed(() => Object.values(answers.value).filter(a => a.correct).length)

onMounted(async () => {
  try {
    const res = await getNotebooks()
    notebooks.value = Array.isArray(res) ? res.filter(n => !n.error) : []
  } catch (e) {
    error.value = 'Failed to load notebooks.'
  }
})

async function generate() {
  error.value = ''
  questions.value = []
  answers.value = {}
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
    answers.value[q.id] = { selected: opt, correct: res.correct, feedback: res.feedback }
  } catch (e) {
    answers.value[q.id] = { selected: opt, correct: false, feedback: 'Scoring failed.' }
  } finally {
    pendingAnswers.delete(q.id)
  }
}

function optionClass(q, opt) {
  const a = answers.value[q.id]
  if (!a) return ''
  if (opt === q.correct_answer) return 'correct-opt'
  return a.selected === opt && !a.correct ? 'wrong-opt' : ''
}
</script>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Manrope:wght@400;500;600;700&family=Inter:wght@400;500;600;700&display=swap');

.command-center {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  padding: 3rem 2rem;
  max-width: 720px;
  margin: 0 auto;
  background: var(--background);
  min-height: 100vh;
}

.cc-header {
  display: flex;
  align-items: flex-end;
  gap: 2rem;
  margin-bottom: 1rem;
}

.cc-title {
  font-family: 'Manrope', sans-serif;
  font-size: 2.5rem;
  font-weight: 700;
  color: var(--on-surface);
  margin: 0;
  letter-spacing: -0.5%;
  line-height: 1.1;
}

.notebook-select {
  flex: 1;
  min-width: 280px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  border: none;
  border-radius: 0.75rem;
  padding: 0.75rem 1rem;
  font-family: 'Inter', sans-serif;
  font-size: 0.95rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.notebook-select:hover {
  background: var(--surface-container-lowest);
}

.notebook-select:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.cc-tabs {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 2rem;
}

.tab-btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 0.75rem;
  background: var(--surface-container);
  color: var(--on-surface);
  cursor: pointer;
  font-family: 'Inter', sans-serif;
  font-size: 0.9rem;
  font-weight: 500;
  transition: all 0.2s ease;
}

.tab-btn:hover {
  background: var(--surface-container-low);
}

.tab-btn.active {
  background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dim) 100%);
  color: var(--on-primary);
  font-weight: 600;
}

.tab-panel {
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

.range-row {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
  margin-bottom: 2rem;
  padding: 1.5rem;
  background: var(--surface-container);
  border-radius: 0.75rem;
}

.range-label {
  font-family: 'Inter', sans-serif;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--muted-text);
  white-space: nowrap;
}

.page-input {
  width: 80px;
  background: var(--surface-container-lowest);
  color: var(--on-surface);
  border: 1px solid rgba(45, 51, 56, 0.2);
  border-radius: 0.5rem;
  padding: 0.5rem 0.75rem;
  font-family: 'Inter', sans-serif;
  font-size: 0.9rem;
  font-weight: 500;
  text-align: center;
  transition: all 0.2s ease;
}

.page-input:focus {
  outline: none;
  border-color: var(--primary);
}

.page-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.generate-btn {
  margin-left: auto;
  padding: 0.75rem 1.75rem;
  background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dim) 100%);
  color: var(--on-primary);
  border: none;
  border-radius: 0.75rem;
  font-family: 'Inter', sans-serif;
  font-size: 0.95rem;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  transition: all 0.2s ease;
  box-shadow: 0 4px 12px rgba(0, 91, 193, 0.15);
}

.generate-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(0, 91, 193, 0.25);
}

.generate-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  transform: none;
  box-shadow: none;
}

.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: var(--on-primary);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.error-msg {
  font-family: 'Inter', sans-serif;
  font-size: 0.9rem;
  font-weight: 500;
  color: #9f403d;
  padding: 1rem;
  background: rgba(159, 64, 61, 0.08);
  border-radius: 0.5rem;
  margin-top: 1rem;
}

.questions-list {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.question-card {
  background: var(--surface-container-low);
  border-radius: 0.75rem;
  padding: 2rem;
  transition: all 0.2s ease;
}

.question-card:hover {
  background: var(--surface-container-lowest);
  box-shadow: 0 20px 40px rgba(45, 51, 56, 0.06);
}

.question-card.answered {
  background: var(--surface-container);
}

.q-prompt {
  font-family: 'Inter', sans-serif;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--on-surface);
  margin-bottom: 1.5rem;
  line-height: 1.5;
}

.q-num {
  color: var(--primary);
  margin-right: 0.5rem;
  font-weight: 700;
}

.options {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.option {
  padding: 1rem 1.25rem;
  border: 1px solid var(--outline-variant);
  border-radius: 0.5rem;
  cursor: pointer;
  font-family: 'Inter', sans-serif;
  font-size: 0.95rem;
  font-weight: 500;
  color: var(--on-surface);
  background: var(--surface-container-lowest);
  transition: all 0.15s ease;
}

.option:hover:not(.correct-opt):not(.wrong-opt) {
  background: var(--surface-container-low);
  border-color: var(--primary);
}

.correct-opt {
  background: rgba(34, 197, 94, 0.1);
  border-color: #22c55e;
  color: #16a34a;
  font-weight: 600;
}

.wrong-opt {
  background: rgba(239, 68, 68, 0.1);
  border-color: #ef4444;
  color: #dc2626;
}

.answer-result {
  margin-top: 1.5rem;
  padding: 1rem 1.25rem;
  border-radius: 0.5rem;
}

.answer-result.correct {
  background: rgba(34, 197, 94, 0.08);
}

.answer-result.wrong {
  background: rgba(239, 68, 68, 0.08);
}

.result-label {
  font-family: 'Inter', sans-serif;
  font-weight: 700;
  font-size: 0.9rem;
  margin-bottom: 0.5rem;
}

.answer-result.correct .result-label {
  color: #16a34a;
}

.answer-result.wrong .result-label {
  color: #dc2626;
}

.result-feedback {
  font-family: 'Inter', sans-serif;
  font-size: 0.875rem;
  color: var(--muted-text);
  line-height: 1.4;
}

.score-bar {
  text-align: center;
  padding: 1.5rem;
  background: var(--surface-container);
  border-radius: 0.75rem;
  font-family: 'Inter', sans-serif;
  font-size: 1rem;
  font-weight: 600;
  color: var(--on-surface);
  margin-top: 1rem;
}


.explorer-stub {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  padding: 6rem 2rem;
  color: var(--muted-text);
  text-align: center;
}

.stub-icon {
  font-size: 4rem;
  opacity: 0.6;
}

.stub-title {
  font-family: 'Manrope', sans-serif;
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--on-surface);
  margin: 0;
}

.stub-desc {
  font-family: 'Inter', sans-serif;
  font-size: 0.95rem;
  line-height: 1.5;
  max-width: 400px;
}
</style>
