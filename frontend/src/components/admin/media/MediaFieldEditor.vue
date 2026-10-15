<template>
  <tr v-if="field" v-show="visible" class="border-b border-gray-100 align-top dark:border-dark-700" :data-testid="`field-${path}-row`">
    <td class="py-2 pl-3 pr-2">
      <div class="flex min-h-8 items-center gap-1" :style="{ paddingLeft: `${depth * 18}px` }">
        <button v-if="nested" type="button" class="flex h-7 w-5 shrink-0 items-center justify-center rounded text-gray-500 hover:bg-gray-100 dark:hover:bg-dark-700" :aria-label="expanded ? label(`收起 ${path}`, `Collapse ${path}`) : label(`展开 ${path}`, `Expand ${path}`)" :aria-expanded="expanded" :data-testid="`field-${path}-expand`" @click="expanded = !expanded">
          <svg class="h-3 w-3 transition-transform" :class="expanded ? 'rotate-90' : ''" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="m6 3 5 5-5 5z" /></svg>
        </button>
        <span v-else class="w-5 shrink-0" />
        <input v-if="renameable" v-model="nameText" class="input h-8 min-w-24 px-2 py-1 font-mono text-xs" :aria-label="label(`${path} 参数名`, `${path} parameter name`)" :data-testid="`field-${path}-name`" @change="commitName" @keydown.enter.prevent="commitName" />
        <span v-else class="font-mono text-xs" :title="path">{{ displayName }}</span>
      </div>
    </td>
    <td class="px-2 py-2">
      <select v-if="availableTypes.length > 1" class="input h-8 w-28 px-2 py-1 text-xs" :value="field.type" :aria-label="label(`${path} 类型`, `${path} type`)" :data-testid="`field-${path}-type`" @change="changeType(($event.target as HTMLSelectElement).value as MediaFieldType)">
        <option v-for="type in availableTypes" :key="type" :value="type">{{ type }}</option>
      </select>
      <span v-else class="inline-flex min-h-8 items-center font-mono text-xs text-gray-500">{{ field.type }}</span>
    </td>
    <td class="px-2 py-2 text-center">
      <input v-if="!arrayItem" type="checkbox" class="mt-2" :checked="!!field.required" :aria-label="label(`${path} 必填`, `${path} required`)" :data-testid="`field-${path}-required`" @change="patch({ required: ($event.target as HTMLInputElement).checked })" />
      <span v-else class="inline-flex min-h-8 items-center text-gray-400">—</span>
    </td>
    <td class="px-2 py-2">
      <textarea v-if="primitive" v-model="draft.enumText" class="input min-h-8 w-full min-w-28 resize-y px-2 py-1.5 font-mono text-xs" rows="1" :aria-label="label(`${path} 枚举值，每行一项`, `${path} enum values, one per line`)" :placeholder="label('每行一项', 'One per line')" :data-testid="`field-${path}-enum`" @input="setEnum(draft.enumText)" />
      <span v-else class="inline-flex min-h-8 items-center text-gray-400">—</span>
    </td>
    <td class="px-2 py-2">
      <input v-model="draft.defaultText" class="input h-8 w-full min-w-28 px-2 py-1 font-mono text-xs" :aria-label="label(`${path} 默认值（JSON）`, `${path} default (JSON)`)" :placeholder="label('JSON，留空不设置', 'JSON; blank to unset')" :data-testid="`field-${path}-default`" @input="setDefault(draft.defaultText)" />
    </td>
    <td class="px-2 py-2">
      <button type="button" class="flex min-h-8 w-full min-w-32 items-center justify-between gap-2 rounded px-2 text-left text-xs text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700" :aria-label="label(`${path} 约束`, `${path} constraints`)" :aria-expanded="constraintsExpanded" :data-testid="`field-${path}-constraints`" @click="constraintsExpanded = !constraintsExpanded">
        <span>{{ constraintSummary }}</span><span aria-hidden="true">{{ constraintsExpanded ? '−' : '+' }}</span>
      </button>
    </td>
    <td class="py-2 pl-2 pr-3 text-right">
      <button v-if="!arrayItem" type="button" class="min-h-8 rounded px-2 text-xs text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/30" :aria-label="label(`删除 ${path}`, `Delete ${path}`)" :data-testid="`field-${path}-remove`" @click="remove">{{ label('删除', 'Delete') }}</button>
    </td>
  </tr>
  <tr v-if="field" v-show="visible && constraintsExpanded" class="border-b border-gray-100 bg-gray-50/70 dark:border-dark-700 dark:bg-dark-800/40" :data-testid="`field-${path}-constraint-details`">
    <td colspan="7" class="px-4 py-2">
      <div class="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs" :style="{ paddingLeft: `${depth * 18}px` }">
        <label class="flex items-center gap-2"><input type="checkbox" :checked="!!field.nullable" :data-testid="`field-${path}-nullable`" @change="patch({ nullable: ($event.target as HTMLInputElement).checked })" />{{ label('允许 null', 'Allow null') }}</label>
        <label v-if="field.type === 'string'" class="flex items-center gap-2"><input type="checkbox" :checked="field.format === 'http_url'" :disabled="lockedUrl" :data-testid="`field-${path}-format`" @change="patch({ format: ($event.target as HTMLInputElement).checked ? 'http_url' : undefined })" />HTTP(S) URL</label>
        <template v-if="field.type === 'integer' || field.type === 'number'">
          <label class="flex items-center gap-2">{{ label('最小值', 'Minimum') }}<input type="number" step="any" class="input h-8 w-28 px-2 py-1 text-xs" :value="field.minimum" :data-testid="`field-${path}-minimum`" @input="numeric('minimum', $event)" /></label>
          <label class="flex items-center gap-2">{{ label('最大值', 'Maximum') }}<input type="number" step="any" class="input h-8 w-28 px-2 py-1 text-xs" :value="field.maximum" :data-testid="`field-${path}-maximum`" @input="numeric('maximum', $event)" /></label>
        </template>
        <template v-if="field.type === 'string' || field.type === 'array'">
          <label class="flex items-center gap-2">{{ field.type === 'array' ? label('最少数量', 'Minimum items') : label('最小长度', 'Minimum length') }}<input type="number" min="0" class="input h-8 w-28 px-2 py-1 text-xs" :value="field.type === 'array' ? field.min_items : field.min_length" :data-testid="`field-${path}-${field.type === 'array' ? 'min_items' : 'min_length'}`" @input="numeric(field.type === 'array' ? 'min_items' : 'min_length', $event)" /></label>
          <label class="flex items-center gap-2">{{ field.type === 'array' ? label('最多数量', 'Maximum items') : label('最大长度', 'Maximum length') }}<input type="number" min="0" class="input h-8 w-28 px-2 py-1 text-xs" :value="field.type === 'array' ? field.max_items : field.max_length" :data-testid="`field-${path}-${field.type === 'array' ? 'max_items' : 'max_length'}`" @input="numeric(field.type === 'array' ? 'max_items' : 'max_length', $event)" /></label>
        </template>
      </div>
    </td>
  </tr>
  <tr v-if="field && localErrors.length" v-show="visible" class="border-b border-red-100 dark:border-red-900/30">
    <td colspan="7" class="px-4 py-1 text-xs text-red-600 dark:text-red-400"><p v-for="message in localErrors" :key="message" role="alert">{{ message }}</p></td>
  </tr>
  <template v-if="field?.type === 'object' && depth < 7">
    <MediaFieldEditor v-for="(child, key) in field.properties || {}" :key="key" :visible="visible && expanded" :path="`${path}.${key}`" :name="String(key)" :field="child" :default-type="child.type" :depth="depth + 1" allow-type-change renameable @update="updateChild(String(key), $event)" @invalid="(fieldPath, message) => emit('invalid', fieldPath, message)" @clear-errors="fieldPath => emit('clear-errors', fieldPath)" @name-change="renameChild(String(key), $event)" @rename="(from, to) => emit('rename', from, to)" />
    <tr v-show="visible && expanded" class="border-b border-gray-100 dark:border-dark-700">
      <td colspan="7" class="px-3 py-2">
        <div class="flex items-center gap-2" :style="{ paddingLeft: `${(depth + 1) * 18 + 24}px` }">
          <input v-model="childName" class="input h-8 w-60 min-w-0 px-2 py-1 font-mono text-xs" :aria-label="label(`${path} 新增字段名`, `${path} new field name`)" :placeholder="label('新增字段名，例如 resolution', 'New field, e.g. resolution')" :data-testid="`field-${path}-new-name`" @input="clearAddError" @keydown.enter.prevent="addChild" />
          <button type="button" class="btn btn-secondary btn-sm h-8 text-xs" :data-testid="`field-${path}-add`" @click="addChild">{{ label('添加字段', 'Add field') }}</button>
        </div>
      </td>
    </tr>
  </template>
  <MediaFieldEditor v-if="field?.type === 'array' && depth < 7" :visible="visible && expanded" :path="`${path}[]`" name="items" :field="field.items || { type: 'string', ...(path === 'image_urls' ? { format: 'http_url' } : {}) }" default-type="string" :depth="depth + 1" :allowed-types="path === 'image_urls' ? ['string'] : undefined" :locked-url="path === 'image_urls'" allow-type-change array-item @update="value => value && patch({ items: value })" @invalid="(fieldPath, message) => emit('invalid', fieldPath, message)" @clear-errors="fieldPath => emit('clear-errors', fieldPath)" @rename="(from, to) => emit('rename', from, to)" />
  <tr v-if="field && depth >= 7 && nested" v-show="visible && expanded"><td colspan="7" class="px-4 py-2 text-xs text-gray-500">{{ label('已达到可配置的最大嵌套层级。', 'Maximum configurable nesting depth reached.') }}</td></tr>
</template>

<script setup lang="ts">
import { computed, inject, reactive, ref, watch } from 'vue'
import type { MediaCapabilityField, MediaFieldType } from '@/types/mediaCapability'
import { useMediaLabels } from './mediaLabels'

defineOptions({ name: 'MediaFieldEditor' })
const props = withDefaults(defineProps<{
  path: string
  name?: string
  field?: MediaCapabilityField
  defaultType: MediaFieldType
  allowTypeChange?: boolean
  allowedTypes?: MediaFieldType[]
  renameable?: boolean
  arrayItem?: boolean
  lockedUrl?: boolean
  depth?: number
  visible?: boolean
}>(), { depth: 0, visible: true })
const emit = defineEmits<{
  update: [field: MediaCapabilityField | undefined]
  invalid: [path: string, message: string]
  'clear-errors': [path: string]
  'name-change': [name: string]
  rename: [from: string, to: string]
}>()
const label = useMediaLabels()
const types: MediaFieldType[] = ['string', 'integer', 'number', 'boolean', 'array', 'object']
interface FieldDraft {
  enumText: string
  defaultText: string
  acceptedEnum: string
  acceptedDefault: string
  nameText: string
  childName: string
  errors: Record<string, string>
  removedChildren: Record<string, MediaCapabilityField>
}
const drafts = inject<Map<string, FieldDraft>>('media-field-drafts', reactive(new Map<string, FieldDraft>()))
function serial(value: unknown) { return value === undefined ? '' : JSON.stringify(value) ?? '' }
if (!drafts.has(props.path)) {
  drafts.set(props.path, {
    enumText: props.field?.enum?.map(item => typeof item === 'string' ? item : JSON.stringify(item)).join('\n') || '',
    defaultText: serial(props.field?.default), acceptedEnum: serial(props.field?.enum), acceptedDefault: serial(props.field?.default), nameText: props.name || props.path, childName: '', errors: {}, removedChildren: Object.create(null)
  })
}
const draft = computed(() => drafts.get(props.path)!)
const expanded = ref(true)
const constraintsExpanded = ref(false)
const childName = computed({ get: () => draft.value.childName, set: value => { draft.value.childName = value } })
const nameText = computed({ get: () => draft.value.nameText, set: value => { draft.value.nameText = value } })
const nested = computed(() => props.field?.type === 'object' || props.field?.type === 'array')
const primitive = computed(() => !!props.field && ['string', 'integer', 'number', 'boolean'].includes(props.field.type))
const availableTypes = computed(() => {
  const allowed = props.allowedTypes || (props.allowTypeChange ? types : [props.field?.type || props.defaultType])
  return props.depth >= 7 ? allowed.filter(type => type !== 'array' || props.field?.type === 'array') : allowed
})
const displayName = computed(() => props.arrayItem ? 'items' : props.name || props.path)
const localErrors = computed(() => Object.values(draft.value.errors).filter(Boolean))
const constraintSummary = computed(() => {
  const field = props.field
  if (!field) return ''
  const parts: string[] = []
  if (field.nullable) parts.push('null')
  if (field.format === 'http_url') parts.push('HTTP(S) URL')
  if (field.minimum !== undefined) parts.push(`≥ ${field.minimum}`)
  if (field.maximum !== undefined) parts.push(`≤ ${field.maximum}`)
  if (field.min_length !== undefined) parts.push(label(`长度 ≥ ${field.min_length}`, `Length ≥ ${field.min_length}`))
  if (field.max_length !== undefined) parts.push(label(`长度 ≤ ${field.max_length}`, `Length ≤ ${field.max_length}`))
  if (field.min_items !== undefined) parts.push(label(`数量 ≥ ${field.min_items}`, `Items ≥ ${field.min_items}`))
  if (field.max_items !== undefined) parts.push(label(`数量 ≤ ${field.max_items}`, `Items ≤ ${field.max_items}`))
  return parts.join(' · ') || label('无额外约束', 'No extra constraints')
})
watch(() => props.name, value => { nameText.value = value || props.path })
watch(() => serial(props.field?.enum), value => {
  if (!props.field || value === draft.value.acceptedEnum) return
  draft.value.acceptedEnum = value
  draft.value.enumText = props.field.enum?.map(item => typeof item === 'string' ? item : JSON.stringify(item)).join('\n') || ''
  setError('enum', '')
})
watch(() => serial(props.field?.default), value => {
  if (!props.field || value === draft.value.acceptedDefault) return
  draft.value.acceptedDefault = value
  draft.value.defaultText = value
  setError('default', '')
})
watch(() => !!props.field, active => {
  if (active) for (const [key, message] of Object.entries(draft.value.errors)) emit('invalid', `${props.path}.${key}`, message)
}, { immediate: true })

function initial(type: MediaFieldType): MediaCapabilityField {
  return { type, ...(type === 'object' ? { properties: {} } : {}), ...(type === 'array' ? { items: { type: 'string' as const } } : {}) }
}
function patch(values: Partial<MediaCapabilityField>) { if (props.field) emit('update', { ...props.field, ...values }) }
function setError(key: string, message: string) {
  if (message) draft.value.errors[key] = message
  else delete draft.value.errors[key]
  emit('invalid', `${props.path}.${key}`, message)
}
function clearDescendantDrafts() {
  for (const path of drafts.keys()) if (path.startsWith(`${props.path}.`) || path.startsWith(`${props.path}[]`)) drafts.delete(path)
}
function remove() {
  emit('clear-errors', props.path)
  emit('update', undefined)
}
function changeType(type: MediaFieldType) {
  if (!props.field || !availableTypes.value.includes(type) || type === props.field.type) return
  emit('clear-errors', props.path)
  clearDescendantDrafts()
  // Remove type-specific constraints while preserving extensions unknown to the visual editor.
  const next = { ...props.field }
  for (const key of ['enum', 'default', 'minimum', 'maximum', 'min_length', 'max_length', 'min_items', 'max_items', 'properties', 'items', 'format'] as const) delete next[key]
  draft.value.enumText = ''; draft.value.defaultText = ''; draft.value.acceptedEnum = ''; draft.value.acceptedDefault = ''; draft.value.errors = {}; draft.value.removedChildren = Object.create(null)
  emit('update', { ...next, ...initial(type) })
}
function numeric(key: 'minimum' | 'maximum' | 'min_length' | 'max_length' | 'min_items' | 'max_items', event: Event) {
  const value = (event.target as HTMLInputElement).value
  patch({ [key]: value === '' ? undefined : Number(value) })
}
function setEnum(value: string) {
  draft.value.enumText = value
  const values = value.split('\n').map(item => item.trim()).filter(Boolean)
  if (!values.length) { draft.value.acceptedEnum = ''; patch({ enum: undefined }); setError('enum', ''); return }
  try {
    const parsed = props.field?.type === 'string' ? values : values.map(item => JSON.parse(item))
    const type = props.field?.type
    if (parsed.some(item => type === 'integer' ? !Number.isInteger(item) : type === 'number' ? typeof item !== 'number' || !Number.isFinite(item) : type === 'boolean' ? typeof item !== 'boolean' : typeof item !== 'string')) throw new Error('type')
    draft.value.acceptedEnum = serial(parsed)
    patch({ enum: parsed }); setError('enum', '')
  } catch { setError('enum', label('枚举值必须与字段类型一致；数值和布尔值使用 JSON。', 'Enum values must match the field type; use JSON for numbers and booleans.')) }
}
function setDefault(value: string) {
  draft.value.defaultText = value
  try {
    const parsed = value.trim() ? JSON.parse(value) : undefined
    draft.value.acceptedDefault = serial(parsed)
    patch({ default: parsed }); setError('default', '')
  } catch { setError('default', label('默认值不是有效 JSON。', 'Default is not valid JSON.')) }
}
function updateChild(key: string, field?: MediaCapabilityField) {
  const properties = { ...props.field?.properties }
  if (field) properties[key] = field
  else { if (properties[key]) draft.value.removedChildren[key] = properties[key]; delete properties[key] }
  patch({ properties })
}
function validName(key: string) {
  return /^[a-zA-Z_][a-zA-Z0-9_]*$/.test(key) && !['__proto__', 'constructor', 'prototype', 'output_size'].includes(key)
}
function clearAddError() { setError('new_field', '') }
function addChild() {
  const key = childName.value.trim()
  if (!validName(key) || Object.prototype.hasOwnProperty.call(props.field?.properties || {}, key)) {
    setError('new_field', label('字段名无效、已存在或为保留字段。', 'Invalid, duplicate, or reserved field name.')); return
  }
  updateChild(key, draft.value.removedChildren[key] || { type: 'string' })
  childName.value = ''; clearAddError()
}
function commitName() {
  const key = nameText.value.trim()
  if (key === props.name) { setError('name', ''); return }
  if (!validName(key)) { setError('name', label('字段名无效或为保留字段。', 'Invalid or reserved field name.')); return }
  setError('name', '')
  emit('name-change', key)
}
function renameChild(previous: string, next: string) {
  const properties = props.field?.properties || {}
  if (previous === next) return
  const currentDefault = props.field?.default
  const defaultConflict = !!currentDefault && typeof currentDefault === 'object' && !Array.isArray(currentDefault) && Object.prototype.hasOwnProperty.call(currentDefault, previous) && Object.prototype.hasOwnProperty.call(currentDefault, next)
  if (!validName(next) || Object.prototype.hasOwnProperty.call(properties, next) || defaultConflict) {
    const message = defaultConflict ? label('对象默认值已包含目标字段名，请先调整默认值。', 'The object default already contains the target field name. Update the default first.') : label('字段名无效、已存在或为保留字段。', 'Invalid, duplicate, or reserved field name.')
    emit('invalid', `${props.path}.${previous}.name`, message)
    const childDraft = drafts.get(`${props.path}.${previous}`)
    if (childDraft) childDraft.errors.name = message
    return
  }
  const from = `${props.path}.${previous}`
  const to = `${props.path}.${next}`
  emit('clear-errors', from)
  const renamed = Object.fromEntries(Object.entries(properties).map(([key, value]) => [key === previous ? next : key, value]))
  for (const [key, value] of [...drafts.entries()]) {
    if (key === from || key.startsWith(`${from}.`) || key.startsWith(`${from}[]`)) {
      drafts.delete(key); drafts.set(`${to}${key.slice(from.length)}`, value)
    }
  }
  let renamedDefault = currentDefault
  if (currentDefault && typeof currentDefault === 'object' && !Array.isArray(currentDefault) && Object.prototype.hasOwnProperty.call(currentDefault, previous)) {
    renamedDefault = Object.fromEntries(Object.entries(currentDefault).map(([key, value]) => [key === previous ? next : key, value]))
  }
  patch({ properties: renamed, ...(currentDefault !== renamedDefault ? { default: renamedDefault } : {}) })
  emit('rename', from, to)
}
</script>
