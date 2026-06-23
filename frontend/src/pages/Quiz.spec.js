import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import Quiz from './Quiz.vue'
import * as appApi from '../services/appApi'

// Create a reactive query object to simulate changing URL queries dynamically
const routeQuery = ref({})

// Mock services/appApi
vi.mock('../services/appApi', () => ({
  activateTask: vi.fn(),
  getTask: vi.fn(),
  submitQuizAttempt: vi.fn(),
  getNotebooks: vi.fn(),
  generateQuizForPageRange: vi.fn(),
  generateFlashcardsForQuizTask: vi.fn()
}))

// Mock vue-router hooks
vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: routeQuery.value
  }),
  useRouter: () => ({
    push: vi.fn()
  })
}))

describe('Quiz.vue Integration & State', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeQuery.value = {}
  })

  it('renders select notebook selector when no taskId is provided', async () => {
    appApi.getNotebooks.mockResolvedValue([
      { id: 'nb-1', title: 'Calculus 101' }
    ])

    const wrapper = mount(Quiz)
    await flushPromises()

    expect(wrapper.find('#quiz-notebook-select').exists()).toBe(true)
    expect(wrapper.find('option[value="nb-1"]').text()).toBe('Calculus 101')
  })

  it('loads quiz questions from active task when taskId query is present', async () => {
    routeQuery.value = { taskId: 'task-123' }

    appApi.getNotebooks.mockResolvedValue([])
    appApi.activateTask.mockResolvedValue({ error: null })
    appApi.getTask.mockResolvedValue({
      task: {
        id: 'task-123',
        task_type: 'QUIZ',
        payload_json: JSON.stringify({
          questions: [
            {
              id: 'q1',
              prompt: 'What is 2+2?',
              options: ['3', '4', '5'],
              correct_answer: '4'
            }
          ],
          passing_score: 100
        })
      }
    })

    const wrapper = mount(Quiz)
    await flushPromises()

    // Loading should finish
    expect(wrapper.find('.state-panel').exists()).toBe(false)
    expect(wrapper.text()).toContain('What is 2+2?')
    expect(wrapper.find('#quiz-submit-btn').attributes('disabled')).toBeDefined()
  })

  it('completes quiz submission and displays passed result', async () => {
    routeQuery.value = { task_id: 'task-123' }

    appApi.getNotebooks.mockResolvedValue([])
    appApi.activateTask.mockResolvedValue({ error: null })
    appApi.getTask.mockResolvedValue({
      task: {
        id: 'task-123',
        task_type: 'QUIZ',
        payload_json: JSON.stringify({
          questions: [
            {
              id: 'q1',
              prompt: 'Is testing good?',
              options: ['Yes', 'No'],
              correct_answer: 'Yes'
            }
          ],
          passing_score: 100
        })
      }
    })

    appApi.submitQuizAttempt.mockResolvedValue({
      result: {
        score: 100,
        passed: true,
        passing_score: 100,
        feedback: 'Excellent work!',
        flashcards_pending: false
      }
    })

    const wrapper = mount(Quiz)
    await flushPromises()

    // Select the "Yes" radio button option
    const radio = wrapper.find('input[type="radio"][value="Yes"]')
    await radio.setValue(true)

    // Form should now allow submission
    const submitBtn = wrapper.find('#quiz-submit-btn')
    expect(submitBtn.element.disabled).toBe(false)

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(appApi.submitQuizAttempt).toHaveBeenCalledWith('task-123', [
      { question_id: 'q1', selected: 'Yes' }
    ])
    expect(wrapper.find('.result-panel').exists()).toBe(true)
    expect(wrapper.text()).toContain('Passed')
    expect(wrapper.text()).toContain('100%')
  })

  it('handles API error elegantly during quiz loading', async () => {
    routeQuery.value = { taskId: 'task-err' }

    appApi.getNotebooks.mockResolvedValue([])
    appApi.activateTask.mockResolvedValue({ error: 'System overload' })

    const wrapper = mount(Quiz)
    await flushPromises()

    expect(wrapper.find('.state-panel--error').exists()).toBe(true)
    expect(wrapper.find('.state-text').text()).toBe('System overload')
  })
})
