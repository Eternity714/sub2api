import { describe, expect, it } from 'vitest'
import type { MediaCapabilitySchema } from '@/types/mediaCapability'
import { findSchemaReferenceErrors, renameSchemaReferences } from '../mediaSchemaEditing'

const emptySchema = (): MediaCapabilitySchema => ({ version: 1, fields: {} })

describe('媒体参数编辑引用', () => {
  it('重命名只匹配完整路径及子树，不误改相似前缀', () => {
    const schema = emptySchema()
    schema.modes = [{ id: 'input', when: [{ path: 'metadata.old.enabled', op: 'present' }], required: ['metadata.old', 'metadata.old.child', 'metadata.older'], forbidden: ['metadata.old[].id'] }]
    schema.rules = [{ kind: 'requires', paths: ['metadata.old', 'metadata.older'], when: [{ path: 'metadata.old', op: 'not_empty' }] }]
    renameSchemaReferences(schema, 'metadata.old', 'metadata.renamed')
    expect(schema.modes[0]).toEqual({ id: 'input', when: [{ path: 'metadata.renamed.enabled', op: 'present' }], required: ['metadata.renamed', 'metadata.renamed.child', 'metadata.older'], forbidden: ['metadata.renamed[].id'] })
    expect(schema.rules[0]).toEqual({ kind: 'requires', paths: ['metadata.renamed', 'metadata.older'], when: [{ path: 'metadata.renamed', op: 'not_empty' }] })
  })

  it('未完整填写的条件保留草稿并返回校验错误，改名不抛出异常', () => {
    const schema = emptySchema()
    schema.modes = [{ id: 'draft', required: [], forbidden: [], when: [{}, null] as unknown as NonNullable<MediaCapabilitySchema['modes']>[number]['when'] }]
    expect(() => renameSchemaReferences(schema, 'metadata.old', 'metadata.renamed')).not.toThrow()
    expect(findSchemaReferenceErrors(schema)).toEqual([expect.objectContaining({ path: 'modes[0].when[0].path', code: 'unknown_path' }), expect.objectContaining({ path: 'modes[0].when[1].path', code: 'unknown_path' })])
  })

  it('同步祖先对象默认值、枚举及 equals 比较对象，并保持幂等', () => {
    const schema = emptySchema()
    schema.fields.metadata = { type: 'object', default: { options: { old: 'high' } }, enum: [{ options: { old: 'low' } }], properties: { options: { type: 'object', default: { renamed: 'high' }, properties: { renamed: { type: 'string' } } } } }
    schema.modes = [{ id: 'mode', required: [], forbidden: [], when: [{ path: 'metadata', op: 'equals', value: { options: { old: 'low' } } }] }]
    schema.rules = [{ kind: 'at_least_one', paths: ['metadata'], when: [{ path: 'metadata.options', op: 'equals', value: { old: 'high' } }] }]
    renameSchemaReferences(schema, 'metadata.options.old', 'metadata.options.renamed')
    renameSchemaReferences(schema, 'metadata.options.old', 'metadata.options.renamed')
    expect(schema.fields.metadata.default).toEqual({ options: { renamed: 'high' } })
    expect(schema.fields.metadata.enum).toEqual([{ options: { renamed: 'low' } }])
    expect(schema.fields.metadata.properties?.options?.default).toEqual({ renamed: 'high' })
    expect(schema.modes[0]?.when[0]?.value).toEqual({ options: { renamed: 'low' } })
    expect(schema.rules[0]?.when?.[0]?.value).toEqual({ renamed: 'high' })
  })

  it('数组 items 内重命名同步各层 default、enum 和祖先 equals 数组', () => {
    const schema = emptySchema()
    schema.fields.metadata = { type: 'object', default: { refs: [{ old: 'a' }, { old: 'b' }] }, properties: { refs: { type: 'array', default: [{ old: 'c' }], enum: [[{ old: 'd' }]], items: { type: 'object', default: { old: 'e' }, enum: [{ old: 'f' }], properties: { renamed: { type: 'string' } } } } } }
    schema.rules = [{ kind: 'at_least_one', paths: ['metadata.refs'], when: [{ path: 'metadata.refs', op: 'equals', value: [{ old: 'g' }] }] }]
    renameSchemaReferences(schema, 'metadata.refs[].old', 'metadata.refs[].renamed')
    expect(schema.fields.metadata.default).toEqual({ refs: [{ renamed: 'a' }, { renamed: 'b' }] })
    const refs = schema.fields.metadata.properties?.refs
    expect(refs?.default).toEqual([{ renamed: 'c' }])
    expect(refs?.enum).toEqual([[{ renamed: 'd' }]])
    expect(refs?.items?.default).toEqual({ renamed: 'e' })
    expect(refs?.items?.enum).toEqual([{ renamed: 'f' }])
    expect(schema.rules[0]?.when?.[0]?.value).toEqual([{ renamed: 'g' }])
  })

  it('拒绝危险字段名，不覆盖已有目标数据', () => {
    const schema = emptySchema()
    schema.fields.metadata = { type: 'object', default: { old: 1, renamed: 2 }, properties: {} }
    schema.rules = [{ kind: 'at_least_one', paths: ['metadata.old'] }]
    renameSchemaReferences(schema, 'metadata.old', 'metadata.__proto__')
    expect(schema.rules[0]?.paths).toEqual(['metadata.old'])
    renameSchemaReferences(schema, 'metadata.old', 'metadata.renamed')
    expect(schema.fields.metadata.default).toEqual({ old: 1, renamed: 2 })
    expect(Object.prototype).not.toHaveProperty('old')
  })

  it('删字段后报告模式与规则的悬空引用，保留草稿规则供修正', () => {
    const schema = emptySchema()
    schema.fields.metadata = { type: 'object', properties: { refs: { type: 'array', items: { type: 'string' } } } }
    schema.modes = [{ id: 'input', required: ['model', 'metadata.gone'], forbidden: ['metadata.refs.child'], when: [{ path: 'metadata.refs[]', op: 'present' }] }]
    schema.rules = [{ kind: 'requires', paths: ['metadata.refs', 'metadata.gone'], when: [{ path: 'metadata.absent', op: 'present' }] }]
    const errors = findSchemaReferenceErrors(schema)
    expect(errors.map(error => error.path)).toEqual(['modes[0].required[1]', 'modes[0].forbidden[0]', 'modes[0].when[0].path', 'rules[0].paths[1]', 'rules[0].when[0].path'])
    expect(errors[0]?.message).toContain('字段 metadata.gone 不存在，请先调整模式/规则')
    expect(schema.rules[0]?.paths).toEqual(['metadata.refs', 'metadata.gone'])
  })

  it('像素规格映射要求 size 与字符串 resolution，修复后可通过', () => {
    const schema = emptySchema()
    schema.image_specs = [{ resolution: '1K', width: 1024, height: 1024 } as NonNullable<MediaCapabilitySchema['image_specs']>[number]]
    expect(findSchemaReferenceErrors(schema)).toEqual(expect.arrayContaining([expect.objectContaining({ path: 'image_specs', message: expect.stringContaining('像素规格映射需要') })]))
    schema.fields.size = { type: 'string' }
    schema.fields.metadata = { type: 'object', properties: { resolution: { type: 'integer' } } }
    expect(findSchemaReferenceErrors(schema)).toHaveLength(1)
    schema.fields.metadata.properties!.resolution = { type: 'string' }
    expect(findSchemaReferenceErrors(schema)).toEqual([])
  })
})
