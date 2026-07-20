<template>
  <section v-if="timelineData && timelineData.length > 0" class="forecast-widget">
    <div class="forecast-card card">
      <div class="forecast-header-row">
        <div>
          <h2 class="forecast-header">Flashcard Review Forecast</h2>
          <p class="forecast-subtitle">Review load forecast by date</p>
        </div>
        <div class="forecast-legend">
          <span class="legend-item"><span class="legend-dot due-dot"></span>Due Cards</span>
        </div>
      </div>

      <div class="chart-container">
        <svg class="forecast-chart" viewBox="0 0 400 300" preserveAspectRatio="xMidYMid meet">
          <!-- Definitions for Gradients -->
          <defs>
            <linearGradient id="chartGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="var(--primary)" stop-opacity="0.25" />
              <stop offset="100%" stop-color="var(--primary)" stop-opacity="0.0" />
            </linearGradient>
          </defs>

          <!-- Axis Lines -->
          <line x1="30" y1="50" x2="30" y2="250" class="axis-line" />
          <line x1="30" y1="250" x2="370" y2="250" class="axis-line" />

          <!-- Y Axis Labels & Grid Lines -->
          <g v-for="tick in yTicks" :key="tick.value" class="chart-y-tick">
            <!-- Grid line -->
            <line
              v-if="tick.value !== 0"
              x1="30"
              :y1="tick.y"
              x2="370"
              :y2="tick.y"
              class="chart-grid-line"
            />
            <!-- Text label -->
            <text x="22" :y="tick.y + 3.5" class="y-axis-label" text-anchor="end">
              {{ tick.value }}
            </text>
          </g>

          <!-- Shading Area under the curve -->
          <path :d="areaPathData" fill="url(#chartGrad)" />

          <!-- Main Line Path -->
          <path
            :d="linePathData"
            fill="none"
            stroke="var(--primary)"
            stroke-width="2.5"
            stroke-linecap="round"
          />

          <!-- Data Points (interactive dots) -->
          <g v-for="(pt, idx) in chartPoints" :key="idx">
            <circle
              :cx="pt.x"
              :cy="pt.y"
              r="5"
              class="chart-dot"
              @mouseenter="hoveredPoint = pt"
              @mouseleave="hoveredPoint = null"
            />
          </g>
        </svg>

        <!-- Tooltip overlay -->
        <div
          v-if="hoveredPoint"
          class="chart-tooltip"
          :style="{ left: hoveredPoint.tooltipX + '%', top: hoveredPoint.percentY + '%' }"
        >
          <div class="tooltip-date">{{ hoveredPoint.dayLabel }} ({{ hoveredPoint.date }})</div>
          <div class="tooltip-value">
            <strong>{{ hoveredPoint.count }}</strong> due cards
          </div>
        </div>
      </div>

      <!-- X Axis Labels -->
      <div class="chart-x-axis">
        <div
          v-for="(pt, idx) in chartPoints"
          :key="idx"
          class="x-label-container"
          :style="{ left: pt.percentX + '%' }"
        >
          <span class="x-label">{{ pt.dayLabel }}</span>
          <span class="x-sublabel">{{ pt.count }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  timelineData: { type: Array, default: () => [] },
  maxFlashcardsLimit: { type: Number, default: 30 },
})

const hoveredPoint = ref(null)

const yAxisMax = computed(() => {
  if (!props.timelineData || props.timelineData.length === 0) return 40
  const counts = props.timelineData.map((d) => d.card_count)
  const rawMax = Math.max(...counts, props.maxFlashcardsLimit, 10)
  let maxVal = Math.ceil(rawMax * 1.2)
  if (maxVal % 4 !== 0) {
    maxVal += 4 - (maxVal % 4)
  }
  return maxVal
})

const yTicks = computed(() => {
  const maxVal = yAxisMax.value
  const steps = [0, 0.25, 0.5, 0.75, 1]
  return steps.map((pct) => {
    return {
      value: Math.round(maxVal * pct),
      y: 250 - pct * 200,
    }
  })
})

const chartPoints = computed(() => {
  if (!props.timelineData || props.timelineData.length === 0) return []
  const maxVal = yAxisMax.value
  const len = props.timelineData.length

  return props.timelineData.map((d, i) => {
    // scale to 400x300 viewport
    const x = len === 1 ? 200 : 30 + (i / (len - 1)) * 340
    const y = 250 - (d.card_count / maxVal) * 200
    const exceeds = d.card_count > props.maxFlashcardsLimit
    const px = len === 1 ? 50 : 7 + (i / (len - 1)) * 86
    return {
      x,
      y,
      percentX: px,
      tooltipX: px,
      tooltipY: y,
      percentY: (y / 300) * 100,
      dayLabel: d.day_label,
      date: d.date,
      count: d.card_count,
      exceeds,
    }
  })
})

const linePathData = computed(() => {
  const pts = chartPoints.value
  if (pts.length === 0) return ''
  return pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ')
})

const areaPathData = computed(() => {
  const pts = chartPoints.value
  if (pts.length === 0) return ''
  const linePath = linePathData.value
  return `${linePath} L ${pts[pts.length - 1].x} 250 L ${pts[0].x} 250 Z`
})
</script>

<style scoped>
.card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 16px;
}

.forecast-widget {
  margin-bottom: 8px;
}

.forecast-card {
  padding: 16px;
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 250px;
}

.forecast-header-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
}

.forecast-header {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: var(--on-surface);
}

.forecast-subtitle {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted-text);
}

.forecast-legend {
  display: flex;
  gap: 12px;
  align-items: center;
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-text);
  margin-top: 4px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.legend-dot.due-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--primary);
}

.chart-container {
  position: relative;
  width: 100%;
  flex: 1;
  min-height: 130px;
  background: var(--surface-container-lowest);
  border-radius: 12px;
}

.forecast-chart {
  width: 100%;
  height: 100%;
  display: block;
}

.chart-grid-line {
  stroke: var(--outline-variant);
  stroke-width: 1px;
  stroke-opacity: 0.6;
  stroke-dasharray: 2 4;
}

.axis-line {
  stroke: var(--outline-variant);
  stroke-width: 1px;
  stroke-opacity: 0.8;
}

.y-axis-label {
  font-size: 10px;
  font-weight: 600;
  fill: var(--muted-text);
  font-family: inherit;
}

.chart-dot {
  fill: var(--surface-container-lowest);
  stroke: var(--primary);
  stroke-width: 2.5px;
  cursor: pointer;
  transition:
    r 0.15s ease,
    fill 0.15s ease;
}

.chart-dot:hover {
  r: 7px;
  fill: var(--primary);
}

/* Tooltip */
.chart-tooltip {
  position: absolute;
  transform: translate(-50%, -100%);
  margin-top: -12px;
  background: var(--surface-bright);
  border: 1px solid var(--outline-variant);
  border-radius: 8px;
  padding: 8px 12px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
  backdrop-filter: blur(8px);
  z-index: 10;
  pointer-events: none;
  min-width: 120px;
  transition:
    left 0.1s ease,
    top 0.1s ease;
}

.tooltip-date {
  font-size: 11px;
  color: var(--muted-text);
  font-weight: 500;
  margin-bottom: 2px;
}

.tooltip-value {
  font-size: 13px;
  color: var(--on-surface);
}

/* X Axis */
.chart-x-axis {
  position: relative;
  height: 36px;
  margin-top: 4px;
  width: 100%;
}

.x-label-container {
  position: absolute;
  transform: translateX(-50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.x-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-text);
}

.x-sublabel {
  font-size: 12px;
  font-weight: 700;
  color: var(--on-surface);
  margin-top: 2px;
}
</style>
