import { ref, reactive, computed } from 'vue';

const supabaseUrl = ref(import.meta.env.VITE_SUPABASE_URL || '');
const supabaseKey = ref(import.meta.env.VITE_SUPABASE_ANON_KEY || '');
const apiBaseUrl = ref(import.meta.env.VITE_API_URL || 'http://localhost:8080');
const sessionToken = ref('');
const classroomCode = ref('');
const showSetup = ref(true);
const connecting = ref(false);
const setupError = ref('');
const loginUsername = ref('');
const loginPassword = ref('');
const isSignUp = ref(false);
const loginClassroom = ref('');
const rememberMe = ref(true);

const error = ref('');
const loading = ref(false);
const loadingAssignments = ref(false);

const students = ref([]);
const assignments = ref([]);
const expandedStudents = reactive({});

const toastMessage = ref('');
const lastSyncedAt = ref(null);
const syncTimeAgo = ref('just now');
let pollInterval = null;
let timeAgoInterval = null;

const searchQuery = ref('');
const filterAlerts = ref(false);
const searchInputRef = ref(null);

const newTitle = ref('');
const newUrl = ref('');
const publishing = ref(false);
const uploadingPdf = ref(false);

function showToast(msg) {
  toastMessage.value = msg;
  setTimeout(() => {
    if (toastMessage.value === msg) {
      toastMessage.value = '';
    }
  }, 3500);
}

function updateSyncTimeAgo() {
  if (!lastSyncedAt.value) {
    syncTimeAgo.value = 'just now';
    return;
  }
  const sec = Math.floor((Date.now() - lastSyncedAt.value) / 1000);
  if (sec < 10) syncTimeAgo.value = 'just now';
  else if (sec < 60) syncTimeAgo.value = `${sec}s ago`;
  else syncTimeAgo.value = `${Math.floor(sec / 60)}m ago`;
}

function startPolling() {
  stopPolling();
  pollInterval = setInterval(() => {
    fetchData(true);
  }, 30000);
  timeAgoInterval = setInterval(() => {
    updateSyncTimeAgo();
  }, 5000);
}

function stopPolling() {
  if (pollInterval) {
    clearInterval(pollInterval);
    pollInterval = null;
  }
  if (timeAgoInterval) {
    clearInterval(timeAgoInterval);
    timeAgoInterval = null;
  }
}

function toggleAuthMode() {
  isSignUp.value = !isSignUp.value;
  setupError.value = '';
}

async function handleAuth(router) {
  if (isSignUp.value) {
    await signupTeacher(router);
  } else {
    await loginTeacher(router);
  }
}

async function signupTeacher(router) {
  connecting.value = true;
  setupError.value = '';

  if (!loginUsername.value.trim() || !loginPassword.value.trim() || !loginClassroom.value.trim()) {
    setupError.value = 'All fields are required for sign up.';
    connecting.value = false;
    return;
  }

  try {
    let success = false;
    if (apiBaseUrl.value) {
      try {
        const res = await fetch(`${apiBaseUrl.value}/api/auth/signup`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            username: loginUsername.value.trim(),
            password: loginPassword.value.trim(),
            role: 'teacher',
            classroom_code: loginClassroom.value.trim().toUpperCase()
          })
        });
        if (res.ok) {
          success = true;
        } else {
          const errText = await res.text();
          let parsed; try { parsed = JSON.parse(errText); } catch(_) {}
          throw new Error(parsed?.error || parsed?.message || errText);
        }
      } catch (e) {
        if (e.message.includes('already exists') || e.message.includes('Signup error')) throw e;
        console.warn('Go API server signup unavailable, falling back to direct Supabase REST insert:', e);
      }
    }

    if (!success && supabaseUrl.value && supabaseKey.value) {
      const checkUrl = `${supabaseUrl.value}/rest/v1/user_accounts?username=eq.${encodeURIComponent(loginUsername.value.trim())}&select=id`;
      const cRes = await fetch(checkUrl, {
        headers: {
          'apikey': supabaseKey.value,
          'Authorization': `Bearer ${supabaseKey.value}`
        }
      });
      if (cRes.ok) {
        const existing = await cRes.json();
        if (existing && existing.length > 0) {
          throw new Error('Username already exists');
        }
      }

      const res = await fetch(`${supabaseUrl.value}/rest/v1/user_accounts`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'apikey': supabaseKey.value,
          'Authorization': `Bearer ${supabaseKey.value}`,
          'Prefer': 'return=representation'
        },
        body: JSON.stringify({
          username: loginUsername.value.trim(),
          password_hash: loginPassword.value.trim(),
          role: 'teacher',
          classroom_code: loginClassroom.value.trim().toUpperCase()
        })
      });

      if (!res.ok) {
        const errText = await res.text();
        let parsedErr; try { parsedErr = JSON.parse(errText); } catch (_) {}
        throw new Error(parsedErr?.message || errText || `Server returned status ${res.status}`);
      }
      success = true;
    }

    showToast('Teacher account created successfully! Logging in...');
    isSignUp.value = false;
    await loginTeacher(router);
  } catch (err) {
    console.error('Sign up failure:', err);
    setupError.value = err.message || 'Failed to sign up.';
  } finally {
    connecting.value = false;
  }
}

async function loginTeacher(router) {
  connecting.value = true;
  setupError.value = '';

  if (!loginUsername.value.trim() || !loginPassword.value.trim()) {
    setupError.value = 'Username/Email and Password are required.';
    connecting.value = false;
    return;
  }

  try {
    let loginData = null;
    if (apiBaseUrl.value) {
      try {
        const res = await fetch(`${apiBaseUrl.value}/api/auth/login`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            username: loginUsername.value.trim(),
            password: loginPassword.value.trim(),
            is_desktop: false
          })
        });
        if (res.ok) {
          loginData = await res.json();
        }
      } catch (e) {
        console.warn('Go API server login unavailable, falling back to direct Supabase REST table query:', e);
      }
    }

    if (!loginData && supabaseUrl.value && supabaseKey.value) {
      const targetUrl = `${supabaseUrl.value}/rest/v1/user_accounts?username=eq.${encodeURIComponent(loginUsername.value.trim())}&select=*`;
      const res = await fetch(targetUrl, {
        headers: {
          'apikey': supabaseKey.value,
          'Authorization': `Bearer ${supabaseKey.value}`
        }
      });
      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `Server returned status ${res.status}`);
      }
      const users = await res.json();
      if (!users || users.length === 0) {
        throw new Error('Invalid username or password');
      }
      const user = users[0];
      const pwd = user.password_hash || user.password;
      if (pwd !== loginPassword.value.trim()) {
        throw new Error('Invalid username or password');
      }
      loginData = {
        role: user.role,
        session_token: user.id || user.username,
        classroom_code: user.classroom_code,
        username: user.username
      };
    }

    if (!loginData) {
      throw new Error('Failed to connect to authentication service.');
    }

    const userRole = loginData.role || (loginData.user && loginData.user.role);
    if (userRole !== 'teacher') {
      throw new Error('Access denied. Only teachers can access this portal.');
    }

    const tokenVal = loginData.session_token || loginData.token || (loginData.user && loginData.user.id);
    const classCodeVal = loginData.classroom_code || (loginData.user && loginData.user.classroom_code);
    const unameVal = loginData.username || (loginData.user && loginData.user.username);

    sessionToken.value = tokenVal;
    classroomCode.value = classCodeVal;

    if (rememberMe.value) {
      localStorage.setItem('session_token', tokenVal);
      localStorage.setItem('classroom_code', classCodeVal);
      sessionStorage.removeItem('session_token');
      sessionStorage.removeItem('classroom_code');
    } else {
      sessionStorage.setItem('session_token', tokenVal);
      sessionStorage.setItem('classroom_code', classCodeVal);
      localStorage.removeItem('session_token');
      localStorage.removeItem('classroom_code');
    }

    showSetup.value = false;
    showToast(`Welcome back, ${unameVal || 'Teacher'}!`);
    fetchData();
    startPolling();

    if (router) {
      router.push('/overview');
    }
  } catch (err) {
    console.error('Login failure:', err);
    setupError.value = err.message || 'Failed to login. Please verify credentials.';
  } finally {
    connecting.value = false;
  }
}

function logoutTeacher(router) {
  stopPolling();
  sessionToken.value = '';
  classroomCode.value = '';
  sessionStorage.removeItem('session_token');
  sessionStorage.removeItem('classroom_code');
  localStorage.removeItem('session_token');
  localStorage.removeItem('classroom_code');
  showSetup.value = true;
  if (router) {
    router.push('/login');
  }
}

async function fetchData(silent = false) {
  if (!classroomCode.value) return;

  if (!silent) loading.value = true;
  error.value = '';

  try {
    let fetchedData = null;
    let success = false;

    // 1. Try Go Backend API
    if (apiBaseUrl.value) {
      try {
        const res = await fetch(`${apiBaseUrl.value}/api/dashboard?classroom_code=${classroomCode.value}`, {
          headers: {
            'x-session-token': sessionToken.value,
            'Authorization': `Bearer ${sessionToken.value}`
          }
        });
        if (res.ok) {
          fetchedData = await res.json();
          success = true;
        }
      } catch (e) {
        console.warn('Go API server unavailable, falling back to Supabase direct REST:', e);
      }
    }

    // 2. Fallback to Supabase direct REST table queries if Go server is not running
    if (!success && supabaseUrl.value && supabaseKey.value) {
      const nbRes = await fetch(`${supabaseUrl.value}/rest/v1/student_notebooks?classroom_code=eq.${encodeURIComponent(classroomCode.value)}&select=*`, {
        headers: {
          'apikey': supabaseKey.value,
          'Authorization': `Bearer ${supabaseKey.value}`
        }
      });
      if (!nbRes.ok) throw new Error(`Dashboard fetch error: ${nbRes.statusText}`);
      const rawNbs = await nbRes.json();

      const userRes = await fetch(`${supabaseUrl.value}/rest/v1/user_accounts?classroom_code=eq.${encodeURIComponent(classroomCode.value)}&role=eq.student&select=username`, {
        headers: {
          'apikey': supabaseKey.value,
          'Authorization': `Bearer ${supabaseKey.value}`
        }
      });
      const studentMap = {};
      const alertMap = {};
      if (userRes.ok) {
        const rawUsers = await userRes.json();
        for (const u of rawUsers) {
          if (u.username) studentMap[u.username] = [];
        }
      }

      for (const nb of rawNbs) {
        const st = nb.student_token;
        if (st) {
          if (!studentMap[st]) studentMap[st] = [];
          studentMap[st].push(nb);
          if (nb.external_help_required) alertMap[st] = (alertMap[st] || 0) + 1;
        }
      }

      const logRes = await fetch(`${supabaseUrl.value}/rest/v1/student_review_logs?select=*`, {
        headers: {
          'apikey': supabaseKey.value,
          'Authorization': `Bearer ${supabaseKey.value}`
        }
      });
      const logMap = {};
      if (logRes.ok) {
        const rawLogs = await logRes.json();
        for (const lg of rawLogs) {
          if (lg.student_token) {
            if (!logMap[lg.student_token]) logMap[lg.student_token] = [];
            logMap[lg.student_token].push(lg);
          }
        }
      }

      const assembledStudents = [];
      for (const token in studentMap) {
        assembledStudents.push({
          token,
          notebooks: studentMap[token] || [],
          logs: logMap[token] || [],
          alertsCount: alertMap[token] || 0,
          lastUpdate: Date.now()
        });
      }

      fetchedData = { is_locked: false, students: assembledStudents };
      success = true;
    }

    if (success && fetchedData) {
      if (typeof fetchedData === 'object' && !Array.isArray(fetchedData) && fetchedData.students) {
        students.value = fetchedData.students || [];
        isLocked.value = !!fetchedData.is_locked;
      } else if (Array.isArray(fetchedData)) {
        students.value = fetchedData;
      }
      lastSyncedAt.value = Date.now();
      updateSyncTimeAgo();
      fetchAssignments();
    } else {
      throw new Error('Failed to fetch data from API server or database.');
    }
  } catch (err) {
    console.error('Data refresh error:', err);
    if (!silent) error.value = `Failed to fetch classroom data: ${err.message}`;
  } finally {
    if (!silent) loading.value = false;
  }
}

async function toggleClassroomLock() {
  const targetState = !isLocked.value;
  try {
    const res = await fetch(`${supabaseUrl.value}/rest/v1/user_accounts?classroom_code=eq.${encodeURIComponent(classroomCode.value)}&role=eq.teacher`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        'apikey': supabaseKey.value,
        'Authorization': `Bearer ${supabaseKey.value}`
      },
      body: JSON.stringify({ is_locked: targetState })
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || `Server returned status ${res.status}`);
    }
    isLocked.value = targetState;
    showToast(targetState ? 'Classroom LOCKED. New student joins disabled.' : 'Classroom UNLOCKED. New students can join.');
  } catch (err) {
    console.error('Failed to toggle classroom lock:', err);
    showToast(`Error: ${err.message}`);
  }
}

async function removeStudent(studentToken) {
  if (!confirm(`Are you sure you want to remove student "${studentToken}" from classroom ${classroomCode.value}?`)) return;

  try {
    await fetch(`${supabaseUrl.value}/rest/v1/student_notebooks?student_token=eq.${encodeURIComponent(studentToken)}&classroom_code=eq.${encodeURIComponent(classroomCode.value)}`, {
      method: 'DELETE',
      headers: {
        'apikey': supabaseKey.value,
        'Authorization': `Bearer ${supabaseKey.value}`
      }
    });
    await fetch(`${supabaseUrl.value}/rest/v1/student_review_logs?student_token=eq.${encodeURIComponent(studentToken)}&classroom_code=eq.${encodeURIComponent(classroomCode.value)}`, {
      method: 'DELETE',
      headers: {
        'apikey': supabaseKey.value,
        'Authorization': `Bearer ${supabaseKey.value}`
      }
    });
    await fetch(`${supabaseUrl.value}/rest/v1/user_accounts?username=eq.${encodeURIComponent(studentToken)}&role=eq.student`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        'apikey': supabaseKey.value,
        'Authorization': `Bearer ${supabaseKey.value}`
      },
      body: JSON.stringify({ classroom_code: '' })
    });
    showToast(`Student ${studentToken} removed from classroom.`);
    fetchData();
  } catch (err) {
    console.error('Failed to remove student:', err);
    showToast(`Error: ${err.message}`);
  }
}

function exportClassroomCSV() {
  if (students.value.length === 0) return;

  const headers = ['Student Token', 'Notebooks Count', 'FSRS Reviews', 'Recall Pass Rate', 'Red Alert Status', 'Last Synced'];
  const rows = students.value.map(s => {
    let passCount = 0;
    s.logs.forEach(l => { if (l.rating > 1) passCount++; });
    const passRate = s.logs.length > 0 ? Math.round((passCount / s.logs.length) * 100) : 0;
    return [
      `"${s.token}"`,
      s.notebooks.length,
      s.logs.length,
      `"${passRate}%"`,
      s.alertsCount > 0 ? 'ALERT_ACTIVE' : 'NORMAL',
      `"${s.lastUpdate ? new Date(s.lastUpdate).toLocaleString() : 'Never'}"`
    ].join(',');
  });

  const csvString = [headers.join(','), ...rows].join('\n');
  const blob = new Blob([csvString], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.setAttribute('href', url);
  link.setAttribute('download', `classroom-${classroomCode.value}-report.csv`);
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
  showToast('Classroom analytics exported to CSV file.');
}

async function fetchAssignments() {
  loadingAssignments.value = true;
  try {
    let list = null;
    if (apiBaseUrl.value) {
      try {
        const res = await fetch(`${apiBaseUrl.value}/api/assignments?classroom_code=${classroomCode.value}`);
        if (res.ok) {
          list = await res.json();
        }
      } catch (_) {}
    }

    if (!list && supabaseUrl.value && supabaseKey.value) {
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
      if (res.ok) list = await res.json();
    }

    if (list) {
      assignments.value = list;
    }
  } catch (err) {
    console.warn('Failed to load assignments:', err);
  } finally {
    loadingAssignments.value = false;
  }
}

async function handleFileUpload(event) {
  const file = event.target.files[0];
  if (!file) return;

  if (file.size > 50 * 1024 * 1024) {
    error.value = 'PDF upload failed: File size exceeds 50MB limit.';
    event.target.value = '';
    return;
  }

  try {
    const headerBuf = await file.slice(0, 4).arrayBuffer();
    const header = new Uint8Array(headerBuf);
    if (header[0] !== 0x25 || header[1] !== 0x50 || header[2] !== 0x44 || header[3] !== 0x46) {
      error.value = 'PDF upload failed: Selected file is not a valid PDF.';
      event.target.value = '';
      return;
    }
  } catch (err) {
    error.value = 'Failed to read PDF file header.';
    event.target.value = '';
    return;
  }

  if (!newTitle.value.trim()) {
    newTitle.value = file.name.replace(/\.pdf$/i, '');
  }

  uploadingPdf.value = true;
  error.value = '';

  try {
    const safeName = file.name.replace(/[^a-zA-Z0-9._-]/g, '_');
    const storagePath = `${classroomCode.value}/${Date.now()}_${safeName}`;

    const res = await fetch(`${supabaseUrl.value}/storage/v1/object/assignments/${storagePath}`, {
      method: 'POST',
      headers: {
        'apikey': supabaseKey.value,
        'Authorization': `Bearer ${supabaseKey.value}`,
        'x-session-token': sessionToken.value,
        'Content-Type': 'application/pdf'
      },
      body: file
    });

    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || `Upload returned status ${res.status}`);
    }

    newUrl.value = `${supabaseUrl.value}/storage/v1/object/public/assignments/${storagePath}`;
    showToast('PDF uploaded to Supabase Storage!');
  } catch (err) {
    console.error('PDF upload error:', err);
    error.value = `Failed to upload PDF: ${err.message}`;
    event.target.value = '';
  } finally {
    uploadingPdf.value = false;
  }
}

async function publishAssignment() {
  if (!newTitle.value.trim() || !newUrl.value.trim()) return;

  const trimmedUrl = newUrl.value.trim();
  if (!trimmedUrl.toLowerCase().startsWith('http://') && !trimmedUrl.toLowerCase().startsWith('https://')) {
    error.value = 'Failed to publish assignment: URL must use http or https scheme.';
    return;
  }

  publishing.value = true;

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

    showToast('Assignment published successfully!');
    fetchAssignments();
  } catch (err) {
    console.error('Publishing error:', err);
    error.value = `Failed to publish assignment: ${err.message}`;
  } finally {
    publishing.value = false;
  }
}

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

    showToast('Assignment removed.');
    fetchAssignments();
  } catch (err) {
    console.error('Delete error:', err);
    error.value = `Failed to delete assignment: ${err.message}`;
  }
}

function toggleStudent(token) {
  expandedStudents[token] = !expandedStudents[token];
}

const stats = computed(() => {
  const count = students.value.length;
  let totalReviews = 0;
  let totalPassingReviews = 0;
  let alertsCount = 0;

  students.value.forEach(s => {
    totalReviews += s.logs.length;
    alertsCount += s.alertsCount;

    s.logs.forEach(log => {
      if (log.rating > 1) {
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

const filteredStudents = computed(() => {
  return students.value.filter(student => {
    if (filterAlerts.value && student.alertsCount === 0) {
      return false;
    }

    const query = searchQuery.value.trim().toLowerCase();
    if (!query) return true;

    if (student.token.toLowerCase().includes(query)) {
      return true;
    }

    const matchesNotebook = student.notebooks.some(nb =>
      nb.title.toLowerCase().includes(query) || nb.filename.toLowerCase().includes(query)
    );
    if (matchesNotebook) return true;

    return false;
  });
});

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
  const ms = unixSeconds > 1e11 ? unixSeconds : unixSeconds * 1000;
  const d = new Date(ms);
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

function initSession() {
  const token = localStorage.getItem('session_token') || sessionStorage.getItem('session_token');
  const cls = localStorage.getItem('classroom_code') || sessionStorage.getItem('classroom_code');

  if (!supabaseUrl.value || !supabaseKey.value) {
    setupError.value = 'Configuration error: Supabase URL or Anon Key is missing.';
    showSetup.value = true;
    return false;
  }

  if (token && cls) {
    sessionToken.value = token;
    classroomCode.value = cls;
    showSetup.value = false;
    fetchData();
    startPolling();
    return true;
  } else {
    showSetup.value = true;
    return false;
  }
}

const isLocked = ref(false);

export function useDashboard() {
  return {
    supabaseUrl,
    supabaseKey,
    sessionToken,
    classroomCode,
    showSetup,
    connecting,
    setupError,
    loginUsername,
    loginPassword,
    isSignUp,
    loginClassroom,
    rememberMe,
    isLocked,
    error,
    loading,
    loadingAssignments,
    students,
    assignments,
    expandedStudents,
    toastMessage,
    lastSyncedAt,
    syncTimeAgo,
    searchQuery,
    filterAlerts,
    searchInputRef,
    newTitle,
    newUrl,
    publishing,
    uploadingPdf,
    stats,
    filteredStudents,
    toggleAuthMode,
    handleAuth,
    loginTeacher,
    signupTeacher,
    logoutTeacher,
    fetchData,
    toggleClassroomLock,
    removeStudent,
    exportClassroomCSV,
    fetchAssignments,
    handleFileUpload,
    publishAssignment,
    deleteAssignment,
    toggleStudent,
    formatRatingLabel,
    formatTime,
    formatDate,
    formatRelativeTime,
    initSession,
    stopPolling
  };
}
