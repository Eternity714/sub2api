export type MediaType = 'image' | 'video'
export type MediaFieldType = 'string' | 'integer' | 'number' | 'boolean' | 'array' | 'object'
export interface MediaCapabilityField {
  type: MediaFieldType
  required?: boolean
  nullable?: boolean
  enum?: unknown[]
  minimum?: number
  maximum?: number
  min_length?: number
  max_length?: number
  min_items?: number
  max_items?: number
  default?: unknown
  properties?: Record<string, MediaCapabilityField>
  items?: MediaCapabilityField
  format?: 'http_url'
}
export interface MediaCondition {
  path: string
  op: 'present' | 'absent' | 'empty' | 'not_empty' | 'equals'
  value?: unknown
}
export interface MediaInputMode { id: string; when: MediaCondition[]; required: string[]; forbidden: string[] }
export interface MediaCapabilityRule {
  kind: 'at_least_one' | 'mutually_exclusive' | 'require_together' | 'requires'
  paths: string[]
  when?: MediaCondition[]
}
export interface MediaImageSpec { aspect_ratio: string; resolution: string; width: number; height: number }
export interface MediaCapabilitySchema {
  version: number
  fields: Record<string, MediaCapabilityField>
  modes?: MediaInputMode[]
  rules?: MediaCapabilityRule[]
  image_specs?: MediaImageSpec[]
  batch?: { native_max_results: number; allow_split: boolean; semantically_linked: boolean }
}
export interface MediaCapabilityInput {
  public_model: string
  media_type: MediaType
  schema: MediaCapabilitySchema
  enabled: boolean
  template_id?: string
  template_version?: number
  notes?: string
}
export interface MediaCapability extends MediaCapabilityInput {
  id: number
  schema_version: number
  revision: number
  created_at: string
  updated_at: string
}
export interface MediaCapabilityTemplate {
  public_model?: string
  source?: string
  evidence?: string
  notes?: string
  example?: Record<string, unknown>
  enabled?: false
  id: string
  version: number
  name: string
  media_type: MediaType
  schema: MediaCapabilitySchema
}
export interface MediaCapabilityList {
  items: MediaCapability[]
  total: number
  page: number
  page_size: number
}
export interface MediaCapabilityListParams {
  page?: number
  page_size?: number
  search?: string
  media_type?: MediaType
  enabled?: boolean
}
export interface MediaValidationError { path: string; code: string; message: string }
export interface MediaValidationRequest {
  public_model: string
  media_type: MediaType
  schema: MediaCapabilitySchema
  request_json: string
}
export interface MediaValidationResult {
  valid: boolean
  errors: MediaValidationError[]
  normalized?: Record<string, unknown>
}
