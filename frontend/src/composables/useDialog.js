import { ref, reactive } from 'vue'

const dialogState = reactive({
  isOpen: false,
  title: '',
  message: '',
  confirmText: 'Confirm',
  cancelText: 'Cancel',
  type: 'danger', // 'danger' | 'warning' | 'info'
  resolve: null,
})

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
      dialogState.title = title
      dialogState.message = message
      dialogState.confirmText = confirmText
      dialogState.cancelText = cancelText
      dialogState.type = type
      dialogState.resolve = resolve
      dialogState.isOpen = true
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
      dialogState.title = title
      dialogState.message = msg
      dialogState.confirmText = confirmText
      dialogState.cancelText = null // hides cancel button for alert mode
      dialogState.type = type
      dialogState.resolve = () => resolve()
      dialogState.isOpen = true
    })
  }

  function handleConfirm() {
    dialogState.isOpen = false
    if (dialogState.resolve) {
      dialogState.resolve(true)
      dialogState.resolve = null
    }
  }

  function handleCancel() {
    dialogState.isOpen = false
    if (dialogState.resolve) {
      dialogState.resolve(false)
      dialogState.resolve = null
    }
  }

  return {
    dialogState,
    confirm,
    alert,
    handleConfirm,
    handleCancel,
  }
}
