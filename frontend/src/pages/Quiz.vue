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
      <button id="tab-marathon" :class="['tab-btn', { active: activeTab === 'marathon' }]" @click="activeTab = 'marathon'">Marathon Mode</button>
      <button id="tab-explorer" :class="['tab-btn', { active: activeTab === 'explorer' }]" @click="activeTab = 'explorer'">Explorer Mode</button>
    </div>

    <!-- Marathon Mode -->
    <section v-if="activeTab === 'marathon'" class="tab-panel">
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
              @click="submitAnswer(q, opt)">
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
const activeTab     = ref('marathon')
const startPage     = ref(1)
const endPage       = ref(10)
const loading       = ref(false)
const error         = ref('')
const questions     = ref([])
const answers       = ref({})

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
  try {
    const res = await scoreAnswer(q.id, opt)
    answers.value[q.id] = { correct: res.correct, feedback: res.feedback }
  } catch (e) {
    answers.value[q.id] = { correct: false, feedback: 'Scoring failed.' }
  }
}

function optionClass(q, opt) {
  const a = answers.value[q.id]
  if (!a) return ''
  if (opt === q.correct_answer) return 'correct-opt'
  return a.correct ? '' : 'wrong-opt'
}
</script>

<style scoped>
.command-center { display: flex; flex-direction: column; gap: 1.5rem; padding: 2rem; max-width: 860px; margin: 0 auto; }

.cc-header { display: flex; align-items: center; gap: 1rem; flex-wrap: wrap; }
.cc-title  { font-size: 1.6rem; font-weight: 700; color: var(--color-heading, #e2e8f0); margin: 0; }

.notebook-select { flex: 1; min-width: 220px; background: var(--color-surface, #1e293b); color: var(--color-text, #cbd5e1); border: 1px solid var(--color-border, #334155); border-radius: 8px; padding: .5rem .75rem; font-size: .95rem; cursor: pointer; }
.notebook-select:disabled { opacity: .5; }

.cc-tabs   { display: flex; gap: .5rem; }
.tab-btn   { padding: .5rem 1.25rem; border: 1px solid var(--color-border, #334155); border-radius: 20px; background: transparent; color: var(--color-text, #94a3b8); cursor: pointer; font-size: .9rem; transition: all .2s; }
.tab-btn.active { background: var(--color-accent, #6366f1); border-color: var(--color-accent, #6366f1); color: #fff; }

.tab-panel { animation: fadeIn .2s ease; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: none; } }

.range-row { display: flex; align-items: center; gap: .75rem; flex-wrap: wrap; margin-bottom: 1rem; }
.range-label { font-size: .85rem; color: var(--color-muted, #64748b); white-space: nowrap; }
.page-input  { width: 72px; background: var(--color-surface, #1e293b); color: var(--color-text, #e2e8f0); border: 1px solid var(--color-border, #334155); border-radius: 8px; padding: .4rem .6rem; font-size: .9rem; text-align: center; }
.page-input:disabled { opacity: .5; }

.generate-btn { margin-left: auto; padding: .55rem 1.4rem; background: var(--color-accent, #6366f1); color: #fff; border: none; border-radius: 8px; font-size: .95rem; font-weight: 600; cursor: pointer; display: flex; align-items: center; gap: .4rem; transition: opacity .2s; }
.generate-btn:disabled { opacity: .4; cursor: default; }

.spinner { width: 16px; height: 16px; border: 2px solid #fff4; border-top-color: #fff; border-radius: 50%; animation: spin .7s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.error-msg { color: #f87171; font-size: .9rem; }

.questions-list { display: flex; flex-direction: column; gap: 1.25rem; }
.question-card  { background: var(--color-surface, #1e293b); border: 1px solid var(--color-border, #334155); border-radius: 12px; padding: 1.25rem; transition: border-color .2s; }
.question-card.answered { border-color: #475569; }

.q-prompt { font-size: 1rem; font-weight: 600; color: var(--color-heading, #e2e8f0); margin-bottom: .85rem; }
.q-num    { color: var(--color-accent, #6366f1); margin-right: .4rem; }

.options  { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: .45rem; }
.option   { padding: .6rem .9rem; border: 1px solid var(--color-border, #334155); border-radius: 8px; cursor: pointer; font-size: .9rem; color: var(--color-text, #cbd5e1); transition: all .15s; }
.option:hover:not(.correct-opt):not(.wrong-opt) { border-color: var(--color-accent, #6366f1); background: #6366f11a; }
.correct-opt { border-color: #22c55e; background: #22c55e18; color: #86efac; }
.wrong-opt   { border-color: #ef4444; background: #ef444418; color: #fca5a5; }

.answer-result { margin-top: .85rem; padding: .65rem .9rem; border-radius: 8px; }
.answer-result.correct { background: #16a34a18; }
.answer-result.wrong   { background: #dc262618; }
.result-label    { font-weight: 700; font-size: .9rem; margin-bottom: .25rem; }
.answer-result.correct .result-label { color: #4ade80; }
.answer-result.wrong .result-label   { color: #f87171; }
.result-feedback { font-size: .85rem; color: var(--color-muted, #94a3b8); }

.score-bar { text-align: center; padding: .75rem; background: var(--color-surface, #1e293b); border-radius: 10px; font-size: .95rem; color: var(--color-text, #cbd5e1); border: 1px solid var(--color-border, #334155); }

.explorer-stub { display: flex; flex-direction: column; align-items: center; gap: .75rem; padding: 4rem 1rem; color: var(--color-muted, #64748b); }
.stub-icon   { font-size: 3rem; }
.stub-title  { font-size: 1.25rem; font-weight: 700; color: var(--color-heading, #e2e8f0); margin: 0; }
.stub-desc   { font-size: .9rem; }
</style>
