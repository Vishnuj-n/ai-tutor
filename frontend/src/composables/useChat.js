import { ref, nextTick } from 'vue'
import { askReaderAI, logFrontendEvent } from '../services/appApi'
import { renderMarkdown } from '../services/markdown'

/**
 * useChat - Extracted AI chat logic for reader component.
 * Handles: chat state, message history, sending messages, markdown rendering.
 * Does NOT handle: reading session state, page navigation (see useReaderBase.js).
 */
export function useChat() {
  // Chat state
  const chatCollapsed = ref(true)
  const chatMessages = ref([])
  const chatInput = ref('')
  const chatLoading = ref(false)
  const chatError = ref('')
  const chatScope = ref('entire_notebook')
  const messagesPane = ref(null)

  /**
   * Toggle chat panel visibility
   */
  function toggleChat() {
    chatCollapsed.value = !chatCollapsed.value
    logFrontendEvent('info', 'ReaderChat', 'chat_toggled', { collapsed: chatCollapsed.value })
  }

  /**
   * Clear chat history
   */
  function clearChat() {
    chatMessages.value = []
    chatError.value = ''
  }

  /**
   * Send a chat message
   * @param {object} context - Reader retrieval context
   * @param {string} context.topicID - Topic identifier (required)
   * @param {string} context.notebookID - Notebook identifier (required)
   * @param {number} context.currentPage - Current page number (required)
   * @param {number} [context.chapterStartPage] - Chapter start page (optional)
   * @param {number} [context.chapterEndPage] - Chapter end page (optional)
   * @returns {Promise<boolean>} Success status
   */
  async function sendMessage(context) {
    const question = chatInput.value.trim()
    if (!question || !context?.topicID) {
      return false
    }

    logFrontendEvent('info', 'ReaderChat', 'send_message_start', {
      scope: chatScope.value,
      topicID: context.topicID,
      notebookID: context.notebookID,
      currentPage: context.currentPage,
      questionLength: question.length,
    })

    chatInput.value = ''
    chatError.value = ''
    const userMsgId =
      typeof crypto !== 'undefined' && crypto.randomUUID
        ? crypto.randomUUID()
        : Math.random().toString(36).substring(2, 9)
    chatMessages.value.push({ id: userMsgId, role: 'user', text: question })
    chatLoading.value = true

    try {
      const result = await askReaderAI(
        context.topicID,
        context.notebookID,
        question,
        chatScope.value,
        context.currentPage,
        context.chapterStartPage,
        context.chapterEndPage
      )

      if (result?.error) {
        chatError.value = result.error
        chatLoading.value = false
        logFrontendEvent('error', 'ReaderChat', 'send_message_api_error', {
          error: result.error,
          topicID: context.topicID,
        })
        return false
      }

      const assistantMsgId =
        typeof crypto !== 'undefined' && crypto.randomUUID
          ? crypto.randomUUID()
          : Math.random().toString(36).substring(2, 9)
      chatMessages.value.push({
        id: assistantMsgId,
        role: 'assistant',
        text: result?.answer || 'No answer returned.',
      })

      logFrontendEvent('info', 'ReaderChat', 'send_message_success', {
        topicID: context.topicID,
        answerLength: (result?.answer || '').length,
      })

      // Auto-scroll to bottom
      await nextTick()
      if (messagesPane.value) {
        messagesPane.value.scrollTop = messagesPane.value.scrollHeight
      }

      return true
    } catch (err) {
      const errMsg = err?.message || String(err)
      chatError.value = errMsg
      logFrontendEvent('error', 'ReaderChat', 'send_message_exception', {
        error: errMsg,
        topicID: context?.topicID,
      })
      return false
    } finally {
      chatLoading.value = false
    }
  }

  /**
   * Check if chat can be used (has topic context)
   * @param {string} topicID
   * @returns {boolean}
   */
  function canChat(topicID) {
    return Boolean(topicID)
  }

  return {
    // State
    chatCollapsed,
    chatMessages,
    chatInput,
    chatLoading,
    chatError,
    chatScope,
    messagesPane,

    // Methods
    toggleChat,
    clearChat,
    sendMessage,
    canChat,
    renderMarkdown,
  }
}
