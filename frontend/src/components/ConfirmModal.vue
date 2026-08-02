<script setup>
import { useDialog } from '../composables/useDialog'

const { dialogState, handleConfirm, handleCancel } = useDialog()
</script>

<template>
  <Teleport to="body">
    <Transition name="dialog-fade">
      <div v-if="dialogState.isOpen" class="dialog-overlay" @click.self="handleCancel">
        <div class="dialog-card" :class="`type-${dialogState.type}`">
          <div class="dialog-header">
            <div class="dialog-icon-wrapper" :class="dialogState.type">
              <svg v-if="dialogState.type === 'danger'" xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="3 6 5 6 21 6"></polyline>
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                <line x1="10" y1="11" x2="10" y2="17"></line>
                <line x1="14" y1="11" x2="14" y2="17"></line>
              </svg>
              <svg v-else-if="dialogState.type === 'warning'" xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"></path>
                <line x1="12" y1="9" x2="12" y2="13"></line>
                <line x1="12" y1="17" x2="12.01" y2="17"></line>
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="16" x2="12" y2="12"></line>
                <line x1="12" y1="8" x2="12.01" y2="8"></line>
              </svg>
            </div>
            <div class="dialog-titles">
              <h3 class="dialog-title">{{ dialogState.title }}</h3>
            </div>
          </div>

          <div v-if="dialogState.message" class="dialog-body">
            <p class="dialog-message">{{ dialogState.message }}</p>
          </div>

          <div class="dialog-actions">
            <button
              v-if="dialogState.cancelText"
              type="button"
              class="dialog-btn cancel-btn"
              @click="handleCancel"
            >
              {{ dialogState.cancelText }}
            </button>
            <button
              type="button"
              class="dialog-btn confirm-btn"
              :class="dialogState.type"
              @click="handleConfirm"
            >
              {{ dialogState.confirmText }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.dialog-overlay {
  position: fixed;
  inset: 0;
  z-index: 100000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.65);
  backdrop-filter: blur(6px);
  padding: 16px;
}

.dialog-card {
  width: 100%;
  max-width: 420px;
  background: var(--surface-container-lowest, #18181b);
  border: 1px solid var(--border-color, rgba(255, 255, 255, 0.12));
  border-radius: 16px;
  padding: 24px;
  box-shadow:
    0 20px 25px -5px rgba(0, 0, 0, 0.4),
    0 8px 10px -6px rgba(0, 0, 0, 0.3);
  display: flex;
  flex-direction: column;
  gap: 16px;
  color: var(--on-surface, #f4f4f5);
}

.dialog-header {
  display: flex;
  align-items: center;
  gap: 14px;
}

.dialog-icon-wrapper {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.dialog-icon-wrapper.danger {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.25);
}

.dialog-icon-wrapper.warning {
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.25);
}

.dialog-icon-wrapper.info {
  background: rgba(59, 130, 246, 0.15);
  color: #3b82f6;
  border: 1px solid rgba(59, 130, 246, 0.25);
}

.dialog-titles {
  flex: 1;
}

.dialog-title {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
  line-height: 1.3;
  color: var(--on-surface, #f4f4f5);
}

.dialog-body {
  margin-top: -4px;
}

.dialog-message {
  margin: 0;
  font-size: 0.9rem;
  color: var(--muted-text, #a1a1aa);
  line-height: 1.5;
}

.dialog-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 8px;
}

.dialog-btn {
  padding: 8px 16px;
  border-radius: 10px;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.15s ease;
}

.dialog-btn:active {
  transform: scale(0.97);
}

.cancel-btn {
  background: transparent;
  color: var(--muted-text, #a1a1aa);
  border-color: var(--border-color, rgba(255, 255, 255, 0.12));
}

.cancel-btn:hover {
  background: rgba(255, 255, 255, 0.06);
  color: var(--on-surface, #ffffff);
}

.confirm-btn.danger {
  background: #dc2626;
  color: #ffffff;
}

.confirm-btn.danger:hover {
  background: #b91c1c;
}

.confirm-btn.warning {
  background: #d97706;
  color: #ffffff;
}

.confirm-btn.warning:hover {
  background: #b45309;
}

.confirm-btn.info {
  background: var(--primary, #3b82f6);
  color: #ffffff;
}

.confirm-btn.info:hover {
  background: var(--primary-hover, #2563eb);
}

/* Vue Transitions */
.dialog-fade-enter-active,
.dialog-fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.dialog-fade-enter-from,
.dialog-fade-leave-to {
  opacity: 0;
}

.dialog-fade-enter-from .dialog-card {
  transform: scale(0.94);
}

.dialog-fade-leave-to .dialog-card {
  transform: scale(0.96);
}
</style>
