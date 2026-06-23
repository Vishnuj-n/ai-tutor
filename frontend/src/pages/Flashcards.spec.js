import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import Flashcards from './Flashcards.vue'
import * as appApi from '../services/appApi'

const routeQuery = ref({})

// Mock services/appApi
vi.mock('../services/appApi', () => ({
  activateTask: vi.fn(),
  completeReviewSession: vi.fn(),
  getNotebooks: vi.fn(),
  generateManualFlashcards: vi.fn(),
  getReviewSession: vi.fn(),
  recordCardReview: vi.fn()
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

describe('Flashcards.vue Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeQuery.value = {}
  })

  it('loads notebooks and renders config panel by default', async () => {
    appApi.getNotebooks.mockResolvedValue([
      { id: 'nb-1', title: 'Calculus 101' }
    ])

    const wrapper = mount(Flashcards)
    await vi.dynamicImportSettled()

    expect(wrapper.find('#fc-notebook-select').exists()).toBe(true)
    expect(wrapper.find('#fc-generate-btn').exists()).toBe(true)
  })

  it('runs comprehensive flashcard generation and reviews cards', async () => {
    appApi.getNotebooks.mockResolvedValue([{ id: 'nb-1', title: 'Calculus 101' }])
    appApi.generateManualFlashcards.mockResolvedValue({
      cards: [
        { id: 'fc-1', prompt: 'Derivative of x^2?', answer: '2x' }
      ]
    })

    const wrapper = mount(Flashcards)
    await vi.dynamicImportSettled()

    // Select notebook and input page range
    await wrapper.find('#fc-notebook-select').setValue('nb-1')
    await wrapper.find('#fc-generate-btn').trigger('click')
    await vi.dynamicImportSettled()

    // Review session should be active
    expect(wrapper.find('.review-session').exists()).toBe(true)
    expect(wrapper.text()).toContain('Derivative of x^2?')
    expect(wrapper.find('.flashcard').classes()).not.toContain('flipped')

    // Click "Show Answer"
    await wrapper.find('#fc-reveal-btn').trigger('click')
    expect(wrapper.find('.flashcard').classes()).toContain('flipped')
    expect(wrapper.text()).toContain('2x')

    // Click Good rating
    await wrapper.find('#fc-rate-good').trigger('click')
    await vi.dynamicImportSettled()

    // Completed session panel
    expect(wrapper.find('.done-panel').exists()).toBe(true)
    expect(wrapper.text()).toContain('1 card reviewed')
  })
})
