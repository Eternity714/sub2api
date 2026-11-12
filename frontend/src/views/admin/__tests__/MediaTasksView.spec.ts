import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import MediaTasksView from '../MediaTasksView.vue'
const { list } = vi.hoisted(() => ({ list: vi.fn() }))
vi.mock('@/api/admin/mediaTasks', () => ({ mediaTasksAPI: { list } }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
const task = {
 id: 'media_task_test', model: 'public-video', media_type: 'video', user_id: 7, api_key_id: 9, group_id: 11,
 status: 'in_progress', progress: 50, created_at: 1792022400, held_amount: 0.5, charged_amount: 0, error: '', result_count: 0,
 units: [{ index: 0, state: 'persisting', account_id: 19, platform: 'toapis', upstream_model: 'provider-video', upstream_task_id: 'remote-123', attempt: 2, count: 1, stored_count: 0, error: '等待转存重试' }],
}
const mountView = () => mount(MediaTasksView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, teleport: true } } })
describe('管理员媒体任务诊断', () => {
 beforeEach(() => { vi.resetAllMocks(); list.mockResolvedValue({ items: [task], total: 1, page: 1, page_size: 20 }) })
 it('展示冻结与扣费，展开上游执行单元并允许只读状态筛选', async () => {
  const wrapper=mountView()
  await flushPromises()
  expect(wrapper.text()).toContain('media_task_test')
  expect(wrapper.text()).toContain('$0.500000')
  expect(wrapper.text()).toContain('$0.000000')
  await wrapper.get('[data-testid="task-units-media_task_test"]').trigger('click')
  expect(wrapper.text()).toContain('remote-123')
  expect(wrapper.text()).toContain('provider-video')
  expect(wrapper.text()).toContain('等待转存重试')
  expect(wrapper.text()).not.toContain('重新生成')
  await wrapper.get('[data-testid="task-status-filter"]').setValue('completed')
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith({ page: 1, page_size: 20, status: 'completed' })
 })
 it('查询失败展示错误并保留刷新入口', async () => {
  list.mockRejectedValue({ status: 403, message: '需要管理员权限' })
  const wrapper=mountView()
  await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toContain('需要管理员权限')
  list.mockResolvedValue({ items: [task], total: 1, page: 1, page_size: 20 })
  await wrapper.get('[data-testid="tasks-refresh"]').trigger('click')
  await flushPromises()
  expect(wrapper.text()).toContain('media_task_test')
  expect(wrapper.find('[role="alert"]').exists()).toBe(false)
 })
})
