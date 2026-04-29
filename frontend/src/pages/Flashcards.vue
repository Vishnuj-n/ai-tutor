<template>
  <div class="command-center">
    <header class="cc-header">
      <h1 class="cc-title">Flashcards</h1>
      <select id="notebook-select" v-model="selectedNotebookID" class="notebook-select" :disabled="loading || reviewing">
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
      <div v-if="!reviewing">
        <div class="range-row">
          <label class="range-label">Start Page</label>
          <input id="start-page" v-model.number="startPage" type="number" min="1" class="page-input" :disabled="loading" />
          <label class="range-label">End Page</label>
          <input id="end-page" v-model.number="endPage" type="number" min="1" class="page-input" :disabled="loading" />
          <button id="btn-generate" class="generate-btn" :disabled="!canGenerate" @click="generate">
            <span v-if="!loading">Generate Cards →</span>
            <span v-else class="spinner" />
          </button>
        </div>
        <p v-if="error" class="error-msg">{{ error }}</p>
      </div>


      <!-- Review session -->
      <div v-if="reviewing && currentCard" class="review-session">
        <p class="progress-text">Card {{ reviewIndex + 1 }} of {{ cards.length }}</p>
        <div class="flashcard" :class="{ flipped }">
          <div class="card-inner">
            <div class="card-front">
              <p class="card-text">{{ currentCard.prompt }}</p>
              <button id="btn-flip" class="flip-btn" @click="flipped = true">Show Answer</button>
            </div>
            <div class="card-back">
              <p class="card-text answer-text">{{ currentCard.answer }}</p>
              <div class="rating-row">
                <button v-for="r in ratings" :key="r.key" :id="'btn-rate-' + r.key"
                  :class="['rating-btn', r.key]" @click="rate(r.key)">
                  {{ r.label }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="reviewing && !currentCard" class="done-banner">
        <span class="done-icon">🎉</span>
        <h2>Session Complete</h2>
        <p>{{ cards.length }} card{{ cards.length !== 1 ? 's' : '' }} reviewed.</p>
        <button id="btn-restart" class="generate-btn" @click="reset">New Session</button>
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
import { getNotebooks, generateMarathonFlashcards, recordFlashcardReview } from '../services/appApi.js'

const notebooks          = ref([])
const selectedNotebookID = ref('')
const activeTab          = ref('comprehensive')
const startPage          = ref(1)
const endPage            = ref(10)
const loading            = ref(false)
const error              = ref('')
const cards              = ref([])
const reviewIndex        = ref(0)
const reviewing          = ref(false)
const flipped            = ref(false)

const ratings = [
  { key: 'again', label: '✕ Again' },
  { key: 'hard',  label: '~ Hard' },
  { key: 'good',  label: '✓ Good' },
  { key: 'easy',  label: '⚡ Easy' },
]

const canGenerate  = computed(() =>
  selectedNotebookID.value && startPage.value > 0 && endPage.value >= startPage.value && !loading.value
)
const currentCard  = computed(() => cards.value[reviewIndex.value] ?? null)

onMounted(async () => {
  try {
    const res = await getNotebooks()
    notebooks.value = Array.isArray(res) ? res.filter(n => !n.error) : []
  } catch { error.value = 'Failed to load notebooks.' }
})

async function generate() {
  error.value = ''
  cards.value = []
  reviewIndex.value = 0
  flipped.value = false
  reviewing.value = false
  loading.value = true
  try {
    const res = await generateMarathonFlashcards(selectedNotebookID.value, startPage.value, endPage.value)
    if (res.error) { error.value = res.error; return }
    cards.value = res.cards ?? []
    if (cards.value.length) reviewing.value = true
  } catch (e) {
    error.value = e?.message ?? 'Flashcard generation failed.'
  } finally {
    loading.value = false
  }
}

async function rate(ratingKey) {
  const card = currentCard.value
  if (!card) return
  try {
    const res = await recordFlashcardReview(card.id, ratingKey)
    if (res.error) {
      error.value = `Failed to save review: ${res.error}`
      return
    }
  } catch (e) {
    error.value = `Failed to save review: ${e?.message ?? 'Unknown error'}`
    return
  }
  flipped.value = false
  reviewIndex.value++
}

function reset() {
  reviewing.value = false
  cards.value = []
  reviewIndex.value = 0
  flipped.value = false
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

.progress-text {
  text-align: center;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--muted-text);
  margin-bottom: 2rem;
  font-family: 'Inter', sans-serif;
}

.review-session {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2rem;
}

.flashcard {
  width: 100%;
  max-width: 540px;
  height: 320px;
  perspective: 1000px;
}

.card-inner {
  width: 100%;
  height: 100%;
  position: relative;
  transform-style: preserve-3d;
  transition: transform 0.6s;
  border-radius: 0.75rem;
}

.flashcard.flipped .card-inner {
  transform: rotateY(180deg);
}

.card-front,
.card-back {
  position: absolute;
  inset: 0;
  backface-visibility: hidden;
  border-radius: 0.75rem;
  background: var(--surface-container-lowest);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2rem;
  gap: 1.5rem;
  box-shadow: 0 20px 40px rgba(45, 51, 56, 0.06);
}

.card-back {
  transform: rotateY(180deg);
}

.card-text {
  font-family: 'Inter', sans-serif;
  font-size: 1.125rem;
  color: var(--on-surface);
  text-align: center;
  line-height: 1.6;
  font-weight: 500;
}

.answer-text {
  color: var(--primary);
  font-weight: 600;
}

.flip-btn {
  padding: 0.75rem 1.5rem;
  background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dim) 100%);
  color: var(--on-primary);
  border: none;
  border-radius: 0.75rem;
  cursor: pointer;
  font-family: 'Inter', sans-serif;
  font-size: 0.9rem;
  font-weight: 600;
  transition: all 0.2s ease;
  box-shadow: 0 4px 12px rgba(0, 91, 193, 0.15);
}

.flip-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(0, 91, 193, 0.25);
}

.rating-row {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
  justify-content: center;
}

.rating-btn {
  padding: 0.5rem 1rem;
  border: 1px solid var(--outline-variant);
  border-radius: 0.5rem;
  cursor: pointer;
  background: var(--surface-container-lowest);
  color: var(--on-surface);
  font-family: 'Inter', sans-serif;
  font-size: 0.875rem;
  font-weight: 500;
  transition: all 0.15s ease;
}

.rating-btn:hover {
  transform: translateY(-1px);
}

.rating-btn.again:hover {
  background: rgba(220, 38, 38, 0.1);
  border-color: #ef4444;
  color: #dc2626;
}

.rating-btn.hard:hover {
  background: rgba(249, 115, 22, 0.1);
  border-color: #f97316;
  color: #ea580c;
}

.rating-btn.good:hover {
  background: rgba(34, 197, 94, 0.1);
  border-color: #22c55e;
  color: #16a34a;
}

.rating-btn.easy:hover {
  background: rgba(124, 58, 237, 0.1);
  border-color: #8b5cf6;
  color: #7c3aed;
}

.done-banner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  padding: 4rem 2rem;
  color: var(--on-surface);
  text-align: center;
  background: var(--surface-container-low);
  border-radius: 0.75rem;
  margin: 2rem 0;
}

.done-icon {
  font-size: 3rem;
  opacity: 0.8;
}

.done-banner h2 {
  font-family: 'Manrope', sans-serif;
  font-size: 1.75rem;
  font-weight: 700;
  margin: 0;
  color: var(--on-surface);
}

.done-banner p {
  font-family: 'Inter', sans-serif;
  font-size: 1rem;
  color: var(--muted-text);
  margin: 0;
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
