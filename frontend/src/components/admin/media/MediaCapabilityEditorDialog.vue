<template>
  <BaseDialog :show="show" :title="capability ? label('编辑媒体模型能力', 'Edit media capability') : label('新增媒体模型能力', 'Create media capability')" width="extra-wide" :close-on-escape="!saving" :show-close-button="!saving" @close="emit('close')">
    <form class="space-y-6" @submit.prevent="save">
      <div v-if="error || validationErrors.length" role="alert" class="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-900/20 dark:text-red-300">
        <p v-if="error">{{ error }}</p>
        <ul v-if="validationErrors.length" class="mt-1 list-inside list-disc">
          <li v-for="(item, index) in validationErrors" :key="index"><code>{{ item.path }}</code>: {{ item.message }}</li>
        </ul>
      </div>
      <fieldset :disabled="saving" class="space-y-6 disabled:opacity-70">
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="input-label">{{ label('公开模型标识', 'Public model') }}
            <input v-model.trim="form.public_model" data-testid="public-model" class="input mt-1" required maxlength="200" autocomplete="off" />
          </label>
          <label class="input-label">{{ label('媒体类型', 'Media type') }}
            <select v-model="form.media_type" data-testid="media-type" class="input mt-1" @change="resetForMediaType">
              <option value="image">{{ label('图片', 'Image') }}</option><option value="video">{{ label('视频', 'Video') }}</option>
            </select>
          </label>
        </div>
        <section class="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800">
          <h4 class="text-sm font-semibold">{{ label('基础模板', 'Base template') }}</h4>
          <p class="mt-1 text-xs text-gray-500">{{ label('模板只复制到当前草稿，不表示上游模型已具备这些能力。请按实际模型文档核对；保存后与模板独立。应用模板会替换当前草稿中的能力规则。', 'Templates copy rules into this draft; they do not prove provider support. Verify against model documentation. Applying replaces draft rules; saved configurations remain independent.') }}</p>
          <div class="mt-3 flex flex-wrap gap-2">
            <select v-model="templateSelection" class="input min-w-0 flex-1" data-testid="template-select" :aria-label="label('选择基础模板', 'Select base template')">
              <option value="">{{ label('选择模板', 'Select template') }}</option>
              <option v-for="item in compatibleTemplates" :key="`${item.id}-${item.version}`" :value="item.id">{{ item.name }} · v{{ item.version }}</option>
            </select>
            <button type="button" class="btn btn-secondary" data-testid="apply-template" :disabled="!templateSelection" @click="applyTemplate">{{ label('复制到草稿', 'Copy to draft') }}</button>
          </div>
          <div v-if="selectedTemplate" class="mt-3 space-y-1 text-xs text-gray-500">
            <p v-if="selectedTemplate.source">{{ label('文档依据', 'Documentation') }}: <a v-if="templateSourceURL" :href="templateSourceURL" data-testid="template-source" target="_blank" rel="noopener noreferrer" class="break-all text-primary-600 underline dark:text-primary-400">{{ selectedTemplate.source }}</a><span v-else class="break-all">{{ selectedTemplate.source }}</span></p>
            <p v-if="selectedTemplate.evidence">{{ selectedTemplate.evidence }}</p>
            <p v-if="selectedTemplate.notes">{{ selectedTemplate.notes }}</p>
          </div>
          <p v-if="form.template_id" class="mt-2 text-xs text-gray-500">{{ label('来源', 'Source') }}: {{ form.template_id }} · v{{ form.template_version }}</p>
        </section>
        <section class="space-y-3">
          <h4 class="font-semibold">{{ label('统一字段与参数限制', 'Fields and constraints') }}</h4>
          <p class="text-xs text-gray-500">{{ label('model 始终必填。仅勾选实际支持的字段；metadata 的子字段同样实行白名单。图片尺寸使用 size + metadata.resolution。', 'model is always required. Enable only supported fields. Metadata properties are also allowlisted. Image sizes use size + metadata.resolution.') }}</p>
          <div class="grid items-start gap-3 lg:grid-cols-2">
            <MediaFieldEditor v-for="name in fieldNames" :key="`${form.media_type}-${name}-${editorGeneration}`" :path="name" :field="form.schema.fields[name]" :default-type="fieldTypes[name] || 'string'" @update="setField(name, $event)" @invalid="setFieldError" />
          </div>
        </section>
        <section class="space-y-3 border-t border-gray-200 pt-5 dark:border-dark-700">
          <div class="flex items-center justify-between gap-2"><h4 class="font-semibold">{{ label('输入模式', 'Input modes') }}</h4><button type="button" class="btn btn-secondary btn-sm" data-testid="add-mode" @click="addMode">{{ label('添加模式', 'Add mode') }}</button></div>
          <p class="text-xs text-gray-500">{{ label('例如参考素材模式可要求 image_urls 并允许省略 prompt。条件基于已有字段；不增加下游顶层 mode。多项条件同时满足才匹配。', 'A reference mode may require image_urls and omit prompt. Conditions use existing fields, not a new top-level mode. All conditions must match.') }}</p>
          <div v-for="(mode, index) in form.schema.modes" :key="index" class="space-y-3 rounded-xl border border-gray-200 p-3 dark:border-dark-700">
            <div class="flex gap-3"><label class="input-label flex-1">{{ label('模式标识', 'Mode ID') }}<input v-model.trim="mode.id" :data-testid="`mode-${index}-id`" class="input mt-1" /></label><button type="button" class="btn btn-ghost self-end" @click="removeMode(index)">{{ label('删除', 'Remove') }}</button></div>
            <div class="grid gap-3 sm:grid-cols-2">
              <label class="input-label">{{ label('必填字段路径（逗号分隔）', 'Required paths (comma separated)') }}<input :value="mode.required.join(', ')" :data-testid="`mode-${index}-required`" class="input mt-1 font-mono text-xs" @input="mode.required = paths(($event.target as HTMLInputElement).value)" /></label>
              <label class="input-label">{{ label('禁止字段路径（逗号分隔）', 'Forbidden paths (comma separated)') }}<input :value="mode.forbidden.join(', ')" class="input mt-1 font-mono text-xs" @input="mode.forbidden = paths(($event.target as HTMLInputElement).value)" /></label>
            </div>
            <label class="input-label">{{ label('匹配条件（JSON 数组）', 'Match conditions (JSON array)') }}<textarea :value="modeConditions[index]" rows="3" class="input mt-1 font-mono text-xs" placeholder='[{"path":"image_urls","op":"not_empty"}]' @input="setModeConditions(index, ($event.target as HTMLTextAreaElement).value)" /></label>
          </div>
        </section>
        <section v-if="form.media_type === 'image'" class="space-y-3 border-t border-gray-200 pt-5 dark:border-dark-700">
          <div class="flex items-center justify-between gap-2"><h4 class="font-semibold">{{ label('已验证的像素规格映射', 'Verified pixel mappings') }}</h4><button type="button" class="btn btn-secondary btn-sm" data-testid="add-image-spec" @click="addImageSpec">{{ label('添加规格', 'Add mapping') }}</button></div>
          <p class="text-xs text-gray-500">{{ label('只填写已有依据的 ratio + resolution → 像素尺寸；不使用全局推算或静默降规格。', 'Only configure documented ratio + resolution pixel mappings. No global approximation or silent downgrade.') }}</p>
          <div v-for="(spec, index) in form.schema.image_specs" :key="index" class="grid items-end gap-2 rounded-xl border border-gray-200 p-3 sm:grid-cols-5 dark:border-dark-700">
            <label class="input-label">{{ label('比例', 'Ratio') }}<input v-model.trim="spec.ratio" :data-testid="`spec-${index}-ratio`" class="input mt-1" placeholder="1:1" /></label>
            <label class="input-label">resolution<input v-model.trim="spec.resolution" :data-testid="`spec-${index}-resolution`" class="input mt-1" placeholder="1K" /></label>
            <label class="input-label">{{ label('宽(px)', 'Width(px)') }}<input v-model.number="spec.width" :data-testid="`spec-${index}-width`" type="number" min="1" step="1" class="input mt-1" /></label>
            <label class="input-label">{{ label('高(px)', 'Height(px)') }}<input v-model.number="spec.height" :data-testid="`spec-${index}-height`" type="number" min="1" step="1" class="input mt-1" /></label>
            <button type="button" class="btn btn-ghost" @click="form.schema.image_specs?.splice(index, 1)">{{ label('删除', 'Remove') }}</button>
          </div>
          <div v-if="form.schema.batch" class="flex flex-wrap items-center gap-4 text-sm">
            <label>{{ label('单次原生结果上限', 'Native results per request') }}<input v-model.number="form.schema.batch.native_max_results" type="number" min="1" step="1" class="input mt-1 w-32" /></label>
            <label class="flex items-center gap-2"><input v-model="form.schema.batch.allow_split" type="checkbox" />{{ label('允许拆分', 'Allow splitting') }}</label>
            <label class="flex items-center gap-2"><input v-model="form.schema.batch.semantically_linked" type="checkbox" />{{ label('结果语义关联', 'Results are semantically linked') }}</label>
          </div>
        </section>
        <details class="rounded-xl border border-gray-200 p-4 dark:border-dark-700" @toggle="openAdvanced">
          <summary class="cursor-pointer text-sm font-medium">{{ label('高级规则与嵌套 schema', 'Advanced rules and nested schema') }}</summary>
          <p class="my-3 text-xs text-gray-500">{{ label('用于组合规则、数组对象及复杂嵌套。先应用到草稿，再保存。服务端会再次严格校验；不能扩展协议顶层字段。', 'For combination rules, object arrays, and complex nesting. Apply to draft before saving. The server validates again; protocol top-level fields cannot be extended.') }}</p>
          <textarea v-model="advancedSchema" rows="12" class="input font-mono text-xs" :aria-label="label('高级 schema JSON', 'Advanced schema JSON')" />
          <button type="button" class="btn btn-secondary btn-sm mt-2" @click="applyAdvanced">{{ label('应用高级配置到草稿', 'Apply advanced configuration') }}</button>
        </details>
        <label class="input-label block">{{ label('备注', 'Notes') }}<textarea v-model="form.notes" data-testid="capability-notes" class="input mt-1" rows="2" maxlength="2000" /></label>
        <label class="flex items-center gap-2 text-sm"><input v-model="form.enabled" data-testid="capability-enabled" type="checkbox" />{{ label('启用这条能力配置', 'Enable capability') }}</label>
        <section class="space-y-3 border-t border-gray-200 pt-5 dark:border-dark-700">
          <h4 class="font-semibold">{{ label('请求校验预览', 'Request validation preview') }}</h4>
          <p class="text-xs text-gray-500">{{ label('只校验当前草稿，不提交上游、不创建任务、不扣费。原始文本直接交给服务端，保留重复字段与尾随内容供检测。', 'Validates this draft only. No upstream request, task creation, or charge. Raw text is sent unchanged so duplicate keys and trailing content can be detected.') }}</p>
          <textarea v-model="requestJSON" data-testid="request-json" class="input font-mono text-xs" rows="5" :aria-label="label('待校验请求 JSON', 'Request JSON to validate')" />
          <button type="button" class="btn btn-secondary" data-testid="validate-request" :disabled="validating || !requestJSON.trim()" @click="validateRequest">{{ validating ? label('校验中…', 'Validating…') : label('校验请求', 'Validate request') }}</button>
          <div v-if="validationResult" data-testid="validation-result" class="rounded-xl bg-gray-50 p-3 text-sm dark:bg-dark-800" aria-live="polite">
            <p :class="validationResult.valid ? 'text-emerald-700 dark:text-emerald-300' : 'text-red-700 dark:text-red-300'">{{ validationResult.valid ? label('校验通过', 'Validation passed') : label('校验未通过', 'Validation failed') }}</p>
            <ul v-if="validationResult.errors.length" class="mt-2 space-y-1"><li v-for="(item, index) in validationResult.errors" :key="index"><code>{{ item.path }}</code>: {{ item.message }}</li></ul>
            <pre v-if="validationResult.valid && validationResult.normalized" class="mt-2 max-h-48 overflow-auto text-xs">{{ JSON.stringify(validationResult.normalized, null, 2) }}</pre>
          </div>
        </section>
      </fieldset>
      <div class="flex justify-end gap-3 border-t border-gray-200 pt-4 dark:border-dark-700">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="emit('close')">{{ label('取消', 'Cancel') }}</button>
        <button type="submit" class="btn btn-primary" :disabled="saving || validating">{{ saving ? label('保存中…', 'Saving…') : label('保存配置', 'Save configuration') }}</button>
      </div>
    </form>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { mediaCapabilitiesAPI } from '@/api/admin/mediaCapabilities'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { MediaCapability, MediaCapabilityField, MediaCapabilityInput, MediaCapabilitySchema, MediaCapabilityTemplate, MediaCondition, MediaFieldType, MediaValidationError, MediaValidationResult } from '@/types/mediaCapability'
import MediaFieldEditor from './MediaFieldEditor.vue'
import { useMediaLabels } from './mediaLabels'
const props = defineProps<{ show: boolean; capability: MediaCapability | null; templates: MediaCapabilityTemplate[] }>()
const emit = defineEmits<{ close: []; saved: [capability: MediaCapability] }>()
const label = useMediaLabels()
const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value)) as T
const blankSchema = (): MediaCapabilitySchema => ({ version: 1, fields: {}, modes: [], rules: [], image_specs: [], batch: { native_max_results: 1, allow_split: false, semantically_linked: false } })
const form = reactive<MediaCapabilityInput>({ public_model: '', media_type: 'image', enabled: false, schema: blankSchema(), notes: '' })
const fieldTypes: Record<string, MediaFieldType> = { prompt: 'string', size: 'string', n: 'integer', duration: 'number', aspect_ratio: 'string', image_urls: 'array', metadata: 'object' }
const fieldNames = computed(() => form.media_type === 'image' ? ['prompt', 'size', 'n', 'image_urls', 'metadata'] : ['prompt', 'duration', 'aspect_ratio', 'image_urls', 'metadata'])
const compatibleTemplates = computed(() => props.templates.filter(item => item.media_type === form.media_type))
const templateSelection = ref('')
const selectedTemplate = computed(() => compatibleTemplates.value.find(item => item.id === templateSelection.value))
const templateSourceURL = computed(() => {
  try { const url = new URL(selectedTemplate.value?.source || ''); return url.protocol === 'https:' ? url.href : undefined }
  catch { return undefined }
})
const saving = ref(false)
const validating = ref(false)
const error = ref('')
const validationErrors = ref<MediaValidationError[]>([])
const fieldErrors = reactive<Record<string, string>>({})
const modeConditions = ref<string[]>([])
const advancedSchema = ref('')
const requestJSON = ref('')
const validationResult = ref<MediaValidationResult | null>(null)
const editorGeneration = ref(0)
let session = 0
function normalizeSchema(schema: MediaCapabilitySchema): MediaCapabilitySchema {
  return { ...clone(schema), modes: clone(schema.modes || []), rules: clone(schema.rules || []), image_specs: clone(schema.image_specs || []), batch: clone(schema.batch || { native_max_results: 1, allow_split: false, semantically_linked: false }) }
}
function syncEditors() {
  modeConditions.value = (form.schema.modes || []).map(mode => JSON.stringify(mode.when || [], null, 2))
  advancedSchema.value = JSON.stringify(form.schema, null, 2)
  for (const key of Object.keys(fieldErrors)) delete fieldErrors[key]
  validationResult.value = null
  editorGeneration.value++
}
watch(() => [props.show, props.capability?.id], () => {
  session++
  if (!props.show) return
  const record = props.capability
  Object.assign(form, { public_model: record?.public_model || '', media_type: record?.media_type || 'image', enabled: record?.enabled ?? false, schema: record ? normalizeSchema(record.schema) : blankSchema(), template_id: record?.template_id, template_version: record?.template_version, notes: record?.notes || '' })
  saving.value = false; validating.value = false; error.value = ''; validationErrors.value = []; templateSelection.value = ''; requestJSON.value = ''; syncEditors()
}, { immediate: true })
onBeforeUnmount(() => { session++ })
function resetForMediaType() {
  form.schema = blankSchema(); form.template_id = undefined; form.template_version = undefined; templateSelection.value = ''; error.value = ''; validationErrors.value = []; syncEditors()
}
function applyTemplate() {
  const item = compatibleTemplates.value.find(template => template.id === templateSelection.value)
  if (!item) return
  form.schema = normalizeSchema(item.schema); form.template_id = item.id; form.template_version = item.version
  if (!form.public_model.trim() && item.public_model) form.public_model = item.public_model
  if (!form.notes?.trim() && item.notes) form.notes = item.notes
  form.enabled = false
  requestJSON.value = item.example ? JSON.stringify({ ...clone(item.example), model: form.public_model }, null, 2) : ''
  error.value = ''; validationErrors.value = []; syncEditors()
}
function setField(name: string, field?: MediaCapabilityField) {
  if (field) form.schema.fields[name] = field
  else { delete form.schema.fields[name]; for (const key of Object.keys(fieldErrors)) if (key === name || key.startsWith(`${name}.`) || key.startsWith(`${name}[`)) delete fieldErrors[key] }
  validationResult.value = null
}
function setFieldError(path: string, message: string) { if (message) fieldErrors[path] = message; else delete fieldErrors[path] }
function paths(value: string) { return value.split(',').map(item => item.trim()).filter(Boolean) }
function addMode() { form.schema.modes?.push({ id: '', when: [], required: [], forbidden: [] }); modeConditions.value.push('[]') }
function removeMode(index: number) { form.schema.modes?.splice(index, 1); modeConditions.value.splice(index, 1); delete fieldErrors[`modes.${index}.when`] }
function setModeConditions(index: number, text: string) {
  modeConditions.value[index] = text
  try {
    const value: unknown = JSON.parse(text)
    if (!Array.isArray(value)) throw new Error()
    form.schema.modes![index]!.when = value as MediaCondition[]
    delete fieldErrors[`modes.${index}.when`]
  } catch { fieldErrors[`modes.${index}.when`] = label('条件必须是 JSON 数组。', 'Conditions must be a JSON array.') }
}
function addImageSpec() { form.schema.image_specs?.push({ ratio: '', resolution: '', width: 0, height: 0 }) }
function openAdvanced(event: Event) { if ((event.target as HTMLDetailsElement).open) advancedSchema.value = JSON.stringify(form.schema, null, 2) }
function applyAdvanced() {
  try {
    const value: unknown = JSON.parse(advancedSchema.value)
    if (!value || typeof value !== 'object' || Array.isArray(value) || !('fields' in value) || !value.fields || typeof value.fields !== 'object' || Array.isArray(value.fields)) throw new Error()
    form.schema = normalizeSchema(value as MediaCapabilitySchema)
    syncEditors(); error.value = ''; validationErrors.value = []
  } catch { error.value = label('高级 schema 必须是包含 fields 对象的有效 JSON。', 'Advanced schema must be valid JSON containing a fields object.') }
}
function checkDraft(): boolean {
  error.value = ''; validationErrors.value = []
  if (!form.public_model.trim()) { error.value = label('请填写公开模型标识。', 'Public model is required.'); return false }
  const errors: MediaValidationError[] = Object.entries(fieldErrors).map(([path, message]) => ({ path, message, code: 'invalid_field' }))
  function checkFields(fields: Record<string, MediaCapabilityField>, prefix: string) {
    for (const [name, field] of Object.entries(fields)) {
      const path = prefix ? `${prefix}.${name}` : name
      for (const [minKey, maxKey] of [['minimum', 'maximum'], ['min_length', 'max_length'], ['min_items', 'max_items']] as const) {
        if (field[minKey] !== undefined && field[maxKey] !== undefined && field[minKey]! > field[maxKey]!) errors.push({ path, code: 'invalid_range', message: label('最小值不能大于最大值。', 'Minimum cannot exceed maximum.') })
      }
      if (field.properties) checkFields(field.properties, path)
    }
  }
  checkFields(form.schema.fields, '')
  validationErrors.value = errors
  return errors.length === 0
}
function showAPIError(cause: unknown) {
  const failure = cause as { status?: number; metadata?: { fields?: string; errors?: MediaValidationError[] } }
  error.value = failure?.status === 409 ? label('配置已被其他操作修改，请关闭后重新加载，再合并你的草稿。', 'Configuration changed. Close and reload before merging your draft.') : extractApiErrorMessage(cause, label('操作失败，请重试。', 'Operation failed. Please retry.'))
  const fields = failure?.metadata?.fields
  if (typeof fields === 'string') {
    try { const parsed: unknown = JSON.parse(fields); if (Array.isArray(parsed)) validationErrors.value = parsed as MediaValidationError[] } catch { /* Keep the safe server message. */ }
  } else if (Array.isArray(failure?.metadata?.errors)) validationErrors.value = failure.metadata.errors
}
async function save() {
  if (saving.value || !checkDraft()) return
  saving.value = true
  const requestSession = session
  try {
    const input = clone(form)
    const saved = props.capability ? await mediaCapabilitiesAPI.update(props.capability.id, { ...input, revision: props.capability.revision }) : await mediaCapabilitiesAPI.create(input)
    if (requestSession === session) emit('saved', saved)
  } catch (cause) { if (requestSession === session) showAPIError(cause) }
  finally { if (requestSession === session) saving.value = false }
}
async function validateRequest() {
  if (validating.value || !checkDraft()) return
  validating.value = true; validationResult.value = null
  const requestSession = session
  try {
    const result = await mediaCapabilitiesAPI.validate({ public_model: form.public_model, media_type: form.media_type, schema: clone(form.schema), request_json: requestJSON.value })
    if (requestSession === session) validationResult.value = result
  } catch (cause) { if (requestSession === session) showAPIError(cause) }
  finally { if (requestSession === session) validating.value = false }
}
</script>
