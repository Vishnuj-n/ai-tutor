<template>
  <div class="setup-overlay fade-in">
    <div class="setup-card animate-fade-in">
      <div style="text-align: center; margin-bottom: 1.75rem;">
        <div style="display: inline-flex; align-items: center; justify-content: center; width: 44px; height: 44px; border-radius: 12px; background: var(--ds-primary-glow); border: 1px solid var(--ds-primary-ring); margin-bottom: 0.75rem;">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--ds-primary)" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <polygon points="12 2 2 7 12 12 22 7 12 2"></polygon>
            <polyline points="2 17 12 22 22 17"></polyline>
            <polyline points="2 12 12 17 22 12"></polyline>
          </svg>
        </div>
        <h2 style="margin: 0 0 0.35rem 0; color: var(--ds-fg); font-size: 1.25rem; font-weight: 700; letter-spacing: -0.02em;">
          {{ isSignUp ? 'Create Teacher Account' : 'Teacher Portal Login' }}
        </h2>
        <p class="muted" style="font-size: 0.82rem; line-height: 1.4; margin: 0;">
          {{ isSignUp ? 'Sign up to manage assignments and monitor student progress.' : 'Sign in with your teacher credentials to access your dashboard.' }}
        </p>
      </div>

      <div v-if="setupError" class="error-message">
        <div style="flex: 1;">{{ setupError }}</div>
      </div>

      <form @submit.prevent="onAuth">
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

        <div v-if="isSignUp" class="form-group animate-fade-in">
          <label for="login-classroom">Default Classroom Code</label>
          <input
            id="login-classroom"
            v-model="loginClassroom"
            type="text"
            required
            placeholder="e.g. BIO101"
          />
        </div>

        <div v-if="!isSignUp" class="form-group" style="display: flex; flex-direction: row; align-items: center; gap: 0.5rem; margin-top: 0.25rem;">
          <input
            id="remember-me"
            v-model="rememberMe"
            type="checkbox"
            style="width: 16px; height: 16px; cursor: pointer; accent-color: var(--ds-primary);"
          />
          <label for="remember-me" style="margin-bottom: 0; font-size: 0.82rem; cursor: pointer; color: var(--ds-muted);">
            Remember me on this device
          </label>
        </div>

        <button class="btn" style="width: 100%; margin-top: 1.25rem; padding: 0.65rem;" :disabled="connecting">
          <span v-if="connecting" class="loading-spinner" style="width: 14px; height: 14px; border-width: 2px;"></span>
          {{ connecting ? (isSignUp ? 'Creating Account...' : 'Signing In...') : (isSignUp ? 'Sign Up' : 'Sign In') }}
        </button>

        <div style="text-align: center; margin-top: 1.25rem;">
          <a href="#" @click.prevent="toggleAuthMode" style="font-size: 0.82rem; text-decoration: none; color: var(--ds-primary); font-weight: 500;">
            {{ isSignUp ? 'Already have an account? Sign In' : 'Need an account? Sign Up' }}
          </a>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router';
import { useDashboard } from '../composables/useDashboard';

const router = useRouter();
const {
  isSignUp,
  setupError,
  loginUsername,
  loginPassword,
  loginClassroom,
  rememberMe,
  connecting,
  toggleAuthMode,
  handleAuth
} = useDashboard();

function onAuth() {
  handleAuth(router);
}
</script>

