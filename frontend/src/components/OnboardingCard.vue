<template>
  <div class="card onboarding-card">
    <div class="onboarding-content">
      <h2>Your study queue is empty</h2>
      <p class="onboarding-desc">
        Upload a PDF textbook and the app builds a study queue of reading tasks, quizzes, and
        flashcards for you.
      </p>
      <div class="onboarding-steps">
        <div class="onboarding-step">
          <span class="step-number">1</span>
          <span class="step-label">Upload a PDF</span>
        </div>
        <div class="onboarding-divider"></div>
        <div class="onboarding-step">
          <span class="step-number">2</span>
          <span class="step-label">Read chapters</span>
        </div>
        <div class="onboarding-divider"></div>
        <div class="onboarding-step">
          <span class="step-number">3</span>
          <span class="step-label">Quiz & review</span>
        </div>
      </div>
      <button class="primary-btn onboarding-cta" @click="$emit('go-to-notebooks')">
        Upload your first textbook
      </button>
    </div>
  </div>
</template>

<script setup>
defineEmits(['go-to-notebooks'])
</script>

<style scoped>
.onboarding-card {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 16px;
  padding: 48px 40px;
  text-align: center;
}

.onboarding-content {
  max-width: 420px;
  margin: 0 auto;
}

.onboarding-card h2 {
  margin: 0 0 12px;
  font-size: 26px;
  font-family: 'Manrope', sans-serif;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.onboarding-desc {
  margin: 0 0 32px;
  font-size: 15px;
  color: var(--muted-text);
  line-height: 1.6;
}

.onboarding-steps {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0;
  margin-bottom: 36px;
}

.onboarding-step {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.step-number {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: rgba(108, 92, 231, 0.1);
  color: var(--primary);
  font-weight: 700;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.step-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--on-surface);
}

.onboarding-divider {
  width: 40px;
  height: 1px;
  background: var(--outline-variant);
  margin: 0 12px;
  margin-bottom: 20px;
}

.onboarding-cta {
  padding: 14px 32px;
  font-size: 15px;
  border-radius: 14px;
}

.primary-btn {
  background: var(--primary);
  color: var(--on-primary);
  border: none;
  border-radius: 10px;
  padding: 10px;
  font-weight: 700;
  cursor: pointer;
  transition: opacity 0.2s;
}

.primary-btn:hover {
  opacity: 0.9;
}
</style>
