import { ref } from 'vue'
import { initializeRAG } from '../services/appApi'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

export function useRAG(settings) {
  const showRagModal = ref(false)
  const isSettingUpRag = ref(false)
  const ragStatus = ref('')
  const ragPercent = ref(0)
  const ragMessage = ref('')
  const ragDetail = ref('')
  const ragError = ref('')
  const ragSetupCompleted = ref(false)

  function onRagToggle(newValue) {
    if (newValue) {
      settings.value.rag_enabled = false
      showRagModal.value = true
      ragSetupCompleted.value = false
      ragError.value = ''
      ragPercent.value = 0
      ragMessage.value = 'Ready to initialize local AI'
      ragDetail.value = ''
      ragStatus.value = ''
    }
  }

  function startRagSetup() {
    ragError.value = ''
    isSettingUpRag.value = true
    ragStatus.value = 'checking'
    ragPercent.value = 5
    ragMessage.value = 'Checking system specifications...'
    ragDetail.value = ''

    EventsOff('rag-setup-progress')
    EventsOn('rag-setup-progress', (data) => {
      console.log('[Settings] RAG setup progress:', data)
      if (data.status) ragStatus.value = data.status
      if (data.percent !== undefined) ragPercent.value = data.percent
      if (data.message) ragMessage.value = data.message
      if (data.detail) ragDetail.value = data.detail
      if (data.errorReason) {
        ragError.value = data.errorReason
        isSettingUpRag.value = false
        EventsOff('rag-setup-progress')
      }
      if (data.status === 'ready') {
        ragSetupCompleted.value = true
        isSettingUpRag.value = false
        EventsOff('rag-setup-progress')
      }
    })

    initializeRAG()
      .then((res) => {
        if (res.error) {
          ragError.value = res.error
          isSettingUpRag.value = false
          EventsOff('rag-setup-progress')
        }
      })
      .catch((err) => {
        ragError.value = err.message || 'RAG setup failed.'
        isSettingUpRag.value = false
        EventsOff('rag-setup-progress')
      })
  }

  function handleRagModalDismiss() {
    if (isSettingUpRag.value) return
    if (ragSetupCompleted.value) {
      closeRagModal()
    } else {
      showRagModal.value = false
    }
  }

  function closeRagModal() {
    showRagModal.value = false
    settings.value.rag_enabled = true
  }

  function cleanup() {
    EventsOff('rag-setup-progress')
  }

  return {
    showRagModal,
    isSettingUpRag,
    ragStatus,
    ragPercent,
    ragMessage,
    ragDetail,
    ragError,
    ragSetupCompleted,
    onRagToggle,
    startRagSetup,
    handleRagModalDismiss,
    closeRagModal,
    cleanup,
  }
}
