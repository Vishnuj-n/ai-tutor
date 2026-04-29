<template>
  <button 
    :class="['base-btn', { 'loading': loading }]" 
    :disabled="disabled || loading"
    :aria-busy="loading"
    @click="$emit('click')"
  >
    <span :class="['btn-content', { 'visually-hidden': loading }]">
      <slot />
    </span>
    <span v-if="loading" class="spinner" aria-hidden="true"></span>
  </button>
</template>

<script setup>
defineProps({
  disabled: {
    type: Boolean,
    default: false
  },
  loading: {
    type: Boolean,
    default: false
  }
})

defineEmits(['click'])
</script>

<style scoped>
.base-btn {
  padding: 0.5rem 1rem;
  background: var(--primary);
  color: white;
  border: none;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  transition: background 0.15s ease;
  white-space: nowrap;
}

.base-btn:hover:not(:disabled) {
  background: #0056b3;
}

.base-btn:active:not(:disabled) {
  background: #004494;
}

.base-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.spinner {
  width: 14px;
  height: 14px;
  border: 2px solid transparent;
  border-top: 2px solid currentColor;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.visually-hidden {
  position: absolute !important;
  width: 1px !important;
  height: 1px !important;
  padding: 0 !important;
  margin: -1px !important;
  overflow: hidden !important;
  clip: rect(0, 0, 0, 0) !important;
  white-space: nowrap !important;
  border: 0 !important;
}
</style>
