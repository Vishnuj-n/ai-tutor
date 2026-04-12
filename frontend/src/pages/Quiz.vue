<template>
  <section class="page">
    <p class="eyebrow">Quiz</p>
    <h1>Topic Quiz</h1>

    <article class="panel controls">
      <label class="field">
        <span>Topic</span>
        <select v-model="selectedTopicID">
          <option disabled value="">Select a topic</option>
          <option v-for="topic in topics" :key="topic.id" :value="topic.id">
            {{ topic.title }}
          </option>
        </select>
      </label>

      <button class="primary" :disabled="isGenerating || !selectedTopicID" @click="onGenerateQuiz">
        {{ isGenerating ? 'Generating...' : 'Generate Quiz' }}
      </button>
    </article>

    <article v-if="errorMessage" class="panel error">{{ errorMessage }}</article>

    <article v-if="currentQuestion" class="panel question-card">
      <header>
        <p class="question-index">Question {{ currentIndex + 1 }} / {{ questions.length }}</p>
        <h2>{{ currentQuestion.prompt }}</h2>
      </header>

      <div class="options" v-if="!feedbackVisible">
        <label v-for="(opt, idx) in currentQuestion.options" :key="opt + idx" class="option">
          <input type="radio" :value="opt" v-model="selectedAnswer" />
          <span>{{ String.fromCharCode(65 + idx) }}. {{ opt }}</span>
        </label>
      </div>

      <div v-if="feedbackVisible && scoreResult" class="feedback" :class="scoreResult.correct ? 'good' : 'bad'">
        <h3>{{ scoreResult.correct ? 'Correct' : 'Not quite' }} · Score {{ scoreResult.score }}</h3>
        <p>{{ scoreResult.feedback }}</p>
        <p class="hint">Hint: {{ scoreResult.hint }}</p>
        <p v-if="!scoreResult.correct" class="expected">Expected: {{ scoreResult.expected }}</p>
      </div>

      <footer class="actions">
        <button
          v-if="!feedbackVisible"
          class="primary"
          :disabled="isScoring || !selectedAnswer"
          @click="onSubmitAnswer"
        >
          {{ isScoring ? 'Scoring...' : 'Submit Answer' }}
        </button>

        <button
          v-if="feedbackVisible"
          class="primary"
          :disabled="currentIndex >= questions.length - 1"
          @click="onNext"
        >
          {{ currentIndex >= questions.length - 1 ? 'Quiz Complete' : 'Next Question' }}
        </button>
      </footer>
    </article>

    <article v-else-if="questions.length === 0" class="panel">
      <h2>Ready to generate</h2>
      <p>Select a topic and generate a quiz to begin.</p>
    </article>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { generateQuiz, getAvailableTopics, scoreAnswer } from '../services/appApi'

const topics = ref([])
const selectedTopicID = ref('')
const questions = ref([])
const currentIndex = ref(0)
const selectedAnswer = ref('')
const scoreResult = ref(null)
const feedbackVisible = ref(false)
const errorMessage = ref('')
const isGenerating = ref(false)
const isScoring = ref(false)

const currentQuestion = computed(() => questions.value[currentIndex.value] || null)

onMounted(async () => {
  try {
    const data = await getAvailableTopics()
    topics.value = Array.isArray(data) ? data : []
    if (topics.value.length > 0) {
      selectedTopicID.value = topics.value[0].id
    }
  } catch (err) {
    errorMessage.value = err?.message || 'Failed to load topics'
  }
})

async function onGenerateQuiz() {
  if (!selectedTopicID.value) {
    return
  }
  isGenerating.value = true
  errorMessage.value = ''
  scoreResult.value = null
  feedbackVisible.value = false
  try {
    const result = await generateQuiz(selectedTopicID.value)
    if (result?.error) {
      errorMessage.value = result.error
      questions.value = []
      return
    }
    questions.value = Array.isArray(result?.questions) ? result.questions : []
    currentIndex.value = 0
    selectedAnswer.value = ''
    scoreResult.value = null
    feedbackVisible.value = false
    if (questions.value.length === 0) {
      errorMessage.value = 'No quiz questions were generated for this topic.'
    }
  } catch (err) {
    errorMessage.value = err?.message || 'Quiz generation failed'
  } finally {
    isGenerating.value = false
  }
}

async function onSubmitAnswer() {
  if (!currentQuestion.value || !selectedAnswer.value) {
    return
  }
  isScoring.value = true
  errorMessage.value = ''
  try {
    const result = await scoreAnswer(currentQuestion.value.id, selectedAnswer.value)
    if (result?.error) {
      errorMessage.value = result.error
      return
    }
    scoreResult.value = result
    feedbackVisible.value = true
  } catch (err) {
    errorMessage.value = err?.message || 'Failed to score answer'
  } finally {
    isScoring.value = false
  }
}

function onNext() {
  if (currentIndex.value < questions.value.length - 1) {
    currentIndex.value += 1
    selectedAnswer.value = ''
    scoreResult.value = null
    feedbackVisible.value = false
  }
}
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
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
  min-width: min(420px, 100%);
}

.field span {
  color: var(--muted-text);
  font-size: 13px;
}

select {
  border: 1px solid color-mix(in srgb, var(--muted-text) 20%, transparent);
  background: white;
  border-radius: 12px;
  padding: 10px 12px;
  font-size: 15px;
}

.primary {
  border: 0;
  border-radius: 12px;
  padding: 10px 16px;
  background: #20222f;
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

h2 {
  margin: 0 0 8px;
  font-family: 'Manrope', sans-serif;
}

p {
  margin: 0;
  color: var(--muted-text);
}

.question-card {
  display: grid;
  gap: 18px;
}

.question-index {
  margin-bottom: 6px;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.12em;
}

.options {
  display: grid;
  gap: 10px;
}

.option {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  border: 1px solid color-mix(in srgb, var(--muted-text) 20%, transparent);
  border-radius: 12px;
  padding: 10px 12px;
}

.feedback {
  border-radius: 12px;
  padding: 14px;
  display: grid;
  gap: 8px;
}

.feedback.good {
  background: #e8f7ee;
}

.feedback.bad {
  background: #fff1ee;
}

.feedback h3 {
  margin: 0;
}

.hint,
.expected {
  color: #2f334a;
}

.actions {
  display: flex;
  justify-content: flex-end;
}

.error {
  border: 1px solid #f3b5a7;
  background: #fff3ef;
  color: #8a2d16;
}
</style>
