<template>
  <article class="panel form-grid">
    <h2>Account &amp; Cloud</h2>

    <div v-if="settings.cloud_api_token" class="signed-in-box">
      <div class="status-indicator">
        <span class="pulse-dot active"></span>
        <strong>Cloud Sync Active</strong>
      </div>
      <div class="user-details">
        <p><strong>Username:</strong> {{ settings.student_username || 'Student' }}</p>
        <p><strong>Classroom:</strong> {{ settings.classroom_code }}</p>
      </div>
      <button type="button" class="sync-btn danger-btn" @click="$emit('logout')">Sign Out</button>
    </div>

    <div v-else class="login-form-container">
      <p class="field-hint" style="margin-bottom: 1.25rem">
        Sign in with your student credentials to enable cloud sync and receive assignments.
      </p>

      <div v-if="loginError" class="login-error-message">
        {{ loginError }}
      </div>

      <div class="form-group">
        <label for="student-username">Student Username / ID</label>
        <input
          id="student-username"
          :value="loginUsername"
          type="text"
          placeholder="e.g. john_doe"
          :disabled="loggingIn"
          @input="$emit('update:loginUsername', $event.target.value)"
        />
      </div>

      <div class="form-group">
        <label for="student-password">Password</label>
        <input
          id="student-password"
          :value="loginPassword"
          type="password"
          placeholder="••••••••"
          :disabled="loggingIn"
          @input="$emit('update:loginPassword', $event.target.value)"
        />
      </div>

      <button type="button" class="sync-btn" :disabled="loggingIn" @click="$emit('login')">
        {{ loggingIn ? 'Signing In...' : 'Sign In & Sync' }}
      </button>
    </div>

    <div v-if="isDev" class="form-group dev-section">
      <label for="cloud-url">
        Sync Server URL
        <span class="dev-badge">DEV</span>
      </label>
      <input
        id="cloud-url"
        v-model="settings.cloud_sync_url"
        type="url"
        placeholder="https://example.com/api/sync"
        :disabled="disabled"
      />
    </div>
  </article>
</template>

<script setup>
defineProps({
  settings: { type: Object, required: true },
  isDev: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
  loginUsername: { type: String, required: true },
  loginPassword: { type: String, required: true },
  loginError: { type: String, default: '' },
  loggingIn: { type: Boolean, default: false },
})

defineEmits([
  'login',
  'logout',
  'update:loginUsername',
  'update:loginPassword',
])
</script>

<style scoped>
label {
  font-weight: 600;
  font-size: 14px;
  color: var(--on-surface);
}

input[type='text'],
input[type='password'],
input[type='url'] {
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 12px 14px;
  font-size: 14px;
  font-family: inherit;
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease;
}

input:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--primary) 15%, transparent);
  outline: none;
}

.hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted-text);
  line-height: 1.4;
}

.form-grid {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

h2 {
  font-size: 20px;
  margin: 0 0 16px;
  font-weight: 700;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 28px;
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  box-shadow: 0 4px 20px color-mix(in srgb, var(--on-surface) 3%, transparent);
}

.signed-in-box {
  background: var(--surface-low);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--success);
}

.pulse-dot.active {
  width: 8px;
  height: 8px;
  background: var(--success);
  border-radius: 50%;
  box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7);
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0% {
    transform: scale(0.95);
    box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7);
  }
  70% {
    transform: scale(1);
    box-shadow: 0 0 0 6px rgba(16, 185, 129, 0);
  }
  100% {
    transform: scale(0.95);
    box-shadow: 0 0 0 0 rgba(16, 185, 129, 0);
  }
}

.user-details {
  font-size: 0.9rem;
  color: var(--on-surface);
  line-height: 1.5;
}

.user-details p {
  margin: 0.25rem 0;
}

.danger-btn {
  background: rgba(239, 68, 68, 0.1) !important;
  border: 1px solid rgba(239, 68, 68, 0.3) !important;
  color: #ef4444 !important;
  transition: all 0.2s ease;
}

.danger-btn:hover {
  background: rgba(239, 68, 68, 0.2) !important;
  border-color: rgba(239, 68, 68, 0.5) !important;
}

.sync-btn {
  border: none;
  border-radius: 12px;
  padding: 12px 24px;
  color: var(--primary);
  font-weight: 700;
  background: var(--surface-container-highest);
  cursor: pointer;
  transition: all 0.2s ease;
}

.sync-btn:hover {
  background: var(--surface-container-low);
}

.login-form-container {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.login-error-message {
  background: rgba(239, 68, 68, 0.08);
  border: 1px solid rgba(239, 68, 68, 0.2);
  color: #f87171;
  padding: 0.75rem 1rem;
  border-radius: 8px;
  font-size: 0.85rem;
}

.field-hint {
  margin: 2px 0 8px;
  color: var(--muted-text);
  font-size: 12px;
  line-height: 1.4;
}

.dev-section {
  margin-top: 1.5rem;
  border-top: 1px solid var(--border);
  padding-top: 1.5rem;
}

.dev-badge {
  display: inline-block;
  margin-left: 6px;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.05em;
  background: color-mix(in srgb, var(--warning, #f0a000) 20%, transparent);
  color: var(--warning, #f0a000);
  vertical-align: middle;
}
</style>
