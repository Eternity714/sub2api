import { apiClient } from '../client'
import type { MediaType } from '@/types/mediaCapability'
export interface AdminMediaUnit {
  index: number
  state: string
  account_id?: number
  platform?: string
  upstream_model?: string
  upstream_task_id?: string
  attempt: number
  count: number
  stored_count: number
  error?: string
}
export interface AdminMediaTask {
  id: string
  model: string
  media_type: MediaType
  user_id: number
  api_key_id: number
  group_id: number
  status: 'queued' | 'in_progress' | 'completed'
  progress: number
  created_at: number
  completed_at?: number
  expires_at?: number
  held_amount: number | string
  charged_amount: number | string
  error: string
  result_count: number
  units: AdminMediaUnit[]
}
export interface AdminMediaTaskList { items: AdminMediaTask[]; total: number; page: number; page_size: number }
export interface AdminMediaTaskParams { page: number; page_size: number; status?: AdminMediaTask['status'] }
export const mediaTasksAPI = {
  async list(params: AdminMediaTaskParams): Promise<AdminMediaTaskList> {
    return (await apiClient.get<AdminMediaTaskList>('/admin/media-tasks', { params })).data
  },
  async get(id: string): Promise<AdminMediaTask> {
    return (await apiClient.get<AdminMediaTask>(`/admin/media-tasks/${encodeURIComponent(id)}`)).data
  },
}
