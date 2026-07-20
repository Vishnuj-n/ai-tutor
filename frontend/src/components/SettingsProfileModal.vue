<template>
  <div class="modal-overlay" @click.self="$emit('close')">
    <div class="modal-card">
      <div class="modal-header">
        <h2>{{ isEdit ? 'Edit Study Profile' : 'Create Study Profile' }}</h2>
        <button class="close-icon-btn" @click="$emit('close')">&times;</button>
      </div>

      <div class="modal-body">
        <div class="form-group">
          <label for="profile-name">Profile Name</label>
          <input
            id="profile-name"
            v-model="localName"
            type="text"
            :placeholder="isEdit ? '' : 'e.g. UPSC, Semester Finals'"
            required
          />
        </div>

        <div class="form-group">
          <label for="target-deadline">Target Deadline</label>
          <input id="target-deadline" v-model="localDeadline" type="date" required />
        </div>
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
  background: rgba(15, 23, 42, 0.4);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  animation: fadeIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.modal-card {
  background: var(--surface-container-lowest, #ffffff);
  border: 1px solid var(--outline-variant, rgba(0, 0, 0, 0.08));
  border-radius: 24px;
  padding: 28px;
  width: 100%;
  max-width: 440px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  box-shadow: 
    0 4px 6px -1px rgba(0, 0, 0, 0.05),
    0 20px 40px -4px rgba(0, 0, 0, 0.12);
  animation: slideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.modal-header h2 {
  margin: 0;
  font-family: 'Manrope', sans-serif;
  font-size: 22px;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: var(--on-surface);
}

.close-icon-btn {
  background: none;
  border: none;
  font-size: 28px;
  line-height: 1;
  color: var(--muted-text, #64748b);
  cursor: pointer;
  padding: 0 4px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}

.close-icon-btn:hover {
  color: var(--on-surface);
  background: var(--surface-container-low, #f1f5f9);
}

.modal-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

label {
  font-weight: 700;
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-text, #64748b);
}

input {
  font-family: 'Inter', sans-serif;
  font-size: 15px;
  padding: 12px 16px;
  border-radius: 12px;
  background: var(--surface-container-low, #f8fafc);
  border: 1px solid var(--outline-variant, #e2e8f0);
  color: var(--on-surface, #0f172a);
  outline: none;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

input:hover {
  border-color: color-mix(in srgb, var(--primary, #4f46e5) 40%, var(--outline-variant));
}

input:focus {
  border-color: var(--primary, #4f46e5);
  background: var(--surface-container-lowest, #ffffff);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--primary, #4f46e5) 15%, transparent);
}

input::placeholder {
  color: var(--muted-text, #94a3b8);
  opacity: 0.8;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 8px;
}

.cancel-btn {
  background: none;
  border: 1px solid var(--outline-variant, #e2e8f0);
  padding: 12px 20px;
  border-radius: 12px;
  font-weight: 700;
  font-size: 14px;
  cursor: pointer;
  color: var(--on-surface, #0f172a);
  transition: all 0.2s ease;
}

.cancel-btn:hover {
  background: var(--surface-container-low, #f1f5f9);
  border-color: var(--outline-variant);
}

.save-btn {
  border: 0;
  border-radius: 12px;
  padding: 12px 24px;
  color: var(--on-primary, #ffffff);
  font-weight: 700;
  font-size: 14px;
  background: linear-gradient(135deg, var(--primary, #4f46e5), var(--primary-dim, #6366f1));
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: 0 4px 12px color-mix(in srgb, var(--primary, #4f46e5) 20%, transparent);
}

.save-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 6px 16px color-mix(in srgb, var(--primary, #4f46e5) 30%, transparent);
}

.save-btn:active:not(:disabled) {
  transform: translateY(0);
}

.save-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  box-shadow: none;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(16px) scale(0.98);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}
</style>
