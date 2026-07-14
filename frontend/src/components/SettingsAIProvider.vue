<template>
  <article class="panel form-grid">
    <h2>AI Provider</h2>
    <p class="hint" style="margin-top: -10px; margin-bottom: 8px">
      Provider settings are saved in SQLite. API keys are saved in the OS credential manager
      through the backend.
    </p>

    <div class="form-group">
      <label for="settings-llm-provider">Provider</label>
      <select
        id="settings-llm-provider"
        v-model="llmSettings.fast.provider"
        :disabled="disabled"
        @change="$emit('apply-preset', 'fast')"
      >
        <option value="groq">Groq</option>
        <option value="openai">ChatGPT / OpenAI</option>
        <option value="openrouter">OpenRouter</option>
        <option value="custom">Custom OpenAI-compatible</option>
      </select>
    </div>

    <div class="form-group">
      <label for="settings-llm-base-url">Base URL</label>
      <input
        id="settings-llm-base-url"
        v-model="llmSettings.fast.base_url"
        type="url"
        :disabled="disabled"
      />
    </div>

    <div class="form-group">
      <label for="settings-llm-model">Fast Model</label>
      <input
        id="settings-llm-model"
        v-model="llmSettings.fast.model"
        type="text"
        :disabled="disabled"
      />
      <p class="hint">Used for quizzes, flashcards, short scoring, and small reader help.</p>
    </div>

    <div class="form-group">
      <label for="settings-llm-key">Fast API Key</label>
      <input
        id="settings-llm-key"
        :value="llmFastKey"
        type="password"
        placeholder="Leave blank to keep existing key"
        :disabled="disabled"
        @input="$emit('update:llmFastKey', $event.target.value)"
      />
      <p class="hint">
        {{ llmSettings.fast.has_api_key ? 'A fast-tier key is stored.' : 'No fast-tier key stored yet.' }}
      </p>
    </div>

    <SettingsToggle
      v-model="llmSettings.use_same_for_heavy"
      :disabled="disabled"
      title="Use same provider and model for heavy AI tasks"
      hint="Heavy tasks include syllabus drafting, Socratic responses, and large-context generation."
    />

    <div v-if="!llmSettings.use_same_for_heavy" class="llm-advanced">
      <div class="form-group">
        <label for="settings-heavy-provider">Heavy Provider</label>
        <select
          id="settings-heavy-provider"
          v-model="llmSettings.heavy.provider"
          :disabled="disabled"
          @change="$emit('apply-preset', 'heavy')"
        >
          <option value="groq">Groq</option>
          <option value="openai">ChatGPT / OpenAI</option>
          <option value="openrouter">OpenRouter</option>
          <option value="custom">Custom OpenAI-compatible</option>
        </select>
      </div>
      <div class="form-group">
        <label for="settings-heavy-base-url">Heavy Base URL</label>
        <input
          id="settings-heavy-base-url"
          v-model="llmSettings.heavy.base_url"
          type="url"
          :disabled="disabled"
        />
      </div>
      <div class="form-group">
        <label for="settings-heavy-model">Heavy Model</label>
        <input
          id="settings-heavy-model"
          v-model="llmSettings.heavy.model"
          type="text"
          :disabled="disabled"
        />
      </div>
      <div class="form-group">
        <label for="settings-heavy-key">Heavy API Key</label>
        <input
          id="settings-heavy-key"
          :value="llmHeavyKey"
          type="password"
          placeholder="Leave blank to keep existing key"
          :disabled="disabled"
          @input="$emit('update:llmHeavyKey', $event.target.value)"
        />
        <p class="hint">
          {{ llmSettings.heavy.has_api_key ? 'A heavy-tier key is stored.' : 'No heavy-tier key stored yet.' }}
        </p>
      </div>
    </div>

    <div class="button-row">
      <button type="button" class="sync-btn" :disabled="disabled" @click="$emit('remove-keys')">
        Remove Stored Keys
      </button>
    </div>
  </article>
</template>

<script setup>
import SettingsToggle from './SettingsToggle.vue'

defineProps({
  llmSettings: { type: Object, required: true },
  llmFastKey: { type: String, required: true },
  llmHeavyKey: { type: String, required: true },
  disabled: { type: Boolean, default: false },
})

defineEmits(['apply-preset', 'remove-keys', 'update:llmFastKey', 'update:llmHeavyKey'])
</script>

<style scoped>
label {
  font-weight: 600;
  font-size: 14px;
  color: var(--on-surface);
}

input[type='text'],
input[type='url'],
input[type='password'],
select {
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 12px 14px;
  font-size: 14px;
  font-family: inherit;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

input:focus,
select:focus {
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

.llm-advanced {
  display: grid;
  gap: 16px;
  padding: 16px;
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  border-radius: 12px;
  background: var(--surface-container-low);
}

.button-row {
  display: flex;
  gap: 12px;
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

.sync-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
