<template>
  <StudyPageLayout
    eyebrow="Retention"
    title="Flashcards"
    :subtitle="queueMode ? `Queue session · ${sessionRemaining} remaining` : ''"
  >
    <!-- Toast notification -->
    <Teleport to="body">
      <Transition name="toast">
        <div v-if="toast.show" class="toast-notification" :class="`toast-${toast.type}`">
          {{ toast.message }}
        </div>
      </Transition>
    </Teleport>
    <!-- Toolbar: notebook selector -->
    <template #toolbar>
      <div class="toolbar-field">
        <label class="field-label" for="fc-notebook-select">Notebook</label>
        <select
          id="fc-notebook-select"
          v-model="selectedNotebookID"
          class="ghost-select"
          :disabled="loading || reviewing"
        >
          <option value="">— Select Notebook —</option>
          <option v-for="nb in notebooks" :key="nb.id" :value="nb.id">{{ nb.title }}</option>
        </select>
      </div>
    </template>

    <!-- Mode tabs -->
    <nav class="mode-tabs" aria-label="Flashcard mode">
      <button
        id="tab-comprehensive"
        :class="['mode-tab', { 'mode-tab--active': activeTab === 'comprehensive' }]"
        @click="activeTab = 'comprehensive'"
      >
        Comprehensive
      </button>
      <button
        id="tab-explorer"
        :class="['mode-tab', { 'mode-tab--active': activeTab === 'explorer' }]"
        @click="activeTab = 'explorer'"
      >
        Semantic Discovery
      </button>
    </nav>

    <!-- ── COMPREHENSIVE TAB ───────────────────── -->
    <section v-if="activeTab === 'comprehensive'" class="tab-content">
      <!-- Config panel: page range -->
      <div v-if="!reviewing" class="config-panel">
        <p class="config-panel__hint">Enter the page range to extract flashcards from.</p>
        <div class="config-panel__row">
          <div class="number-field">
            <label class="field-label" for="fc-start">Start Page</label>
            <input
              id="fc-start"
              v-model.number="startPage"
              class="ghost-input"
              type="number"
              min="1"
              :disabled="loading"
            />
          </div>
          <div class="number-field">
            <label class="field-label" for="fc-end">End Page</label>
            <input
              id="fc-end"
              v-model.number="endPage"
              class="ghost-input"
              type="number"
              min="1"
              :disabled="loading"
            />
          </div>
          <BaseButton
            id="fc-generate-btn"
            :disabled="!canGenerate"
            :loading="loading"
            @click="generate"
          >
            Generate Cards
          </BaseButton>
        </div>
        <ErrorMessage :message="error" />
      </div>

      <!-- Review session -->
      <div v-if="reviewing && currentCard" class="review-session">
        <!-- Progress row -->
        <div class="progress-row">
          <p class="progress-label">Card {{ reviewIndex + 1 }} of {{ cards.length }}</p>
          <div class="progress-track">
            <div
              class="progress-fill"
              :style="{ width: `${((reviewIndex + 1) / cards.length) * 100}%` }"
            />
          </div>
        </div>

        <!-- Flashcard -->
        <div class="flashcard" :class="{ flipped }">
          <div class="card-inner">
            <!-- Front -->
            <div class="card-face card-front">
              <button
                v-if="queueMode"
                class="suspend-btn"
                title="Suspend card (Shift+S)"
                :disabled="isSubmittingReview"
                @click="suspendCard"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="12" cy="12" r="10"/>
                  <line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/>
                </svg>
              </button>
              <p class="card-text">{{ currentCard.prompt }}</p>
              <button id="fc-reveal-btn" class="reveal-btn" @click="flipped = true">
                Show Answer
              </button>
            </div>
            <!-- Back -->
            <div class="card-face card-back">
              <button
                v-if="queueMode"
                class="suspend-btn"
                title="Suspend card (Shift+S)"
                :disabled="isSubmittingReview"
                @click="suspendCard"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="12" cy="12" r="10"/>
                  <line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/>
                </svg>
              </button>
              <p class="card-text answer-text">{{ currentCard.answer }}</p>
              <div class="rating-row">
                <button
                  v-for="r in ratings"
                  :id="`fc-rate-${r.key}`"
                  :key="r.key"
                  :class="['rating-btn', `rating-btn--${r.key}`]"
                  :disabled="isSubmittingReview"
                  @click="rate(r.key)"
                >
                  {{ r.label }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Info tooltip -->
        <div class="info-trigger" @mouseenter="showInfoTooltip = true" @mouseleave="showInfoTooltip = false">
          <button class="info-btn" type="button">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"/>
              <line x1="12" y1="16" x2="12" y2="12"/>
              <line x1="12" y1="8" x2="12.01" y2="8"/>
            </svg>
          </button>
          <Transition name="tooltip">
            <div v-if="showInfoTooltip" class="info-tooltip">
              <p class="tooltip-title">Rating Guide</p>
              <div class="tooltip-items">
                <div class="tooltip-item">
                  <span class="tooltip-key again">Again</span>
                  <span class="tooltip-desc">You forgot it completely. You'll see this card again soon.</span>
                </div>
                <div class="tooltip-item">
                  <span class="tooltip-key hard">Hard</span>
                  <span class="tooltip-desc">You got it, but it was a struggle.</span>
                </div>
                <div class="tooltip-item">
                  <span class="tooltip-key good">Good</span>
                  <span class="tooltip-desc">You knew it after thinking for a moment.</span>
                </div>
                <div class="tooltip-item">
                  <span class="tooltip-key easy">Easy</span>
                  <span class="tooltip-desc">You knew it right away, no thinking needed.</span>
                </div>
                <div class="tooltip-divider"></div>
                <div class="tooltip-item">
                  <span class="tooltip-key suspend">Suspend</span>
                  <span class="tooltip-desc">Bad card? Hide it forever. Your data stays safe.</span>
                </div>
                <div class="tooltip-shortcut">Press <kbd>Shift</kbd> + <kbd>S</kbd> to suspend</div>
              </div>
            </div>
          </Transition>
        </div>
      </div>

      <!-- Session complete -->
      <div v-if="reviewing && !currentCard" class="done-panel">
        <p class="eyebrow-inline">Session Complete</p>
        <p class="done-count">
          <span class="done-num">{{ cards.length }}</span>
          card{{ cards.length !== 1 ? 's' : '' }} reviewed.
        </p>
        <BaseButton id="fc-new-session-btn" @click="reset">New Session</BaseButton>
      </div>
    </section>

    <!-- ── EXPLORER TAB (stub) ────────────────── -->
    <section v-else class="tab-content stub-panel">
      <p class="eyebrow-inline">Coming in Phase 2</p>
      <p class="stub-text">
        Semantic Discovery will surface concept clusters across your notebooks.
      </p>
    </section>
  </StudyPageLayout>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  activateTask,
  completeReviewSession,
  getNotebooks,
  generateManualFlashcards as generateManualFlashcards,
  getReviewSession,
  recordCardReview,
  suspendFlashcard,
} from '../services/appApi.js'
import BaseButton from '../components/BaseButton.vue'
import ErrorMessage from '../components/ErrorMessage.vue'
import StudyPageLayout from '../components/StudyPageLayout.vue'

const route = useRoute()
const router = useRouter()
const notebooks = ref([])
const selectedNotebookID = ref('')
const activeTab = ref('comprehensive')
const startPage = ref(1)
const endPage = ref(10)
const loading = ref(false)
const error = ref('')
const cards = ref([])
const reviewIndex = ref(0)
const reviewing = ref(false)
const flipped = ref(false)
const isSubmittingReview = ref(false)
const reviewTaskID = ref('')
const sessionRemaining = ref(0)
const queueMode = computed(() => !!reviewTaskID.value)
const toast = ref({ show: false, message: '', type: 'info' })
const showInfoTooltip = ref(false)

const ratings = [
  { key: 'again', label: '✕ Again', value: 1 },
  { key: 'hard', label: '~ Hard', value: 2 },
  { key: 'good', label: '✓ Good', value: 3 },
  { key: 'easy', label: '⚡ Easy', value: 4 },
]

const canGenerate = computed(
  () =>
    selectedNotebookID.value &&
    startPage.value > 0 &&
    endPage.value >= startPage.value &&
    !loading.value
)
const currentCard = computed(() => cards.value[reviewIndex.value] ?? null)

onMounted(async () => {
  window.addEventListener('keydown', handleKeydown)
  try {
    const res = await getNotebooks()
    notebooks.value = Array.isArray(res) ? res.filter((n) => !n.error) : []
  } catch {
    error.value = 'Failed to load notebooks.'
  }
  if (route.query.taskId) {
    await loadQueueSession(String(route.query.taskId), String(route.query.notebookId || ''))
  }
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})

async function generate() {
  error.value = ''
  cards.value = []
  reviewIndex.value = 0
  flipped.value = false
  reviewing.value = false
  loading.value = true
  try {
    const res = await generateManualFlashcards(
      selectedNotebookID.value,
      startPage.value,
      endPage.value
    )
    if (res.error) {
      error.value = res.error
      return
    }
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
  if (!card || isSubmittingReview.value) return

  // Validate ratingKey against available ratings
  const validRating = ratings.find((r) => r.key === ratingKey)
  if (!validRating) {
    error.value = 'Invalid rating selection. Please try again.'
    return
  }

  isSubmittingReview.value = true
  console.warn('[FLASHCARD_PIPELINE] frontend_review_rating_submit', {
    queueMode: queueMode.value,
    reviewTaskID: reviewTaskID.value,
    cardID: queueMode.value ? card.card_id : card.id,
    rating: ratingKey,
  })
  try {
    if (queueMode.value) {
      const res = await recordCardReview(reviewTaskID.value, card.card_id, validRating.value)
      if (res.error) {
        error.value = `Failed to save review: ${res.error}`
        return
      }
      flipped.value = false
      sessionRemaining.value = Number(res.remaining ?? 0)
      if (sessionRemaining.value <= 0) {
        const completeRes = await completeReviewSession(reviewTaskID.value)
        if (completeRes?.error) {
          error.value = `Failed to complete session: ${completeRes.error}`
          return
        }
        console.warn('[FLASHCARDS] flashcard_review_completed_dashboard_redirect')
        router.push('/dashboard')
        return
      }
      await loadQueueSession(reviewTaskID.value, selectedNotebookID.value)
      return
    }
    flipped.value = false
    reviewIndex.value++
  } catch (e) {
    error.value = `Failed to save review: ${e?.message ?? 'Unknown error'}`
  } finally {
    isSubmittingReview.value = false
  }
}

function reset() {
  reviewing.value = false
  cards.value = []
  reviewIndex.value = 0
  flipped.value = false
  isSubmittingReview.value = false
  error.value = ''
  reviewTaskID.value = ''
  sessionRemaining.value = 0
}

function showToast(message, type = 'info') {
  toast.value = { show: true, message, type }
  setTimeout(() => {
    toast.value.show = false
  }, 2000)
}

async function suspendCard() {
  const card = currentCard.value
  if (!card || !queueMode.value || isSubmittingReview.value) return

  isSubmittingReview.value = true
  try {
    const res = await suspendFlashcard(reviewTaskID.value, card.card_id)
    if (res.error) {
      error.value = `Failed to suspend card: ${res.error}`
      return
    }
    showToast('Card Suspended', 'success')
    flipped.value = false
    sessionRemaining.value = Number(res.remaining ?? 0)
    if (sessionRemaining.value <= 0) {
      const completeRes = await completeReviewSession(reviewTaskID.value)
      if (completeRes?.error) {
        error.value = `Failed to complete session: ${completeRes.error}`
        return
      }
      router.push('/dashboard')
      return
    }
    await loadQueueSession(reviewTaskID.value, selectedNotebookID.value)
  } catch (e) {
    error.value = `Failed to suspend card: ${e?.message ?? 'Unknown error'}`
  } finally {
    isSubmittingReview.value = false
  }
}

function handleKeydown(e) {
  if (!reviewing.value || !currentCard.value) return
  if (e.key === 'S' && e.shiftKey && !e.ctrlKey && !e.metaKey) {
    e.preventDefault()
    suspendCard()
  }
}

async function loadQueueSession(taskID, notebookID = '') {
  error.value = ''
  loading.value = true
  reviewTaskID.value = taskID
  if (notebookID) selectedNotebookID.value = notebookID
  console.warn('[FLASHCARD_PIPELINE] frontend_review_task_rendering start', { taskID, notebookID })
  try {
    const activateRes = await activateTask(taskID)
    if (activateRes?.error && activateRes.code !== 409) {
      error.value = activateRes.error
      reviewing.value = false
      cards.value = []
      reviewTaskID.value = ''
      return
    }
    const res = await getReviewSession(taskID, notebookID)
    if (res.error) {
      error.value = res.error
      reviewing.value = false
      cards.value = []
      reviewTaskID.value = ''
      return
    }
    const session = res.session
    cards.value = Array.isArray(session?.cards) ? session.cards : []
    reviewIndex.value = Number(session?.next_pending_idx ?? -1)
    sessionRemaining.value = Number(session?.remaining ?? 0)
    reviewing.value = cards.value.length > 0
    console.warn('[FLASHCARD_PIPELINE] frontend_review_task_rendering result', {
      taskID,
      cards: cards.value.length,
      nextPendingIdx: reviewIndex.value,
      remaining: sessionRemaining.value,
      reviewing: reviewing.value,
    })
    flipped.value = false
    if (reviewIndex.value < 0) {
      reviewIndex.value = cards.value.length
    }
  } catch (e) {
    error.value = e?.message ?? 'Failed to load queue session'
    reviewing.value = false
    cards.value = []
    reviewTaskID.value = ''
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
/* ── Toolbar ──────────────────────────────────── */
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

/* ── Mode tabs (pill style, no divider line) ──── */
.mode-tabs {
  display: flex;
  gap: 4px;
  padding: 4px;
  background: var(--surface-container-low);
  border-radius: 12px;
  width: fit-content;
}

.mode-tab {
  padding: 7px 16px;
  border: 0;
  border-radius: 9px;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  color: var(--muted-text);
  background: transparent;
  cursor: pointer;
  transition:
    background 0.15s ease,
    color 0.15s ease;
}

.mode-tab:hover:not(.mode-tab--active) {
  color: var(--on-surface);
  background: color-mix(in srgb, var(--on-surface) 6%, transparent);
}

.mode-tab--active {
  background: var(--surface-container-lowest);
  color: var(--on-surface);
}

/* ── Tab content ──────────────────────────────── */
.tab-content {
  display: grid;
  gap: 16px;
  animation: fadeIn 0.18s ease;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
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

/* ── Review session ───────────────────────────── */
.review-session {
  display: grid;
  gap: 16px;
  justify-items: center;
}

/* Progress row */
.progress-row {
  width: 100%;
  max-width: 560px;
  display: grid;
  gap: 6px;
}

.progress-label {
  margin: 0;
  font-size: 12px;
  color: var(--muted-text);
  font-weight: 600;
  letter-spacing: 0.04em;
  text-align: center;
}

.progress-track {
  height: 3px;
  background: var(--surface-container-low);
  border-radius: 999px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--primary-dim), var(--primary));
  border-radius: 999px;
  transition: width 0.3s ease;
}

/* Flashcard */
.flashcard {
  width: 100%;
  max-width: 560px;
  perspective: 1200px;
}

.card-inner {
  position: relative;
  width: 100%;
  padding-bottom: 62%; /* aspect ratio ~16:10 */
  transform-style: preserve-3d;
  transition: transform 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  border-radius: 16px;
}

.flashcard.flipped .card-inner {
  transform: rotateY(180deg);
}

.card-face {
  position: absolute;
  inset: 0;
  backface-visibility: hidden;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 32px 28px;
}

.card-front {
  background: var(--surface-container-lowest);
}

.card-back {
  background: var(--surface-container-low);
  transform: rotateY(180deg);
}

.card-text {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
  color: var(--on-surface);
  text-align: center;
  line-height: 1.6;
  max-width: 48ch;
}

.answer-text {
  font-weight: 600;
  color: var(--primary);
}

/* Reveal button */
.reveal-btn {
  border: 0;
  border-radius: 12px;
  padding: 10px 24px;
  font: inherit;
  font-size: 14px;
  font-weight: 700;
  color: var(--on-primary);
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  cursor: pointer;
  transition:
    transform 0.14s ease,
    filter 0.14s ease;
}

.reveal-btn:hover {
  filter: brightness(1.08);
}

.reveal-btn:active {
  transform: scale(0.96);
}

/* Rating buttons — semantic tonal variation */
.rating-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: center;
}

.rating-btn {
  padding: 8px 16px;
  border: 0;
  border-radius: 10px;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition:
    transform 0.1s ease,
    filter 0.12s ease;
  background: var(--surface-container-lowest);
  color: var(--on-surface);
}

.rating-btn:active:not(:disabled) {
  transform: scale(0.95);
}

.rating-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Hover: tonal tints only, no heavy borders */
.rating-btn--again:hover:not(:disabled) {
  background: color-mix(in srgb, #ef4444 14%, var(--surface-container-lowest));
  color: #dc2626;
}

.rating-btn--hard:hover:not(:disabled) {
  background: color-mix(in srgb, #f97316 14%, var(--surface-container-lowest));
  color: #ea580c;
}

.rating-btn--good:hover:not(:disabled) {
  background: color-mix(in srgb, #22c55e 14%, var(--surface-container-lowest));
  color: #16a34a;
}

.rating-btn--easy:hover:not(:disabled) {
  background: color-mix(in srgb, #8b5cf6 14%, var(--surface-container-lowest));
  color: #7c3aed;
}

/* ── Session done ─────────────────────────────── */
.done-panel {
  background: var(--surface-container-low);
  border-radius: 16px;
  padding: 48px 24px;
  display: grid;
  gap: 16px;
  justify-items: center;
  text-align: center;
}

.eyebrow-inline {
  margin: 0;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--muted-text);
}

.done-count {
  margin: 0;
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.done-num {
  font-family: 'Manrope', sans-serif;
  font-size: 40px;
  font-weight: 700;
  letter-spacing: -0.03em;
  line-height: 1;
  color: var(--on-surface);
}

/* ── Suspend button ──────────────────────────── */
.suspend-btn {
  position: absolute;
  top: 12px;
  right: 12px;
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 8px;
  background: color-mix(in srgb, var(--on-surface) 6%, transparent);
  color: var(--muted-text);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
  z-index: 2;
}

.suspend-btn:hover:not(:disabled) {
  background: color-mix(in srgb, #ef4444 14%, transparent);
  color: #dc2626;
}

.suspend-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

/* ── Toast notification ──────────────────────── */
.toast-notification {
  position: fixed;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  padding: 12px 24px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 600;
  color: white;
  z-index: 9999;
  pointer-events: none;
}

.toast-success {
  background: #16a34a;
}

.toast-error {
  background: #dc2626;
}

.toast-info {
  background: var(--primary);
}

.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(20px);
}

/* ── Info tooltip ────────────────────────────── */
.info-trigger {
  position: relative;
  display: inline-flex;
}

.info-btn {
  width: 28px;
  height: 28px;
  border: 0;
  border-radius: 50%;
  background: color-mix(in srgb, var(--on-surface) 6%, transparent);
  color: var(--muted-text);
  cursor: help;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
}

.info-btn:hover {
  background: color-mix(in srgb, var(--on-surface) 10%, transparent);
  color: var(--on-surface);
}

.info-tooltip {
  position: absolute;
  bottom: calc(100% + 12px);
  left: 50%;
  transform: translateX(-50%);
  width: 280px;
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  padding: 16px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  z-index: 100;
}

.tooltip-title {
  margin: 0 0 12px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted-text);
}

.tooltip-items {
  display: grid;
  gap: 8px;
}

.tooltip-item {
  display: flex;
  gap: 10px;
  align-items: baseline;
}

.tooltip-key {
  font-size: 12px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 6px;
  flex-shrink: 0;
}

.tooltip-key.again {
  background: color-mix(in srgb, #ef4444 14%, transparent);
  color: #dc2626;
}

.tooltip-key.hard {
  background: color-mix(in srgb, #f97316 14%, transparent);
  color: #ea580c;
}

.tooltip-key.good {
  background: color-mix(in srgb, #22c55e 14%, transparent);
  color: #16a34a;
}

.tooltip-key.easy {
  background: color-mix(in srgb, #8b5cf6 14%, transparent);
  color: #7c3aed;
}

.tooltip-key.suspend {
  background: color-mix(in srgb, #ef4444 14%, transparent);
  color: #dc2626;
}

.tooltip-desc {
  font-size: 12px;
  color: var(--muted-text);
  line-height: 1.4;
}

.tooltip-divider {
  height: 1px;
  background: var(--outline-variant);
  margin: 4px 0;
}

.tooltip-shortcut {
  font-size: 11px;
  color: var(--muted-text);
  text-align: center;
  margin-top: 4px;
}

.tooltip-shortcut kbd {
  display: inline-block;
  padding: 2px 6px;
  font-family: inherit;
  font-size: 10px;
  font-weight: 700;
  background: var(--surface-container-low);
  border: 1px solid var(--outline-variant);
  border-radius: 4px;
}

.tooltip-enter-active,
.tooltip-leave-active {
  transition: all 0.2s ease;
}

.tooltip-enter-from,
.tooltip-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(8px);
}

/* ── Explorer stub ────────────────────────────── */
.stub-panel {
  background: var(--surface-container-low);
  border-radius: 16px;
  padding: 64px 24px;
  text-align: center;
}

.stub-text {
  margin: 8px 0 0;
  font-size: 15px;
  color: var(--muted-text);
  line-height: 1.6;
  max-width: 52ch;
  margin-inline: auto;
}

/* ── Responsive ───────────────────────────────── */
@media (max-width: 720px) {
  .card-face {
    padding: 24px 16px;
  }

  .card-text {
    font-size: 14px;
  }

  .config-panel__row {
    flex-direction: column;
    align-items: flex-start;
  }
}

@media (prefers-reduced-motion: reduce) {
  .card-inner {
    transition: none;
  }

  .tab-content {
    animation: none;
  }
}
</style>
