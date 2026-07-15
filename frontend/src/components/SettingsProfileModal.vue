<template>
  <div class="modal-overlay" @click.self="$emit('close')">
    <div class="modal-card">
      <h2>{{ isEdit ? 'Edit Profile' : 'New Study Profile' }}</h2>
      <div class="form-group">
        <label>Profile Name</label>
        <input
          v-model="localName"
          type="text"
          :placeholder="isEdit ? '' : 'e.g. UPSC, Semester Finals'"
        />
      </div>
      <div class="form-group">
        <label>Target Deadline</label>
        <input v-model="localDeadline" type="date" />
      </div>
      <div class="modal-actions">
        <button class="cancel-btn" @click="$emit('close')">Cancel</button>
        <button
          class="save-btn"
          :disabled="!localName || !localDeadline"
          @click="$emit('save', { name: localName, deadline: localDeadline })"
        >
          {{ isEdit ? 'Save Changes' : 'Create Profile' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  isEdit: { type: Boolean, default: false },
  initialName: { type: String, default: '' },
  initialDeadline: { type: String, default: '' },
})

defineEmits(['close', 'save'])

const localName = ref(props.initialName)
const localDeadline = ref(props.initialDeadline)

watch(
  () => props.initialName,
  (v) => {
    localName.value = v
  }
)
watch(
  () => props.initialDeadline,
  (v) => {
    localDeadline.value = v
  }
)
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
}

.modal-card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 20px;
  padding: 24px;
  width: 100%;
  max-width: 400px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.2);
}

.modal-card h2 {
  margin: 0;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 10px;
}

label {
  font-weight: 600;
  font-size: 14px;
  color: var(--on-surface);
}

.cancel-btn {
  background: none;
  border: 1px solid var(--outline-variant);
  padding: 10px 20px;
  border-radius: 10px;
  font-weight: 700;
  cursor: pointer;
  color: var(--on-surface);
}

.cancel-btn:hover {
  background: var(--surface-container-low);
}

.save-btn {
  border: 0;
  border-radius: 12px;
  padding: 12px 24px;
  color: var(--on-primary);
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.save-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px color-mix(in srgb, var(--primary) 25%, transparent);
}

.save-btn:active {
  transform: translateY(0);
}

.save-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
