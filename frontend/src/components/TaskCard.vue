<template>
  <article class="card task-card">
    <div class="task-header">
      <span class="task-type" :class="task.action_type.toLowerCase()">{{
        formatTaskType(task.action_type)
      }}</span>
      <span
        v-if="task.action_type !== 'flashcard_generate' && task.estimate_minutes > 0"
        class="task-estimate"
        >{{ task.estimate_minutes }} min</span
      >
    </div>
    <!-- ponytail: if reading task, show Continue Reading heading prefix -->
    <h3 v-if="task.action_type === 'READING' || task.action_type === 'REREAD'">
      Continue Reading: {{ task.title }}
    </h3>
    <h3 v-else>{{ task.title }}</h3>
    <p class="task-meta">
      {{
        task.meta
          ? task.meta
          : task.start_page !== undefined &&
              task.start_page !== null &&
              task.end_page !== undefined &&
              task.end_page !== null
            ? 'Pages ' + task.start_page + '-' + task.end_page
            : 'Pages N/A'
      }}
    </p>
    <button
      type="button"
      class="primary-btn"
      :class="{ 'sync-btn': task.action_type === 'flashcard_generate' }"
      :aria-label="'Start task ' + (task.title || task.id)"
      :disabled="task.action_type === 'flashcard_generate' && isSyncing"
      @click="$emit('start', task)"
    >
      <span v-if="task.action_type === 'flashcard_generate' && isSyncing">Generating...</span>
      <span v-else-if="task.action_type === 'flashcard_generate'">Generate</span>
      <span v-else-if="task.action_type === 'READING' || task.action_type === 'REREAD'">Resume</span>
      <span v-else>Start</span>
    </button>
  </article>
</template>

<script setup>
import { formatTaskType } from '../utils/dateFormat'

defineProps({
  task: { type: Object, required: true },
  isSyncing: { type: Boolean, default: false },
})

defineEmits(['start'])
</script>

<style scoped>
.card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 16px;
}

.task-card {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  transition:
    transform 0.2s,
    border-color 0.2s;
}

.task-card:hover {
  transform: translateY(-2px);
  border-color: var(--primary);
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.task-type {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--primary);
  background: rgba(108, 92, 231, 0.1);
  padding: 4px 8px;
  border-radius: 6px;
}

.task-estimate {
  font-size: 12px;
  color: var(--muted-text);
}

.task-card h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  line-height: 1.3;
}

.task-meta {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
  flex: 1;
}

.primary-btn {
  background: var(--primary);
  color: var(--on-primary);
  border: none;
  border-radius: 10px;
  padding: 10px;
  font-weight: 700;
  cursor: pointer;
  transition: opacity 0.2s;
}

.primary-btn:hover {
  opacity: 0.9;
}

.task-type.flashcard_generate {
  color: #c0392b;
  background: rgba(192, 41, 43, 0.1);
}

.task-type.socratic_remedial {
  color: #d35400;
  background: rgba(211, 84, 0, 0.1);
}

.primary-btn.sync-btn {
  background: linear-gradient(135deg, #c0392b, #e74c3c);
  box-shadow: 0 4px 10px rgba(192, 41, 43, 0.15);
}
</style>
