import { ref, watch } from 'vue'
import {
  getLLMSettings, updateLLMSettings, saveLLMAPIKey,
  deleteLLMAPIKey, getLLMProviderPreset,
} from '../services/appApi'

export function useLLM(loading, errorRef, successRef) {
  const savingLLM = ref(false)
  const presetLoading = ref(false)

  const llmSettings = ref({
    use_same_for_heavy: true,
    fast: {
      tier: 'fast', provider: 'groq',
      base_url: 'https://api.groq.com/openai',
      model: 'openai/gpt-oss-120b',
      timeout_ms: 60000, api_key_source: 'keyring', has_api_key: false,
    },
    heavy: {
      tier: 'heavy', provider: 'groq',
      base_url: 'https://api.groq.com/openai',
      model: 'openai/gpt-oss-120b',
      timeout_ms: 90000, api_key_source: 'keyring', has_api_key: false,
    },
  })

  const llmFastKey = ref('')
  const llmHeavyKey = ref('')

  let saveTimer = null
  watch([llmSettings, llmFastKey, llmHeavyKey], () => {
    if (loading.value || savingLLM.value) return
    clearTimeout(saveTimer)
    saveTimer = setTimeout(() => saveLLMProviderSettings(), 800)
  }, { deep: true })

  async function applyProviderPreset(tier) {
    presetLoading.value = true
    errorRef.value = ''
    const target = tier === 'heavy' ? llmSettings.value.heavy : llmSettings.value.fast
    try {
      const preset = await getLLMProviderPreset(target.provider)
      target.base_url = preset.base_url
      target.model = preset.model
    } catch (err) {
      errorRef.value = err.message || `Failed to load preset for ${target.provider}.`
    } finally {
      presetLoading.value = false
    }
  }

  async function saveLLMProviderSettings() {
    if (presetLoading.value || errorRef.value) return
    errorRef.value = ''
    successRef.value = ''
    try {
      savingLLM.value = true
      const res = await updateLLMSettings({
        use_same_for_heavy: llmSettings.value.use_same_for_heavy,
        fast: { ...llmSettings.value.fast },
        heavy: { ...llmSettings.value.heavy },
      })
      if (res.error) { errorRef.value = res.error; return }

      if (llmFastKey.value.trim()) {
        const keyRes = await saveLLMAPIKey('fast', llmFastKey.value.trim())
        if (keyRes.error) { errorRef.value = keyRes.error; return }
        if (llmSettings.value.use_same_for_heavy) {
          const heavyKeyRes = await saveLLMAPIKey('heavy', llmFastKey.value.trim())
          if (heavyKeyRes.error) { errorRef.value = heavyKeyRes.error; return }
        }
      }
      if (!llmSettings.value.use_same_for_heavy && llmHeavyKey.value.trim()) {
        const keyRes = await saveLLMAPIKey('heavy', llmHeavyKey.value.trim())
        if (keyRes.error) { errorRef.value = keyRes.error; return }
      }

      llmFastKey.value = ''
      llmHeavyKey.value = ''
      successRef.value = 'AI provider settings updated successfully.'
      setTimeout(() => (successRef.value = ''), 4000)
    } catch (err) {
      errorRef.value = err.message || 'Failed to save AI provider settings'
    } finally {
      savingLLM.value = false
    }
  }

  async function removeLLMKeys() {
    if (!confirm('Remove stored LLM API keys from the OS credential manager?')) return
    errorRef.value = ''
    successRef.value = ''
    try {
      savingLLM.value = true
      const fastRes = await deleteLLMAPIKey('fast')
      if (fastRes.error) { errorRef.value = fastRes.error; return }
      const heavyRes = await deleteLLMAPIKey('heavy')
      if (heavyRes.error) { errorRef.value = heavyRes.error; return }
      successRef.value = 'Stored AI provider keys removed.'
      setTimeout(() => (successRef.value = ''), 4000)
    } catch (err) {
      errorRef.value = err.message || 'Failed to remove stored keys'
    } finally {
      savingLLM.value = false
    }
  }

  async function loadLLM() {
    const res = await getLLMSettings()
    if (res.error) { errorRef.value = res.error; return false }
    if (res.settings) llmSettings.value = res.settings
    return true
  }

  function cleanup() {
    clearTimeout(saveTimer)
  }

  return {
    llmSettings, llmFastKey, llmHeavyKey,
    savingLLM, presetLoading,
    applyProviderPreset, saveLLMProviderSettings, removeLLMKeys,
    loadLLM, cleanup,
  }
}
