<template>
  <div class="modal-overlay" @click.self="$emit('dismiss')">
    <div class="modal-card">
      <h2>Local AI Setup (RAG)</h2>
      <p class="description">
        We will run system specs check, stage DLLs, and initialize the ONNX embedding engine. This
        will take a few seconds and run completely on your system.
      </p>

      <div class="rag-setup-box">
        <div class="setup-header">
          <span v-if="ragStatus" class="status-badge" :class="ragStatus">{{
            ragStatus.toUpperCase()
          }}</span>
          <span class="setup-msg">{{ ragMessage }}</span>
        </div>

        <div class="progress-bar-mini">
          <div class="progress-fill-mini" :style="{ width: ragPercent + '%' }"></div>
        </div>

        <p class="setup-detail">{{ ragDetail }}</p>
        <div v-if="ragError" class="error-banner">{{ ragError }}</div>
      </div>

      <div class="modal-actions">
        <button class="cancel-btn" :disabled="isSettingUpRag" @click="$emit('dismiss')">
          Cancel
        </button>

        <button
          v-if="!ragSetupCompleted"
          class="save-btn"
          :disabled="isSettingUpRag"
          @click="$emit('start')"
        >
          {{ isSettingUpRag ? 'Setting Up...' : 'Start Setup' }}
        </button>

        <button v-else class="save-btn" @click="$emit('finish')">Finish</button>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  isSettingUpRag: { type: Boolean, default: false },
  ragStatus: { type: String, default: '' },
  ragPercent: { type: Number, default: 0 },
  ragMessage: { type: String, default: '' },
  ragDetail: { type: String, default: '' },
  ragError: { type: String, default: '' },
  ragSetupCompleted: { type: Boolean, default: false },
})

defineEmits(['dismiss', 'start', 'finish'])
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
}

.modal-card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 20px;
  padding: 24px;
  width: 100%;
  max-width: 400px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.2);
}

.modal-card h2 {
  margin: 0;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 10px;
}

.rag-setup-box {
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  padding: 16px;
  margin: 20px 0;
  color: #ffffff;
}

.setup-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.status-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 4px;
  background: #a0a0a0;
  color: #121212;
}

.status-badge.checking {
  background: #f59e0b;
  color: #121212;
}
.status-badge.acquiring {
  background: #3b82f6;
  color: #ffffff;
}
.status-badge.verifying {
  background: #8b5cf6;
  color: #ffffff;
}
.status-badge.extracting {
  background: #14b8a6;
  color: #ffffff;
}
.status-badge.initializing {
  background: #06b6d4;
  color: #ffffff;
}
.status-badge.ready {
  background: #10b981;
  color: #ffffff;
}
.status-badge.failed {
  background: #ef4444;
  color: #ffffff;
}

.setup-msg {
  font-size: 13px;
  font-weight: 600;
  color: #ffffff;
}

.progress-bar-mini {
  height: 6px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 3px;
  overflow: hidden;
  margin-bottom: 8px;
}

.progress-fill-mini {
  height: 100%;
  background: #6366f1;
  transition: width 0.3s ease;
}

.setup-detail {
  font-size: 11px;
  color: #888888;
  margin: 0;
}

.cancel-btn {
  background: none;
  border: 1px solid var(--outline-variant);
  padding: 10px 20px;
  border-radius: 10px;
  font-weight: 700;
  cursor: pointer;
  color: var(--on-surface);
}

.cancel-btn:hover {
  background: var(--surface-container-low);
}

.save-btn {
  border: 0;
  border-radius: 12px;
  padding: 12px 24px;
  color: var(--on-primary);
  font-weight: 700;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.save-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px color-mix(in srgb, var(--primary) 25%, transparent);
}

.save-btn:active {
  transform: translateY(0);
}

.save-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
