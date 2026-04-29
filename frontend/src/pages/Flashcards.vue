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
      <button id="tab-marathon" :class="['tab-btn', { active: activeTab === 'marathon' }]" @click="activeTab = 'marathon'">Marathon Mode</button>
      <button id="tab-explorer" :class="['tab-btn', { active: activeTab === 'explorer' }]" @click="activeTab = 'explorer'">Explorer Mode</button>
    </div>

    <!-- Marathon Mode -->
    <section v-if="activeTab === 'marathon'" class="tab-panel">
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
const activeTab          = ref('marathon')
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
.generate-btn { margin-left: auto; padding: .55rem 1.4rem; background: var(--color-accent, #6366f1); color: #fff; border: none; border-radius: 8px; font-size: .95rem; font-weight: 600; cursor: pointer; display: flex; align-items: center; gap: .4rem; transition: opacity .2s; }
.generate-btn:disabled { opacity: .4; cursor: default; }
.spinner { width: 16px; height: 16px; border: 2px solid #fff4; border-top-color: #fff; border-radius: 50%; animation: spin .7s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.error-msg { color: #f87171; font-size: .9rem; }
.progress-text { text-align: center; font-size: .85rem; color: var(--color-muted, #64748b); margin-bottom: 1rem; }
.review-session { display: flex; flex-direction: column; align-items: center; gap: 1rem; }
.flashcard { width: 100%; max-width: 540px; height: 260px; perspective: 1000px; }
.card-inner { width: 100%; height: 100%; position: relative; transform-style: preserve-3d; transition: transform .5s; border-radius: 16px; }
.flashcard.flipped .card-inner { transform: rotateY(180deg); }
.card-front, .card-back {
  position: absolute; inset: 0; backface-visibility: hidden; border-radius: 16px;
  background: var(--color-surface, #1e293b); border: 1px solid var(--color-border, #334155);
  display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 1.5rem; gap: 1rem;
}
.card-back { transform: rotateY(180deg); }
.card-text   { font-size: 1rem; color: var(--color-text, #e2e8f0); text-align: center; line-height: 1.5; }
.answer-text { color: #86efac; font-weight: 600; }
.flip-btn    { padding: .45rem 1.2rem; background: var(--color-accent, #6366f1); color: #fff; border: none; border-radius: 8px; cursor: pointer; font-size: .9rem; }
.rating-row  { display: flex; gap: .5rem; flex-wrap: wrap; justify-content: center; }
.rating-btn  { padding: .4rem .9rem; border: 1px solid var(--color-border, #334155); border-radius: 8px; cursor: pointer; background: transparent; color: var(--color-text, #cbd5e1); font-size: .85rem; transition: all .15s; }
.rating-btn.again:hover { background: #dc262628; border-color: #ef4444; color: #f87171; }
.rating-btn.hard:hover  { background: #f9731628; border-color: #f97316; color: #fdba74; }
.rating-btn.good:hover  { background: #16a34a28; border-color: #22c55e; color: #4ade80; }
.rating-btn.easy:hover  { background: #7c3aed28; border-color: #8b5cf6; color: #c4b5fd; }
.done-banner { display: flex; flex-direction: column; align-items: center; gap: .75rem; padding: 3rem 1rem; color: var(--color-text, #cbd5e1); }
.done-icon   { font-size: 2.5rem; }
.explorer-stub { display: flex; flex-direction: column; align-items: center; gap: .75rem; padding: 4rem 1rem; color: var(--color-muted, #64748b); }
.stub-icon  { font-size: 3rem; }
.stub-title { font-size: 1.25rem; font-weight: 700; color: var(--color-heading, #e2e8f0); margin: 0; }
.stub-desc  { font-size: .9rem; }
</style>
