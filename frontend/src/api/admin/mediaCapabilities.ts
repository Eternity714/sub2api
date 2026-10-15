import { apiClient } from '../client'
import type {
  MediaCapability, MediaCapabilityInput, MediaCapabilityList, MediaCapabilityListParams,
  MediaCapabilityTemplate, MediaValidationRequest, MediaValidationResult,
} from '@/types/mediaCapability'

const base = '/admin/media-capabilities'
export const mediaCapabilitiesAPI = {
  async list(params: MediaCapabilityListParams = {}): Promise<MediaCapabilityList> {
    return (await apiClient.get<MediaCapabilityList>(base, { params })).data
  },
  async templates(): Promise<MediaCapabilityTemplate[]> {
    return (await apiClient.get<{ items: MediaCapabilityTemplate[] }>(`${base}/templates`)).data.items
  },
  async get(id: number): Promise<MediaCapability> {
    return (await apiClient.get<MediaCapability>(`${base}/${id}`)).data
  },
  async create(input: MediaCapabilityInput): Promise<MediaCapability> {
    return (await apiClient.post<MediaCapability>(base, input)).data
  },
  async update(id: number, input: MediaCapabilityInput & { revision: number }): Promise<MediaCapability> {
    return (await apiClient.put<MediaCapability>(`${base}/${id}`, input)).data
  },
  async delete(id: number, revision: number): Promise<void> {
    await apiClient.delete(`${base}/${id}`, { params: { revision } })
  },
  async validate(input: MediaValidationRequest): Promise<MediaValidationResult> {
    return (await apiClient.post<MediaValidationResult>(`${base}/validate`, input)).data
  },
}
