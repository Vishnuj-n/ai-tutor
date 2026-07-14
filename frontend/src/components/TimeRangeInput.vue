<template>
  <div class="time-range-section">
    <div class="time-range-header">
      <label>Study Schedule</label>
      <span v-if="duration" class="duration-badge">{{ duration }}</span>
    </div>

    <div class="time-range-container">
      <div class="time-input-group">
        <label for="study-start-time" class="time-label">Start</label>
        <div class="time-input-wrapper">
          <svg class="time-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="2" x2="12" y2="4"/>
            <line x1="22" y1="12" x2="20" y2="12"/>
            <line x1="12" y1="20" x2="12" y2="22"/>
            <line x1="2" y1="12" x2="4" y2="12"/>
            <line x1="12" y1="4" x2="12" y2="8"/>
            <line x1="12" y1="12" x2="12" y2="8"/>
            <line x1="12" y1="12" x2="15.5" y2="14.5"/>
            <circle cx="12" cy="12" r="1" fill="currentColor"/>
          </svg>
          <input
            id="study-start-time"
            :value="startValue"
            type="time"
            class="time-input"
            :disabled="disabled"
            required
            @input="$emit('update:startValue', $event.target.value)"
          />
        </div>
      </div>

      <div class="time-connector">
        <svg viewBox="0 0 24 8" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M0 4 L20 4 M16 1 L20 4 L16 7"/>
        </svg>
      </div>

      <div class="time-input-group">
        <label for="study-end-time" class="time-label">End</label>
        <div class="time-input-wrapper">
          <svg class="time-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="2" x2="12" y2="4"/>
            <line x1="22" y1="12" x2="20" y2="12"/>
            <line x1="12" y1="20" x2="12" y2="22"/>
            <line x1="2" y1="12" x2="4" y2="12"/>
            <line x1="12" y1="4" x2="12" y2="8"/>
            <line x1="12" y1="12" x2="12" y2="8"/>
            <line x1="12" y1="12" x2="15.5" y2="14.5"/>
            <circle cx="12" cy="12" r="1" fill="currentColor"/>
          </svg>
          <input
            id="study-end-time"
            :value="endValue"
            type="time"
            class="time-input"
            :disabled="disabled"
            required
            @input="$emit('update:endValue', $event.target.value)"
          />
        </div>
      </div>
    </div>

    <div class="quick-durations">
      <button
        v-for="preset in durationPresets"
        :key="preset.label"
        type="button"
        class="duration-preset"
        :class="{ active: duration === preset.label }"
        :disabled="disabled"
        @click="$emit('apply-preset', preset)"
      >
        {{ preset.label }}
      </button>
    </div>
  </div>
</template>

<script setup>
defineProps({
  startValue: { type: String, required: true },
  endValue: { type: String, required: true },
  duration: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
})

defineEmits(['update:startValue', 'update:endValue', 'apply-preset'])

const durationPresets = [
  { label: '30 min', minutes: 30 },
  { label: '1 hour', minutes: 60 },
  { label: '1.5 hours', minutes: 90 },
  { label: '2 hours', minutes: 120 },
  { label: '3 hours', minutes: 180 },
]
</script>

<style scoped>
label {
  font-weight: 600;
  font-size: 14px;
  color: var(--on-surface);
}

.duration-badge {
  background: var(--primary);
  color: var(--on-primary);
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 700;
}

.duration-preset:hover:not(:disabled) {
  background: var(--surface-container);
  border-color: color-mix(in srgb, var(--primary) 30%, transparent);
  color: var(--on-surface);
}

.duration-preset:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

@media (max-width: 480px) {
  .time-range-container {
    flex-direction: column;
    align-items: stretch;
  }

  .time-connector {
    transform: rotate(90deg);
    width: 100%;
    height: 24px;
  }

  .quick-durations {
    justify-content: center;
  }
}
</style>
