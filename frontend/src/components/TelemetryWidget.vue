<template>
  <section v-if="pace && pace.has_deadline" class="telemetry-widget" style="margin-top: 24px">
    <div class="telemetry-card card">
      <h2 class="telemetry-header">Profile Study Pacing ({{ profileName }})</h2>
      <div class="telemetry-grid">
        <div class="telemetry-item">
          <div class="telemetry-title-row">
            <span class="telemetry-doc-title">Target Exam Deadline: {{ pace.deadline }}</span>
            <span class="telemetry-days-left" :class="{ warning: pace.days_remaining <= 3 }">
              ({{ formatDaysRemaining(pace.days_remaining) }})
            </span>
          </div>
          <div class="telemetry-metric-row">
            <div class="telemetry-metric">
              <span class="metric-value">{{ pace.daily_pace }}</span>
              <span class="metric-label">words / day</span>
            </div>
            <div class="telemetry-metric">
              <span class="metric-value">{{ sessionsPerDay }}</span>
              <span class="metric-label">sessions / day</span>
            </div>
            <div class="telemetry-progress-info">
              <div class="progress-details">
                <span
                  >Remaining words: <strong>{{ pace.remaining_words }}</strong></span
                >
              </div>
              <div v-if="paceLabel" class="pace-label">
                {{ paceLabel }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { formatDaysRemaining } from '../utils/dateFormat'

const props = defineProps({
  pace: { type: Object, default: null },
  profileName: { type: String, default: 'Unknown' },
})

const sessionsPerDay = computed(() => {
  if (!props.pace) return 0
  return Math.ceil(props.pace.sessions_per_day)
})

const paceLabel = computed(() => {
  if (!props.pace) return ''
  return props.pace.pace_label || ''
})
</script>

<style scoped>
.card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 16px;
}

.telemetry-widget {
  margin-bottom: 8px;
}

.telemetry-card {
  padding: 24px;
}

.telemetry-header {
  margin: 0 0 16px;
  font-size: 18px;
  font-weight: 700;
}

.telemetry-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.telemetry-item {
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  padding: 16px;
  background: var(--surface-container-low);
}

.telemetry-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 12px;
}

.telemetry-doc-title {
  flex: 1;
}

.telemetry-days-left {
  font-size: 12px;
  color: var(--muted-text);
}

.telemetry-days-left.warning {
  color: #eb5e55;
  font-weight: 700;
}

.telemetry-metric-row {
  display: flex;
  align-items: center;
  gap: 24px;
}

.telemetry-metric {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.metric-value {
  font-size: 28px;
  font-family: 'Manrope', sans-serif;
  font-weight: 800;
  color: var(--primary);
  line-height: 1;
}

.metric-label {
  font-size: 12px;
  color: var(--muted-text);
  font-weight: 600;
}

.telemetry-progress-info {
  flex: 1;
  text-align: right;
  font-size: 13px;
  color: var(--muted-text);
}

.pace-label {
  margin-top: 4px;
  font-size: 12px;
  font-weight: 600;
  color: var(--primary);
}
</style>
