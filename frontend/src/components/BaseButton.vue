<template>
  <button 
    :class="['base-btn', { 'loading': loading }]" 
    :disabled="disabled || loading"
    @click="$emit('click')"
  >
    <span v-if="!loading" class="btn-content">
      <slot />
    </span>
    <span v-else class="spinner"></span>
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
</style>
