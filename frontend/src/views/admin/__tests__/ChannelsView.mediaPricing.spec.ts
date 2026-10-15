import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelsView from '../ChannelsView.vue'
import type { ChannelModelPricing } from '@/api/admin/channels'

const { list, update, showError } = vi.hoisted(() => ({ list: vi.fn(), update: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin', () => ({
  adminAPI: {
    channels: { list, update },
    groups: { getAll: vi.fn().mockResolvedValue([{ id: 2, platform: 'composite', name: 'Media', status: 'active' }]) },
    settings: { getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false }) },
  },
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

let wrapper: ReturnType<typeof mount> | undefined
afterEach(() => wrapper?.unmount())

async function editPricing(price: number | null, tiers: boolean) {
  const pricing: ChannelModelPricing = {
    platform: 'toapis', models: ['video-model'], billing_mode: 'video',
    input_price: null, output_price: null, cache_write_price: null, cache_read_price: null,
    image_input_price: null, image_output_price: null, per_request_price: price,
    intervals: tiers ? [{
      min_tokens: 0, max_tokens: null, tier_label: '720p', sort_order: 0,
      input_price: null, output_price: null, cache_write_price: null, cache_read_price: null,
      input_multiplier: null, output_multiplier: null, cache_write_multiplier: null, cache_read_multiplier: null,
      per_request_price: 0.7007,
    }] : [], time_pricing: null,
  }
  list.mockResolvedValue({ items: [{
    id: 1, name: 'Media channel', status: 'active', group_ids: [2], model_pricing: [pricing], model_mapping: {},
  }], total: 1 })
  wrapper = mount(ChannelsView, { global: { stubs: {
    AppLayout: defineComponent({ template: '<main><slot /></main>' }),
    TablePageLayout: defineComponent({ template: '<section><slot name="filters" /><slot name="table" /></section>' }),
    DataTable: defineComponent({ props: ['data'], template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>' }),
    BaseDialog: defineComponent({ props: ['show'], template: '<section v-if="show"><slot /><slot name="footer" /></section>' }),
    PricingEntryCard: true, Pagination: true, ConfirmDialog: true, EmptyState: true,
    Select: true, Icon: true, PlatformIcon: true, Toggle: true,
  } } })
  await flushPromises()
  await wrapper.findAll('button').find(button => button.text() === 'common.edit')!.trigger('click')
  await flushPromises()
  return { wrapper, pricing }
}

describe('channel media pricing submission', () => {
  beforeEach(() => { vi.clearAllMocks(); localStorage.clear(); update.mockResolvedValue({}) })

  it('preserves per-second resolution tiers and Composite binding when saved', async () => {
    const { wrapper, pricing } = await editPricing(null, true)
    await wrapper.get('#channel-form').trigger('submit')
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(update).toHaveBeenCalledOnce()
    expect(update.mock.calls[0][1]).toMatchObject({ group_ids: [2], model_pricing: [pricing] })
  })

  it('rejects a video price entry without a default price or resolution tier', async () => {
    const { wrapper } = await editPricing(null, false)
    await wrapper.get('#channel-form').trigger('submit')
    await flushPromises()

    expect(update).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.channels.form.perRequestPriceRequired')
  })
})
