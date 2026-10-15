import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import MediaCapabilitiesView from '../MediaCapabilitiesView.vue'

const { list, templates, remove } = vi.hoisted(() => ({ list: vi.fn(), templates: vi.fn(), remove: vi.fn() }))
vi.mock('@/api/admin/mediaCapabilities', () => ({ mediaCapabilitiesAPI: { list, templates, delete: remove } }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn() }) }))
const makeRecord = (id: number, type: 'image' | 'video') => ({ id, public_model: 'same-model', media_type: type, schema_version: 1, revision: 3, enabled: true, schema: { version: 1, fields: {} }, created_at: '', updated_at: '' })
const mountView = () => mount(MediaCapabilitiesView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, teleport: true } } })

describe('媒体能力管理列表', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    list.mockResolvedValue({ items: [makeRecord(1, 'image'), makeRecord(2, 'video')], total: 2, page: 1, page_size: 20 })
    templates.mockResolvedValue([])
  })
  it('同名图片/视频能力独立显示并按媒体类型过滤', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('same-model')
    expect(wrapper.text()).toContain('图片')
    expect(wrapper.text()).toContain('视频')
    await wrapper.get('[data-testid="media-filter"]').setValue('video')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ media_type: 'video', page: 1 }))
  })
  it('服务端拒绝访问时展示错误并不展示空列表成功状态', async () => {
    list.mockRejectedValue({ status: 403, message: '需要管理员权限' })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('需要管理员权限')
    expect(wrapper.find('[data-testid="capabilities-empty"]').exists()).toBe(false)
  })
})
