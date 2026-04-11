<template>
  <section class="page">
    <div>
      <p class="eyebrow">Reader</p>
      <h1>{{ topicTitle }}</h1>
      <p class="muted">{{ sectionCount }} section(s) available</p>
    </div>

    <div class="split">
      <article class="panel content-panel">
        <h2>Content</h2>
        <div v-if="loading" class="loading">Loading content...</div>
        <div v-else-if="error" class="error">{{ error }}</div>
        <div v-else>
          <div v-for="section in sections" :key="section.id" class="section">
            <h3>{{ section.heading }}</h3>
            <p>{{ section.content }}</p>
          </div>
        </div>
      </article>

      <article class="panel">
        <h2>Ask AI</h2>
        <textarea
          v-model="question"
          placeholder="Ask for concept clarification in this topic..."
          :disabled="aiLoading || !topicID"
        ></textarea>
        <button
          type="button"
          class="primary-btn"
          :disabled="aiLoading || !question.trim() || !topicID"
          @click="askAI"
        >
          {{ aiLoading ? 'Asking...' : 'Ask' }}
        </button>

        <div v-if="aiResponse" class="response">
          <h3>Response</h3>
          <p class="answer">{{ aiResponse.answer }}</p>
          <div
            v-if="aiResponse.cited_sections && aiResponse.cited_sections.length > 0"
            class="citations"
          >
            <p class="citation-label">Based on:</p>
            <ul>
              <li v-for="(section, idx) in aiResponse.cited_sections" :key="idx">
                {{ section }}
              </li>
            </ul>
          </div>
        </div>

        <div v-if="aiError" class="error-box">
          {{ aiError }}
        </div>
      </article>
    </div>
  </section>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { askAI as askAIRequest, getTopicContent } from '../services/appApi'

const route = useRoute()

const topicID = ref(route.query.topic || 'os-scheduling')
const topicTitle = ref('Loading...')
const sections = ref([])
const sectionCount = ref(0)
const question = ref('')
const aiResponse = ref(null)
const aiError = ref('')
const loading = ref(true)
const aiLoading = ref(false)
const error = ref('')

onMounted(async () => {
  await loadTopicContent()
})

async function loadTopicContent() {
  try {
    loading.value = true
    const content = await getTopicContent(topicID.value)

    if (content.error) {
      error.value = content.error
      return
    }

    topicTitle.value = content.title
    sections.value = content.sections || []
    sectionCount.value = content.sections?.length || 0
  } catch (err) {
    error.value = `Failed to load content: ${err.message}`
  } finally {
    loading.value = false
  }
}

async function askAI() {
  if (!question.value.trim()) return

  try {
    aiLoading.value = true
    aiError.value = ''
    aiResponse.value = null

    const result = await askAIRequest(topicID.value, question.value)

    if (result.error) {
      aiError.value = result.error
      return
    }

    aiResponse.value = result
  } catch (err) {
    aiError.value = `Error: ${err.message}`
  } finally {
    aiLoading.value = false
  }
}
</script>

<style scoped>
.page {
  display: grid;
  gap: 24px;
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
  margin: 8px 0 0;
  font-size: 46px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

h2 {
  margin: 0 0 16px;
  font-size: 30px;
  font-family: 'Manrope', sans-serif;
}

h3 {
  margin: 16px 0 8px;
  font-size: 18px;
  font-family: 'Manrope', sans-serif;
  font-weight: 600;
}

h3:first-child {
  margin-top: 0;
}

.muted {
  margin: 12px 0 0;
  color: var(--muted-text);
}

.split {
  display: grid;
  grid-template-columns: 1.6fr 1fr;
  gap: 16px;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 24px;
}

.content-panel {
  max-height: 600px;
  overflow-y: auto;
}

.section {
  margin-bottom: 20px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--surface-container-low);
}

.section:last-child {
  border-bottom: none;
  margin-bottom: 0;
  padding-bottom: 0;
}

.section p {
  margin: 8px 0 0;
  line-height: 1.6;
  font-size: 14px;
}

textarea {
  width: 100%;
  min-height: 100px;
  border: 0;
  outline: 0;
  border-radius: 12px;
  background: var(--surface-container-low);
  padding: 12px;
  font-family: inherit;
  color: var(--on-surface);
  resize: vertical;
  font-size: 14px;
}

textarea:disabled {
  opacity: 0.6;
}

.primary-btn {
  margin-top: 14px;
  border: 0;
  border-radius: 12px;
  padding: 12px 20px;
  color: var(--on-primary);
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  cursor: pointer;
  transition: opacity 0.2s;
}

.primary-btn:hover:not(:disabled) {
  opacity: 0.9;
}

.primary-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.response {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid var(--surface-container-low);
}

.response h3 {
  margin-top: 0;
}

.answer {
  margin: 8px 0 0;
  line-height: 1.6;
  font-size: 14px;
  color: var(--on-surface);
}

.citations {
  margin-top: 12px;
  padding: 12px;
  background: var(--surface-container-low);
  border-radius: 8px;
}

.citation-label {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-text);
}

.citations ul {
  margin: 0;
  padding-left: 20px;
  list-style: disc;
}

.citations li {
  margin: 4px 0;
  font-size: 13px;
  color: var(--on-surface);
}

.error,
.error-box {
  padding: 12px;
  background: var(--surface-container-low);
  color: #c53030;
  border-radius: 8px;
  font-size: 14px;
}

.error-box {
  margin-top: 20px;
}

.loading {
  padding: 20px;
  text-align: center;
  color: var(--muted-text);
  font-size: 14px;
}

@media (max-width: 960px) {
  .split {
    grid-template-columns: 1fr;
  }

  h1 {
    font-size: 34px;
  }

  .content-panel {
    max-height: none;
  }
}
</style>
