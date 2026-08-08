<template>
  <div class="students-page fade-in">
    <section class="section-card">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; flex-wrap: wrap; gap: 1rem;">
        <div>
          <h2 class="section-title" style="margin-bottom: 0; border: none; padding: 0;">
            Student Profiles
          </h2>
          <div class="micro-label" style="margin-top: 0.25rem;">CONNECTED CLASSROOM ACCOUNTS</div>
        </div>

        <div style="display: flex; gap: 0.6rem; align-items: center; flex-wrap: wrap;">
          <div class="search-container">
            <input
              ref="searchInputRef"
              type="text"
              v-model="searchQuery"
              class="search-input"
              placeholder="Filter student or topic..."
              style="width: 220px;"
            />
            <kbd class="search-kbd">/</kbd>
          </div>

          <button
            class="btn-ghost"
            @click="toggleClassroomLock"
            :style="isLocked ? 'border-color: var(--ds-danger-ring); color: var(--ds-danger); background: var(--ds-danger-bg);' : 'border-color: var(--ds-success-ring); color: var(--ds-success);'"
            :title="isLocked ? 'Classroom is LOCKED. Click to allow new student joins.' : 'Classroom is OPEN. Click to block new student joins.'"
          >
            <span>{{ isLocked ? '🔒 Locked' : '🔓 Open' }}</span>
          </button>

          <button
            class="btn-ghost"
            :class="{ danger: filterAlerts }"
            @click="filterAlerts = !filterAlerts"
            :style="filterAlerts ? 'border-color: var(--ds-danger-ring); color: var(--ds-danger); background: var(--ds-danger-bg);' : ''"
          >
            Alerts Only
          </button>

          <button
            class="btn-ghost"
            @click="exportClassroomCSV"
            :disabled="students.length === 0"
            title="Export classroom stats to CSV"
          >
            Export CSV
          </button>

          <button class="btn" @click="() => fetchData(false)" :disabled="loading">
            <span v-if="loading" class="loading-spinner" style="width: 12px; height: 12px; border-width: 2px;"></span>
            {{ loading ? 'Syncing...' : 'Refresh' }}
          </button>
        </div>
      </div>

      <div v-if="loading && students.length === 0" class="text-center" style="padding: 4rem 2rem;">
        <div class="loading-spinner"></div>
        <p class="muted" style="margin-top: 1rem; font-size: 0.85rem;">Fetching classroom database...</p>
      </div>
      <div v-else-if="filteredStudents.length === 0" class="text-center" style="padding: 3rem 2rem; border: 1px dashed var(--ds-border-hi); border-radius: 12px; background: var(--ds-surface-low);">
        <h3 style="margin-top: 0.75rem; margin-bottom: 0.35rem; color: var(--ds-fg);">No students synced yet for "{{ classroomCode }}"</h3>
        <p class="muted" style="margin-bottom: 1.25rem; font-size: 0.82rem; max-width: 460px; margin-left: auto; margin-right: auto;">
          Students connect to your analytical workspace using your classroom code. Once connected, their flashcard review logs and study progress will stream here live.
        </p>
        <div style="background: var(--ds-surface); border: 1px solid var(--ds-border-hi); border-radius: 10px; padding: 1.25rem; text-align: left; max-width: 480px; margin: 0 auto; font-size: 0.8rem; line-height: 1.5;">
          <strong style="color: var(--ds-primary);">Student Setup Instructions:</strong>
          <ol style="margin-top: 0.5rem; margin-bottom: 0; padding-left: 1.25rem; color: var(--ds-muted);">
            <li>Open the <strong>StudyLoop Desktop App</strong></li>
            <li>Select (or create) your <strong>Study Profile</strong> for this course</li>
            <li>Navigate to <strong>Settings</strong> &rarr; <strong>Account & Cloud</strong></li>
            <li>Click <strong>Create Account</strong> (or <strong>Sign In</strong>)</li>
            <li>Enter your credentials and Classroom Code: <code style="color: var(--ds-fg); background: var(--ds-surface-hi); padding: 0.15rem 0.4rem; border-radius: 4px; font-family: var(--ds-mono);">{{ classroomCode }}</code></li>
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
          :style="{ animationDelay: `${(index + 2) * 40}ms` }"
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
                  {{ student.notebooks.length }} Notebooks &bull; {{ student.logs.length }} reviews synced &bull; Updated {{ formatRelativeTime(student.lastUpdate) }}
                </div>
              </div>
            </div>

            <div class="student-metrics" style="display: flex; align-items: center; gap: 0.75rem;">
              <span v-if="student.alertsCount > 0" class="badge" style="background: var(--ds-danger-bg); border: 1px solid var(--ds-danger-ring); color: var(--ds-danger); font-size: 0.7rem; padding: 0.2rem 0.5rem; border-radius: 999px; font-weight: 600;">
                {{ student.alertsCount }} Alert{{ student.alertsCount > 1 ? 's' : '' }}
              </span>
              <button
                class="btn-ghost danger"
                @click.stop="removeStudent(student.token)"
                style="padding: 0.2rem 0.5rem; font-size: 0.72rem;"
                title="Remove student from classroom"
              >
                Remove
              </button>
              <svg
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2.5"
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
              <div style="display: flex; flex-direction: column; gap: 1.25rem;">
                <div v-if="student.logs.length > 0" style="border-bottom: 1px solid var(--ds-border); padding-bottom: 1.25rem;">
                  <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
                    <span class="micro-label">RETENTION HISTORY (CHRONOLOGICAL)</span>
                    <div style="display: flex; gap: 0.75rem; font-size: 0.7rem; color: var(--ds-muted);">
                      <span style="display: flex; align-items: center; gap: 0.2rem;"><span class="heatmap-node rating-1" style="width:10px;height:10px;display:inline-block;"></span> Fail</span>
                      <span style="display: flex; align-items: center; gap: 0.2rem;"><span class="heatmap-node rating-2" style="width:10px;height:10px;display:inline-block;"></span> Hard</span>
                      <span style="display: flex; align-items: center; gap: 0.2rem;"><span class="heatmap-node rating-3" style="width:10px;height:10px;display:inline-block;"></span> Good</span>
                      <span style="display: flex; align-items: center; gap: 0.2rem;"><span class="heatmap-node rating-4" style="width:10px;height:10px;display:inline-block;"></span> Easy</span>
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
                        <div style="margin-top: 0.15rem; color: var(--ds-muted);">Interval: {{ log.scheduled_days }}d &bull; Pg {{ log.page_number }}</div>
                        <div style="font-size: 0.65rem; color: var(--ds-muted); margin-top: 0.15rem;">{{ formatTime(log.reviewed_at) }}</div>
                      </div>
                    </div>
                  </div>
                </div>

                <div>
                  <span class="micro-label" style="display: block; margin-bottom: 0.6rem;">INGESTION & STUDY PROGRESS</span>
                  <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 0.75rem;">
                    <div
                      v-for="nb in student.notebooks"
                      :key="nb.file_hash"
                      class="card"
                      style="padding: 0.85rem; border-radius: 8px;"
                      :style="{ borderColor: nb.external_help_required ? 'var(--ds-danger-ring)' : 'var(--ds-border)' }"
                    >
                      <div style="display: flex; justify-content: space-between; align-items: flex-start; gap: 0.5rem;">
                        <div style="min-width: 0; flex: 1;">
                          <div style="font-size: 0.82rem; font-weight: 600; color: var(--ds-fg); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;" :title="nb.title">{{ nb.title }}</div>
                          <div style="font-size: 0.7rem; color: var(--ds-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;" :title="nb.filename">{{ nb.filename }}</div>
                        </div>
                        <span style="font-size: 0.68rem; font-weight: 600; padding: 0.15rem 0.4rem; border-radius: 4px; background: var(--ds-surface-hi); color: var(--ds-fg); font-family: var(--ds-mono);">
                          {{ nb.study_status }}
                        </span>
                      </div>

                      <div v-if="nb.external_help_required" style="background: var(--ds-danger-bg); border: 1px solid var(--ds-danger-ring); color: var(--ds-danger); border-radius: 6px; padding: 0.3rem; margin-top: 0.5rem; font-size: 0.7rem; text-align: center;">
                        Socratic rescue failed. Needs support!
                      </div>
                    </div>
                  </div>
                </div>

                <div>
                  <span class="micro-label" style="display: block; margin-bottom: 0.6rem;">DETAILED SPACED REPETITION LOGS</span>
                  <div v-if="student.logs.length === 0" class="muted" style="font-size: 0.8rem; font-style: italic; padding: 0.5rem 0;">
                    No flashcard reviews completed yet.
                  </div>
                  <div v-else class="logs-table-wrapper">
                    <table class="logs-table">
                      <thead>
                        <tr>
                          <th>Time</th>
                          <th>Notebook</th>
                          <th>Page</th>
                          <th>Type</th>
                          <th>Rating</th>
                          <th>Interval</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="log in student.logs" :key="log.id">
                          <td style="font-family: var(--ds-mono);">{{ formatTime(log.reviewed_at) }}</td>
                          <td class="muted" style="font-size: 0.75rem;" :title="'Hash: ' + log.file_hash">
                            {{ getNotebookName(student, log.file_hash) }}
                          </td>
                          <td style="font-family: var(--ds-mono);">{{ log.page_number }}</td>
                          <td>
                            <span style="font-size: 0.65rem; background: var(--ds-surface-hi); padding: 0.15rem 0.4rem; border-radius: 4px; color: var(--ds-muted); font-family: var(--ds-mono);">
                              {{ log.activity_type }}
                            </span>
                          </td>
                          <td>
                            <span
                              style="font-size: 0.72rem; font-weight: 600; padding: 0.15rem 0.5rem; border-radius: 4px;"
                              :style="{
                                background: log.rating === 4 ? 'var(--ds-primary-glow)' : log.rating === 3 ? 'var(--ds-success-bg)' : log.rating === 2 ? 'rgba(245,158,11,0.1)' : 'var(--ds-danger-bg)',
                                color: log.rating === 4 ? 'var(--ds-primary)' : log.rating === 3 ? 'var(--ds-success)' : log.rating === 2 ? 'var(--ds-warning)' : 'var(--ds-danger)',
                                border: `1px solid ${log.rating === 4 ? 'var(--ds-primary-ring)' : log.rating === 3 ? 'var(--ds-success-ring)' : log.rating === 2 ? 'rgba(245,158,11,0.3)' : 'var(--ds-danger-ring)'}`
                              }"
                            >
                              {{ formatRatingLabel(log.rating) }}
                            </span>
                          </td>
                          <td style="font-family: var(--ds-mono);">{{ log.scheduled_days }}d</td>
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

function getNotebookName(student, fileHash) {
  if (!fileHash) return 'Unknown Notebook';
  if (student && student.notebooks) {
    const nb = student.notebooks.find(n => n.file_hash === fileHash);
    if (nb) {
      return nb.title || nb.filename || `${fileHash.substring(0, 10)}...`;
    }
  }
  return `${fileHash.substring(0, 10)}...`;
}
</script>

