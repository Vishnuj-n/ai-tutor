import { ref, nextTick } from 'vue'
import { askReaderAI } from '../services/appApi'
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
   * @returns {Promise<boolean>} Success status
   */
  async function sendMessage(context) {
    const question = chatInput.value.trim()
    if (!question || !context?.topicID) {
      return false
    }

    chatInput.value = ''
    chatError.value = ''
    chatMessages.value.push({ role: 'user', text: question })
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
        return false
      }

      chatMessages.value.push({
        role: 'assistant',
        text: result?.answer || 'No answer returned.'
      })

      // Auto-scroll to bottom
      await nextTick()
      if (messagesPane.value) {
        messagesPane.value.scrollTop = messagesPane.value.scrollHeight
      }

      return true
    } catch (err) {
      chatError.value = err?.message || 'Failed to send message'
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
    renderMarkdown
  }
}
