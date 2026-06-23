import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ErrorMessage from './ErrorMessage.vue'

describe('ErrorMessage.vue', () => {
  it('renders message when message prop is provided', () => {
    const wrapper = mount(ErrorMessage, {
      props: {
        message: 'Something went wrong!'
      }
    })
    expect(wrapper.text()).toContain('Something went wrong!')
    expect(wrapper.find('.error-msg').exists()).toBe(true)
  })

  it('does not render when message prop is empty', () => {
    const wrapper = mount(ErrorMessage, {
      props: {
        message: ''
      }
    })
    expect(wrapper.find('.error-msg').exists()).toBe(false)
  })
})
