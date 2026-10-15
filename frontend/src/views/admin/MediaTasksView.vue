<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="max-w-2xl text-sm text-gray-500 dark:text-gray-400">{{ label('查看任务进度、额度与上游执行状态。此页面仅读取已持久化状态。', 'Inspect persisted task progress, charges, and upstream execution states.') }}</p>
        <button type="button" data-testid="tasks-refresh" class="btn btn-secondary" :disabled="loading" @click="load">{{ loading ? label('加载中…', 'Loading…') : label('刷新', 'Refresh') }}</button>
      </div>
      <label class="flex items-center gap-3 text-sm">{{ label('任务状态', 'Task status') }}
        <select v-model="status" data-testid="task-status-filter" class="input w-48" @change="filter"><option value="">{{ label('全部', 'All') }}</option><option value="queued">queued</option><option value="in_progress">in_progress</option><option value="completed">completed</option></select>
      </label>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
      <div v-if="loading && !items.length" class="card p-8 text-center text-gray-500">{{ label('加载中…', 'Loading…') }}</div>
      <div v-else-if="!items.length && !error" class="card p-8 text-center text-gray-500">{{ label('暂无匹配任务', 'No matching tasks') }}</div>
      <article v-for="task in items" :key="task.id" class="card overflow-hidden p-4 sm:p-5">
        <div class="flex flex-wrap justify-between gap-3">
          <div class="min-w-0"><h3 class="break-all font-mono text-sm font-semibold">{{ task.id }}</h3><p class="mt-1 break-all text-sm text-gray-600 dark:text-gray-300">{{ task.model }} · {{ task.media_type }}</p></div>
          <span class="badge badge-gray self-start">{{ task.status }} · {{ task.progress }}%</span>
        </div>
        <dl class="mt-4 grid gap-x-6 gap-y-3 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <div><dt class="text-xs text-gray-500">{{ label('用户 / API Key / 分组', 'User / API key / group') }}</dt><dd class="mt-1 font-mono">{{ task.user_id }} / {{ task.api_key_id }} / {{ task.group_id }}</dd></div>
          <div><dt class="text-xs text-gray-500">{{ label('预留 / 正式费用', 'Held / charged') }}</dt><dd class="mt-1 font-mono">{{ money(task.held_amount) }} / {{ money(task.charged_amount) }}</dd></div>
          <div><dt class="text-xs text-gray-500">{{ label('成功交付结果数', 'Delivered results') }}</dt><dd class="mt-1">{{ task.result_count }}</dd></div>
          <div><dt class="text-xs text-gray-500">{{ label('创建时间', 'Created') }}</dt><dd class="mt-1">{{ timestamp(task.created_at) }}</dd></div>
          <div v-if="task.completed_at"><dt class="text-xs text-gray-500">{{ label('处理结束时间', 'Completed') }}</dt><dd class="mt-1">{{ timestamp(task.completed_at) }}</dd></div>
          <div v-if="task.expires_at"><dt class="text-xs text-gray-500">{{ label('结果有效期', 'Results expire') }}</dt><dd class="mt-1">{{ timestamp(task.expires_at) }}</dd></div>
        </dl>
        <p v-if="task.error" class="mt-3 whitespace-pre-wrap break-all rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ task.error }}</p>
        <button type="button" :data-testid="`task-units-${task.id}`" class="btn btn-ghost btn-sm mt-3" :aria-expanded="expanded.has(task.id)" @click="toggle(task.id)">{{ expanded.has(task.id) ? label('收起执行单元', 'Hide execution units') : label('展开执行单元', 'Show execution units') }} ({{ task.units?.length || 0 }})</button>
        <div v-if="expanded.has(task.id)" class="mt-3 space-y-2 border-t border-gray-200 pt-3 dark:border-dark-700">
          <div v-for="unit in task.units || []" :key="unit.index" class="rounded-xl border border-gray-200 bg-gray-50 p-3 text-xs dark:border-dark-700 dark:bg-dark-800">
            <div class="flex flex-wrap items-center justify-between gap-2"><strong>#{{ unit.index }} · {{ unit.state }}</strong><span>{{ unit.platform || '—' }} · {{ label('账号', 'Account') }} #{{ unit.account_id || '—' }}</span></div>
            <div class="mt-2 grid gap-2 sm:grid-cols-2"><p class="break-all">{{ label('上游模型', 'Upstream model') }}: {{ unit.upstream_model || '—' }}</p><p class="break-all">{{ label('上游任务', 'Upstream task') }}: {{ unit.upstream_task_id || '—' }}</p><p>{{ label('提交次数', 'Submission attempts') }}: {{ unit.attempt }}</p><p>{{ label('已转存 / 请求结果数', 'Stored / requested results') }}: {{ unit.stored_count }} / {{ unit.count }}</p></div>
            <p v-if="unit.error" class="mt-2 whitespace-pre-wrap break-all text-red-700 dark:text-red-300">{{ unit.error }}</p>
          </div>
          <p v-if="!task.units?.length" class="text-xs text-gray-500">{{ label('暂未创建执行单元', 'No execution units yet') }}</p>
        </div>
      </article>
      <Pagination :total="total" :page="page" :page-size="pageSize" @update:page="changePage" @update:page-size="changePageSize" />
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import { mediaTasksAPI, type AdminMediaTask } from '@/api/admin/mediaTasks'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useMediaLabels } from '@/components/admin/media/mediaLabels'
const label=useMediaLabels()
const items=ref<AdminMediaTask[]>([])
const total=ref(0)
const page=ref(1)
const pageSize=ref(20)
const status=ref<'' | AdminMediaTask['status']>('')
const loading=ref(false)
const error=ref('')
const expanded=reactive(new Set<string>())
let revision=0
let active=true
async function load(){
 const current=++revision
 loading.value=true;error.value=''
 try{
  const result=await mediaTasksAPI.list({page:page.value,page_size:pageSize.value,status:status.value||undefined})
  if(active&&current===revision){items.value=result.items;total.value=result.total}
 }catch(cause){if(active&&current===revision)error.value=extractApiErrorMessage(cause,label('读取媒体任务失败，请刷新重试。','Failed to load media tasks. Refresh to retry.'))}
 finally{if(active&&current===revision)loading.value=false}
}
function filter(){page.value=1;void load()}
function changePage(value:number){page.value=value;void load()}
function changePageSize(value:number){pageSize.value=value;filter()}
function toggle(id:string){if(expanded.has(id))expanded.delete(id);else expanded.add(id)}
function money(value:number|string){const amount=Number(value);return Number.isFinite(amount)?`$${amount.toFixed(6)}`:'—'}
function timestamp(value:number){const date=new Date(value*1000);return Number.isNaN(date.getTime())?'—':date.toLocaleString()}
onMounted(()=>{void load()})
onBeforeUnmount(()=>{active=false;revision++})
</script>
