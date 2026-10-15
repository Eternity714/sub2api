<template>
  <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
    <table class="w-full min-w-[880px] border-collapse text-sm" data-testid="media-fields-table" :aria-label="label('统一字段与参数限制', 'Unified fields and parameter constraints')">
      <thead class="bg-gray-50 text-left text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300">
        <tr>
          <th scope="col" class="min-w-48 py-2.5 pl-3 pr-2 font-medium">{{ label('参数名', 'Parameter') }}</th>
          <th scope="col" class="px-2 py-2.5 font-medium">{{ label('类型', 'Type') }}</th>
          <th scope="col" class="w-14 px-2 py-2.5 text-center font-medium">{{ label('必填', 'Required') }}</th>
          <th scope="col" class="w-1/5 px-2 py-2.5 font-medium">{{ label('枚举值', 'Enum') }}</th>
          <th scope="col" class="w-1/5 px-2 py-2.5 font-medium">{{ label('默认值', 'Default') }}</th>
          <th scope="col" class="px-2 py-2.5 font-medium">{{ label('约束', 'Constraints') }}</th>
          <th scope="col" class="w-16 py-2.5 pl-2 pr-3 text-right font-medium">{{ label('操作', 'Actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <MediaFieldEditor v-for="key in declaredFields" :key="key" :path="key" :name="key" :field="fields[key]" :default-type="fieldType(key)" :allowed-types="key === 'duration' ? ['integer', 'number'] : [fieldType(key)]" @update="updateField(key, $event)" @invalid="(path, message) => emit('invalid', path, message)" @clear-errors="path => emit('clear-errors', path)" @rename="renameField" />
        <tr v-if="!declaredFields.length"><td colspan="7" class="px-4 py-5 text-center text-xs text-gray-500">{{ label('暂无可选参数，请从下方添加。', 'No optional parameters. Add one below.') }}</td></tr>
        <tr class="bg-gray-50/50 dark:bg-dark-800/30">
          <td colspan="7" class="px-3 py-2">
            <div class="flex items-center gap-2">
              <select v-model="newField" class="input h-8 w-56 px-2 py-1 font-mono text-xs" :aria-label="label('选择要添加的统一字段', 'Select a unified field to add')" data-testid="top-add-select">
                <option value="">{{ availableFields.length ? label('选择参数', 'Select parameter') : label('已添加全部统一字段', 'All unified fields added') }}</option>
                <option v-for="key in availableFields" :key="key" :value="key">{{ key }}</option>
              </select>
              <button type="button" class="btn btn-secondary btn-sm h-8 text-xs" :disabled="!newField" data-testid="top-add-field" @click="addField">{{ label('添加参数', 'Add parameter') }}</button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed, provide, reactive, ref, watch } from 'vue'
import type { MediaCapabilityField, MediaFieldType, MediaType } from '@/types/mediaCapability'
import MediaFieldEditor from './MediaFieldEditor.vue'
import { useMediaLabels } from './mediaLabels'

const props = defineProps<{ fields: Record<string, MediaCapabilityField>; mediaType: MediaType }>()
const emit = defineEmits<{
  update: [fields: Record<string, MediaCapabilityField>]
  invalid: [path: string, message: string]
  'clear-errors': [path: string]
  rename: [from: string, to: string]
}>()
const label = useMediaLabels()
provide('media-field-drafts', reactive(new Map()))
const removedFields = new Map<string, MediaCapabilityField>()
const newField = ref('')
const fieldTypes: Record<string, MediaFieldType> = { prompt: 'string', size: 'string', n: 'integer', duration: 'number', aspect_ratio: 'string', image_urls: 'array', metadata: 'object' }
const supportedFields = computed(() => props.mediaType === 'image' ? ['prompt', 'size', 'n', 'image_urls', 'metadata'] : ['prompt', 'duration', 'aspect_ratio', 'image_urls', 'metadata'])
const declaredFields = computed(() => Object.keys(props.fields))
const availableFields = computed(() => supportedFields.value.filter(key => !Object.prototype.hasOwnProperty.call(props.fields, key)))
watch(() => props.mediaType, () => { newField.value = ''; removedFields.clear() })
function fieldType(key: string): MediaFieldType { return fieldTypes[key] || props.fields[key]?.type || 'string' }
function updateField(key: string, field?: MediaCapabilityField) {
  const fields = { ...props.fields }
  if (field) fields[key] = field
  else {
    if (fields[key]) removedFields.set(key, fields[key])
    delete fields[key]
    emit('clear-errors', key)
  }
  emit('update', fields)
}
function addField() {
  const key = newField.value
  if (!availableFields.value.includes(key)) return
  const type = fieldType(key)
  updateField(key, removedFields.get(key) || { type, ...(type === 'object' ? { properties: {} } : {}), ...(type === 'array' ? { items: { type: 'string', format: 'http_url' } } : {}) })
  newField.value = ''
}
function renameField(from: string, to: string) { emit('rename', from, to) }
</script>
