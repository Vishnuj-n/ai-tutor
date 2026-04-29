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
      <button id="tab-marathon" :class="['tab-btn', { active: activeTab === 'marathon' }]" @click="activeTab = 'marathon'">Marathon Mode</button>
      <button id="tab-explorer" :class="['tab-btn', { active: activeTab === 'explorer' }]" @click="activeTab = 'explorer'">Explorer Mode</button>
    </div>

    <!-- Marathon Mode -->
    <section v-if="activeTab === 'marathon'" class="tab-panel">
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
          <span class="page-badge">Pages {{ question.source_page_start }}–{{ question.source_page_end }}</span>
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
            <span class="fsrs-days">Next: {{ result.scheduled_days }}d</span>
          </div>
        </div>
        <button id="btn-new" class="generate-btn" @click="reset">New Question</button>
      </div>
    </section>

    <!-- Explorer stub -->
    <section v-else class="tab-panel explorer-stub">
      <div class="stub-icon">🔭</div>
      <h2 class="stub-title">Explorer Mode</h2>
      <p class="stub-desc">Search by topic or concept — coming in Phase 2.</p>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getNotebooks, generateMarathonExam, scoreShortAnswer } from '../services/appApi.js'

const notebooks          = ref([])
const selectedNotebookID = ref('')
const activeTab          = ref('marathon')
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
    const res = await generateMarathonExam(selectedNotebookID.value, startPage.value, endPage.value)
    if (res.error) { error.value = res.error; return }
    question.value = res
  } catch (e) {
    error.value = e?.message ?? 'Exam generation failed.'
  } finally {
    loading.value = false
  }
}

async function submitAnswer() {
  if (!question.value || !userAnswer.value.trim()) return
  if (!question.value.questionID) {
    error.value = 'Invalid question: missing question ID'
    return
  }
  scoring.value = true
  try {
    const res = await scoreShortAnswer(question.value.questionID, userAnswer.value.trim())
    if (res.error) { error.value = res.error; return }
    result.value = res
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
.command-center { display: flex; flex-direction: column; gap: 1.5rem; padding: 2rem; max-width: 860px; margin: 0 auto; }
.cc-header { display: flex; align-items: center; gap: 1rem; flex-wrap: wrap; }
.cc-title  { font-size: 1.6rem; font-weight: 700; color: var(--color-heading, #e2e8f0); margin: 0; }
.notebook-select { flex: 1; min-width: 220px; background: var(--color-surface, #1e293b); color: var(--color-text, #cbd5e1); border: 1px solid var(--color-border, #334155); border-radius: 8px; padding: .5rem .75rem; font-size: .95rem; }
.notebook-select:disabled { opacity: .5; }
.cc-tabs { display: flex; gap: .5rem; }
.tab-btn  { padding: .5rem 1.25rem; border: 1px solid var(--color-border, #334155); border-radius: 20px; background: transparent; color: var(--color-text, #94a3b8); cursor: pointer; font-size: .9rem; transition: all .2s; }
.tab-btn.active { background: var(--color-accent, #6366f1); border-color: var(--color-accent, #6366f1); color: #fff; }
.tab-panel { animation: fadeIn .2s ease; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; } }
.range-row { display: flex; align-items: center; gap: .75rem; flex-wrap: wrap; margin-bottom: 1rem; }
.range-label { font-size: .85rem; color: var(--color-muted, #64748b); white-space: nowrap; }
.page-input  { width: 72px; background: var(--color-surface, #1e293b); color: var(--color-text, #e2e8f0); border: 1px solid var(--color-border, #334155); border-radius: 8px; padding: .4rem .6rem; font-size: .9rem; text-align: center; }
.generate-btn { padding: .55rem 1.4rem; background: var(--color-accent, #6366f1); color: #fff; border: none; border-radius: 8px; font-size: .95rem; font-weight: 600; cursor: pointer; display: flex; align-items: center; gap: .4rem; transition: opacity .2s; }
.generate-btn:disabled { opacity: .4; cursor: default; }
.range-row .generate-btn { margin-left: auto; }
.spinner { width: 16px; height: 16px; border: 2px solid #fff4; border-top-color: #fff; border-radius: 50%; animation: spin .7s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.error-msg { color: #f87171; font-size: .9rem; }

.exam-panel  { display: flex; flex-direction: column; gap: 1rem; }
.question-box { background: var(--color-surface, #1e293b); border: 1px solid var(--color-border, #334155); border-radius: 12px; padding: 1.25rem; }
.exam-prompt  { font-size: 1.05rem; font-weight: 600; color: var(--color-heading, #e2e8f0); line-height: 1.5; margin-bottom: .5rem; }
.page-badge   { font-size: .75rem; color: var(--color-muted, #64748b); background: #1e293b; border: 1px solid #334155; border-radius: 12px; padding: .2rem .6rem; }
.answer-textarea { width: 100%; background: var(--color-surface, #1e293b); color: var(--color-text, #e2e8f0); border: 1px solid var(--color-border, #334155); border-radius: 10px; padding: .85rem 1rem; font-size: .95rem; font-family: inherit; resize: vertical; line-height: 1.6; box-sizing: border-box; }
.answer-textarea:focus { outline: none; border-color: var(--color-accent, #6366f1); }
.answer-textarea:disabled { opacity: .5; }
.exam-actions { display: flex; gap: .75rem; align-items: center; }
.discard-btn  { padding: .55rem 1rem; background: transparent; border: 1px solid var(--color-border, #334155); color: var(--color-muted, #64748b); border-radius: 8px; cursor: pointer; font-size: .9rem; transition: all .15s; }
.discard-btn:hover { border-color: #ef4444; color: #f87171; }

.result-panel { display: flex; flex-direction: column; align-items: center; gap: 1.25rem; padding: 2rem; background: var(--color-surface, #1e293b); border: 1px solid var(--color-border, #334155); border-radius: 16px; }
.score-ring  { display: flex; align-items: baseline; gap: .25rem; }
.score-num   { font-size: 4rem; font-weight: 800; line-height: 1; }
.score-denom { font-size: 1.25rem; color: var(--color-muted, #64748b); }
.score-ring.great .score-num { color: #4ade80; }
.score-ring.ok    .score-num { color: #fbbf24; }
.score-ring.low   .score-num { color: #f87171; }
.result-body { text-align: center; display: flex; flex-direction: column; gap: .5rem; }
.result-feedback { font-size: .95rem; color: var(--color-text, #cbd5e1); line-height: 1.6; max-width: 500px; }
.fsrs-chip   { display: inline-flex; gap: .5rem; align-items: center; background: #0f172a; border: 1px solid #334155; border-radius: 20px; padding: .3rem .75rem; font-size: .8rem; }
.fsrs-label  { color: var(--color-accent, #818cf8); font-weight: 600; }
.fsrs-days   { color: var(--color-muted, #64748b); }

.explorer-stub { display: flex; flex-direction: column; align-items: center; gap: .75rem; padding: 4rem 1rem; color: var(--color-muted, #64748b); }
.stub-icon  { font-size: 3rem; }
.stub-title { font-size: 1.25rem; font-weight: 700; color: var(--color-heading, #e2e8f0); margin: 0; }
.stub-desc  { font-size: .9rem; }
</style>
