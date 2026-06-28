<template>
  <div class="dashboard-container">
    <!-- Setup Overlay (if credentials not configured) -->
    <div v-if="showSetup" class="setup-overlay">
      <div class="setup-card">
        <h2 style="margin-top: 0; margin-bottom: 0.5rem; color: #fff;">Connect to Supabase</h2>
        <p class="muted" style="margin-bottom: 2rem; font-size: 0.9rem;">
          Enter your Supabase credentials to access the teacher workspace.
        </p>

        <div v-if="setupError" class="error-message">{{ setupError }}</div>

        <form @submit.prevent="saveCredentials">
          <div class="form-group">
            <label for="setup-url">Supabase Project URL</label>
            <input
              id="setup-url"
              v-model="setupUrl"
              type="url"
              required
              placeholder="https://your-project-id.supabase.co"
            />
          </div>

          <div class="form-group">
            <label for="setup-key">Supabase Anon Key</label>
            <input
              id="setup-key"
              v-model="setupKey"
              type="password"
              required
              placeholder="eyJhbGciOiJIUzI1NiIsIn..."
            />
          </div>

          <div class="form-group">
            <label for="setup-class">Default Classroom Code</label>
            <input
              id="setup-class"
              v-model="setupClassroom"
              type="text"
              required
              placeholder="e.g. BIO101"
            />
          </div>

          <button class="btn" style="width: 100%; margin-top: 1rem;" :disabled="connecting">
            {{ connecting ? 'Connecting...' : 'Connect Workspace' }}
          </button>
        </form>
      </div>
    </div>

    <!-- Main Header -->
    <header class="header">
      <div>
        <h1>☁️ AI Tutor <span class="muted" style="font-weight: 400; font-size: 1.1rem;">Cloud Portal</span></h1>
        <div class="subtitle">Teacher Analytical Workspace</div>
      </div>

      <div style="display: flex; align-items: center; gap: 1rem;">
        <span v-if="classroomCode" class="classroom-badge">
          Classroom: {{ classroomCode }}
        </span>
        <button class="btn btn-secondary" @click="openSettings" style="padding: 0.5rem 1rem;">
          ⚙️ Connection Settings
        </button>
      </div>
    </header>

    <!-- Dashboard Content -->
    <main v-if="!showSetup" class="main-content">
      <!-- Error Bar -->
      <div v-if="error" class="error-message" style="margin-bottom: 0;">
        {{ error }}
      </div>

      <!-- Overview Stats Grid -->
      <section class="stats-grid">
        <!-- Stat: Enrolled Students -->
        <div class="stat-card">
          <div class="stat-header">
            <span class="stat-title">Students Syncing</span>
            <span class="stat-icon">👥</span>
          </div>
          <div class="stat-value">{{ stats.studentsCount }}</div>
          <div class="stat-desc">Distinct active profiles in class</div>
        </div>

        <!-- Stat: Total Review Logs -->
        <div class="stat-card">
          <div class="stat-header">
            <span class="stat-title">FSRS Flashcard Reviews</span>
            <span class="stat-icon">⚡</span>
          </div>
          <div class="stat-value">{{ stats.totalLogs }}</div>
          <div class="stat-desc">Total synced review instances</div>
        </div>

        <!-- Stat: Flashcard Mastery / Pass Rate -->
        <div class="stat-card">
          <div class="stat-header">
            <span class="stat-title">Recall Pass Rate</span>
            <span class="stat-icon">📈</span>
          </div>
          <div class="stat-value">{{ stats.passRate }}%</div>
          <div class="stat-desc">Rating > 1 (Again/Fail) fraction</div>
        </div>

        <!-- Stat: Active Red Alerts -->
        <div class="stat-card" :class="{ 'alert-active': stats.alertsCount > 0 }">
          <div class="stat-header">
            <span class="stat-title">Red Alerts</span>
            <div v-if="stats.alertsCount > 0" class="pulsing-dot"></div>
            <span v-else class="stat-icon">🛡️</span>
          </div>
          <div class="stat-value" :style="{ color: stats.alertsCount > 0 ? 'var(--danger)' : '#fff' }">
            {{ stats.alertsCount }}
          </div>
          <div class="stat-desc">Remediation failures needing support</div>
        </div>
      </section>

      <!-- Workspace: Students View & Assignments Manager -->
      <div class="workspace-grid">
        <!-- Column 1: Students Directory -->
        <section class="section-card">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem;">
            <h2 class="section-title" style="margin-bottom: 0; border: none; padding: 0;">
              Student Profiles
            </h2>
            <div style="display: flex; gap: 0.75rem; align-items: center;">
              <!-- Filter Controls -->
              <input 
                type="text" 
                v-model="searchQuery" 
                placeholder="Search student or file..." 
                style="background: var(--surface-low); border: 1px solid var(--border); padding: 0.4rem 0.8rem; border-radius: 6px; color: #fff; font-size: 0.85rem;"
              />
              <button 
                class="btn btn-secondary" 
                :class="{ active: filterAlerts }" 
                @click="filterAlerts = !filterAlerts"
                style="padding: 0.4rem 0.8rem; font-size: 0.85rem; display: flex; align-items: center; gap: 0.25rem;"
                :style="filterAlerts ? 'border-color: var(--danger); color: var(--danger); background: var(--danger-glow);' : ''"
              >
                🚨 Alerts Only
              </button>
              <button class="btn btn-secondary" @click="fetchData" style="padding: 0.4rem 0.8rem; font-size: 0.85rem;" :disabled="loading">
                {{ loading ? 'Refreshing...' : '🔄 Refresh' }}
              </button>
            </div>
          </div>

          <!-- Empty State -->
          <div v-if="loading && students.length === 0" class="text-center" style="padding: 3rem;">
            <div class="loading-spinner"></div>
            <p class="muted" style="margin-top: 1rem;">Loading student database...</p>
          </div>
          <div v-else-if="filteredStudents.length === 0" class="text-center" style="padding: 3rem; border: 1px dashed var(--border); border-radius: 12px;">
            <span style="font-size: 2rem;">📭</span>
            <p class="muted" style="margin-top: 1rem; margin-bottom: 0;">No matching student data synced for classroom "{{ classroomCode }}" yet.</p>
          </div>

          <!-- Student Accordion list -->
          <div v-else class="student-list">
            <div
              v-for="student in filteredStudents"
              :key="student.token"
              class="student-row"
            >
              <!-- Accordion Header -->
              <div class="student-header" @click="toggleStudent(student.token)">
                <div class="student-info">
                  <div class="student-avatar">
                    {{ student.token.substring(0, 2).toUpperCase() }}
                  </div>
                  <div>
                    <div class="student-name">Student Token: {{ student.token }}</div>
                    <div class="student-meta">
                      {{ student.notebooks.length }} Notebooks &bull; {{ student.logs.length }} reviews synced &bull; Last updated {{ formatRelativeTime(student.lastUpdate) }}
                    </div>
                  </div>
                </div>

                <div class="student-metrics">
                  <!-- Red Alert Warning Badge -->
                  <div v-if="student.alertsCount > 0" class="alert-indicator">
                    🚨 {{ student.alertsCount }} Red Alert{{ student.alertsCount > 1 ? 's' : '' }}
                  </div>
                  <span style="font-size: 1.25rem;">
                    {{ expandedStudents[student.token] ? '▲' : '▼' }}
                  </span>
                </div>
              </div>

              <!-- Accordion Body -->
              <div v-if="expandedStudents[student.token]" class="student-details">
                <!-- Part A: Notebook Statuses -->
                <div>
                  <h3 style="margin-top: 0; margin-bottom: 1rem; font-size: 1rem; color: #fff;">Notebook Ingestion & Study Progress</h3>
                  <div class="notebooks-grid">
                    <div
                      v-for="nb in student.notebooks"
                      :key="nb.file_hash"
                      class="notebook-card"
                    >
                      <div class="notebook-header">
                        <div>
                          <h4 class="notebook-title">{{ nb.title }}</h4>
                          <span class="notebook-filename">{{ nb.filename }}</span>
                        </div>
                        <span class="status-tag" :class="nb.study_status.toLowerCase()">
                          {{ nb.study_status.toUpperCase() }}
                        </span>
                      </div>
                      
                      <!-- Red Alert Notice -->
                      <div v-if="nb.external_help_required" class="alert-indicator" style="width: 100%; justify-content: center; padding: 0.4rem;">
                        🚨 Remediation failed. External help required!
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Part B: Review History Log -->
                <div>
                  <h3 style="margin-top: 1rem; margin-bottom: 1rem; font-size: 1rem; color: #fff;">Spaced Repetition Log</h3>
                  <div v-if="student.logs.length === 0" class="muted" style="font-size: 0.875rem; font-style: italic;">
                    No flashcard reviews completed yet.
                  </div>
                  <div v-else class="logs-table-wrapper">
                    <table class="logs-table">
                      <thead>
                        <tr>
                          <th>Time</th>
                          <th>Notebook Hash</th>
                          <th>Pg</th>
                          <th>Type</th>
                          <th>Rating Given</th>
                          <th>Interval</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="log in student.logs" :key="log.id">
                          <td>{{ formatTime(log.reviewed_at) }}</td>
                          <td class="muted" style="font-family: monospace; font-size: 0.75rem;" :title="log.file_hash">
                            {{ log.file_hash.substring(0, 10) }}...
                          </td>
                          <td>{{ log.page_number }}</td>
                          <td>
                            <span class="status-tag dormant" style="padding: 0.1rem 0.3rem; font-size: 0.7rem;">
                              {{ log.activity_type.toUpperCase() }}
                            </span>
                          </td>
                          <td>
                            <!-- FSRS Rating visualization -->
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
                            <span style="font-size: 0.75rem; margin-left: 0.5rem; vertical-align: middle;">
                              {{ formatRatingLabel(log.rating) }}
                            </span>
                          </td>
                          <td>{{ log.scheduled_days }}d</td>
                        </tr>
                      </tbody>
                    </table>
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
            <h3 style="margin-top: 0; font-size: 0.95rem; color: #fff; margin-bottom: 1rem;">Publish New PDF</h3>
            
            <div class="form-group">
              <label for="assign-title">Notebook Title</label>
              <input
                id="assign-title"
                v-model="newTitle"
                type="text"
                required
                placeholder="e.g. Chapter 4: Photosynthesis"
              />
            </div>

            <div class="form-group">
              <label for="assign-url">Direct PDF URL</label>
              <input
                id="assign-url"
                v-model="newUrl"
                type="url"
                required
                placeholder="e.g. https://server.com/files/chap4.pdf"
              />
            </div>

            <button class="btn" style="width: 100%;" :disabled="publishing">
              {{ publishing ? 'Publishing...' : 'Publish to Class' }}
            </button>
          </form>

          <!-- List of Published Assignments -->
          <div>
            <h3 style="font-size: 0.95rem; color: #fff; margin-bottom: 1rem; border-top: 1px solid var(--border); padding-top: 1.5rem;">
              Active Assignments ({{ assignments.length }})
            </h3>

            <div v-if="loadingAssignments" class="text-center" style="padding: 1rem 0;">
              <div class="loading-spinner" style="width: 16px; height: 16px;"></div>
            </div>

            <div v-else-if="assignments.length === 0" class="muted" style="font-size: 0.85rem; font-style: italic; text-align: center; padding: 1.5rem 0; border: 1px dashed var(--border); border-radius: 8px;">
              No custom homework assignments published.
            </div>

            <div v-else class="assignments-list">
              <div
                v-for="asm in assignments"
                :key="asm.id"
                class="assignment-item"
              >
                <div class="assignment-info">
                  <h4 class="assignment-title" :title="asm.title">{{ asm.title }}</h4>
                  <a :href="asm.download_url" target="_blank" class="assignment-url" :title="asm.download_url">
                    {{ asm.download_url }}
                  </a>
                  <span class="assignment-date">Published {{ formatDate(asm.created_at) }}</span>
                </div>
                <button
                  class="btn btn-secondary btn-danger"
                  style="padding: 0.35rem 0.6rem; font-size: 0.75rem; border-radius: 4px;"
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
import { ref, reactive, computed, onMounted } from 'vue';

// Setup State
const showSetup = ref(true);
const connecting = ref(false);
const setupError = ref('');
const setupUrl = ref('');
const setupKey = ref('');
const setupClassroom = ref('');

// Core State
const supabaseUrl = ref('');
const supabaseKey = ref('');
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

// New Assignment Form
const newTitle = ref('');
const newUrl = ref('');
const publishing = ref(false);

// Check configuration on mount
onMounted(() => {
  const url = localStorage.getItem('supabase_url');
  const key = localStorage.getItem('supabase_key');
  const cls = localStorage.getItem('classroom_code');

  if (url && key && cls) {
    supabaseUrl.value = url;
    supabaseKey.value = key;
    classroomCode.value = cls;
    
    setupUrl.value = url;
    setupKey.value = key;
    setupClassroom.value = cls;
    
    showSetup.value = false;
    fetchData();
  } else {
    showSetup.value = true;
  }
});

// Save credentials from setup screen
async function saveCredentials() {
  connecting.value = true;
  setupError.value = '';
  
  // Format check
  let url = setupUrl.value.trim();
  if (url.endsWith('/')) {
    url = url.slice(0, -1);
  }
  const key = setupKey.value.trim();
  const cls = setupClassroom.value.trim().toUpperCase();

  try {
    // Ping/test connection by making a quick select to teacher_assignments
    const testRes = await fetch(`${url}/rest/v1/teacher_assignments?select=id&limit=1`, {
      headers: {
        'apikey': key,
        'Authorization': `Bearer ${key}`
      }
    });

    if (!testRes.ok) {
      const errText = await testRes.text();
      throw new Error(errText || `Server returned status ${testRes.status}`);
    }

    // Success! Save to state and storage
    supabaseUrl.value = url;
    supabaseKey.value = key;
    classroomCode.value = cls;

    localStorage.setItem('supabase_url', url);
    localStorage.setItem('supabase_key', key);
    localStorage.setItem('classroom_code', cls);

    showSetup.value = false;
    fetchData();
  } catch (err) {
    console.error('Setup connection failure:', err);
    setupError.value = `Failed to connect to Supabase: ${err.message}. Please double-check your credentials and SQL schemas.`;
  } finally {
    connecting.value = false;
  }
}

// Reset setup configurations
function openSettings() {
  setupError.value = '';
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
          'Authorization': `Bearer ${supabaseKey.value}`
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
          'Authorization': `Bearer ${supabaseKey.value}`
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
          'Authorization': `Bearer ${supabaseKey.value}`
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
        'Authorization': `Bearer ${supabaseKey.value}`
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
