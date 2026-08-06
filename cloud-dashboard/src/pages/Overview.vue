<template>
  <div class="overview-page">
    <section class="stats-grid">
      <!-- Stat: Enrolled Students -->
      <div class="stat-card animate-fade-in" style="animation-delay: 0ms">
        <div class="stat-header">
          <span class="stat-title">Students Syncing</span>
        </div>
        <div style="display: flex; justify-content: space-between; align-items: flex-end;">
          <div class="stat-value">{{ stats.studentsCount }}</div>
          <div v-if="students.length > 0" class="avatar-stack">
            <div v-for="student in students.slice(0, 4)" :key="student.token" class="avatar-stacked" :title="student.token">
              {{ student.token.substring(0, 2).toUpperCase() }}
            </div>
            <div v-if="students.length > 4" class="avatar-stacked" style="background: var(--surface-highest); color: var(--muted-text);">
              +{{ students.length - 4 }}
            </div>
          </div>
        </div>
        <div class="stat-desc">Distinct active profiles in class</div>
      </div>

      <!-- Stat: Total Review Logs -->
      <div class="stat-card animate-fade-in" style="animation-delay: 60ms">
        <div class="stat-header">
          <span class="stat-title">FSRS Reviews</span>
        </div>
        <div class="stat-value">{{ stats.totalLogs }}</div>
        <div class="stat-desc">Avg. {{ stats.studentsCount > 0 ? Math.round(stats.totalLogs / stats.studentsCount) : 0 }} cards reviewed per student</div>
      </div>

      <!-- Stat: Flashcard Mastery / Pass Rate -->
      <div class="stat-card animate-fade-in" style="animation-delay: 120ms">
        <div class="stat-header">
          <span class="stat-title">Recall Pass Rate</span>
        </div>
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <div class="stat-value">{{ stats.passRate }}%</div>
          <svg class="progress-ring" width="36" height="36">
            <circle class="progress-ring__circle" stroke="rgba(255,255,255,0.06)" stroke-width="3" fill="transparent" r="12" cx="18" cy="18"/>
            <circle
              class="progress-ring__circle"
              :stroke="stats.passRate > 75 ? 'var(--success)' : stats.passRate > 55 ? 'var(--warning)' : 'var(--danger)'"
              stroke-width="3"
              fill="transparent"
              r="12"
              cx="18"
              cy="18"
              :style="{ strokeDashoffset: 75.39 - (75.39 * stats.passRate / 100) }"
            />
          </svg>
        </div>
        <div class="stat-desc">Rating &gt; 1 (Again/Fail) fraction</div>
      </div>

      <!-- Stat: Active Red Alerts -->
      <div class="stat-card animate-fade-in" :class="{ 'alert-active': stats.alertsCount > 0 }" style="animation-delay: 180ms">
        <div class="stat-header">
          <span class="stat-title">Red Alerts</span>
          <div v-if="stats.alertsCount > 0" class="pulsing-dot"></div>
        </div>
        <div class="stat-value" :style="{ color: stats.alertsCount > 0 ? 'var(--danger)' : 'var(--on-surface)' }">
          {{ stats.alertsCount }}
        </div>
        <div class="stat-desc">
          {{ stats.alertsCount > 0 ? 'Remediation failures needing support' : 'All students on track' }}
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { useDashboard } from '../composables/useDashboard';

const { stats, students } = useDashboard();
</script>
