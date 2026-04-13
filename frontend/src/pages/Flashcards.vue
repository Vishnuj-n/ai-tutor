<template>
  <section class="page">
    <p class="eyebrow">Flashcards</p>
    <h1>Review Session</h1>

    <article class="panel controls">
      <label class="field">
        <span>Notebook</span>
        @media (max-width: 960px) {
          h1 {
            font-size: 38px;
          }

          .card-header {
            flex-direction: column;
            align-items: flex-start;
          }

          .card-face {
            min-height: 180px;
          }
        }
          <option disabled value="">Select a notebook</option>
          <option v-for="notebook in notebookTree" :key="notebook.notebook_id" :value="notebook.notebook_id">
            {{ notebook.title }}
          </option>
        </select>
            </label>

            <label class="field">
        <span>Topic</span>
        <select v-model="selectedTopicID" :disabled="busy || availableTopics.length === 0" @change="onTopicChange">
          <option disabled value="">
            {{ availableTopics.length === 0 ? 'No topics available yet' : 'Select a topic' }}
          </option>
          <option v-for="topic in availableTopics" :key="topic.topic_id" :value="topic.topic_id">
            {{ topic.title }}
          </option>
        </select>
      </label>

      <button class="secondary" :disabled="busy || !selectedTopicID" @click="onPrepareCards">
        {{ isGenerating ? 'Preparing...' : 'Prepare Cards' }}
      </button>

      <button class="primary" :disabled="busy || !selectedTopicID" @click="loadDueCards">
        {{ isLoadingCards ? 'Loading...' : 'Start Review' }}
      </button>
    </article>

    <article v-if="errorMessage" class="panel error">{{ errorMessage }}</article>

    <article v-if="currentCard" class="panel card-shell">
      <header class="card-header">
        <div>
          <p class="card-count">Card {{ currentIndex + 1 }} / {{ cards.length }}</p>
          <h2>{{ currentCard.prompt }}</h2>
        </div>
        <span class="badge">{{ currentTopicTitle }}</span>
      </header>

      <div class="card-face" :class="{ revealed: answerVisible }">
        <p class="label">{{ answerVisible ? 'Answer' : 'Prompt' }}</p>
        <p class="content">{{ answerVisible ? currentCard.answer : currentCard.prompt }}</p>
      </div>

      <footer class="actions">
        <button v-if="!answerVisible" class="secondary" :disabled="busy" @click="answerVisible = true">
          Reveal Answer
        </button>

        <template v-else>
          <button class="rating danger" :disabled="busy" @click="onRateCard('again')">Again</button>
          <button class="rating warn" :disabled="busy" @click="onRateCard('hard')">Hard</button>
          <button class="rating good" :disabled="busy" @click="onRateCard('good')">Good</button>
          <button class="rating easy" :disabled="busy" @click="onRateCard('easy')">Easy</button>
        </template>
      </footer>
    </article>

    <article v-else class="panel empty">
      <h2>{{ emptyState.title }}</h2>
      <p>{{ emptyState.message }}</p>
    </article>
  </section>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  generateFlashcards,
  getFlashcards,
  getNotebookTopicTree,
  recordFlashcardReview,
} from '../services/appApi'

const route = useRoute()

const notebookTree = ref([])
const selectedNotebookID = ref('')
const selectedTopicID = ref('')
const cards = ref([])
const currentIndex = ref(0)
const answerVisible = ref(false)
const errorMessage = ref('')
const isGenerating = ref(false)
const isLoadingCards = ref(false)
const isReviewing = ref(false)
const hasPreparedCards = ref(false)

const currentCard = computed(() => cards.value[currentIndex.value] || null)
const selectedNotebook = computed(() =>
  notebookTree.value.find((notebook) => notebook.notebook_id === selectedNotebookID.value) || null
)
const availableTopics = computed(() => selectedNotebook.value?.topics || [])
const currentTopicTitle = computed(() => {
  const match = availableTopics.value.find((topic) => topic.topic_id === selectedTopicID.value)
  return match?.title || 'Topic'
})
const busy = computed(() => isGenerating.value || isLoadingCards.value || isReviewing.value)

const emptyState = computed(() => {
  if (notebookTree.value.length === 0) {
    return {
      title: 'No notebooks yet',
      message: 'Upload a notebook to create topic-scoped flashcards.',
    }
  }
  if (selectedNotebook.value && availableTopics.value.length === 0) {
    return {
      title: 'No topics yet',
      message: 'This notebook has not finished topic extraction yet.',
    }
  }
  if (!selectedTopicID.value) {
    return {
      title: 'Choose a topic',
      message: 'Select a notebook and topic, then prepare cards or start a due review session.',
    }
  }
  if (hasPreparedCards.value) {
    return {
      title: 'All due cards cleared',
      message: 'This topic has no cards due right now. Come back later or review another topic.',
    }
  }
  return {
    title: 'Ready to review',
    message: 'Prepare cards for this topic if it is new, or start review to load cards that are due now.',
  }
})

watch(selectedTopicID, () => {
  resetSession()
  errorMessage.value = ''
})

onMounted(async () => {
  try {
    const data = await getNotebookTopicTree()
    notebookTree.value = Array.isArray(data) ? data : []
    applyInitialSelection(getPreferredTopicID())
  } catch (err) {
    errorMessage.value = err?.message || 'Failed to load notebook topics'
  }
})

function resetSession() {
  cards.value = []
  currentIndex.value = 0
  answerVisible.value = false
  hasPreparedCards.value = false
}

async function onPrepareCards() {
  if (!selectedTopicID.value) {
    return
  }
  isGenerating.value = true
  errorMessage.value = ''
  try {
    const result = await generateFlashcards(selectedTopicID.value)
    if (result?.error) {
      errorMessage.value = result.error
      return
    }
    hasPreparedCards.value = true
    await loadDueCards()
  } catch (err) {
    errorMessage.value = err?.message || 'Failed to prepare flashcards'
  } finally {
    isGenerating.value = false
  }
}

async function loadDueCards() {
  if (!selectedTopicID.value) {
    return
  }
  isLoadingCards.value = true
  errorMessage.value = ''
  try {
    const result = await getFlashcards(selectedTopicID.value, true)
    if (result?.error) {
      errorMessage.value = result.error
      return
    }
    cards.value = Array.isArray(result?.cards) ? result.cards : []
    currentIndex.value = 0
    answerVisible.value = false
    hasPreparedCards.value = true
  } catch (err) {
    errorMessage.value = err?.message || 'Failed to load due flashcards'
  } finally {
    isLoadingCards.value = false
  }
}

async function onRateCard(rating) {
  if (!currentCard.value) {
    return
  }
  isReviewing.value = true
  errorMessage.value = ''
  try {
    const result = await recordFlashcardReview(currentCard.value.id, rating)
    if (result?.error) {
      errorMessage.value = result.error
      return
    }

    cards.value.splice(currentIndex.value, 1)
    if (currentIndex.value >= cards.value.length) {
      currentIndex.value = Math.max(0, cards.value.length - 1)
    }
    answerVisible.value = false
  } catch (err) {
    errorMessage.value = err?.message || 'Failed to record flashcard review'
  } finally {
    isReviewing.value = false
  }
}

function applyInitialSelection(preferredTopicID) {
  if (notebookTree.value.length === 0) {
    selectedNotebookID.value = ''
    selectedTopicID.value = ''
    return
  }

  if (preferredTopicID) {
    for (const notebook of notebookTree.value) {
      const topic = Array.isArray(notebook.topics)
        ? notebook.topics.find((item) => item.topic_id === preferredTopicID)
        : null
      if (topic) {
        selectedNotebookID.value = notebook.notebook_id
        selectedTopicID.value = topic.topic_id
        return
      }
    }
  }

  const firstNotebookWithTopics = notebookTree.value.find(
    (notebook) => Array.isArray(notebook.topics) && notebook.topics.length > 0
  )
  const fallbackNotebook = firstNotebookWithTopics || notebookTree.value[0]
  selectedNotebookID.value = fallbackNotebook?.notebook_id || ''
  selectedTopicID.value = fallbackNotebook?.topics?.[0]?.topic_id || ''
}

function onNotebookChange() {
  const nextTopicID = availableTopics.value[0]?.topic_id || ''
  if (!availableTopics.value.some((topic) => topic.topic_id === selectedTopicID.value)) {
    selectedTopicID.value = nextTopicID
  }
}

function getPreferredTopicID() {
  return typeof route.query.topic === 'string' ? route.query.topic : ''
}
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
  overflow-x: hidden;
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-weight: 700;
}

h1 {
  margin: 0;
  font-size: 46px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 24px;
  width: 100%;
  box-sizing: border-box;
}

.controls {
  display: flex;
  align-items: end;
  gap: 14px;
  flex-wrap: wrap;
}

.field {
  display: grid;
  gap: 8px;
  flex: 1 1 auto;
  min-width: clamp(200px, 100%, 360px);
}

.field span,
.label,
.card-count {
  color: var(--muted-text);
  font-size: 13px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

select {
  border: 1px solid color-mix(in srgb, var(--muted-text) 20%, transparent);
  background: white;
  border-radius: 12px;
  width: 100%;
  box-sizing: border-box;
  padding: 10px 12px;
  font-size: 15px;
}

button {
  border: 0;
  border-radius: 12px;
  padding: 10px 14px;
  font-weight: 700;
  font-size: 14px;
  cursor: pointer;
}

button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.primary {
  background: #20222f;
  color: #fff;
}

.secondary {
  background: var(--surface-container-low);
  color: var(--on-surface);
}

.card-shell {
  display: grid;
  gap: 22px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: start;
}

.card-header h2,
.empty h2 {
  margin: 6px 0 0;
  font-family: 'Manrope', sans-serif;
  font-size: 30px;
  line-height: 1.1;
}

.badge {
  border-radius: 999px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 700;
}

.card-face {
  min-height: 220px;
  border-radius: 22px;
  padding: 24px;
  background:
    radial-gradient(circle at top right, rgba(114, 160, 193, 0.18), transparent 34%),
    linear-gradient(145deg, #f8fbff, #eef2f8);
  display: grid;
  align-content: center;
  gap: 10px;
}

.card-face.revealed {
  background:
    radial-gradient(circle at top right, rgba(126, 180, 116, 0.18), transparent 34%),
    linear-gradient(145deg, #f8fff8, #eef7ef);
}

.content {
  margin: 0;
  color: var(--on-surface);
  font-size: clamp(22px, 3vw, 34px);
  line-height: 1.2;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  justify-content: flex-end;
}

.rating {
  min-width: 92px;
}

.danger {
  background: #ffe8e3;
  color: #8b341f;
}

.warn {
  background: #fff4df;
  color: #8a5811;
}

.good {
  background: #e9f7ef;
  color: #1f6a3a;
}

.easy {
  background: #e6f3ff;
  color: #1c5f8b;
}

.empty p,
.error {
  margin: 0;
  color: var(--muted-text);
}

.error {
  border: 1px solid #f3b5a7;
  background: #fff3ef;
  color: #8a2d16;
}

@media (max-width: 960px) {
  h1 {
    font-size: 38px;
  }

  .card-header {
    grid-template-columns: 1fr;
  }

  .card-face {
    min-height: 180px;
  }
}
</style>
