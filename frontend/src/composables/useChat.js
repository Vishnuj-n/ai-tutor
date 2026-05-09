import { ref, nextTick } from 'vue'
import { askAI, explainReaderSection } from '../services/appApi'
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
   * @param {string} topicID - Current topic ID for context
   * @param {string} sectionID - Optional section ID for section-aware questions
   * @returns {Promise<boolean>} Success status
   */
  async function sendMessage(topicID, sectionID = '') {
    const question = chatInput.value.trim()
    if (!question || !topicID) {
      return false
    }

    chatInput.value = ''
    chatError.value = ''
    chatMessages.value.push({ role: 'user', text: question })
    chatLoading.value = true

    try {
      const result = sectionID
        ? await explainReaderSection(sectionID, question)
        : await askAI(topicID, question)

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
    messagesPane,

    // Methods
    toggleChat,
    clearChat,
    sendMessage,
    canChat,
    renderMarkdown
  }
}
