<template>
  <div class="flashcards-page">
    <header class="page-header">
      <h1 class="page-title">Flashcards</h1>
      <select v-model="selectedNotebookID" class="notebook-select" :disabled="loading || reviewing">
        <option value="">— Select Notebook —</option>
        <option v-for="nb in notebooks" :key="nb.id" :value="nb.id">{{ nb.title }}</option>
      </select>
    </header>

    <nav class="tabs">
      <button :class="['tab-btn', { active: activeTab === 'comprehensive' }]" @click="activeTab = 'comprehensive'">Comprehensive Extraction</button>
      <button :class="['tab-btn', { active: activeTab === 'explorer' }]" @click="activeTab = 'explorer'">Key Concept Extraction</button>
    </nav>

    <section v-if="activeTab === 'comprehensive'" class="content">
      <div v-if="!reviewing">
        <div class="controls">
          <div class="input-group">
            <label>Start Page</label>
            <input v-model.number="startPage" type="number" min="1" :disabled="loading" />
          </div>
          <div class="input-group">
            <label>End Page</label>
            <input v-model.number="endPage" type="number" min="1" :disabled="loading" />
          </div>
          <BaseButton :disabled="!canGenerate" :loading="loading" @click="generate">Generate Cards</BaseButton>
        </div>
        <ErrorMessage :message="error" />
      </div>

      <div v-if="reviewing && currentCard" class="review-session">
        <p class="progress">Card {{ reviewIndex + 1 }} of {{ cards.length }}</p>
        <div class="flashcard" :class="{ flipped }">
          <div class="card-inner">
            <div class="card-front">
              <p class="card-text">{{ currentCard.prompt }}</p>
              <button class="flip-btn" @click="flipped = true">Show Answer</button>
            </div>
            <div class="card-back">
              <p class="card-text answer-text">{{ currentCard.answer }}</p>
              <div class="rating-buttons">
                <button v-for="r in ratings" :key="r.key" :class="['rating-btn', r.key]" @click="rate(r.key)">
                  {{ r.label }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="reviewing && !currentCard" class="done">
        <h2>Session Complete</h2>
        <p>{{ cards.length }} card{{ cards.length !== 1 ? 's' : '' }} reviewed.</p>
        <BaseButton @click="reset">New Session</BaseButton>
      </div>
    </section>

    <section v-else class="content stub">
      <p>Explorer Mode — coming in Phase 2.</p>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getNotebooks, generateMarathonFlashcards, getFlashcards, recordFlashcardReview } from '../services/appApi.js'
import BaseButton from '../components/BaseButton.vue'
import ErrorMessage from '../components/ErrorMessage.vue'

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
.flashcards-page {
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

.review-session {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1.5rem;
}

.progress {
  text-align: center;
  font-size: 0.85rem;
  color: #666;
  margin-bottom: 0.5rem;
}

.flashcard {
  width: 100%;
  max-width: 500px;
  height: 280px;
  perspective: 1000px;
}

.card-inner {
  width: 100%;
  height: 100%;
  position: relative;
  transform-style: preserve-3d;
  transition: transform 0.5s;
  border-radius: 8px;
}

.flashcard.flipped .card-inner {
  transform: rotateY(180deg);
}

.card-front,
.card-back {
  position: absolute;
  inset: 0;
  backface-visibility: hidden;
  border-radius: 8px;
  background: white;
  border: 1px solid #e0e0e0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
  gap: 1rem;
}

.card-back {
  transform: rotateY(180deg);
}

.card-text {
  font-size: 1rem;
  color: var(--on-surface);
  text-align: center;
  line-height: 1.5;
  font-weight: 500;
}

.answer-text {
  color: var(--primary);
  font-weight: 600;
}

.flip-btn {
  padding: 0.5rem 1.25rem;
  background: var(--primary);
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  font-weight: 600;
}

.flip-btn:hover {
  background: #0056b3;
}

.rating-buttons {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  justify-content: center;
}

.rating-btn {
  padding: 0.4rem 0.75rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  cursor: pointer;
  background: white;
  color: var(--on-surface);
  font-size: 0.85rem;
  font-weight: 500;
}

.rating-btn:hover.again {
  background: #fee2e2;
  border-color: #ef4444;
  color: #dc2626;
}

.rating-btn:hover.hard {
  background: #ffedd5;
  border-color: #f97316;
  color: #ea580c;
}

.rating-btn:hover.good {
  background: #dcfce7;
  border-color: #22c55e;
  color: #16a34a;
}

.rating-btn:hover.easy {
  background: #ede9fe;
  border-color: #8b5cf6;
  color: #7c3aed;
}

.done {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  padding: 3rem;
  text-align: center;
  background: #f5f5f5;
  border-radius: 8px;
}

.done h2 {
  font-size: 1.5rem;
  font-weight: 700;
  margin: 0;
  color: var(--on-surface);
}

.done p {
  font-size: 0.95rem;
  color: #666;
  margin: 0;
}

.stub {
  text-align: center;
  padding: 3rem;
  color: #666;
}
</style>
