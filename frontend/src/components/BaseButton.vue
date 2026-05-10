<template>
  <button
    :class="['base-btn', { loading }]"
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
  disabled: { type: Boolean, default: false },
  loading:  { type: Boolean, default: false },
})

defineEmits(['click'])
</script>

<style scoped>
.base-btn {
  padding: 10px 24px;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  color: var(--on-primary);
  border: 0;
  border-radius: 12px;
  font-family: inherit;
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  transition: transform 0.14s ease, filter 0.14s ease;
  white-space: nowrap;
}

.base-btn:hover:not(:disabled) {
  filter: brightness(1.08);
}

.base-btn:active:not(:disabled) {
  transform: scale(0.96);
}

.base-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.35);
  border-top-color: currentColor;
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
