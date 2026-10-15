import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AdminGroup } from '@/types'
import GroupsView from '../GroupsView.vue'
const { list, listRoutes, createRoute, preview } = vi.hoisted(() => ({ list: vi.fn(), listRoutes: vi.fn(), createRoute: vi.fn(), preview: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { list, listCompositeRoutes: listRoutes, createCompositeRoute: createRoute, previewCompositeRoute: preview, getModelAllowlistCandidates: vi.fn().mockResolvedValue([]), getUsageSummary: vi.fn().mockResolvedValue([]), getCapacitySummary: vi.fn().mockResolvedValue([]), getLiveCapability: vi.fn().mockResolvedValue({ supported: false }) } } }))
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


describe('Composite 媒体候选路由', () => {
 beforeEach(() => {
  vi.clearAllMocks()
  list.mockResolvedValue({ items: [{ ...sourceGroup, platform: 'composite' }], total: 1, page: 1, page_size: 20, pages: 1 })
  listRoutes.mockResolvedValue([])
  createRoute.mockResolvedValue({ id: 1 })
  preview.mockResolvedValue({ matched: true, source: 'route', group_id: 42, public_model: 'shared', endpoint: 'videos', target_platform: 'grsai', upstream_model: 'grs-video', candidates: [
   { matched: true, source: 'route', group_id: 42, public_model: 'shared', endpoint: 'videos', target_platform: 'grsai', upstream_model: 'grs-video', route: { id: 1, priority: 10, capability_model: 'grs/shared' } },
   { matched: true, source: 'route', group_id: 42, public_model: 'shared', endpoint: 'videos', target_platform: 'toapis', upstream_model: 'toa-video', route: { id: 2, priority: 20, capability_model: 'toa/shared' } },
  ] })
 })
 it('可创建视频候选并保留独立能力记录，预览显示全部候选顺序', async () => {
  const wrapper = mountView()
  await flushPromises()
  await wrapper.get('[data-testid="group-composite-routes"]').trigger('click')
  await flushPromises()
  await wrapper.get('[data-testid="composite-public-model"]').setValue('shared')
  await wrapper.get('[data-testid="composite-endpoint"]').setValue('videos')
  await wrapper.get('[data-testid="composite-platform"]').setValue('toapis')
  await wrapper.get('[data-testid="composite-capability-model"]').setValue('toa/shared')
  await wrapper.get('[data-testid="composite-route-form"]').trigger('submit')
  await flushPromises()
  expect(createRoute).toHaveBeenCalledWith(42, expect.objectContaining({ public_model: 'shared', endpoint: 'videos', target_platform: 'toapis', capability_model: 'toa/shared' }))
  await wrapper.get('[data-testid="composite-preview-model"]').setValue('shared')
  await wrapper.get('[data-testid="composite-preview-endpoint"]').setValue('videos')
  await wrapper.get('[data-testid="composite-preview-submit"]').trigger('click')
  await flushPromises()
  const candidates=wrapper.get('[data-testid="composite-media-candidates"]')
  expect(candidates.text()).toContain('grs-video')
  expect(candidates.text()).toContain('toa-video')
  expect(candidates.text().indexOf('grs-video')).toBeLessThan(candidates.text().indexOf('toa-video'))
  wrapper.unmount()
 })
})
