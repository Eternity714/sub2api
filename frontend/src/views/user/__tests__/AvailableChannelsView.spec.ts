import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import AvailableChannelsView from '../AvailableChannelsView.vue'
import zh from '@/i18n/locales/zh'
import type { UserAvailableChannel } from '@/api/channels'

const { getAvailable, getUserGroupRates } = vi.hoisted(() => ({
  getAvailable: vi.fn(), getUserGroupRates: vi.fn()
}))
vi.mock('@/api/channels', () => ({ default: { getAvailable } }))
vi.mock('@/api/groups', () => ({ default: { getUserGroupRates } }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key.split('.').reduce<unknown>((value, part) =>
    (value as Record<string, unknown>)?.[part], zh) ?? key })
}))

const channels: UserAvailableChannel[] = [{
  name: '主渠道', description: '完整模型服务', platforms: [{
    platform: 'openai',
    groups: [1, 2].map(id => ({
      id, name: `分组 ${id}`, platform: 'openai', subscription_type: 'standard',
      rate_multiplier: id, peak_rate_enabled: false, peak_start: '', peak_end: '',
      peak_rate_multiplier: 1, is_exclusive: id === 2
    })),
    supported_models: [{
      name: 'gpt-test', platform: 'openai', pricing: {
        billing_mode: 'token', input_price: 3e-6, output_price: 15e-6,
        cache_write_price: 3.75e-6, cache_read_price: 3e-7,
        image_input_price: null, image_output_price: null, per_request_price: null, intervals: []
      }
    }]
  }]
}, {
  name: '媒体渠道', description: '', platforms: [{
    platform: 'sora', groups: [], supported_models: [{ name: 'sora-test', platform: 'sora', pricing: null }]
  }]
}]

async function render(list: UserAvailableChannel[] = channels) {
  getAvailable.mockResolvedValue(structuredClone(list))
  getUserGroupRates.mockResolvedValue({ 2: 0 })
  const wrapper = mount(AvailableChannelsView, {
    global: {
      plugins: [createPinia()],
      stubs: { AppLayout: { template: '<main><slot /></main>' } }
    }
  })
  await flushPromises()
  return wrapper
}

describe('AvailableChannelsView plaza filters', () => {
  it('keeps a shared group selectable across all of its channel platforms', async () => {
    const list = structuredClone(channels)
    list[0]!.platforms.push({
      platform: 'anthropic', groups: [list[0]!.platforms[0]!.groups[0]!],
      supported_models: [{ name: 'claude-shared', platform: 'anthropic', pricing: null }]
    })
    const wrapper = await render(list)
    try {
      const buttons = () => wrapper.findAll('button')
      await buttons().find(button => button.text() === 'openai')!.trigger('click')
      const group = buttons().find(button => button.text() === '分组 1')!
      expect(group.attributes('disabled')).toBeUndefined()
      await group.trigger('click')
      const platform = buttons().find(button => button.text() === 'anthropic')!
      expect(platform.attributes('disabled')).toBeUndefined()
      await platform.trigger('click')
      expect(wrapper.get('table tbody').text()).toContain('claude-shared')
      expect(buttons().filter(button => button.text() === '分组 1')).toHaveLength(1)
    } finally { wrapper.unmount() }
  })

  it('allows a platform without group metadata to be selected', async () => {
    const wrapper = await render()
    try {
      const platform = wrapper.findAll('button').find(button => button.text() === 'sora')!
      expect(platform.attributes('disabled')).toBeUndefined()
      await platform.trigger('click')
      expect(wrapper.text()).toContain('媒体渠道')
      expect(wrapper.text()).not.toContain('主渠道')
    } finally { wrapper.unmount() }
  })

  it('filters channel sections by group and personal rate without multiplying channel list prices', async () => {
    const wrapper = await render()
    try {
      const rate = wrapper.findAll('button').find(button => button.text() === '0x')!
      expect(rate, 'zero personal rates must be available as a filter').toBeDefined()
      await rate.trigger('click')
      expect(wrapper.text()).toContain('主渠道')
      expect(wrapper.text()).not.toContain('媒体渠道')
      expect(wrapper.get('section').text()).toContain('分组 2')
      expect(wrapper.get('section').text()).not.toContain('分组 1')
      expect(wrapper.get('table tbody').text()).toContain('$3.00')
      expect(wrapper.text()).toContain('渠道标价')
    } finally { wrapper.unmount() }
  })

  it('keeps channel search, empty state and refresh functional', async () => {
    const wrapper = await render()
    try {
      const input = wrapper.get('input')
      await input.setValue('sora-test')
      expect(wrapper.text()).toContain('媒体渠道')
      expect(wrapper.text()).not.toContain('主渠道')
      await input.setValue('missing')
      expect(wrapper.find('table').exists()).toBe(false)
      await input.setValue('完整模型')
      expect(wrapper.text()).toContain('gpt-test')
      const calls = getAvailable.mock.calls.length
      await wrapper.get('button[aria-label="刷新"]').trigger('click')
      await flushPromises()
      expect(getAvailable.mock.calls.length).toBe(calls + 1)
    } finally { wrapper.unmount() }
  })
})
