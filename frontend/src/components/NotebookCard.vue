<template>
  <div class="notebook-card" :class="variantClass">
    <button
      class="btn-edit-pen"
      title="Edit notebook and chapters"
      @click="$emit('edit-syllabus', notebook.id, notebook.title)"
    >
      ✎
    </button>

    <div class="notebook-header-card">
      <div class="file-icon" :class="{ 'active-icon': variant === 'active' }">
        {{ fileIcon }}
      </div>
      <div class="notebook-info">
        <h3>{{ notebook.title }}</h3>
        <p class="meta">{{ notebook.file_type.toUpperCase() }}</p>
        <p v-if="notebook.page_count > 0" class="meta">{{ notebook.page_count }} pages</p>
        <p class="meta">{{ notebook.chunk_count }} chunks</p>
        <p v-if="variant === 'dormant'" class="meta">Status: {{ formattedStatus }}</p>
      </div>
    </div>

    <div
      v-if="notebook.topic_id"
      class="notebook-topic"
      style="display: flex; align-items: center; gap: 8px"
    >
      <span class="badge">{{ topicTitle }}</span>
      <RouterLink
        v-if="ragEnabled && ragNotebookChapter"
        :to="`/tutor?topic_id=${notebook.topic_id}&notebook_id=${notebook.id}`"
        class="tutor-link-btn"
        title="Ask Tutor (RAG)"
        style="text-decoration: none; font-size: 13px"
      >
        ◎ Ask Tutor
      </RouterLink>
    </div>

    <div v-else-if="variant === 'dormant'" class="notebook-topic">
      <span class="badge muted">No topic linked</span>
    </div>

    <div class="notebook-priority">
      <label class="priority-label">Priority:</label>
      <select
        :value="notebook.priority || 5"
        class="priority-select"
        @change="(e) => $emit('update-priority', notebook.id, Number.parseInt(e.target.value))"
      >
        <option v-for="n in 10" :key="n" :value="n">{{ n }}</option>
      </select>
    </div>

    <div class="notebook-date">Uploaded: {{ formattedDate }}</div>

    <div class="notebook-actions">
      <button
        v-if="variant === 'active'"
        class="btn-sleep"
        @click="$emit('change-status', notebook.id, 'dormant')"
      >
        Sleep
      </button>
      <button
        v-if="variant === 'dormant'"
        class="btn-activate"
        :disabled="activeLimitReached"
        @click="$emit('change-status', notebook.id, 'active')"
      >
        Activate
      </button>
      <button class="btn-delete" @click="$emit('delete', notebook.id)">Delete</button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  notebook: { type: Object, required: true },
  availableTopics: { type: Array, default: () => [] },
  ragEnabled: { type: Boolean, default: false },
  ragNotebookChapter: { type: Boolean, default: true },
  variant: {
    type: String,
    default: 'dormant',
    validator: (v) => ['active', 'dormant'].includes(v),
  },
  activeLimitReached: { type: Boolean, default: false },
})

defineEmits(['edit-syllabus', 'update-priority', 'change-status', 'delete'])

const FILE_ICONS = { pdf: '📕', txt: '📄', md: '📝' }

const fileIcon = computed(() => FILE_ICONS[props.notebook.file_type] || '📄')

const topicTitle = computed(() => {
  const topic = props.availableTopics.find((t) => t.id === props.notebook.topic_id)
  return topic ? topic.title : 'No topic'
})

const formattedStatus = computed(() => {
  const status = props.notebook.status
  if (!status) return 'uploaded'
  return status.replaceAll('_', ' ')
})

const formattedDate = computed(() => {
  return new Date(props.notebook.uploaded_at).toLocaleDateString()
})

const variantClass = computed(() =>
  props.variant === 'active' ? 'active-notebook-card' : 'dormant-notebook-card'
)
</script>

<style scoped>
.notebook-card {
  background: var(--surface-container);
  border-radius: 12px;
  padding: 16px;
  border: 1px solid var(--outline-variant);
  transition: all 0.2s;
  position: relative;
}

.notebook-card:hover {
  box-shadow: 0 2px 8px rgba(45, 51, 56, 0.06);
}

.notebook-header-card {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.btn-edit-pen {
  position: absolute;
  top: 10px;
  right: 10px;
  border: 0;
  border-radius: 8px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  width: 30px;
  height: 30px;
  font-size: 15px;
  cursor: pointer;
}

.btn-edit-pen:hover {
  background: var(--surface-container-high, #e6e9ef);
}

.file-icon {
  font-size: 28px;
  flex-shrink: 0;
}

.notebook-info h3 {
  margin: 0;
  font-size: 16px;
  color: var(--on-surface);
  word-break: break-word;
}

.meta {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted-text);
}

.notebook-topic {
  margin-bottom: 12px;
}

.badge {
  display: inline-block;
  background: var(--surface-container-low);
  color: var(--primary);
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
}

.badge.muted {
  color: var(--muted-text);
}

.notebook-date {
  font-size: 12px;
  color: var(--muted-text);
  margin-bottom: 12px;
}

.notebook-priority {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.priority-label {
  font-size: 12px;
  color: var(--muted-text);
}

.priority-select {
  padding: 4px 8px;
  border: 1px solid var(--outline-variant);
  border-radius: 4px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  font-size: 12px;
  cursor: pointer;
}

.priority-select:hover {
  border-color: var(--primary);
}

.notebook-actions {
  display: flex;
  gap: 8px;
}

.btn-delete {
  flex: 1;
  padding: 8px 12px;
  border: none;
  border-radius: 6px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  font-weight: 600;
  background: #ffe9e8;
  color: #b5423d;
}

.btn-delete:hover {
  opacity: 0.9;
}

/* ── Active variant ────────────────────────────────── */
.active-notebook-card {
  border-color: var(--primary);
  box-shadow:
    0 0 0 1px var(--primary),
    0 4px 12px rgba(108, 92, 231, 0.15);
}

.active-icon {
  color: var(--primary);
}

/* ── Action buttons ────────────────────────────────── */
.btn-activate {
  background: linear-gradient(135deg, var(--primary), #7c3aed);
  color: white;
  border: none;
  border-radius: 8px;
  padding: 8px 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-activate:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(108, 92, 231, 0.3);
}

.btn-activate:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-sleep {
  background: linear-gradient(135deg, #f59e0b, #d97706);
  color: white;
  border: none;
  border-radius: 8px;
  padding: 8px 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-sleep:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(245, 158, 11, 0.3);
}
</style>
