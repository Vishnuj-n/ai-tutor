<template>
  <article class="panel form-grid">
    <h2>Study Budget & Routine</h2>

    <div class="form-group">
      <label for="max-flashcards">Max Flashcards per Session</label>
      <input
        id="max-flashcards"
        v-model.number="settings.max_flashcards_per_session"
        type="number"
        min="5"
        max="200"
        step="5"
        :disabled="disabled"
        required
      />
      <p class="hint">Caps the number of FSRS reviews active in any single study session.</p>
    </div>

    <TimeRangeInput
      :start-value="settings.study_start_time"
      :end-value="settings.study_end_time"
      :duration="studyDuration"
      :disabled="disabled"
      @update:start-value="settings.study_start_time = $event"
      @update:end-value="settings.study_end_time = $event"
      @apply-preset="applyDurationPreset"
    />

    <SettingsToggle
      v-model="settings.reminders_enabled"
      :disabled="disabled"
      title="Enable Study Reminders"
      hint="Notify when daily study time starts and ends."
    />

    <SettingsToggle
      v-model="settings.analytics_enabled"
      :disabled="disabled"
      title="Help improve the app by sharing anonymous usage data"
      hint="Telemetry events are anonymized. No personal information is ever collected."
    />

    <SettingsToggle
      v-model="settings.skip_to_reading_active"
      :disabled="disabled"
      title="Enable &quot;Skip to Reading&quot; (Escape Hatch)"
      hint="Temporarily deprioritizes review backlogs, letting you read new material first. FSRS records remain safe."
    />
  </article>
</template>

<script setup>
import SettingsToggle from './SettingsToggle.vue'
import TimeRangeInput from './TimeRangeInput.vue'

defineProps({
  settings: { type: Object, required: true },
  studyDuration: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['apply-duration-preset'])

function applyDurationPreset(preset) {
  emit('apply-duration-preset', preset)
}
</script>

<style scoped>
label {
  font-weight: 600;
  font-size: 14px;
  color: var(--on-surface);
}

input[type='number'] {
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 12px 14px;
  font-size: 14px;
  font-family: inherit;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

input[type='number']:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--primary) 15%, transparent);
  outline: none;
}

.hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted-text);
  line-height: 1.4;
}

.form-grid {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

h2 {
  font-size: 20px;
  margin: 0 0 16px;
  font-weight: 700;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 28px;
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  box-shadow: 0 4px 20px color-mix(in srgb, var(--on-surface) 3%, transparent);
}
</style>
