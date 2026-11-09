import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AdminGroup } from '@/types'
import GroupsView from '../GroupsView.vue'
const { list, listRoutes, createRoute, updateRoute, preview } = vi.hoisted(() => ({ list: vi.fn(), listRoutes: vi.fn(), createRoute: vi.fn(), updateRoute: vi.fn(), preview: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { list, listCompositeRoutes: listRoutes, createCompositeRoute: createRoute, updateCompositeRoute: updateRoute, previewCompositeRoute: preview, getModelAllowlistCandidates: vi.fn().mockResolvedValue([]), getUsageSummary: vi.fn().mockResolvedValue([]), getCapacitySummary: vi.fn().mockResolvedValue([]), getLiveCapability: vi.fn().mockResolvedValue({ supported: false }) } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isSimpleMode: false }) }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ isCurrentStep: () => false }) }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
const sourceGroup: AdminGroup = {
  id: 42,
  name: 'Primary',
  description: null,
  platform: 'openai',
  rate_multiplier: 1,
  rpm_limit: 0,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  allow_image_generation: false,
  allow_batch_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  batch_image_discount_multiplier: 0.5,
  batch_image_hold_multiplier: 0.6,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  video_price_480p: null,
  video_price_720p: null,
  video_price_1080p: null,
  web_search_price_per_call: null,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  default_mapped_model: '',
  messages_dispatch_model_config: undefined,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: '2026-07-16T00:00:00Z',
  updated_at: '2026-07-16T00:00:00Z',
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  account_count: 1,
  active_account_count: 1,
  rate_limited_account_count: 0,
  model_allowlist: undefined,
  sort_order: 10
}

const AppLayoutStub = defineComponent({
  template: '<main><slot /></main>'
})

const TablePageLayoutStub = defineComponent({
  template: '<section><slot name="filters" /><slot name="table" /><slot name="pagination" /></section>'
})

const DataTableStub = defineComponent({
  props: {
    data: { type: Array, default: () => [] },
    columns: { type: Array, default: () => [] },
    loading: { type: Boolean, default: false }
  },
  template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>'
})

const BaseDialogStub = defineComponent({
  props: {
    show: { type: Boolean, default: false }
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

function mountView() {
  return mount(GroupsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        Select: defineComponent({ props: ['modelValue', 'options'], emits: ['update:modelValue'], template: `<select :value="modelValue" @change="$emit('update:modelValue', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>` }),
        PlatformIcon: true,
        Icon: true,
        RouterLink: { template: '<a><slot /></a>' },
        GroupCapacityBadge: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        VueDraggable: true
      }
    }
  })
}

async function openRoutes() {
  const wrapper = mountView()
  await flushPromises()
  await wrapper.get('[data-testid="group-composite-routes"]').trigger('click')
  await flushPromises()
  return wrapper
}


describe('Composite 媒体候选路由', () => {
 beforeEach(() => {
  vi.clearAllMocks()
  list.mockResolvedValue({ items: [{ ...sourceGroup, platform: 'composite' }], total: 1, page: 1, page_size: 20, pages: 1 })
  listRoutes.mockResolvedValue([])
  createRoute.mockResolvedValue({ id: 1 })
  updateRoute.mockResolvedValue({ id: 9 })
  preview.mockResolvedValue({ matched: true, source: 'route', group_id: 42, public_model: 'shared', endpoint: 'videos', target_platform: 'grsai', upstream_model: 'grs-video', candidates: [
   { matched: true, source: 'route', group_id: 42, public_model: 'shared', endpoint: 'videos', target_platform: 'grsai', upstream_model: 'grs-video', route: { id: 1, priority: 10, capability_model: '' } },
   { matched: true, source: 'route', group_id: 42, public_model: 'shared', endpoint: 'videos', target_platform: 'toapis', upstream_model: 'toa-video', route: { id: 2, priority: 20, capability_model: '' } },
  ] })
 })
 it('可创建原生视频候选并由 Provider 适配，预览显示全部候选顺序', async () => {
  const wrapper = mountView()
  await flushPromises()
  await wrapper.get('[data-testid="group-composite-routes"]').trigger('click')
  await flushPromises()
  await wrapper.get('[data-testid="composite-public-model"]').setValue('shared')
  await wrapper.get('[data-testid="composite-endpoint"]').setValue('videos')
  await wrapper.get('[data-testid="composite-platform"]').setValue('toapis')
  expect(wrapper.find('[data-testid="composite-capability-model"]').exists()).toBe(false)
  await wrapper.get('[data-testid="composite-route-form"]').trigger('submit')
  await flushPromises()
  expect(createRoute).toHaveBeenCalledWith(42, expect.objectContaining({ public_model: 'shared', endpoint: 'videos', target_platform: 'toapis', capability_model: '' }))
  await wrapper.get('[data-testid="composite-preview-model"]').setValue('shared')
  await wrapper.get('[data-testid="composite-preview-endpoint"]').setValue('videos')
  await wrapper.get('[data-testid="composite-preview-submit"]').trigger('click')
  await flushPromises()
  const candidates=wrapper.get('[data-testid="composite-media-candidates"]')
  expect(candidates.text()).toContain('grs-video')
  expect(candidates.text()).toContain('toa-video')
  expect(candidates.text().indexOf('grs-video')).toBeLessThan(candidates.text().indexOf('toa-video'))
  expect(candidates.text()).toContain('admin.groups.compositeRoutes.nativeProviderAdapterHint')
  expect(candidates.text()).not.toMatch(/schema:|grs\/shared|toa\/shared/)
  wrapper.unmount()
 })

 it.each(['grsai', 'toapis', 'kuaipao'])('编辑 %s 仅改优先级和备注时只读展示并保留旧候选能力', async platform => {
  listRoutes.mockResolvedValue([{
   id: 9, group_id: 42, public_model: 'gpt-image-2.6', match_type: 'exact', target_platform: platform,
   upstream_model: 'gpt-image-2.6-1k', capability_model: `${platform}/gpt-image-2.6-1k`,
   endpoint: 'images', priority: 100, enabled: true, notes: '',
  }])
  const wrapper = await openRoutes()
  try {
   await wrapper.get('table button[title="common.edit"]').trigger('click')
   expect(wrapper.find('[data-testid="composite-capability-model"]').exists()).toBe(false)
   expect(wrapper.get('[data-testid="composite-legacy-capability"]').text()).toBe(`${platform}/gpt-image-2.6-1k`)
   await wrapper.get('[data-testid="composite-route-form"] input[type="number"]').setValue(50)
   await wrapper.get('[data-testid="composite-route-form"] textarea').setValue('仅更新备注')
   await wrapper.get('[data-testid="composite-route-form"]').trigger('submit')
   await flushPromises()
   expect(updateRoute).toHaveBeenCalledWith(42, 9, expect.objectContaining({
    public_model: 'gpt-image-2.6', target_platform: platform, upstream_model: 'gpt-image-2.6-1k',
    endpoint: 'images', capability_model: `${platform}/gpt-image-2.6-1k`, priority: 50, notes: '仅更新备注',
   }))
  } finally { wrapper.unmount() }
 })

 it.each(['grsai', 'toapis', 'kuaipao'])('旧 %s 引用仅在明确切换 Provider 自动适配后才清空', async platform => {
  listRoutes.mockResolvedValue([{
   id: 9, group_id: 42, public_model: 'gpt-image-2.6', match_type: 'exact', target_platform: platform,
   upstream_model: 'gpt-image-2.6-1k', capability_model: `${platform}/legacy`,
   endpoint: 'images', priority: 100, enabled: true, notes: '',
  }])
  const wrapper = await openRoutes()
  try {
   await wrapper.get('table button[title="common.edit"]').trigger('click')
   expect(wrapper.get('[data-testid="composite-legacy-capability"]').text()).toBe(`${platform}/legacy`)
   await wrapper.get('[data-testid="composite-use-provider-adapter"]').trigger('click')
   expect(wrapper.find('[data-testid="composite-legacy-capability"]').exists()).toBe(false)
   expect(wrapper.get('[data-testid="composite-route-form"]').text()).toContain('admin.groups.compositeRoutes.nativeProviderAdapterHint')
   expect(updateRoute).not.toHaveBeenCalled()
   await wrapper.get('[data-testid="composite-route-form"]').trigger('submit')
   await flushPromises()
   expect(updateRoute).toHaveBeenCalledWith(42, 9, expect.objectContaining({ capability_model: '' }))
  } finally { wrapper.unmount() }
 })

 it('旧原生引用切换到另一平台后清空，不带入不相关能力配置', async () => {
  listRoutes.mockResolvedValue([{
   id: 9, group_id: 42, public_model: 'gpt-image-2.6', match_type: 'exact', target_platform: 'grsai',
   upstream_model: 'gpt-image-2.6-1k', capability_model: 'grsai/legacy', endpoint: 'images',
   priority: 100, enabled: true, notes: '',
  }])
  const wrapper = await openRoutes()
  try {
   await wrapper.get('table button[title="common.edit"]').trigger('click')
   expect(wrapper.get('[data-testid="composite-legacy-capability"]').text()).toBe('grsai/legacy')
   await wrapper.get('[data-testid="composite-platform"]').setValue('toapis')
   expect(wrapper.find('[data-testid="composite-legacy-capability"]').exists()).toBe(false)
   await wrapper.get('[data-testid="composite-route-form"]').trigger('submit')
   await flushPromises()
   expect(updateRoute).toHaveBeenCalledWith(42, 9, expect.objectContaining({ target_platform: 'toapis', capability_model: '' }))
  } finally { wrapper.unmount() }
 })

 it('旧原生候选预览明确展示兼容引用，不误报已切换为自动适配', async () => {
  preview.mockResolvedValue({ matched: true, source: 'route', group_id: 42, public_model: 'gpt-image-2.6', endpoint: 'images', target_platform: 'kuaipao', upstream_model: 'gpt-image-2.6-1k', candidates: [
   { target_platform: 'kuaipao', upstream_model: 'gpt-image-2.6-1k', public_model: 'gpt-image-2.6', route: { id: 9, priority: 100, capability_model: 'kuaipao/legacy' } },
  ] })
  const wrapper = await openRoutes()
  try {
   await wrapper.get('[data-testid="composite-preview-model"]').setValue('gpt-image-2.6')
   await wrapper.get('[data-testid="composite-preview-endpoint"]').setValue('images')
   await wrapper.get('[data-testid="composite-preview-submit"]').trigger('click')
   await flushPromises()
   const candidates = wrapper.get('[data-testid="composite-media-candidates"]')
   expect(candidates.text()).toContain('kuaipao/legacy')
   expect(candidates.text()).toContain('admin.groups.compositeRoutes.legacyCapability')
   expect(candidates.text()).not.toContain('admin.groups.compositeRoutes.nativeProviderAdapterHint')
  } finally { wrapper.unmount() }
 })

 it.each(['grsai', 'toapis', 'kuaipao'])('从 OpenAI 图片兼容切换到 %s 后不再提交旧候选能力', async platform => {
  const wrapper = await openRoutes()
  try {
   await wrapper.get('[data-testid="composite-public-model"]').setValue('gpt-image-2.6')
   await wrapper.get('[data-testid="composite-endpoint"]').setValue('images')
   await wrapper.get('[data-testid="composite-capability-model"]').setValue('openai/legacy-image')
   await wrapper.get('[data-testid="composite-platform"]').setValue(platform)
   expect(wrapper.find('[data-testid="composite-capability-model"]').exists()).toBe(false)
   await wrapper.get('[data-testid="composite-route-form"]').trigger('submit')
   await flushPromises()
   expect(createRoute).toHaveBeenCalledWith(42, expect.objectContaining({ target_platform: platform, capability_model: '' }))
  } finally { wrapper.unmount() }
 })

 it('切回 OpenAI 时显示已清空的兼容字段，仍允许显式保存配置', async () => {
  const wrapper = await openRoutes()
  try {
   await wrapper.get('[data-testid="composite-public-model"]').setValue('gpt-image-2.6')
   await wrapper.get('[data-testid="composite-endpoint"]').setValue('images')
   await wrapper.get('[data-testid="composite-capability-model"]').setValue('openai/old-image')
   await wrapper.get('[data-testid="composite-platform"]').setValue('kuaipao')
   await wrapper.get('[data-testid="composite-platform"]').setValue('openai')
   expect(wrapper.get<HTMLInputElement>('[data-testid="composite-capability-model"]').element.value).toBe('')
   await wrapper.get('[data-testid="composite-capability-model"]').setValue('openai/explicit-image')
   await wrapper.get('[data-testid="composite-route-form"]').trigger('submit')
   await flushPromises()
   expect(createRoute).toHaveBeenCalledWith(42, expect.objectContaining({ target_platform: 'openai', endpoint: 'images', capability_model: 'openai/explicit-image' }))
  } finally { wrapper.unmount() }
 })

 it.each([['openai', 'images'], ['grok', 'videos']])('编辑 %s/%s 保留已有兼容能力配置', async (platform, endpoint) => {
  listRoutes.mockResolvedValue([{
   id: 9, group_id: 42, public_model: 'public-model', match_type: 'exact', target_platform: platform,
   upstream_model: 'upstream-model', capability_model: 'legacy/explicit-capability',
   endpoint, priority: 100, enabled: true, notes: '',
  }])
  const wrapper = await openRoutes()
  try {
   await wrapper.get('table button[title="common.edit"]').trigger('click')
   expect(wrapper.get<HTMLInputElement>('[data-testid="composite-capability-model"]').element.value).toBe('legacy/explicit-capability')
   await wrapper.get('[data-testid="composite-route-form"]').trigger('submit')
   await flushPromises()
   expect(updateRoute).toHaveBeenCalledWith(42, 9, expect.objectContaining({ target_platform: platform, endpoint, capability_model: 'legacy/explicit-capability' }))
  } finally { wrapper.unmount() }
 })

 it('保留其它视频路由的显式能力配置，文字路由不携带媒体能力', async () => {
  const wrapper = await openRoutes()
  try {
   await wrapper.get('[data-testid="composite-public-model"]').setValue('grok-video')
   await wrapper.get('[data-testid="composite-endpoint"]').setValue('videos')
   await wrapper.get('[data-testid="composite-platform"]').setValue('grok')
   await wrapper.get('[data-testid="composite-capability-model"]').setValue('grok/video-explicit')
   await wrapper.get('[data-testid="composite-route-form"]').trigger('submit')
   await flushPromises()
   expect(createRoute).toHaveBeenLastCalledWith(42, expect.objectContaining({ target_platform: 'grok', endpoint: 'videos', capability_model: 'grok/video-explicit' }))
   await wrapper.get('[data-testid="composite-public-model"]').setValue('gpt-chat')
   await wrapper.get('[data-testid="composite-endpoint"]').setValue('responses')
   expect(wrapper.find('[data-testid="composite-capability-model"]').exists()).toBe(false)
   await wrapper.get('[data-testid="composite-route-form"]').trigger('submit')
   await flushPromises()
   expect(createRoute).toHaveBeenLastCalledWith(42, expect.objectContaining({ endpoint: 'responses', capability_model: '' }))
  } finally { wrapper.unmount() }
 })

 it('OpenAI 图片候选预览仍显示显式兼容能力配置', async () => {
  preview.mockResolvedValue({ matched: true, source: 'route', group_id: 42, public_model: 'gpt-image-2.6', endpoint: 'images', target_platform: 'openai', upstream_model: 'image-upstream', candidates: [
   { target_platform: 'openai', upstream_model: 'image-upstream', public_model: 'gpt-image-2.6', route: { id: 9, priority: 100, capability_model: 'openai/image-explicit' } },
  ] })
  const wrapper = await openRoutes()
  try {
   await wrapper.get('[data-testid="composite-preview-model"]').setValue('gpt-image-2.6')
   await wrapper.get('[data-testid="composite-preview-endpoint"]').setValue('images')
   await wrapper.get('[data-testid="composite-preview-submit"]').trigger('click')
   await flushPromises()
   const candidates = wrapper.get('[data-testid="composite-media-candidates"]')
   expect(candidates.text()).toContain('openai/image-explicit')
   expect(candidates.text()).not.toContain('admin.groups.compositeRoutes.nativeProviderAdapterHint')
  } finally { wrapper.unmount() }
 })
})
