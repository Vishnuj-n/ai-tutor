<template>
  <div class="dashboard-container">
    <!-- Setup Overlay (Teacher Login) -->
    <div v-if="showSetup" class="setup-overlay">
      <div class="setup-card animate-fade-in">
        <div style="text-align: center; margin-bottom: 1.5rem;">
          <span style="font-size: 2.5rem;">🏫</span>
          <h2 style="margin-top: 0.5rem; margin-bottom: 0.5rem; color: #fff; letter-spacing: -0.015em;">Teacher Portal Login</h2>
          <p class="muted" style="font-size: 0.85rem; line-height: 1.4;">
            Sign in with your teacher credentials to manage assignments and monitor student progress.
          </p>
        </div>

        <div v-if="setupError" class="error-message">
          <span style="font-size: 1.1rem;">⚠️</span>
          <div style="flex: 1;">{{ setupError }}</div>
        </div>

        <form @submit.prevent="loginTeacher">
          <div class="form-group">
            <label for="login-username">Email / Username</label>
            <input
              id="login-username"
              v-model="loginUsername"
              type="text"
              required
              placeholder="e.g. teacher@school.edu"
            />
          </div>

          <div class="form-group">
            <label for="login-password">Password</label>
            <input
              id="login-password"
              v-model="loginPassword"
              type="password"
              required
              placeholder="••••••••"
            />
          </div>

          <button class="btn" style="width: 100%; margin-top: 1.25rem;" :disabled="connecting">
            <span v-if="connecting" class="loading-spinner" style="width: 14px; height: 14px; border-width: 2px;"></span>
            {{ connecting ? 'Signing In...' : '🔑 Sign In' }}
          </button>
        </form>
      </div>
    </div>

    <!-- Main Header -->
    <header class="header">
      <div>
        <h1>
          <span>☁️</span>
          <span>AI Tutor</span>
          <span style="color: var(--primary); font-weight: 500;">Portal</span>
        </h1>
        <div class="subtitle">Teacher Analytical Workspace</div>
      </div>

      <div style="display: flex; align-items: center; gap: 1.25rem;">
        <!-- Live status ping dot -->
        <div v-if="!showSetup" style="display: flex; align-items: center; gap: 0.5rem; background: rgba(16, 185, 129, 0.04); border: 1px solid rgba(16, 185, 129, 0.15); padding: 0.3rem 0.6rem; border-radius: 6px;">
          <span class="pulsing-dot" style="background-color: var(--success); box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.4);"></span>
          <span style="font-size: 0.7rem; color: var(--success); font-weight: 600; font-family: var(--font-mono); letter-spacing: 0.02em;">LIVE SYNCED</span>
        </div>

        <span v-if="classroomCode" class="classroom-badge">
          CLASSROOM: {{ classroomCode }}
        </span>
        
        <button class="btn btn-secondary" @click="logoutTeacher" style="padding: 0.45rem 0.85rem; font-size: 0.8rem;">
          🚪 Sign Out
        </button>
      </div>
    </header>

    <!-- Dashboard Content -->
    <main v-if="!showSetup" class="main-content">
      <!-- Error Bar -->
      <div v-if="error" class="error-message">
        <span style="font-size: 1.1rem;">⚠️</span>
        <div style="flex: 1;">{{ error }}</div>
      </div>

      <!-- Overview Stats Grid -->
      <section class="stats-grid">
        <!-- Stat: Enrolled Students -->
        <div class="stat-card animate-fade-in" style="animation-delay: 0ms">
          <div class="stat-header">
            <span class="stat-title">Students Syncing</span>
            <span class="stat-icon">👥</span>
          </div>
          <div style="display: flex; justify-content: space-between; align-items: flex-end;">
            <div class="stat-value">{{ stats.studentsCount }}</div>
            <!-- Avatar stack mockup -->
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
            <span class="stat-icon">⚡</span>
          </div>
          <div class="stat-value">{{ stats.totalLogs }}</div>
          <div class="stat-desc">Avg. {{ stats.studentsCount > 0 ? Math.round(stats.totalLogs / stats.studentsCount) : 0 }} cards reviewed per student</div>
        </div>

        <!-- Stat: Flashcard Mastery / Pass Rate -->
        <div class="stat-card animate-fade-in" style="animation-delay: 120ms">
          <div class="stat-header">
            <span class="stat-title">Recall Pass Rate</span>
            <span class="stat-icon">📈</span>
          </div>
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <div class="stat-value">{{ stats.passRate }}%</div>
            <!-- SVG circular gauge -->
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
            <span v-else class="stat-icon">🛡️</span>
          </div>
          <div class="stat-value" :style="{ color: stats.alertsCount > 0 ? 'var(--danger)' : 'var(--on-surface)' }">
            {{ stats.alertsCount }}
          </div>
          <div class="stat-desc">
            {{ stats.alertsCount > 0 ? 'Remediation failures needing support' : 'All students on track' }}
          </div>
        </div>
      </section>

      <!-- Workspace: Students View & Assignments Manager -->
      <div class="workspace-grid">
        <!-- Column 1: Students Directory -->
        <section class="section-card">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; flex-wrap: wrap; gap: 1rem;">
            <h2 class="section-title" style="margin-bottom: 0; border: none; padding: 0;">
              👥 Student Profiles
            </h2>
            
            <div style="display: flex; gap: 0.65rem; align-items: center; flex-wrap: wrap;">
              <!-- Modern Filter Controls with Keyboard hint -->
              <div class="search-container">
                <span class="search-icon">🔍</span>
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
                :class="{ active: filterAlerts }" 
                @click="filterAlerts = !filterAlerts"
                style="padding: 0.45rem 0.85rem; font-size: 0.8rem; display: flex; align-items: center; gap: 0.35rem;"
                :style="filterAlerts ? 'border-color: var(--danger); color: var(--danger); background: var(--danger-glow);' : ''"
              >
                🚨 Alerts Only
              </button>

              <button class="btn" @click="fetchData" style="padding: 0.45rem 0.85rem; font-size: 0.8rem;" :disabled="loading">
                <span v-if="loading" class="loading-spinner" style="width: 12px; height: 12px; border-width: 2px;"></span>
                {{ loading ? 'Syncing...' : '🔄 Refresh' }}
              </button>
            </div>
          </div>

          <!-- Empty State -->
          <div v-if="loading && students.length === 0" class="text-center" style="padding: 4rem 2rem;">
            <div class="loading-spinner"></div>
            <p class="muted" style="margin-top: 1rem; font-size: 0.9rem;">Fetching classroom database...</p>
          </div>
          <div v-else-if="filteredStudents.length === 0" class="text-center" style="padding: 4rem 2rem; border: 1px dashed var(--border); border-radius: 12px; background: rgba(255,255,255,0.01);">
            <span style="font-size: 2rem;">📭</span>
            <p class="muted" style="margin-top: 1rem; margin-bottom: 0; font-size: 0.9rem;">
              No students synced for classroom "{{ classroomCode }}" matching the filters.
            </p>
          </div>

          <!-- Student Accordion list -->
          <div v-else class="student-list">
            <div
              v-for="(student, index) in filteredStudents"
              :key="student.token"
              class="student-row animate-fade-in"
              :class="{ expanded: expandedStudents[student.token] }"
              :style="{ animationDelay: `${(index + 2) * 50}ms` }"
            >
              <!-- Accordion Header -->
              <div class="student-header" @click="toggleStudent(student.token)" aria-label="Toggle student details">
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

                <div class="student-metrics">
                  <!-- Red Alert Warning Badge -->
                  <div v-if="student.alertsCount > 0" class="alert-indicator" style="animation: hazard-pulse 2s infinite ease-in-out;">
                    🚨 {{ student.alertsCount }} Alert{{ student.alertsCount > 1 ? 's' : '' }}
                  </div>
                  <!-- Rotating Chevron Chevron -->
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

              <!-- Accordion Body (CSS Grid height transition wrapper) -->
              <div class="student-details-wrapper">
                <div class="student-details">
                  <div class="student-details-content">
                    
                    <!-- FSRS Review Heatmap Strip -->
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
                          <!-- Custom HTML Tooltip hover -->
                          <div class="tooltip-text">
                            <div><strong>{{ formatRatingLabel(log.rating) }}</strong></div>
                            <div style="margin-top: 0.15rem; color: var(--muted-text);">Interval: {{ log.scheduled_days }}d &bull; Pg {{ log.page_number }}</div>
                            <div style="font-size: 0.65rem; color: var(--muted-text); margin-top: 0.15rem;">{{ formatTime(log.reviewed_at) }}</div>
                          </div>
                        </div>
                      </div>
                    </div>

                    <!-- Notebook Statuses -->
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
                          
                          <!-- Red Alert Notice -->
                          <div v-if="nb.external_help_required" class="alert-indicator" style="width: 100%; justify-content: center; padding: 0.35rem; margin-top: 0.25rem;">
                            🚨 Socratic rescue failed. Needs support!
                          </div>
                        </div>
                      </div>
                    </div>

                    <!-- Review History Table (Fallback details) -->
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
                                <!-- FSRS Rating bar visualization -->
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

        <!-- Column 2: Teacher Assignments Manager -->
        <section class="section-card" style="align-self: flex-start;">
          <h2 class="section-title">
            📚 Course Assignments
          </h2>

          <!-- Publish Form -->
          <form @submit.prevent="publishAssignment" style="margin-bottom: 2rem;">
            <h3 style="margin-top: 0; font-size: 0.85rem; color: var(--on-surface); margin-bottom: 0.85rem; font-weight: 600; letter-spacing: 0.05em; text-transform: uppercase;">
              Publish New PDF
            </h3>
            
            <div class="form-group">
              <label for="assign-title">Assignment Title</label>
              <input
                id="assign-title"
                v-model="newTitle"
                type="text"
                required
                placeholder="e.g. Chapter 4: Cell Division"
              />
            </div>

            <div class="form-group">
              <label for="assign-url">Direct PDF URL</label>
              <input
                id="assign-url"
                v-model="newUrl"
                type="url"
                required
                placeholder="https://example.com/files/cell_chap4.pdf"
              />
            </div>

            <button class="btn" style="width: 100%; margin-top: 0.5rem;" :disabled="publishing">
              <span v-if="publishing" class="loading-spinner" style="width: 12px; height: 12px; border-width: 2px;"></span>
              {{ publishing ? 'Publishing...' : 'Publish to Class' }}
            </button>
          </form>

          <!-- List of Published Assignments -->
          <div>
            <h3 style="font-size: 0.85rem; color: var(--muted-text); margin-bottom: 0.85rem; border-top: 1px solid var(--border); padding-top: 1.25rem; font-weight: 600; letter-spacing: 0.05em; text-transform: uppercase;">
              Active Assignments ({{ assignments.length }})
            </h3>

            <div v-if="loadingAssignments" class="text-center" style="padding: 1.5rem 0;">
              <div class="loading-spinner"></div>
            </div>

            <div v-else-if="assignments.length === 0" class="muted" style="font-size: 0.8rem; font-style: italic; text-align: center; padding: 2rem 1rem; border: 1px dashed var(--border); border-radius: 8px; background: rgba(255,255,255,0.01);">
              No assignments published yet.
            </div>

            <div v-else class="assignments-list">
              <div
                v-for="asm in assignments"
                :key="asm.id"
                class="assignment-item"
              >
                <div class="assignment-info">
                  <h4 class="assignment-title" :title="asm.title">📄 {{ asm.title }}</h4>
                  <a :href="asm.download_url" target="_blank" class="assignment-url" :title="asm.download_url">
                    {{ asm.download_url }}
                  </a>
                  <span class="assignment-date">Published {{ formatDate(asm.created_at) }}</span>
                </div>
                <button
                  class="btn btn-secondary btn-danger"
                  style="padding: 0.35rem 0.55rem; font-size: 0.75rem; border-radius: 6px; min-height: unset;"
                  @click="deleteAssignment(asm.id)"
                  title="Remove assignment"
                >
                  🗑️
                </button>
              </div>
            </div>
          </div>
        </section>
      </div>
    </main>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue';

// Setup/Login State
const showSetup = ref(true);
const connecting = ref(false);
const setupError = ref('');
const loginUsername = ref('');
const loginPassword = ref('');

// Core State
const supabaseUrl = ref(import.meta.env.VITE_SUPABASE_URL || 'https://dkqahgkkighcpycexovi.supabase.co');
const supabaseKey = ref(import.meta.env.VITE_SUPABASE_ANON_KEY || 'sb_publishable_Gno-X5ppMB6YZza52F4Nog__7kxobfX');
const sessionToken = ref('');
const classroomCode = ref('');
const error = ref('');
const loading = ref(false);
const loadingAssignments = ref(false);

const students = ref([]);
const assignments = ref([]);
const expandedStudents = reactive({});

// Search/Filter State
const searchQuery = ref('');
const filterAlerts = ref(false);
const searchInputRef = ref(null);

// New Assignment Form
const newTitle = ref('');
const newUrl = ref('');
const publishing = ref(false);

// Global keyboard listeners for focus
const handleGlobalKeydown = (e) => {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault();
    searchInputRef.value?.focus();
  } else if (e.key === '/' && document.activeElement !== searchInputRef.value && !['INPUT', 'TEXTAREA'].includes(document.activeElement?.tagName)) {
    e.preventDefault();
    searchInputRef.value?.focus();
  }
};

// Check session on mount
onMounted(() => {
  const token = sessionStorage.getItem('session_token');
  const cls = sessionStorage.getItem('classroom_code');

  if (token && cls) {
    sessionToken.value = token;
    classroomCode.value = cls;
    showSetup.value = false;
    fetchData();
  } else {
    showSetup.value = true;
  }

  window.addEventListener('keydown', handleGlobalKeydown);
});

onUnmounted(() => {
  window.removeEventListener('keydown', handleGlobalKeydown);
});

// loginTeacher handles teacher credentials validation via login_user RPC
async function loginTeacher() {
  connecting.value = true;
  setupError.value = '';
  
  if (!loginUsername.value.trim() || !loginPassword.value.trim()) {
    setupError.value = 'Username/Email and Password are required.';
    connecting.value = false;
    return;
  }

  try {
    const payload = {
      p_username: loginUsername.value.trim(),
      p_password: loginPassword.value.trim(),
      p_is_desktop: false // web
    };

    const res = await fetch(`${supabaseUrl.value}/rest/v1/rpc/login_user`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'apikey': supabaseKey.value,
        'Authorization': `Bearer ${supabaseKey.value}`
      },
      body: JSON.stringify(payload)
    });

    if (!res.ok) {
      const errText = await res.text();
      let parsedErr;
      try { parsedErr = JSON.parse(errText); } catch(_) {}
      throw new Error(parsedErr?.message || errText || `Server returned status ${res.status}`);
    }

    const loginData = await res.json();
    
    if (loginData.role !== 'teacher') {
      throw new Error('Access denied. Only teachers can access this portal.');
    }

    sessionToken.value = loginData.session_token;
    classroomCode.value = loginData.classroom_code;
    
    sessionStorage.setItem('session_token', loginData.session_token);
    sessionStorage.setItem('classroom_code', loginData.classroom_code);

    showSetup.value = false;
    fetchData();
  } catch (err) {
    console.error('Login failure:', err);
    setupError.value = err.message || 'Failed to login. Please verify credentials.';
  } finally {
    connecting.value = false;
  }
}

// logoutTeacher signs the teacher out by clearing session variables and redirecting to the login screen
function logoutTeacher() {
  sessionToken.value = '';
  classroomCode.value = '';
  sessionStorage.removeItem('session_token');
  sessionStorage.removeItem('classroom_code');
  showSetup.value = true;
}

// Unified data fetching
async function fetchData() {
  if (!supabaseUrl.value || !supabaseKey.value || !classroomCode.value) return;

  loading.value = true;
  error.value = '';

  try {
    // 1. Fetch Student Notebooks
    const nbRes = await fetch(
      `${supabaseUrl.value}/rest/v1/student_notebooks?classroom_code=eq.${classroomCode.value}&order=updated_at.desc`,
      {
        headers: {
          'apikey': supabaseKey.value,
          'Authorization': `Bearer ${supabaseKey.value}`,
          'x-session-token': sessionToken.value
        }
      }
    );
    if (!nbRes.ok) throw new Error(`Notebooks fetch error: ${nbRes.statusText}`);
    const notebooksData = await nbRes.json();

    // 2. Fetch Review Logs
    const logRes = await fetch(
      `${supabaseUrl.value}/rest/v1/student_review_logs?classroom_code=eq.${classroomCode.value}&order=reviewed_at.desc`,
      {
        headers: {
          'apikey': supabaseKey.value,
          'Authorization': `Bearer ${supabaseKey.value}`,
          'x-session-token': sessionToken.value
        }
      }
    );
    if (!logRes.ok) throw new Error(`Logs fetch error: ${logRes.statusText}`);
    const logsData = await logRes.json();

    // Group data by student_token
    const studentsMap = {};

    // Initialize map with students from notebooks
    notebooksData.forEach(nb => {
      const token = nb.student_token;
      if (!studentsMap[token]) {
        studentsMap[token] = {
          token,
          notebooks: [],
          logs: [],
          lastUpdate: 0,
          alertsCount: 0
        };
      }
      studentsMap[token].notebooks.push(nb);
      if (nb.external_help_required) {
        studentsMap[token].alertsCount++;
      }
      const updateTime = new Date(nb.updated_at).getTime();
      if (updateTime > studentsMap[token].lastUpdate) {
        studentsMap[token].lastUpdate = updateTime;
      }
    });

    // Append logs
    logsData.forEach(log => {
      const token = log.student_token;
      if (!studentsMap[token]) {
        studentsMap[token] = {
          token,
          notebooks: [],
          logs: [],
          lastUpdate: 0,
          alertsCount: 0
        };
      }
      studentsMap[token].logs.push(log);
      const logTime = log.reviewed_at * 1000;
      if (logTime > studentsMap[token].lastUpdate) {
        studentsMap[token].lastUpdate = logTime;
      }
    });

    // Convert map to sorted array
    students.value = Object.values(studentsMap).sort((a, b) => b.lastUpdate - a.lastUpdate);

    // Fetch assignments
    fetchAssignments();
  } catch (err) {
    console.error('Data refresh error:', err);
    error.value = `Failed to fetch classroom data: ${err.message}`;
  } finally {
    loading.value = false;
  }
}

// Fetch active assignments list
async function fetchAssignments() {
  loadingAssignments.value = true;
  try {
    const res = await fetch(
      `${supabaseUrl.value}/rest/v1/teacher_assignments?classroom_code=eq.${classroomCode.value}&order=created_at.desc`,
      {
        headers: {
          'apikey': supabaseKey.value,
          'Authorization': `Bearer ${supabaseKey.value}`,
          'x-session-token': sessionToken.value
        }
      }
    );
    if (!res.ok) throw new Error(`Assignments fetch error: ${res.statusText}`);
    assignments.value = await res.json();
  } catch (err) {
    console.error('Failed to load assignments:', err);
    error.value = `Assignments warning: ${err.message}`;
  } finally {
    loadingAssignments.value = false;
  }
}

// Publish custom assignment
async function publishAssignment() {
  if (!newTitle.value.trim() || !newUrl.value.trim()) return;
  publishing.value = true;

  // Safe unique assignment ID generation
  const asmId = typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : 'asn-' + Math.random().toString(36).substring(2, 15) + Math.random().toString(36).substring(2, 15);

  const payload = {
    id: asmId,
    classroom_code: classroomCode.value,
    title: newTitle.value.trim(),
    download_url: newUrl.value.trim()
  };

  try {
    const res = await fetch(`${supabaseUrl.value}/rest/v1/teacher_assignments`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'apikey': supabaseKey.value,
        'Authorization': `Bearer ${supabaseKey.value}`,
        'x-session-token': sessionToken.value,
        'Prefer': 'return=representation'
      },
      body: JSON.stringify(payload)
    });

    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || `Server returned code ${res.status}`);
    }

    newTitle.value = '';
    newUrl.value = '';
    
    // Refresh assignments list
    fetchAssignments();
  } catch (err) {
    console.error('Publishing error:', err);
    error.value = `Failed to publish assignment: ${err.message}`;
  } finally {
    publishing.value = false;
  }
}

// Remove an assignment
async function deleteAssignment(id) {
  if (!confirm('Are you sure you want to remove this assignment? Syncing clients will no longer download it.')) return;
  
  try {
    const res = await fetch(`${supabaseUrl.value}/rest/v1/teacher_assignments?id=eq.${id}`, {
      method: 'DELETE',
      headers: {
        'apikey': supabaseKey.value,
        'Authorization': `Bearer ${supabaseKey.value}`,
        'x-session-token': sessionToken.value
      }
    });

    if (!res.ok) throw new Error(`Delete failed with status ${res.status}`);
    
    // Refresh assignments
    fetchAssignments();
  } catch (err) {
    console.error('Delete error:', err);
    error.value = `Failed to delete assignment: ${err.message}`;
  }
}

// Toggle accordion
function toggleStudent(token) {
  expandedStudents[token] = !expandedStudents[token];
}

// Computed stats rollup
const stats = computed(() => {
  const count = students.value.length;
  let totalReviews = 0;
  let totalPassingReviews = 0;
  let alertsCount = 0;

  students.value.forEach(s => {
    totalReviews += s.logs.length;
    alertsCount += s.alertsCount;
    
    s.logs.forEach(log => {
      if (log.rating > 1) { // 1 = Again/Fail in FSRS algorithm
        totalPassingReviews++;
      }
    });
  });

  const passRate = totalReviews > 0 ? Math.round((totalPassingReviews / totalReviews) * 100) : 0;

  return {
    studentsCount: count,
    totalLogs: totalReviews,
    passRate,
    alertsCount
  };
});

// Search & filter students
const filteredStudents = computed(() => {
  return students.value.filter(student => {
    // 1. Alert filter
    if (filterAlerts.value && student.alertsCount === 0) {
      return false;
    }

    // 2. Search query filter
    const query = searchQuery.value.trim().toLowerCase();
    if (!query) return true;

    // Search by student token
    if (student.token.toLowerCase().includes(query)) {
      return true;
    }

    // Search by notebook title or filename
    const matchesNotebook = student.notebooks.some(nb => 
      nb.title.toLowerCase().includes(query) || nb.filename.toLowerCase().includes(query)
    );
    if (matchesNotebook) return true;

    return false;
  });
});

// UI formatting helpers
function formatRatingLabel(rating) {
  switch (rating) {
    case 1: return 'Again (Fail)';
    case 2: return 'Hard';
    case 3: return 'Good';
    case 4: return 'Easy';
    default: return 'Unknown';
  }
}

function formatTime(unixSeconds) {
  const d = new Date(unixSeconds * 1000);
  return d.toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}

function formatDate(isoString) {
  const d = new Date(isoString);
  return d.toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' });
}

function formatRelativeTime(timestamp) {
  if (!timestamp) return 'never';
  const diff = Date.now() - timestamp;
  const mins = Math.floor(diff / 60000);
  
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}
</script>

<style>
/* App.vue is styled globally through style.css */
</style>
