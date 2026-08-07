<template>
  <div class="students-page">
    <section class="section-card">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; flex-wrap: wrap; gap: 1rem;">
        <h2 class="section-title" style="margin-bottom: 0; border: none; padding: 0;">
          Student Profiles
        </h2>

        <div style="display: flex; gap: 0.65rem; align-items: center; flex-wrap: wrap;">
          <div class="search-container">
            <span class="search-icon">Find:</span>
            <input
              ref="searchInputRef"
              type="text"
              v-model="searchQuery"
              class="search-input"
              placeholder="Filter student or topic..."
              style="background: var(--surface-low); border: 1px solid var(--border); padding: 0.4rem 0.8rem; border-radius: 6px; color: #fff; font-size: 0.85rem;"
            />
            <kbd class="search-kbd">/</kbd>
          </div>

          <button
            class="btn btn-secondary"
            @click="toggleClassroomLock"
            style="padding: 0.45rem 0.85rem; font-size: 0.8rem; display: flex; align-items: center; gap: 0.35rem;"
            :style="isLocked ? 'border-color: var(--danger); color: var(--danger); background: var(--danger-glow);' : 'border-color: var(--success); color: var(--success);'"
            :title="isLocked ? 'Classroom is LOCKED. Click to allow new student joins.' : 'Classroom is OPEN. Click to block new student joins.'"
          >
            <span>{{ isLocked ? '🔒 Locked' : '🔓 Open' }}</span>
          </button>

          <button
            class="btn btn-secondary"
            :class="{ active: filterAlerts }"
            @click="filterAlerts = !filterAlerts"
            style="padding: 0.45rem 0.85rem; font-size: 0.8rem; display: flex; align-items: center; gap: 0.35rem;"
            :style="filterAlerts ? 'border-color: var(--danger); color: var(--danger); background: var(--danger-glow);' : ''"
          >
            Alerts Only
          </button>

          <button
            class="btn btn-secondary"
            @click="exportClassroomCSV"
            :disabled="students.length === 0"
            style="padding: 0.45rem 0.85rem; font-size: 0.8rem; display: flex; align-items: center; gap: 0.35rem;"
            title="Export classroom stats to CSV"
          >
            Export CSV
          </button>

          <button class="btn" @click="() => fetchData(false)" style="padding: 0.45rem 0.85rem; font-size: 0.8rem;" :disabled="loading">
            <span v-if="loading" class="loading-spinner" style="width: 12px; height: 12px; border-width: 2px;"></span>
            {{ loading ? 'Syncing...' : 'Refresh' }}
          </button>
        </div>
      </div>

      <div v-if="loading && students.length === 0" class="text-center" style="padding: 4rem 2rem;">
        <div class="loading-spinner"></div>
        <p class="muted" style="margin-top: 1rem; font-size: 0.9rem;">Fetching classroom database...</p>
      </div>
      <div v-else-if="filteredStudents.length === 0" class="text-center" style="padding: 3rem 2rem; border: 1px dashed var(--border); border-radius: 12px; background: rgba(255,255,255,0.01);">
        <h3 style="margin-top: 0.75rem; margin-bottom: 0.35rem; color: #fff;">No students synced yet for "{{ classroomCode }}"</h3>
        <p class="muted" style="margin-bottom: 1.25rem; font-size: 0.85rem; max-width: 460px; margin-left: auto; margin-right: auto;">
          Students connect to your analytical workspace using your classroom code. Once connected, their flashcard review logs and study progress will stream here live.
        </p>
        <div style="background: var(--surface-low); border: 1px solid var(--border); border-radius: 8px; padding: 1rem; text-align: left; max-width: 480px; margin: 0 auto; font-size: 0.8rem; line-height: 1.5;">
          <strong style="color: var(--primary);">Student Setup Instructions:</strong>
          <ol style="margin-top: 0.4rem; margin-bottom: 0; padding-left: 1.25rem; color: var(--muted-text);">
            <li>Open the <strong>AI Tutor Desktop App</strong></li>
            <li>Select (or create) your <strong>Study Profile</strong> for this course</li>
            <li>Navigate to <strong>Settings</strong> &rarr; <strong>Account & Cloud</strong></li>
            <li>Click <strong>Create Account</strong> (or <strong>Sign In</strong> if already registered)</li>
            <li>Enter your <strong>Username</strong>, <strong>Password</strong>, and Classroom Code: <code style="color: #fff; background: rgba(255,255,255,0.1); padding: 0.1rem 0.35rem; border-radius: 4px;">{{ classroomCode }}</code></li>
            <li>Click <strong>Sign Up & Sync</strong></li>
          </ol>
        </div>
      </div>

      <div v-else class="student-list">
        <div
          v-for="(student, index) in filteredStudents"
          :key="student.token"
          class="student-row animate-fade-in"
          :class="{ expanded: expandedStudents[student.token] }"
          :style="{ animationDelay: `${(index + 2) * 50}ms` }"
        >
          <div
            class="student-header"
            role="button"
            tabindex="0"
            :aria-expanded="!!expandedStudents[student.token]"
            @click="toggleStudent(student.token)"
            @keydown.enter.prevent="toggleStudent(student.token)"
            @keydown.space.prevent="toggleStudent(student.token)"
            aria-label="Toggle student details"
          >
            <div class="student-info">
              <div class="student-avatar">
                {{ student.token.substring(0, 2).toUpperCase() }}
              </div>
              <div>
                <div class="student-name">token:{{ student.token.substring(0, 12) }}...</div>
                <div class="student-meta">
                  {{ student.notebooks.length }} Notebooks &bull; {{ student.logs.length }} reviews synced &bull; Last updated {{ formatRelativeTime(student.lastUpdate) }}
                </div>
              </div>
            </div>

            <div class="student-metrics" style="display: flex; align-items: center; gap: 0.75rem;">
              <div v-if="student.alertsCount > 0" class="alert-indicator" style="animation: hazard-pulse 2s infinite ease-in-out;">
                {{ student.alertsCount }} Alert{{ student.alertsCount > 1 ? 's' : '' }}
              </div>
              <button
                class="btn btn-secondary"
                @click.stop="removeStudent(student.token)"
                style="padding: 0.25rem 0.55rem; font-size: 0.75rem; border-color: rgba(239, 68, 68, 0.4); color: var(--danger);"
                title="Remove student from classroom"
              >
                Remove
              </button>
              <svg
                width="12"
                height="12"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="3"
                stroke-linecap="round"
                stroke-linejoin="round"
                style="transition: transform 0.25s cubic-bezier(0.16, 1, 0.3, 1);"
                :style="{ transform: expandedStudents[student.token] ? 'rotate(180deg)' : 'rotate(0deg)' }"
              >
                <polyline points="6 9 12 15 18 9"></polyline>
              </svg>
            </div>
          </div>

          <div class="student-details-wrapper">
            <div class="student-details">
              <div class="student-details-content">
                <div v-if="student.logs.length > 0" style="border-bottom: 1px solid var(--border); padding-bottom: 1.25rem;">
                  <div class="heatmap-title-container">
                    <span style="font-size: 0.75rem; font-weight: 600; color: var(--muted-text); letter-spacing: 0.05em;">RETENTION HISTORY (CHRONOLOGICAL)</span>
                    <div class="heatmap-legend">
                      <span style="display: flex; align-items: center; gap: 0.2rem;"><span class="heatmap-legend-box rating-1"></span> Fail</span>
                      <span style="display: flex; align-items: center; gap: 0.2rem;"><span class="heatmap-legend-box rating-2"></span> Hard</span>
                      <span style="display: flex; align-items: center; gap: 0.2rem;"><span class="heatmap-legend-box rating-3"></span> Good</span>
                      <span style="display: flex; align-items: center; gap: 0.2rem;"><span class="heatmap-legend-box rating-4"></span> Easy</span>
                    </div>
                  </div>

                  <div class="heatmap-strip">
                    <div
                      v-for="log in student.logs.slice().reverse()"
                      :key="log.id"
                      class="heatmap-node"
                      :class="'rating-' + log.rating"
                    >
                      <div class="tooltip-text">
                        <div><strong>{{ formatRatingLabel(log.rating) }}</strong></div>
                        <div style="margin-top: 0.15rem; color: var(--muted-text);">Interval: {{ log.scheduled_days }}d &bull; Pg {{ log.page_number }}</div>
                        <div style="font-size: 0.65rem; color: var(--muted-text); margin-top: 0.15rem;">{{ formatTime(log.reviewed_at) }}</div>
                      </div>
                    </div>
                  </div>
                </div>

                <div>
                  <h3 style="margin-top: 0; margin-bottom: 0.85rem; font-size: 0.8rem; font-weight: 600; color: var(--muted-text); letter-spacing: 0.05em; text-transform: uppercase;">
                    Ingestion & Study Progress
                  </h3>
                  <div class="notebooks-grid">
                    <div
                      v-for="nb in student.notebooks"
                      :key="nb.file_hash"
                      class="notebook-card"
                      :style="{ borderColor: nb.external_help_required ? 'rgba(239, 68, 68, 0.3)' : 'var(--border)' }"
                    >
                      <div class="notebook-header">
                        <div style="min-width: 0; flex: 1;">
                          <h4 class="notebook-title" :title="nb.title">{{ nb.title }}</h4>
                          <span class="notebook-filename" :title="nb.filename">{{ nb.filename }}</span>
                        </div>
                        <span class="status-tag" :class="nb.study_status.toLowerCase()">
                          {{ nb.study_status }}
                        </span>
                      </div>

                      <div v-if="nb.external_help_required" class="alert-indicator" style="width: 100%; justify-content: center; padding: 0.35rem; margin-top: 0.25rem;">
                        Socratic rescue failed. Needs support!
                      </div>
                    </div>
                  </div>
                </div>

                <div>
                  <h3 style="margin-top: 0.5rem; margin-bottom: 0.85rem; font-size: 0.8rem; font-weight: 600; color: var(--muted-text); letter-spacing: 0.05em; text-transform: uppercase;">
                    Detailed Spaced Repetition Logs
                  </h3>
                  <div v-if="student.logs.length === 0" class="muted" style="font-size: 0.8rem; font-style: italic; padding: 0.5rem 0;">
                    No flashcard reviews completed yet.
                  </div>
                  <div v-else class="logs-table-wrapper">
                    <table class="logs-table">
                      <thead>
                        <tr>
                          <th>Time</th>
                          <th>Notebook Hash</th>
                          <th>Page</th>
                          <th>Type</th>
                          <th>Rating</th>
                          <th>Interval</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="log in student.logs" :key="log.id">
                          <td style="font-family: var(--font-mono);">{{ formatTime(log.reviewed_at) }}</td>
                          <td class="muted" style="font-family: var(--font-mono); font-size: 0.7rem;" :title="log.file_hash">
                            {{ log.file_hash.substring(0, 10) }}...
                          </td>
                          <td style="font-family: var(--font-mono);">{{ log.page_number }}</td>
                          <td>
                            <span class="status-tag dormant" style="padding: 0.1rem 0.3rem; font-size: 0.65rem;">
                              {{ log.activity_type }}
                            </span>
                          </td>
                          <td>
                            <div class="rating-bar" :title="'Rating Code: ' + log.rating">
                              <span
                                v-for="dot in 4"
                                :key="dot"
                                class="rating-dot"
                                :class="{
                                  filled: dot <= log.rating,
                                  hard: log.rating === 2,
                                  bad: log.rating === 1
                                }"
                              ></span>
                            </div>
                            <span style="font-size: 0.7rem; margin-left: 0.4rem; vertical-align: middle;">
                              {{ formatRatingLabel(log.rating) }}
                            </span>
                          </td>
                          <td style="font-family: var(--font-mono);">{{ log.scheduled_days }}d</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </div>

              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { useDashboard } from '../composables/useDashboard';

const {
  students,
  filteredStudents,
  expandedStudents,
  classroomCode,
  isLocked,
  loading,
  searchQuery,
  filterAlerts,
  searchInputRef,
  fetchData,
  toggleClassroomLock,
  removeStudent,
  exportClassroomCSV,
  toggleStudent,
  formatRatingLabel,
  formatTime,
  formatRelativeTime
} = useDashboard();
</script>
