import { reactive } from 'vue'

const dialogState = reactive({
  isOpen: false,
  title: '',
  message: '',
  confirmText: 'Confirm',
  cancelText: 'Cancel',
  type: 'danger', // 'danger' | 'warning' | 'info'
  resolve: null,
})

const dialogQueue = []

function processQueue() {
  if (dialogState.isOpen || dialogQueue.length === 0) return
  const nextReq = dialogQueue.shift()
  dialogState.title = nextReq.title
  dialogState.message = nextReq.message
  dialogState.confirmText = nextReq.confirmText
  dialogState.cancelText = nextReq.cancelText
  dialogState.type = nextReq.type
  dialogState.resolve = nextReq.resolve
  dialogState.isOpen = true
}

function settle(value) {
  dialogState.isOpen = false
  const currentResolve = dialogState.resolve
  dialogState.resolve = null
  if (currentResolve) {
    currentResolve(value)
  }
  processQueue()
}

export function useDialog() {
  /**
   * Prompts a styled confirmation dialog returning a Promise<boolean>.
   * @param {Object} options
   * @param {string} options.title - Dialog heading
   * @param {string} options.message - Dialog message/description
   * @param {string} [options.confirmText='Confirm'] - Text for confirm button
   * @param {string} [options.cancelText='Cancel'] - Text for cancel button
   * @param {'danger'|'warning'|'info'} [options.type='danger'] - Type visual style
   * @returns {Promise<boolean>}
   */
  function confirm({ title = 'Are you sure?', message = '', confirmText = 'Confirm', cancelText = 'Cancel', type = 'danger' }) {
    return new Promise((resolve) => {
      dialogQueue.push({ title, message, confirmText, cancelText, type, resolve })
      processQueue()
    })
  }

  /**
   * Prompts a styled alert dialog returning a Promise<void>.
   * @param {string|Object} titleOrOptions
   * @param {string} [message]
   */
  function alert(titleOrOptions, message = '') {
    const title = typeof titleOrOptions === 'string' ? titleOrOptions : (titleOrOptions.title || 'Notice')
    const msg = typeof titleOrOptions === 'string' ? message : (titleOrOptions.message || '')
    const confirmText = typeof titleOrOptions === 'object' && titleOrOptions.confirmText ? titleOrOptions.confirmText : 'OK'
    const type = typeof titleOrOptions === 'object' && titleOrOptions.type ? titleOrOptions.type : 'info'

    return new Promise((resolve) => {
      dialogQueue.push({
        title,
        message: msg,
        confirmText,
        cancelText: null,
        type,
        resolve: () => resolve(),
      })
      processQueue()
    })
  }

  function handleConfirm() {
    settle(true)
  }

  function handleCancel() {
    settle(false)
  }

  return {
    dialogState,
    confirm,
    alert,
    handleConfirm,
    handleCancel,
  }
}
