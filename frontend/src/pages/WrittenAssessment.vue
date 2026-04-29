<template>
  <div class="command-center">
    <header class="cc-header">
      <h1 class="cc-title">Written Assessment</h1>
      <select id="notebook-select" v-model="selectedNotebookID" class="notebook-select" :disabled="loading">
        <option value="">— Select Notebook —</option>
        <option v-for="nb in notebooks" :key="nb.id" :value="nb.id">{{ nb.title }}</option>
      </select>
    </header>

    <div class="cc-tabs">
      <button id="tab-comprehensive" :class="['tab-btn', { active: activeTab === 'comprehensive' }]" @click="activeTab = 'comprehensive'">Comprehensive Extraction</button>
      <button id="tab-explorer" :class="['tab-btn', { active: activeTab === 'explorer' }]" @click="activeTab = 'explorer'">Semantic Discovery</button>
    </div>

    <!-- Comprehensive Extraction -->
    <section v-if="activeTab === 'comprehensive'" class="tab-panel">
      <div v-if="!question">
        <div class="range-row">
          <label class="range-label">Start Page</label>
          <input id="start-page" v-model.number="startPage" type="number" min="1" class="page-input" :disabled="loading" />
          <label class="range-label">End Page</label>
          <input id="end-page" v-model.number="endPage" type="number" min="1" class="page-input" :disabled="loading" />
          <button id="btn-generate" class="generate-btn" :disabled="!canGenerate" @click="generate">
            <span v-if="!loading">Generate Question →</span>
            <span v-else class="spinner" />
          </button>
        </div>
        <p v-if="error" class="error-msg">{{ error }}</p>
      </div>


      <!-- Exam prompt -->
      <div v-if="question && !result" class="exam-panel">
        <div class="question-box">
          <p class="exam-prompt">{{ question.prompt }}</p>
          <span class="page-badge">Pages {{ question.sourcePageStart }}–{{ question.sourcePageEnd }}</span>
        </div>
        <textarea id="answer-input" v-model="userAnswer" rows="6" class="answer-textarea"
          placeholder="Write your answer here…" :disabled="scoring" />
        <div class="exam-actions">
          <button id="btn-submit" class="generate-btn" :disabled="!userAnswer.trim() || scoring" @click="submitAnswer">
            <span v-if="!scoring">Submit Answer →</span>
            <span v-else class="spinner" />
          </button>
          <button id="btn-discard" class="discard-btn" :disabled="scoring" @click="reset">Discard</button>
        </div>
      </div>

      <!-- Result -->
      <div v-if="result" class="result-panel">
        <div class="score-ring" :class="scoreClass">
          <span class="score-num">{{ result.score }}</span>
          <span class="score-denom">/10</span>
        </div>
        <div class="result-body">
          <p class="result-feedback">{{ result.feedback }}</p>
          <div class="fsrs-chip" v-if="result.fsrsRating">
            <span class="fsrs-label">{{ result.fsrsRating }}</span>
            <span class="fsrs-days">Next: {{ result.scheduledDays }}d</span>
          </div>
        </div>
        <button id="btn-new" class="generate-btn" @click="reset">New Question</button>
      </div>
    </section>

    <!-- Explorer stub -->
    <section v-else class="tab-panel explorer-stub">
      <div class="stub-icon">🔭</div>
      <h2 class="stub-title">Semantic Discovery</h2>
      <p class="stub-desc">Search by topic or concept — coming in Phase 2.</p>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getNotebooks, generateComprehensiveExam, scoreShortAnswer } from '../services/appApi.js'

const notebooks          = ref([])
const selectedNotebookID = ref('')
const activeTab          = ref('comprehensive')
const startPage          = ref(1)
const endPage            = ref(10)
const loading            = ref(false)
const scoring            = ref(false)
const error              = ref('')
const question           = ref(null)
const userAnswer         = ref('')
const result             = ref(null)

const canGenerate = computed(() =>
  selectedNotebookID.value && startPage.value > 0 && endPage.value >= startPage.value && !loading.value
)
const scoreClass = computed(() => {
  const s = result.value?.score ?? 0
  if (s >= 8) return 'great'
  if (s >= 5) return 'ok'
  return 'low'
})

onMounted(async () => {
  try {
    const res = await getNotebooks()
    notebooks.value = Array.isArray(res) ? res.filter(n => !n.error) : []
  } catch { error.value = 'Failed to load notebooks.' }
})

async function generate() {
  error.value = ''
  question.value = null
  result.value = null
  userAnswer.value = ''
  loading.value = true
  try {
    const res = await generateComprehensiveExam(selectedNotebookID.value, startPage.value, endPage.value)
    if (res.error) { error.value = res.error; return }
    // Normalize API response to camelCase
    question.value = {
      questionId: res.questionID,
      prompt: res.prompt,
      topicId: res.topicID,
      notebookId: res.notebook_id,
      startPage: res.start_page,
      endPage: res.end_page,
      llmTier: res.llm_tier,
      sourcePageStart: res.source_page_start,
      sourcePageEnd: res.source_page_end
    }
  } catch (e) {
    error.value = e?.message ?? 'Exam generation failed.'
  } finally {
    loading.value = false
  }
}

async function submitAnswer() {
  if (!question.value || !userAnswer.value.trim()) return
  if (!question.value.questionId) {
    error.value = 'Invalid question: missing question ID'
    return
  }
  scoring.value = true
  try {
    const res = await scoreShortAnswer(question.value.questionId, userAnswer.value.trim())
    if (res.error) { error.value = res.error; return }
    // Normalize API response to camelCase
    result.value = {
      questionId: res.question_id,
      prompt: res.prompt,
      score: res.score,
      feedback: res.feedback,
      fsrsRating: res.fsrsRating,
      scheduledDays: res.scheduled_days
    }
  } catch (e) {
    error.value = e?.message ?? 'Scoring failed.'
  } finally {
    scoring.value = false
  }
}

function reset() {
  question.value = null
  result.value = null
  userAnswer.value = ''
  error.value = ''
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
  background: var(--surface-container-low);
  border-radius: 0.75rem;
  transition: all 0.3s ease;
}

.range-row:hover {
  background: var(--surface-container-lowest);
  transform: translateY(-2px);
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
  background: var(--surface-container-lowest);
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
  transform: translateY(-2px);
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

.exam-panel  { 
  display: flex; 
  flex-direction: column; 
  gap: 1rem; 
}

.question-box { 
  background: var(--surface-container-low); 
  border-radius: 0.75rem; 
  padding: 1.5rem; 
  transition: all 0.3s ease;
}

.question-box:hover {
  background: var(--surface-container-lowest);
  box-shadow: 0 20px 40px rgba(45, 51, 56, 0.06);
  transform: translateY(-2px);
}

.exam-prompt  { 
  font-family: 'Inter', sans-serif;
  font-size: 1.125rem; 
  font-weight: 600; 
  color: var(--on-surface); 
  line-height: 1.5; 
  margin-bottom: 0.75rem; 
}

.page-badge   { 
  font-family: 'Inter', sans-serif;
  font-size: 0.75rem; 
  color: var(--muted-text); 
  background: var(--surface-container-highest); 
  border-radius: 0.5rem; 
  padding: 0.25rem 0.75rem; 
  font-weight: 500;
}

.answer-textarea { 
  width: 100%; 
  background: var(--surface-container-lowest); 
  color: var(--on-surface); 
  border: 1px solid rgba(45, 51, 56, 0.2); 
  border-radius: 0.75rem; 
  padding: 1rem; 
  font-family: 'Inter', sans-serif;
  font-size: 0.95rem; 
  resize: vertical; 
  line-height: 1.6; 
  box-sizing: border-box; 
  transition: all 0.2s ease;
}

.answer-textarea:focus { 
  outline: none; 
  border-color: var(--primary); 
  background: var(--surface-container-lowest);
}

.answer-textarea:disabled { 
  opacity: 0.5; 
}

.exam-actions { 
  display: flex; 
  gap: 0.75rem; 
  align-items: center; 
}

.discard-btn  { 
  padding: 0.75rem 1.5rem; 
  background: transparent; 
  border: 1px solid rgba(45, 51, 56, 0.2); 
  color: var(--on-surface); 
  border-radius: 0.75rem; 
  cursor: pointer; 
  font-family: 'Inter', sans-serif;
  font-size: 0.9rem; 
  font-weight: 500;
  transition: all 0.15s ease; 
}

.discard-btn:hover { 
  border-color: #ef4444; 
  color: #dc2626; 
  transform: translateY(-2px);
}

.result-panel { 
  display: flex; 
  flex-direction: column; 
  align-items: center; 
  gap: 1.5rem; 
  padding: 2.5rem; 
  background: var(--surface-container-low); 
  border-radius: 0.75rem; 
  transition: all 0.3s ease;
}

.result-panel:hover {
  background: var(--surface-container-lowest);
  box-shadow: 0 20px 40px rgba(45, 51, 56, 0.06);
  transform: translateY(-2px);
}

.score-ring  { 
  display: flex; 
  align-items: baseline; 
  gap: 0.25rem; 
}

.score-num   { 
  font-family: 'Manrope', sans-serif;
  font-size: 4rem; 
  font-weight: 800; 
  line-height: 1; 
}

.score-denom { 
  font-family: 'Inter', sans-serif;
  font-size: 1.25rem; 
  color: var(--muted-text); 
  font-weight: 500;
}

.score-ring.great .score-num { 
  color: #16a34a; 
}

.score-ring.ok    .score-num { 
  color: #ea580c; 
}

.score-ring.low   .score-num { 
  color: #dc2626; 
}

.result-body { 
  text-align: center; 
  display: flex; 
  flex-direction: column; 
  gap: 0.75rem; 
}

.result-feedback { 
  font-family: 'Inter', sans-serif;
  font-size: 1rem; 
  color: var(--on-surface); 
  line-height: 1.6; 
  max-width: 500px; 
  font-weight: 500;
}

.fsrs-chip   { 
  display: inline-flex; 
  gap: 0.5rem; 
  align-items: center; 
  background: var(--surface-container-highest); 
  border: 1px solid var(--outline-variant); 
  border-radius: 1rem; 
  padding: 0.375rem 0.875rem; 
  font-size: 0.8rem; 
}

.fsrs-label  { 
  font-family: 'Inter', sans-serif;
  color: var(--primary); 
  font-weight: 600; 
}

.fsrs-days   { 
  font-family: 'Inter', sans-serif;
  color: var(--muted-text); 
  font-weight: 500;
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

.stub-icon  { 
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

.stub-desc  { 
  font-family: 'Inter', sans-serif;
  font-size: 0.95rem; 
  line-height: 1.5; 
  max-width: 400px; 
}
</style>
