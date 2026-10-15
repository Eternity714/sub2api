<template>
  <section class="rounded-xl border border-gray-200 p-3 dark:border-dark-700">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <label class="flex items-center gap-2 font-mono text-sm font-medium">
        <input type="checkbox" :checked="!!field" :data-testid="`field-${path}-supported`" @change="toggle(($event.target as HTMLInputElement).checked)" />
        {{ path }}
      </label>
      <span v-if="field" class="text-xs text-gray-500">{{ field.type }}</span>
    </div>
    <div v-if="field" class="mt-3 space-y-3">
      <div class="flex flex-wrap items-center gap-4 text-sm">
        <label class="flex items-center gap-2"><input type="checkbox" :checked="!!field.required" :data-testid="`field-${path}-required`" @change="patch({ required: ($event.target as HTMLInputElement).checked })" />{{ label('必填', 'Required') }}</label>
        <label class="flex items-center gap-2"><input type="checkbox" :checked="!!field.nullable" @change="patch({ nullable: ($event.target as HTMLInputElement).checked })" />{{ label('允许 null', 'Allow null') }}</label>
        <label v-if="allowTypeChange" class="flex items-center gap-2">{{ label('类型', 'Type') }}
          <select class="input w-32" :value="field.type" @change="changeType(($event.target as HTMLSelectElement).value as MediaFieldType)">
            <option v-for="type in types" :key="type" :value="type">{{ type }}</option>
          </select>
        </label>
        <label v-if="field.type === 'string'" class="flex items-center gap-2"><input type="checkbox" :checked="field.format === 'http_url'" @change="patch({ format: ($event.target as HTMLInputElement).checked ? 'http_url' : undefined })" />HTTP(S) URL</label>
      </div>
      <div v-if="['string', 'integer', 'number', 'boolean'].includes(field.type)" class="grid gap-3 sm:grid-cols-2">
        <label class="text-xs text-gray-600 dark:text-gray-400">{{ label('枚举值（每行一项）', 'Enum values (one per line)') }}
          <textarea class="input mt-1 font-mono text-xs" rows="2" :value="enumText" :data-testid="`field-${path}-enum`" @input="setEnum(($event.target as HTMLTextAreaElement).value)" />
        </label>
        <label class="text-xs text-gray-600 dark:text-gray-400">{{ label('默认值（JSON；留空表示不设置）', 'Default (JSON; blank means unset)') }}
          <input class="input mt-1 font-mono text-xs" :value="defaultText" :data-testid="`field-${path}-default`" @change="setDefault(($event.target as HTMLInputElement).value)" />
        </label>
      </div>
      <div v-if="field.type === 'integer' || field.type === 'number'" class="grid grid-cols-2 gap-3">
        <label class="text-xs">{{ label('最小值', 'Minimum') }}<input type="number" class="input mt-1" :value="field.minimum" :data-testid="`field-${path}-minimum`" @input="numeric('minimum', $event)" /></label>
        <label class="text-xs">{{ label('最大值', 'Maximum') }}<input type="number" class="input mt-1" :value="field.maximum" :data-testid="`field-${path}-maximum`" @input="numeric('maximum', $event)" /></label>
      </div>
      <div v-if="field.type === 'string' || field.type === 'array'" class="grid grid-cols-2 gap-3">
        <label class="text-xs">{{ field.type === 'array' ? label('最少数量', 'Minimum items') : label('最小长度', 'Minimum length') }}<input type="number" min="0" class="input mt-1" :value="field.type === 'array' ? field.min_items : field.min_length" @input="numeric(field.type === 'array' ? 'min_items' : 'min_length', $event)" /></label>
        <label class="text-xs">{{ field.type === 'array' ? label('最多数量', 'Maximum items') : label('最大长度', 'Maximum length') }}<input type="number" min="0" class="input mt-1" :value="field.type === 'array' ? field.max_items : field.max_length" @input="numeric(field.type === 'array' ? 'max_items' : 'max_length', $event)" /></label>
      </div>
      <template v-if="field.type === 'object' && depth < 4">
        <MediaFieldEditor v-for="(child, key) in field.properties || {}" :key="key" :path="`${path}.${key}`" :field="child" :default-type="child.type" :depth="depth + 1" allow-type-change @update="updateChild(String(key), $event)" @invalid="(fieldPath, message) => emit('invalid', fieldPath, message)" />
        <div class="flex gap-2">
          <input v-model="childName" class="input min-w-0 flex-1" :placeholder="label('新增 metadata 字段名，例如 resolution', 'New metadata field, e.g. resolution')" :data-testid="`field-${path}-new-name`" />
          <button type="button" class="btn btn-secondary btn-sm" :data-testid="`field-${path}-add`" @click="addChild">{{ label('添加字段', 'Add field') }}</button>
        </div>
      </template>
      <MediaFieldEditor v-if="field.type === 'array' && depth < 4" :path="`${path}[]`" :field="field.items" default-type="string" :depth="depth + 1" allow-type-change @update="patch({ items: $event })" @invalid="(fieldPath, message) => emit('invalid', fieldPath, message)" />
      <p v-if="depth >= 4 && ['array', 'object'].includes(field.type)" class="text-xs text-gray-500">{{ label('更深的嵌套结构请使用高级配置。', 'Use advanced configuration for deeper nesting.') }}</p>
      <p v-if="localError" class="text-xs text-red-600 dark:text-red-400">{{ localError }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { MediaCapabilityField, MediaFieldType } from '@/types/mediaCapability'
import { useMediaLabels } from './mediaLabels'
defineOptions({ name: 'MediaFieldEditor' })
const props = withDefaults(defineProps<{ path: string; field?: MediaCapabilityField; defaultType: MediaFieldType; allowTypeChange?: boolean; depth?: number }>(), { depth: 0 })
const emit = defineEmits<{ update: [field: MediaCapabilityField | undefined]; invalid: [path: string, message: string] }>()
const label = useMediaLabels()
const types: MediaFieldType[] = ['string', 'integer', 'number', 'boolean', 'array', 'object']
const localError = ref('')
const childName = ref('')
const enumText = ref('')
watch(() => props.field?.enum, value => { enumText.value = value?.map(item => typeof item === 'string' ? item : JSON.stringify(item)).join('\n') || '' }, { immediate: true })
const defaultText = ref('')
watch(() => props.field?.default, value => { defaultText.value = value === undefined ? '' : JSON.stringify(value) }, { immediate: true })
function initial(type: MediaFieldType): MediaCapabilityField {
  return { type, ...(type === 'object' ? { properties: {} } : {}), ...(type === 'array' ? { items: { type: 'string' as const } } : {}) }
}
function toggle(enabled: boolean) { emit('update', enabled ? initial(props.defaultType) : undefined) }
function patch(values: Partial<MediaCapabilityField>) { if (props.field) emit('update', { ...props.field, ...values }) }
function changeType(type: MediaFieldType) { emit('update', { ...initial(type), required: props.field?.required, nullable: props.field?.nullable }) }
function numeric(key: 'minimum' | 'maximum' | 'min_length' | 'max_length' | 'min_items' | 'max_items', event: Event) {
  const value = (event.target as HTMLInputElement).value
  patch({ [key]: value === '' ? undefined : Number(value) })
}
function setEnum(value: string) {
  enumText.value = value
  const values = value.split('\n').map(item => item.trim()).filter(Boolean)
  if (!values.length) { patch({ enum: undefined }); localError.value = ''; emit('invalid', `${props.path}.enum`, ''); return }
  if (props.field?.type === 'string') { patch({ enum: values }); localError.value = ''; emit('invalid', `${props.path}.enum`, '') }
  else {
    try { patch({ enum: values.map(item => JSON.parse(item)) }); localError.value = ''; emit('invalid', `${props.path}.enum`, '') }
    catch { localError.value = label('数值和布尔枚举请使用有效 JSON 值。', 'Use valid JSON for numeric and boolean enum values.'); emit('invalid', `${props.path}.enum`, localError.value) }
  }
}
function setDefault(value: string) {
  defaultText.value = value
  try { patch({ default: value.trim() ? JSON.parse(value) : undefined }); localError.value = ''; emit('invalid', `${props.path}.default`, '') }
  catch { localError.value = label('默认值不是有效 JSON。', 'Default is not valid JSON.'); emit('invalid', `${props.path}.default`, localError.value) }
}
function updateChild(key: string, field?: MediaCapabilityField) {
  const properties = { ...props.field?.properties }
  if (field) properties[key] = field
  else delete properties[key]
  patch({ properties })
}
function addChild() {
  const key = childName.value.trim()
  if (!/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(key) || ['__proto__', 'constructor', 'prototype', 'output_size'].includes(key) || Object.prototype.hasOwnProperty.call(props.field?.properties || {}, key)) {
    localError.value = label('字段名无效、已存在或为保留字段。', 'Invalid, duplicate, or reserved field name.'); emit('invalid', props.path, localError.value); return
  }
  updateChild(key, { type: 'string' })
  childName.value = ''; localError.value = ''; emit('invalid', props.path, '')
}
</script>
