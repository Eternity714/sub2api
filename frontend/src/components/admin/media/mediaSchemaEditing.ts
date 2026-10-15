import type {
  MediaCapabilityField,
  MediaCapabilitySchema,
  MediaCondition,
  MediaValidationError,
} from '@/types/mediaCapability'

const unsafeNames = new Set(['__proto__', 'prototype', 'constructor'])
const own = (value: object, key: string) => Object.prototype.hasOwnProperty.call(value, key)
const objectValue = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value)

// [] identifies a schema's array item; it is never accepted as a mode/rule request path.
function pathParts(path: string): string[] | null {
  if (typeof path !== 'string' || !/^[A-Za-z_][A-Za-z0-9_]*(?:\[\])*(?:\.[A-Za-z_][A-Za-z0-9_]*(?:\[\])*)*$/.test(path)) return null
  const parts = path.replace(/\[\]/g, '.[]').split('.')
  return parts.some(part => unsafeNames.has(part)) ? null : parts
}

function startsWithParts(parts: string[], prefix: string[]): boolean {
  return prefix.length < parts.length && prefix.every((part, index) => part === parts[index])
}

function renameValueKey(value: unknown, parts: string[], nextName: string): void {
  const [head, ...rest] = parts
  if (!head) return
  if (head === '[]') {
    if (Array.isArray(value)) for (const item of value) renameValueKey(item, rest, nextName)
    return
  }
  if (!objectValue(value) || !own(value, head)) return
  if (rest.length) {
    renameValueKey(value[head], rest, nextName)
  } else if (!own(value, nextName)) {
    // A direct parent's value may already have been migrated by the row editor.
    // Preserve both values on a collision so validation can reveal it without data loss.
    value[nextName] = value[head]
    delete value[head]
  }
}

/** Update references after the row editor has renamed a field within the same parent. */
export function renameSchemaReferences(schema: MediaCapabilitySchema, from: string, to: string): void {
  const sourceParts = pathParts(from)
  const targetParts = pathParts(to)
  if (!sourceParts || !targetParts || from === to || sourceParts.length !== targetParts.length) return
  if (!sourceParts.slice(0, -1).every((part, index) => part === targetParts[index])) return
  const nextName = targetParts[targetParts.length - 1]!
  if (nextName === '[]' || sourceParts[sourceParts.length - 1] === '[]') return
  const renamedPath = (path: string) => typeof path === 'string' && (path === from || path.startsWith(`${from}.`) || path.startsWith(`${from}[]`))
    ? to + path.slice(from.length)
    : path
  function migrateAncestorValue(value: unknown, ancestorPath: string) {
    const ancestorParts = pathParts(ancestorPath)
    if (ancestorParts && startsWithParts(sourceParts!, ancestorParts)) {
      renameValueKey(value, sourceParts!.slice(ancestorParts.length), nextName)
    }
  }
  function visitField(field: MediaCapabilityField, path: string) {
    if (own(field, 'default')) migrateAncestorValue(field.default, path)
    for (const value of field.enum || []) migrateAncestorValue(value, path)
    for (const [name, child] of Object.entries(field.properties || {})) visitField(child, `${path}.${name}`)
    if (field.items) visitField(field.items, `${path}[]`)
  }
  for (const [name, field] of Object.entries(schema.fields)) visitField(field, name)
  function updateConditions(conditions: MediaCondition[]) {
    for (const condition of conditions) {
      if (!condition || typeof condition !== 'object') continue
      if (condition.op === 'equals') migrateAncestorValue(condition.value, condition.path)
      condition.path = renamedPath(condition.path)
    }
  }
  for (const mode of schema.modes || []) {
    mode.required = (mode.required || []).map(renamedPath)
    mode.forbidden = (mode.forbidden || []).map(renamedPath)
    updateConditions(mode.when || [])
  }
  for (const rule of schema.rules || []) {
    rule.paths = rule.paths.map(renamedPath)
    updateConditions(rule.when || [])
  }
}

function fieldAt(schema: MediaCapabilitySchema, path: string): MediaCapabilityField | undefined {
  if (path === 'model') return { type: 'string', required: true }
  const parts = pathParts(path)
  if (!parts || parts.includes('[]')) return undefined
  let fields = schema.fields
  let field: MediaCapabilityField | undefined
  for (const [index, name] of parts.entries()) {
    if (!own(fields, name)) return undefined
    field = fields[name]
    if (!field) return undefined
    if (index < parts.length - 1) {
      if (field.type !== 'object') return undefined
      fields = field.properties || {}
    }
  }
  return field
}

/** Check dependencies only; the server remains the authority for the full schema contract. */
export function findSchemaReferenceErrors(schema: MediaCapabilitySchema): MediaValidationError[] {
  const errors: MediaValidationError[] = []
  function check(path: string, reference: string) {
    if (!fieldAt(schema, reference)) errors.push({ path, code: 'unknown_path', message: `字段 ${reference} 不存在，请先调整模式/规则。` })
  }
  function conditions(values: MediaCondition[], prefix: string) {
    values.forEach((condition, index) => check(`${prefix}[${index}].path`, condition?.path))
  }
  const modes = schema.modes || []
  modes.forEach((mode, index) => {
    const required = mode.required || []
    const forbidden = mode.forbidden || []
    required.forEach((reference, i) => check(`modes[${index}].required[${i}]`, reference))
    forbidden.forEach((reference, i) => check(`modes[${index}].forbidden[${i}]`, reference))
    conditions(mode.when || [], `modes[${index}].when`)
  })
  const rules = schema.rules || []
  rules.forEach((rule, index) => {
    rule.paths.forEach((reference, i) => check(`rules[${index}].paths[${i}]`, reference))
    conditions(rule.when || [], `rules[${index}].when`)
  })
  if (schema.image_specs?.length) {
    if (!fieldAt(schema, 'size')) errors.push({ path: 'image_specs', code: 'unknown_path', message: '像素规格映射需要 size 字段，请先补回字段或调整映射。' })
    if (fieldAt(schema, 'metadata.resolution')?.type !== 'string') errors.push({ path: 'image_specs', code: 'unknown_path', message: '像素规格映射需要字符串类型的 metadata.resolution 字段，请先补回字段或调整映射。' })
  }
  return errors
}


