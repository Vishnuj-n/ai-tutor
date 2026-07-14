import { ref, watch } from 'vue'
import { getUserSettings, updateUserSettings } from '../services/appApi'

export function useSettings(errorRef, successRef) {
  const loading = ref(true)
  const saving = ref(false)

  const settings = ref({
    max_flashcards_per_session: 30,
    study_start_time: '17:00',
    study_end_time: '18:00',
    reminders_enabled: true,
    active_profile_id: '',
    skip_to_reading_active: false,
    cloud_sync_url: '',
    cloud_api_token: '',
    theme: 'light-classic',
    rag_enabled: false,
    rag_notebook_chapter: true,
    rag_entire_notebook: true,
    rag_queue_study: true,
    default_remedial_strategy: 'CLASSIC',
    classroom_code: '',
    analytics_enabled: false,
  })

  const studyDuration = ref('')

  function computeDuration() {
    const start = settings.value.study_start_time
    const end = settings.value.study_end_time
    if (!start || !end) { studyDuration.value = ''; return }
    const [startH, startM] = start.split(':').map(Number)
    const [endH, endM] = end.split(':').map(Number)
    const diff = endH * 60 + endM - (startH * 60 + startM)
    if (diff <= 0) { studyDuration.value = ''; return }
    if (diff < 60) { studyDuration.value = `${diff} min`; return }
    const hours = Math.floor(diff / 60)
    const mins = diff % 60
    studyDuration.value = mins === 0
      ? (hours === 1 ? '1 hour' : `${hours} hours`)
      : `${hours}h ${mins}m`
  }

  function applyDurationPreset(preset) {
    const [h, m] = settings.value.study_start_time.split(':').map(Number)
    const endMinutes = h * 60 + m + preset.minutes
    const endH = Math.floor(endMinutes / 60) % 24
    const endM = endMinutes % 60
    settings.value.study_end_time = `${String(endH).padStart(2, '0')}:${String(endM).padStart(2, '0')}`
  }

  let saveTimer = null
  watch(settings, () => {
    computeDuration()
    if (loading.value) return

    const val = settings.value.max_flashcards_per_session
    if (typeof val !== 'number' || isNaN(val) || val < 5 || val > 200) {
      errorRef.value = 'Max flashcards per session must be between 5 and 200.'
      return
    }
    const start = settings.value.study_start_time
    const end = settings.value.study_end_time
    if (!start || !end || start >= end) {
      errorRef.value = 'Study start time must be strictly earlier than end time.'
      return
    }
    if (
      errorRef.value === 'Max flashcards per session must be between 5 and 200.' ||
      errorRef.value === 'Study start time must be strictly earlier than end time.'
    ) {
      errorRef.value = ''
    }

    clearTimeout(saveTimer)
    saveTimer = setTimeout(() => saveUserSettings(), 800)
  }, { deep: true })

  async function saveUserSettings() {
    errorRef.value = ''
    successRef.value = ''
    try {
      saving.value = true
      const s = settings.value
      const res = await updateUserSettings(
        s.max_flashcards_per_session, s.study_start_time, s.study_end_time,
        s.reminders_enabled, s.active_profile_id, s.skip_to_reading_active,
        s.cloud_sync_url, s.cloud_api_token, s.theme,
        s.rag_enabled, s.rag_notebook_chapter, s.rag_entire_notebook,
        s.rag_queue_study, s.default_remedial_strategy,
        s.classroom_code || '', s.analytics_enabled, s.anonymous_user_id || ''
      )
      if (res.error) { errorRef.value = res.error; return }
      successRef.value = 'Settings updated successfully.'
      window.dispatchEvent(new CustomEvent('settings-updated'))
      setTimeout(() => (successRef.value = ''), 4000)
    } catch (err) {
      errorRef.value = err.message || 'Failed to save settings'
    } finally {
      saving.value = false
    }
  }

  async function loadSettings() {
    const res = await getUserSettings()
    if (res.error) { errorRef.value = res.error; return false }
    if (!res.default_remedial_strategy) res.default_remedial_strategy = 'CLASSIC'
    settings.value = res
    computeDuration()
    return true
  }

  function cleanup() {
    clearTimeout(saveTimer)
  }

  return {
    settings, loading, saving,
    studyDuration, applyDurationPreset,
    loadSettings, saveUserSettings, cleanup,
  }
}
