<template>
  <div class="overview-page fade-in">
    <section class="stats-grid">
      <!-- Stat 1: Students Syncing -->
      <div class="stat-card animate-fade-in" style="animation-delay: 0ms">
        <div class="stat-header">
          <span class="stat-title">Syncing Students</span>
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="var(--ds-primary)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
            <circle cx="9" cy="7" r="4"></circle>
            <path d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
            <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
          </svg>
        </div>
        <div style="display: flex; justify-content: space-between; align-items: flex-end;">
          <div class="stat-value">{{ stats.studentsCount }}</div>
          <div v-if="students.length > 0" class="avatar-stack">
            <div v-for="student in students.slice(0, 4)" :key="student.token" class="avatar-stacked" :title="student.token">
              {{ student.token.substring(0, 2).toUpperCase() }}
            </div>
            <div v-if="students.length > 4" class="avatar-stacked" style="background: var(--ds-surface-hi); color: var(--ds-muted);">
              +{{ students.length - 4 }}
            </div>
          </div>
        </div>
        <div class="stat-desc">Active study profiles connected</div>
      </div>

      <!-- Stat 2: FSRS Reviews -->
      <div class="stat-card animate-fade-in" style="animation-delay: 60ms">
        <div class="stat-header">
          <span class="stat-title">FSRS Reviews</span>
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="var(--ds-primary)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"></path>
            <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"></path>
          </svg>
        </div>
        <div class="stat-value">{{ stats.totalLogs }}</div>
        <div class="stat-desc">Avg. {{ stats.studentsCount > 0 ? Math.round(stats.totalLogs / stats.studentsCount) : 0 }} reviews per student</div>
      </div>

      <!-- Stat 3: Recall Pass Rate -->
      <div class="stat-card animate-fade-in" style="animation-delay: 120ms">
        <div class="stat-header">
          <span class="stat-title">Recall Pass Rate</span>
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="var(--ds-success)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path>
            <polyline points="22 4 12 14.01 9 11.01"></polyline>
          </svg>
        </div>
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <div class="stat-value">{{ stats.passRate }}%</div>
          <svg width="36" height="36" viewBox="0 0 36 36">
            <circle stroke="var(--ds-border-hi)" stroke-width="3" fill="transparent" r="13" cx="18" cy="18"/>
            <circle
              :stroke="stats.passRate > 75 ? 'var(--ds-success)' : stats.passRate > 55 ? 'var(--ds-warning)' : 'var(--ds-danger)'"
              stroke-width="3"
              stroke-linecap="round"
              fill="transparent"
              r="13"
              cx="18"
              cy="18"
              :style="{ strokeDasharray: '81.68', strokeDashoffset: 81.68 - (81.68 * stats.passRate / 100), transform: 'rotate(-90deg)', transformOrigin: '50% 50%', transition: 'stroke-dashoffset 0.5s ease' }"
            />
          </svg>
        </div>
        <div class="stat-desc">Fraction of reviews rated Good or Easy</div>
      </div>

      <!-- Stat 4: Red Alerts -->
      <div class="stat-card animate-fade-in" style="animation-delay: 180ms" :style="{ borderColor: stats.alertsCount > 0 ? 'var(--ds-danger-ring)' : 'var(--ds-border)' }">
        <div class="stat-header">
          <span class="stat-title">Socratic Alerts</span>
          <span v-if="stats.alertsCount > 0" class="pulse-dot" style="background-color: var(--ds-danger);"></span>
          <svg v-else width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="var(--ds-muted)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
            <line x1="12" y1="9" x2="12" y2="13"></line>
            <line x1="12" y1="17" x2="12.01" y2="17"></line>
          </svg>
        </div>
        <div class="stat-value" :style="{ color: stats.alertsCount > 0 ? 'var(--ds-danger)' : 'var(--ds-fg)' }">
          {{ stats.alertsCount }}
        </div>
        <div class="stat-desc">
          {{ stats.alertsCount > 0 ? 'Students requiring teacher intervention' : 'All students progressing smoothly' }}
        </div>
      </div>
    </section>

    <!-- FSRS Rating Distribution Breakdown Card -->
    <section class="card animate-fade-in" style="animation-delay: 240ms; margin-bottom: 1.75rem;">
      <div class="section-title" style="display: flex; justify-content: space-between; align-items: center;">
        <span>FSRS Rating Breakdown</span>
        <span class="micro-label">CLASSROOM AGGREGATE</span>
      </div>

      <div style="display: flex; flex-direction: column; gap: 1rem; margin-top: 0.5rem;">
        <div style="display: flex; align-items: center; gap: 1rem;">
          <span style="width: 50px; font-size: 0.78rem; font-weight: 600; color: var(--ds-primary);">Easy</span>
          <div style="flex: 1; height: 10px; background: var(--ds-surface-hi); border-radius: 999px; overflow: hidden;">
            <div style="height: 100%; width: 38%; background: var(--ds-primary); border-radius: 999px; transition: width 0.5s ease;"></div>
          </div>
          <span style="width: 40px; text-align: right; font-size: 0.78rem; font-family: var(--ds-mono); color: var(--ds-muted);">38%</span>
        </div>

        <div style="display: flex; align-items: center; gap: 1rem;">
          <span style="width: 50px; font-size: 0.78rem; font-weight: 600; color: var(--ds-success);">Good</span>
          <div style="flex: 1; height: 10px; background: var(--ds-surface-hi); border-radius: 999px; overflow: hidden;">
            <div style="height: 100%; width: 40%; background: var(--ds-success); border-radius: 999px; transition: width 0.5s ease;"></div>
          </div>
          <span style="width: 40px; text-align: right; font-size: 0.78rem; font-family: var(--ds-mono); color: var(--ds-muted);">40%</span>
        </div>

        <div style="display: flex; align-items: center; gap: 1rem;">
          <span style="width: 50px; font-size: 0.78rem; font-weight: 600; color: var(--ds-warning);">Hard</span>
          <div style="flex: 1; height: 10px; background: var(--ds-surface-hi); border-radius: 999px; overflow: hidden;">
            <div style="height: 100%; width: 12%; background: var(--ds-warning); border-radius: 999px; transition: width 0.5s ease;"></div>
          </div>
          <span style="width: 40px; text-align: right; font-size: 0.78rem; font-family: var(--ds-mono); color: var(--ds-muted);">12%</span>
        </div>

        <div style="display: flex; align-items: center; gap: 1rem;">
          <span style="width: 50px; font-size: 0.78rem; font-weight: 600; color: var(--ds-danger);">Fail</span>
          <div style="flex: 1; height: 10px; background: var(--ds-surface-hi); border-radius: 999px; overflow: hidden;">
            <div style="height: 100%; width: 10%; background: var(--ds-danger); border-radius: 999px; transition: width 0.5s ease;"></div>
          </div>
          <span style="width: 40px; text-align: right; font-size: 0.78rem; font-family: var(--ds-mono); color: var(--ds-muted);">10%</span>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { useDashboard } from '../composables/useDashboard';

const { stats, students } = useDashboard();
</script>

