<template>
  <AppLayout>
    <TablePageLayout>
      <template #actions>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="max-w-2xl text-sm text-gray-500 dark:text-gray-400">{{ label('按公开模型和媒体类型维护生成参数。基础模板需要按模型核对；此页面的校验不会发起生成或收费。', 'Manage generation parameters by public model and media type. Verify templates against model support. Validation does not generate media or charge.') }}</p>
          <button type="button" class="btn btn-primary" :disabled="loading || !!templateError" @click="openEditor(null)">{{ label('新增能力配置', 'Add capability') }}</button>
        </div>
      </template>
      <template #filters>
        <form class="flex flex-wrap gap-3" @submit.prevent="reload">
          <input v-model.trim="search" class="input min-w-48 flex-1" :aria-label="label('搜索公开模型', 'Search public model')" :placeholder="label('搜索公开模型', 'Search public model')" />
          <select v-model="mediaFilter" data-testid="media-filter" class="input w-40" :aria-label="label('媒体类型筛选', 'Media type filter')" @change="reload"><option value="">{{ label('全部媒体类型', 'All media types') }}</option><option value="image">{{ label('图片', 'Image') }}</option><option value="video">{{ label('视频', 'Video') }}</option></select>
          <button type="submit" class="btn btn-secondary" :disabled="loading">{{ label('查询', 'Search') }}</button>
        </form>
        <p v-if="error || templateError" role="alert" class="mt-3 rounded-xl bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error || templateError }} <button type="button" class="underline" @click="retry">{{ label('重试', 'Retry') }}</button></p>
      </template>
      <template #table>
        <DataTable v-if="!error" :columns="columns" :data="items" :loading="loading" row-key="id">
          <template #cell-public_model="{ row }"><span class="font-mono text-sm">{{ row.public_model }}</span><p class="mt-1 text-xs text-gray-500">#{{ row.id }} · {{ label('修订', 'Revision') }} {{ row.revision }}</p></template>
          <template #cell-media_type="{ row }"><span class="badge badge-gray">{{ row.media_type === 'image' ? label('图片', 'Image') : label('视频', 'Video') }}</span></template>
          <template #cell-enabled="{ row }"><span :class="['badge', row.enabled ? 'badge-success' : 'badge-gray']">{{ row.enabled ? label('启用', 'Enabled') : label('禁用', 'Disabled') }}</span></template>
          <template #cell-template_id="{ row }"><span v-if="row.template_id">{{ row.template_id }} · v{{ row.template_version }}</span><span v-else class="text-gray-400">—</span></template>
          <template #cell-actions="{ row }"><div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary btn-sm" @click="openEditor(row)">{{ label('编辑', 'Edit') }}</button><button type="button" class="btn btn-ghost btn-sm text-red-600" @click="deleteTarget = row">{{ label('删除', 'Delete') }}</button></div></template>
          <template #empty><p data-testid="capabilities-empty" class="p-8 text-center text-sm text-gray-500">{{ label('暂无能力配置。请选择基础模板并按实际模型支持范围配置。', 'No capabilities configured. Start from a template and verify model support.') }}</p></template>
        </DataTable>
      </template>
      <template #pagination><Pagination :total="total" :page="page" :page-size="pageSize" @update:page="changePage" @update:page-size="changePageSize" /></template>
    </TablePageLayout>
    <MediaCapabilityEditorDialog v-if="editorOpen" :show="editorOpen" :capability="editing" :templates="templates" @close="editorOpen = false" @saved="saved" />
    <ConfirmDialog :show="!!deleteTarget" :title="label('删除能力配置', 'Delete capability')" :message="label('删除只影响后续新请求；已有任务继续使用创建时的能力快照。确认删除这条配置？', 'Deletion affects new requests. Existing tasks keep their accepted capability snapshot. Delete this configuration?')" :confirm-text="deleting ? label('删除中…', 'Deleting…') : undefined" @confirm="deleteCapability" @cancel="deleteTarget = null" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { mediaCapabilitiesAPI } from '@/api/admin/mediaCapabilities'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Column } from '@/components/common/types'
import type { MediaCapability, MediaCapabilityTemplate, MediaType } from '@/types/mediaCapability'
import MediaCapabilityEditorDialog from '@/components/admin/media/MediaCapabilityEditorDialog.vue'
import { useMediaLabels } from '@/components/admin/media/mediaLabels'
const label = useMediaLabels()
const items = ref<MediaCapability[]>([])
const templates = ref<MediaCapabilityTemplate[]>([])
const search = ref('')
const mediaFilter = ref<'' | MediaType>('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const loading = ref(false)
const error = ref('')
const templateError = ref('')
const editorOpen = ref(false)
const editing = ref<MediaCapability | null>(null)
const deleteTarget = ref<MediaCapability | null>(null)
const deleting = ref(false)
let listRevision = 0
let active = true
const columns = computed<Column[]>(() => [
  { key: 'public_model', label: label('公开模型', 'Public model') },
  { key: 'media_type', label: label('媒体类型', 'Media type') },
  { key: 'enabled', label: label('状态', 'Status') },
  { key: 'template_id', label: label('模板来源', 'Template source') },
  { key: 'actions', label: label('操作', 'Actions') },
])
async function load() {
  const revision = ++listRevision
  loading.value = true; error.value = ''
  try {
    const response = await mediaCapabilitiesAPI.list({ page: page.value, page_size: pageSize.value, search: search.value || undefined, media_type: mediaFilter.value || undefined })
    if (!active || revision !== listRevision) return
    items.value = response.items; total.value = response.total
  } catch (cause) { if (active && revision === listRevision) { error.value = extractApiErrorMessage(cause, label('加载能力配置失败。', 'Failed to load capabilities.')); items.value = [] } }
  finally { if (active && revision === listRevision) loading.value = false }
}
async function loadTemplates() {
  try { const result = await mediaCapabilitiesAPI.templates(); if (active) { templates.value = result; templateError.value = '' } }
  catch (cause) { if (active) templateError.value = extractApiErrorMessage(cause, label('加载基础模板失败。', 'Failed to load templates.')) }
}
function reload() { page.value = 1; void load() }
function retry() { void load(); void loadTemplates() }
function changePage(value: number) { page.value = value; void load() }
function changePageSize(value: number) { pageSize.value = value; reload() }
function openEditor(item: MediaCapability | null) { editing.value = item; editorOpen.value = true }
function saved() { editorOpen.value = false; void load() }
async function deleteCapability() {
  const item = deleteTarget.value
  if (!item || deleting.value) return
  deleting.value = true
  try { await mediaCapabilitiesAPI.delete(item.id, item.revision); if (active) { deleteTarget.value = null; await load() } }
  catch (cause) { if (active) { error.value = extractApiErrorMessage(cause, label('删除失败；配置可能已修改，请重新加载。', 'Deletion failed; the capability may have changed. Reload and retry.')); deleteTarget.value = null } }
  finally { if (active) deleting.value = false }
}
onMounted(() => { void load(); void loadTemplates() })
onBeforeUnmount(() => { active = false; listRevision++ })
</script>
