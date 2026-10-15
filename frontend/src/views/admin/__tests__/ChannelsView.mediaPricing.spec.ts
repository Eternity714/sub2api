import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelsView from '../ChannelsView.vue'
import type { Channel, ChannelModelPricing } from '@/api/admin/channels'

const { list, update, getGroups, showError } = vi.hoisted(() => ({ list: vi.fn(), update: vi.fn(), getGroups: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin', () => ({
  adminAPI: {
    channels: { list, update },
    groups: { getAll: getGroups },
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

async function editPricing(price: number | null, tiers: boolean, overrides: Partial<Channel> = {}) {
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
    ...overrides,
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

function checkedPlatforms(view: ReturnType<typeof mount>) {
  return view.findAll('label')
    .filter(label => /^admin\.groups\.platforms\.\w+$/.test(label.text()))
    .filter(label => (label.get('input[type="checkbox"]').element as HTMLInputElement).checked)
    .map(label => label.text())
}

async function saveAndReopen(view: ReturnType<typeof mount>) {
  update.mockImplementationOnce(async (id, request) => {
    list.mockResolvedValue({ items: [{ id, ...request }], total: 1 })
    return { id, ...request }
  })
  await view.get('#channel-form').trigger('submit')
  await flushPromises()
  await view.findAll('button').find(button => button.text() === 'common.edit')!.trigger('click')
  await flushPromises()
}

describe('channel media pricing submission', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    update.mockResolvedValue({})
    getGroups.mockResolvedValue([{ id: 2, platform: 'composite', name: 'Media', status: 'active' }])
  })

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

  it('restores only configured media platforms for a Composite group', async () => {
    const { wrapper } = await editPricing(null, true, { model_mapping: { grsai: {}, toapis: {} } })

    expect(checkedPlatforms(wrapper)).toEqual(['admin.groups.platforms.grsai', 'admin.groups.platforms.toapis'])
  })

  it('persists an enabled platform with no pricing or mappings so it survives reopening', async () => {
    const { wrapper } = await editPricing(null, true, { model_mapping: { grsai: {}, toapis: {} } })
    update.mockImplementationOnce(async (id, request) => {
      list.mockResolvedValue({ items: [{ id, ...request }], total: 1 })
      return { id, ...request }
    })
    await wrapper.get('#channel-form').trigger('submit')
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(update).toHaveBeenCalledOnce()
    expect(update.mock.calls[0][1].model_mapping).toEqual({ grsai: {}, toapis: {} })

    await wrapper.findAll('button').find(button => button.text() === 'common.edit')!.trigger('click')
    await flushPromises()
    expect(checkedPlatforms(wrapper)).toEqual(['admin.groups.platforms.grsai', 'admin.groups.platforms.toapis'])
  })

  it('sends an empty description when an existing description is cleared', async () => {
    const { wrapper } = await editPricing(null, true, { description: 'Previous description' })
    await wrapper.get('textarea').setValue('   ')
    await wrapper.get('#channel-form').trigger('submit')
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(update).toHaveBeenCalledOnce()
    expect(update.mock.calls[0][1].description).toBe('')
  })

  it('keeps a Composite-only binding without inventing platform sections', async () => {
    const { wrapper } = await editPricing(null, false, { model_pricing: [], model_mapping: {} })
    expect(checkedPlatforms(wrapper)).toEqual([])
    await wrapper.get('#channel-form').trigger('submit')
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(update.mock.calls[0][1]).toMatchObject({ group_ids: [2], model_pricing: [], model_mapping: {} })
  })

  it('allows disabling the last configured platform to remove its Composite binding', async () => {
    const { wrapper } = await editPricing(null, true, { model_mapping: { toapis: {} } })
    const toapis = wrapper.findAll('label').find(label => label.text() === 'admin.groups.platforms.toapis')!
    await toapis.get('input[type="checkbox"]').setValue(false)
    await wrapper.get('#channel-form').trigger('submit')
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(update.mock.calls[0][1]).toMatchObject({ group_ids: [], model_pricing: [], model_mapping: {} })
  })

  it('still infers a concrete platform from its bound group', async () => {
    getGroups.mockResolvedValue([{ id: 2, platform: 'openai', name: 'OpenAI', status: 'active' }])
    const { wrapper } = await editPricing(null, false, { model_pricing: [], model_mapping: {} })
    expect(checkedPlatforms(wrapper)).toEqual(['admin.groups.platforms.openai'])
    await wrapper.get('#channel-form').trigger('submit')
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(update.mock.calls[0][1]).toMatchObject({ group_ids: [2], model_mapping: { openai: {} } })
  })

  it('preserves Composite account-statistics pricing without other platform configuration after reopening', async () => {
    const pricing: ChannelModelPricing = {
      platform: 'toapis', models: ['stats-model'], billing_mode: 'per_request',
      input_price: null, output_price: null, cache_write_price: null, cache_read_price: null,
      image_input_price: null, image_output_price: null, per_request_price: 0.77,
      intervals: [], time_pricing: null,
    }
    const rule = { name: 'Statistics', group_ids: [2], account_ids: [], pricing: [pricing] }
    const { wrapper } = await editPricing(null, false, { model_pricing: [], model_mapping: {}, account_stats_pricing_rules: [rule] })

    expect(checkedPlatforms(wrapper)).toEqual(['admin.groups.platforms.toapis'])
    await saveAndReopen(wrapper)
    expect(checkedPlatforms(wrapper)).toEqual(['admin.groups.platforms.toapis'])
    await wrapper.get('#channel-form').trigger('submit')
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(update).toHaveBeenCalledTimes(2)
    for (const [, request] of update.mock.calls) {
      expect(request.account_stats_pricing_rules).toMatchObject([rule])
    }
  })

  it('preserves enabled Composite features and the global Bedrock boolean after reopening', async () => {
    const features = {
      web_search_emulation: { anthropic: true },
      codex_image_generation_bridge: { openai: true },
      bedrock_cc_compat: true,
      unmanaged_flag: 'preserved',
    }
    const { wrapper } = await editPricing(null, false, { model_pricing: [], model_mapping: {}, features_config: features })

    expect(checkedPlatforms(wrapper)).toEqual(['admin.groups.platforms.anthropic', 'admin.groups.platforms.openai'])
    await saveAndReopen(wrapper)
    expect(checkedPlatforms(wrapper)).toEqual(['admin.groups.platforms.anthropic', 'admin.groups.platforms.openai'])
    await wrapper.get('#channel-form').trigger('submit')
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(update).toHaveBeenCalledTimes(2)
    for (const [, request] of update.mock.calls) {
      expect(request.features_config).toEqual(features)
    }
  })

  it('preserves the global Bedrock switch for an OpenAI group without adding an Anthropic section', async () => {
    getGroups.mockResolvedValue([{ id: 2, platform: 'openai', name: 'OpenAI', status: 'active' }])
    const { wrapper } = await editPricing(null, false, {
      model_pricing: [], model_mapping: {}, features_config: { bedrock_cc_compat: true },
    })

    expect(checkedPlatforms(wrapper)).toEqual(['admin.groups.platforms.openai'])
    await saveAndReopen(wrapper)
    expect(checkedPlatforms(wrapper)).toEqual(['admin.groups.platforms.openai'])
    expect(showError).not.toHaveBeenCalled()
    expect(update.mock.calls[0][1]).toMatchObject({ group_ids: [2], features_config: { bedrock_cc_compat: true } })
  })

  it.each([true, { anthropic: true }])('keeps a Composite binding and opaque global Bedrock value %j without inferring a platform', async value => {
    const { wrapper } = await editPricing(null, false, {
      model_pricing: [], model_mapping: {}, features_config: { bedrock_cc_compat: value },
    })

    expect(checkedPlatforms(wrapper)).toEqual([])
    await saveAndReopen(wrapper)
    expect(checkedPlatforms(wrapper)).toEqual([])
    expect(showError).not.toHaveBeenCalled()
    expect(update.mock.calls[0][1]).toMatchObject({ group_ids: [2], features_config: { bedrock_cc_compat: value } })
  })
})
