<template>
  <div class="assessment-page">
    <header class="page-header">
      <h1 class="page-title">Written Assessment</h1>
      <select v-if="!isContextLocked" v-model="selectedNotebookID" class="notebook-select" :disabled="loading || scoring">
        <option value="">— Select Notebook —</option>
        <option v-for="nb in notebooks" :key="nb.id" :value="nb.id">{{ nb.title }}</option>
      </select>
    </header>

    <nav v-if="!isContextLocked" class="tabs">
      <button :class="['tab-btn', { active: activeTab === 'comprehensive' }]" @click="activeTab = 'comprehensive'">Comprehensive Exam</button>
      <button :class="['tab-btn', { active: activeTab === 'explorer' }]" @click="activeTab = 'explorer'">Semantic Discovery</button>
    </nav>

    <section v-if="activeTab === 'comprehensive'" class="content">
      <div v-if="!isContextLocked && !question">
        <div class="controls">
          <div class="input-group">
            <label>Start Page</label>
            <input v-model.number="startPage" type="number" min="1" :disabled="loading" />
          </div>
          <div class="input-group">
            <label>End Page</label>
            <input v-model.number="endPage" type="number" min="1" :disabled="loading" />
          </div>
          <BaseButton :disabled="!canGenerate" :loading="loading" @click="generate">Generate Question</BaseButton>
        </div>
        <ErrorMessage :message="error" />
      </div>

      <div v-if="question && !result" class="exam">
        <div class="question-box">
          <p class="prompt">{{ question.prompt }}</p>
          <span class="badge">Pages {{ question.sourcePageStart }}–{{ question.sourcePageEnd }}</span>
        </div>
        <textarea v-model="userAnswer" rows="6" class="answer-input"
          placeholder="Write your answer here…" :disabled="scoring" />
        <div class="actions">
          <BaseButton :disabled="!userAnswer.trim() || scoring" :loading="scoring" @click="submitAnswer">Submit Answer</BaseButton>
          <button class="discard-btn" :disabled="scoring" @click="reset">Discard</button>
        </div>
      </div>

      <div v-if="result" class="result">
        <div class="score" :class="scoreClass">
          <span class="num">{{ result.score }}</span>
          <span class="denom">/10</span>
        </div>
        <div class="result-body">
          <p class="feedback">{{ result.feedback }}</p>
          <div class="fsrs" v-if="result.fsrsRating">
            <span class="label">{{ result.fsrsRating }}</span>
            <span class="days">Next: {{ result.scheduledDays }}d</span>
          </div>
        </div>
        <BaseButton v-if="!isContextLocked" @click="reset">New Question</BaseButton>
        <BaseButton v-else @click="returnToDashboard">Return to Dashboard</BaseButton>
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
import { getDailyAgenda, getNotebooks, generateComprehensiveExam, generateTopicWrittenAssessment, scoreShortAnswer } from '../services/appApi.js'
import BaseButton from '../components/BaseButton.vue'
import ErrorMessage from '../components/ErrorMessage.vue'

const route = useRoute()
const router = useRouter()

// Phase 3: Context-locked mode
const isContextLocked = computed(() => Boolean(route.query.topicId))

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
const navigatingToDashboard = ref(false)

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
  if (isContextLocked.value) {
    // Auto-generate question from route context
    await loadContextLockedAssessment()
  } else {
    try {
      const res = await getNotebooks()
      notebooks.value = Array.isArray(res) ? res.filter(n => !n.error) : []
    } catch { error.value = 'Failed to load notebooks.' }
  }
})

async function loadContextLockedAssessment() {
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
    const res = await generateTopicWrittenAssessment(topicId, start, end)
    if (res.error) {
      error.value = res.error
      return
    }
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
    error.value = e?.message ?? 'Assessment generation failed.'
  } finally {
    loading.value = false
  }
}

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
    
    // Phase 3: Refresh agenda and auto-return to dashboard in context-locked mode
    if (isContextLocked.value) {
      setTimeout(async () => {
        await getDailyAgenda() // Refresh agenda
        returnToDashboard()
      }, 1500)
    }
  } catch (e) {
    error.value = e?.message ?? 'Scoring failed.'
  } finally {
    scoring.value = false
  }
}

function returnToDashboard() {
  navigatingToDashboard.value = true
  router.push('/dashboard')
}

function reset() {
  question.value = null
  result.value = null
  userAnswer.value = ''
  error.value = ''
  navigatingToDashboard.value = false
}
</script>

<style scoped>
.assessment-page {
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

.exam {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.question-box {
  background: #f5f5f5;
  border-radius: 8px;
  padding: 1rem;
  border: 1px solid #e0e0e0;
}

.prompt {
  font-size: 1rem;
  font-weight: 600;
  color: var(--on-surface);
  line-height: 1.5;
  margin: 0 0 0.5rem 0;
}

.badge {
  font-size: 0.75rem;
  color: #666;
  background: white;
  border-radius: 4px;
  padding: 0.25rem 0.5rem;
  font-weight: 500;
}

.answer-input {
  width: 100%;
  background: white;
  color: var(--on-surface);
  border: 1px solid #ccc;
  border-radius: 4px;
  padding: 0.75rem;
  font-size: 0.9rem;
  resize: vertical;
  line-height: 1.5;
  box-sizing: border-box;
}

.answer-input:focus {
  outline: none;
  border-color: var(--primary);
}

.answer-input:disabled {
  opacity: 0.5;
}

.actions {
  display: flex;
  gap: 0.75rem;
  align-items: center;
}

.discard-btn {
  padding: 0.5rem 1rem;
  background: transparent;
  border: 1px solid #ccc;
  color: var(--on-surface);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  font-weight: 500;
}

.discard-btn:hover {
  border-color: #ef4444;
  color: #dc2626;
}

.result {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1.5rem;
  padding: 2rem;
  background: #f5f5f5;
  border-radius: 8px;
  border: 1px solid #e0e0e0;
}

.score {
  display: flex;
  align-items: baseline;
  gap: 0.25rem;
}

.num {
  font-size: 3rem;
  font-weight: 800;
  line-height: 1;
}

.denom {
  font-size: 1rem;
  color: #666;
  font-weight: 500;
}

.score.great .num {
  color: #16a34a;
}

.score.ok .num {
  color: #ea580c;
}

.score.low .num {
  color: #dc2626;
}

.result-body {
  text-align: center;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.feedback {
  font-size: 0.95rem;
  color: var(--on-surface);
  line-height: 1.5;
  max-width: 500px;
  font-weight: 500;
  margin: 0;
}

.fsrs {
  display: inline-flex;
  gap: 0.5rem;
  align-items: center;
  background: white;
  border: 1px solid #e0e0e0;
  border-radius: 12px;
  padding: 0.375rem 0.75rem;
  font-size: 0.8rem;
}

.label {
  color: var(--primary);
  font-weight: 600;
}

.days {
  color: #666;
  font-weight: 500;
}

.stub {
  text-align: center;
  padding: 3rem;
  color: #666;
}
</style>
