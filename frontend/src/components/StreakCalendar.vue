<template>
  <section class="streak-calendar-widget">
    <div v-if="streakError" class="streak-error" style="color: var(--danger); font-size: 0.85rem; padding: 1rem; text-align: center;">
      {{ streakError }}
    </div>
    <template v-else>
      <header class="streak-header-row">
        <div class="streak-title-container">
          <span class="streak-flame-icon" :class="{ active: streakState.today_completed }">🔥</span>
          <div class="streak-counts">
            <span class="streak-count-val">{{ streakState.current_streak }}</span>
            <span class="streak-count-label">day streak</span>
          </div>
        </div>
        <div class="streak-stats-row">
          <span class="streak-stat-item">Longest: <strong>{{ streakState.longest_streak }}d</strong></span>
        </div>
      </header>

      <div class="calendar-month-title">{{ monthLabel }}</div>
      
      <div class="calendar-grid">
        <!-- Weekday Headers -->
        <span v-for="day in ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']" :key="day" class="calendar-header-cell">
          {{ day }}
        </span>
        
        <!-- Empty Cells for Padding -->
        <div 
          v-for="(_, index) in calendarDays.filter(d => d.dayNum === null)" 
          :key="'empty-' + index" 
          class="calendar-day-cell empty"
        ></div>
        
        <!-- Actual Day Cells -->
        <div
          v-for="day in calendarDays.filter(d => d.dayNum !== null)"
          :key="day.dayNum"
          class="calendar-day-cell"
          :class="{ 
            active: day.active, 
            today: day.today,
            'has-hover': day.active
          }"
        >
          <span class="day-number">{{ day.dayNum }}</span>
          <span v-if="day.active" class="active-indicator"></span>
          
          <!-- Tooltip for active days -->
          <div v-if="day.active" class="cell-tooltip">
            Activity logged! Streak kept active.
          </div>
        </div>
      </div>
    </template>
  </section>
</template>

<script setup>
defineProps({
  streakState: {
    type: Object,
    default: () => ({ current_streak: 0, longest_streak: 0, active_dates: [], today_completed: false }),
  },
  streakError: { type: String, default: '' },
  calendarDays: { type: Array, default: () => [] },
  monthLabel: { type: String, default: '' },
})
</script>

<style scoped>
.streak-calendar-widget {
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: var(--surface-container-low);
  border: 1px solid var(--outline-variant);
  border-radius: 16px;
}

.streak-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--outline-variant);
  padding-bottom: 12px;
}

.streak-title-container {
  display: flex;
  align-items: center;
  gap: 10px;
}

.streak-flame-icon {
  font-size: 24px;
  display: inline-block;
  transition: transform 0.3s ease;
}

.streak-flame-icon.active {
  animation: pulse-flame 2s infinite ease-in-out;
}

@keyframes pulse-flame {
  0% { transform: scale(1); filter: drop-shadow(0 0 2px rgba(230, 126, 34, 0.4)); }
  50% { transform: scale(1.15); filter: drop-shadow(0 0 8px rgba(230, 126, 34, 0.8)); }
  100% { transform: scale(1); filter: drop-shadow(0 0 2px rgba(230, 126, 34, 0.4)); }
}

.streak-counts {
  display: flex;
  flex-direction: column;
}

.streak-count-val {
  font-family: 'Manrope', sans-serif;
  font-size: 20px;
  font-weight: 800;
  line-height: 1.1;
  color: var(--on-surface);
}

.streak-count-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-text);
}

.streak-stats-row {
  font-size: 12px;
  color: var(--muted-text);
}

.streak-stats-row strong {
  color: var(--on-surface);
}

.calendar-month-title {
  font-family: 'Manrope', sans-serif;
  font-size: 14px;
  font-weight: 700;
  color: var(--on-surface);
  margin-bottom: -4px;
  text-align: left;
}

.calendar-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 4px;
  width: 100%;
}

.calendar-header-cell {
  font-size: 11px;
  font-weight: 700;
  text-align: center;
  color: var(--muted-text);
  padding: 4px 0;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.calendar-day-cell {
  position: relative;
  aspect-ratio: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 8px;
  cursor: default;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.calendar-day-cell.empty {
  background: transparent;
  border-color: transparent;
  pointer-events: none;
}

.day-number {
  font-size: 12px;
  font-weight: 600;
  color: var(--on-surface);
  z-index: 1;
}

.calendar-day-cell.today {
  border-color: var(--primary);
  box-shadow: inset 0 0 0 1px var(--primary);
}

.calendar-day-cell.today .day-number {
  color: var(--primary);
  font-weight: 800;
}

/* Active Completed State */
.calendar-day-cell.active {
  background: var(--primary-container);
  border-color: var(--primary);
}

.calendar-day-cell.active .day-number {
  color: var(--on-primary-container);
  font-weight: 700;
}

.active-indicator {
  position: absolute;
  bottom: 4px;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background-color: var(--primary);
}

/* Hover and Tooltips */
.calendar-day-cell.has-hover:hover {
  transform: scale(1.12);
  z-index: 10;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
}

.cell-tooltip {
  visibility: hidden;
  opacity: 0;
  position: absolute;
  bottom: 125%;
  left: 50%;
  transform: translateX(-50%) scale(0.9);
  background: var(--surface-container-high);
  color: var(--on-surface);
  font-size: 11px;
  font-weight: 500;
  padding: 6px 10px;
  border-radius: 6px;
  border: 1px solid var(--outline-variant);
  white-space: nowrap;
  box-shadow: 0 4px 10px rgba(0, 0, 0, 0.15);
  z-index: 20;
  pointer-events: none;
  transition: opacity 0.2s ease, transform 0.2s ease, visibility 0.2s;
}

.calendar-day-cell.has-hover:hover .cell-tooltip {
  visibility: visible;
  opacity: 1;
  transform: translateX(-50%) scale(1);
}

/* Arrow for tooltip */
.cell-tooltip::after {
  content: "";
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  border-width: 5px;
  border-style: solid;
  border-color: var(--surface-container-high) transparent transparent transparent;
}
</style>
