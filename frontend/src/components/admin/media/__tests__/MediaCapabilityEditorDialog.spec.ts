import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import MediaCapabilityEditorDialog from '../MediaCapabilityEditorDialog.vue'

const { create, update, validate } = vi.hoisted(() => ({ create: vi.fn(), update: vi.fn(), validate: vi.fn() }))
vi.mock('@/api/admin/mediaCapabilities', () => ({ mediaCapabilitiesAPI: { create, update, validate } }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)

const schema = () => ({
  version: 1,
  fields: {
    prompt: { type: 'string', required: true },
    size: { type: 'string', enum: ['1:1'] },
    n: { type: 'integer', minimum: 1, maximum: 3, default: 1 },
    image_urls: { type: 'array', items: { type: 'string', format: 'http_url' }, max_items: 3 },
    metadata: { type: 'object', properties: { resolution: { type: 'string', enum: ['1K', '2K'] } } },
  },
  modes: [], rules: [], image_specs: [],
  batch: { native_max_results: 1, allow_split: true, semantically_linked: false },
})
const template = () => ({ id: 'base-image', version: 1, name: '基础图片模板', media_type: 'image' as const, schema: schema() })
const record = () => ({
  id: 7, public_model: 'my-image', media_type: 'image' as const, schema_version: 1, revision: 4,
  enabled: true, schema: schema(), template_id: 'base-image', template_version: 1,
  notes: '', created_at: '2026-10-15T00:00:00Z', updated_at: '2026-10-15T00:00:00Z',
})
const mountEditor = (capability: ReturnType<typeof record> | null = null) => mount(MediaCapabilityEditorDialog, {
  props: { show: true, capability, templates: [template()] },
  global: { stubs: { teleport: true } },
})

describe('媒体能力编辑', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    create.mockResolvedValue({ ...record(), revision: 1 })
    update.mockResolvedValue({ ...record(), revision: 5 })
    validate.mockResolvedValue({ valid: true, errors: [] })
  })

  // AC-018 (admin UI scope): copy and customize a template into an independent saved capability; runtime routing is separate.
  it('从模板复制后用结构化控件修改规则并保存独立配置', async () => {
    const wrapper = mountEditor()
    await wrapper.get('[data-testid="public-model"]').setValue('custom-image')
    await wrapper.get('[data-testid="template-select"]').setValue('base-image')
    await wrapper.get('[data-testid="apply-template"]').trigger('click')
    await wrapper.get('[data-testid="field-size-enum"]').setValue('1:1\n16:9')
    await wrapper.get('[data-testid="field-n-maximum"]').setValue('5')
    await wrapper.get('[data-testid="field-prompt-required"]').setValue(false)
    await wrapper.get('[data-testid="capability-enabled"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(create).toHaveBeenCalledWith(expect.objectContaining({
      public_model: 'custom-image', media_type: 'image', enabled: true,
      template_id: 'base-image', template_version: 1,
      schema: expect.objectContaining({ fields: expect.objectContaining({
        size: expect.objectContaining({ enum: ['1:1', '16:9'] }),
        n: expect.objectContaining({ maximum: 5 }),
        prompt: expect.objectContaining({ required: false }),
      }) }),
    }))
    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(template().schema.fields.size.enum).toEqual(['1:1'])
  })

  it('模型模板展示依据并复制模型与校验示例，草稿默认禁用', async () => {
    const wrapper = mountEditor()
    await wrapper.setProps({ templates: [{ ...template(), id: 'toapis-image', public_model: 'toapis/example-image', source: 'https://docs.toapis.com/model/example', evidence: '普通生成参数', notes: '仅支持普通生成', example: { model: 'example-image', prompt: '一只猫', size: '1:1' }, enabled: false }] })
    await wrapper.get('[data-testid="template-select"]').setValue('toapis-image')
    expect(wrapper.get('[data-testid="template-source"]').attributes('href')).toBe('https://docs.toapis.com/model/example')
    expect(wrapper.text()).toContain('普通生成参数')
    await wrapper.get('[data-testid="capability-enabled"]').setValue(true)
    await wrapper.get('[data-testid="apply-template"]').trigger('click')
    expect(wrapper.get('[data-testid="public-model"]').element).toHaveProperty('value', 'toapis/example-image')
    expect(wrapper.get('[data-testid="capability-enabled"]').element).toHaveProperty('checked', false)
    expect(wrapper.get('[data-testid="capability-notes"]').element).toHaveProperty('value', '仅支持普通生成')
    expect(JSON.parse((wrapper.get('[data-testid="request-json"]').element as HTMLTextAreaElement).value)).toEqual({ model: 'toapis/example-image', prompt: '一只猫', size: '1:1' })
  })

  it('编辑保留服务器修订号且不能通过界面新增协议顶层字段', async () => {
    const wrapper = mountEditor(record())
    expect(wrapper.find('[data-testid="field-model-supported"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="field-duration-supported"]').exists()).toBe(false)
    await wrapper.get('[data-testid="capability-notes"]').setValue('仅普通生成')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ revision: 4, notes: '仅普通生成' }))
    expect(create).not.toHaveBeenCalled()
  })

  it('无效范围在界面定位且阻止保存', async () => {
    const wrapper = mountEditor(record())
    await wrapper.get('[data-testid="field-n-minimum"]').setValue('6')
    await wrapper.get('[data-testid="field-n-maximum"]').setValue('3')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('n')
    expect(update).not.toHaveBeenCalled()
  })

  it('服务器字段错误保留草稿并定位 metadata.resolution', async () => {
    update.mockRejectedValue({ status: 400, message: '配置无效', metadata: { errors: [{ path: 'schema.fields.metadata.properties.resolution', code: 'invalid_enum', message: '枚举值重复' }] } })
    const wrapper = mountEditor(record())
    await wrapper.get('[data-testid="capability-notes"]').setValue('仍保留的修改')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('resolution')
    expect(wrapper.get('[data-testid="capability-notes"]').element).toHaveProperty('value', '仍保留的修改')
    expect(wrapper.emitted('saved')).toBeUndefined()
  })

  it('模式必填与精确图片规格可直接编辑且不增加 output_size', async () => {
    const wrapper = mountEditor(record())
    await wrapper.get('[data-testid="add-mode"]').trigger('click')
    await wrapper.get('[data-testid="mode-0-id"]').setValue('reference')
    await wrapper.get('[data-testid="mode-0-required"]').setValue('image_urls')
    await wrapper.get('[data-testid="add-image-spec"]').trigger('click')
    await wrapper.get('[data-testid="spec-0-ratio"]').setValue('1:1')
    await wrapper.get('[data-testid="spec-0-resolution"]').setValue('1K')
    await wrapper.get('[data-testid="spec-0-width"]').setValue('1024')
    await wrapper.get('[data-testid="spec-0-height"]').setValue('1024')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ schema: expect.objectContaining({
      modes: [expect.objectContaining({ id: 'reference', required: ['image_urls'] })],
      image_specs: [{ ratio: '1:1', resolution: '1K', width: 1024, height: 1024 }],
    }) }))
    expect(JSON.stringify(update.mock.calls[0])).not.toContain('output_size')
  })

  it('校验示例把原样 request_json 交给服务端且不创建配置', async () => {
    const wrapper = mountEditor(record())
    const raw = '{"model":"my-image","model":"duplicate"}  false'
    await wrapper.get('[data-testid="request-json"]').setValue(raw)
    await wrapper.get('[data-testid="validate-request"]').trigger('click')
    await flushPromises()
    expect(validate).toHaveBeenCalledWith(expect.objectContaining({ request_json: raw }))
    expect(create).not.toHaveBeenCalled()
    expect(update).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="validation-result"]').text()).toContain('通过')
  })

  it('修正无效默认值后可保存，不能继续把旧错误留在草稿', async () => {
    const wrapper = mountEditor(record())
    const field = wrapper.get('[data-testid="field-n-default"]')
    await field.setValue('invalid-json')
    await field.trigger('change')
    await wrapper.get('form').trigger('submit')
    expect(update).not.toHaveBeenCalled()
    await field.setValue('2')
    await field.trigger('change')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ schema: expect.objectContaining({ fields: expect.objectContaining({ n: expect.objectContaining({ default: 2 }) }) }) }))
  })

  it('重复点击保存不产生第二次写入，409 明确提示重新加载', async () => {
    let reject!: (error: unknown) => void
    update.mockReturnValue(new Promise((_resolve, rejectPromise) => { reject = rejectPromise }))
    const wrapper = mountEditor(record())
    await wrapper.get('form').trigger('submit')
    await wrapper.get('form').trigger('submit')
    expect(update).toHaveBeenCalledTimes(1)
    reject({ status: 409, message: '修订冲突' })
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('重新加载')
    expect(wrapper.emitted('saved')).toBeUndefined()
  })
})
