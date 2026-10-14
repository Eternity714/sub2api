<template>
  <section class="card p-6" :data-testid="`media-upload-storage-${kind}`" :aria-labelledby="`${id}-title`">
    <div class="mb-4">
      <h3 :id="`${id}-title`" class="text-base font-semibold text-gray-900 dark:text-white">
        {{ t(`admin.backup.mediaUploadStorage.${kind}.title`) }}
      </h3>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.backup.mediaUploadStorage.description') }}</p>
      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
        <code>POST /v1/uploads/{{ kind }}</code>
        <span class="ml-2">{{ t(`admin.backup.mediaUploadStorage.${kind}.formats`) }}</span>
      </p>
    </div>

    <p v-if="loading" class="mb-3 text-sm text-gray-500" role="status">{{ t('common.loading') }}</p>
    <div v-if="loadError" class="mb-3 flex flex-wrap items-center gap-2" role="alert">
      <span class="text-sm text-red-600 dark:text-red-400">{{ t('admin.backup.mediaUploadStorage.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ t('admin.backup.mediaUploadStorage.retry') }}</button>
    </div>

    <fieldset :disabled="busy || !loaded" class="space-y-3">
      <div class="flex flex-wrap gap-x-6 gap-y-3">
        <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="form.enabled" type="checkbox" data-field="enabled" />
          <span>{{ t('admin.backup.mediaUploadStorage.enabled') }}</span>
        </label>
        <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="form.reuse_backup_s3" type="checkbox" data-field="reuse-backup-s3" />
          <span>{{ t('admin.backup.mediaUploadStorage.reuseBackupS3') }}</span>
        </label>
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.backup.mediaUploadStorage.reuseHint') }}</p>

      <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
        <div>
          <label :for="`${id}-bucket`" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.backup.s3.bucket') }}</label>
          <input :id="`${id}-bucket`" v-model="form.bucket" data-field="bucket" class="input w-full" :placeholder="form.reuse_backup_s3 ? t('admin.backup.imageStorage.bucketInherited') : ''" />
        </div>
        <div>
          <label :for="`${id}-prefix`" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.backup.s3.prefix') }}</label>
          <input :id="`${id}-prefix`" v-model="form.prefix" data-field="prefix" class="input w-full" :placeholder="`uploads/${kind}/`" />
        </div>
        <template v-if="!form.reuse_backup_s3">
          <div>
            <label :for="`${id}-endpoint`" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.backup.s3.endpoint') }}</label>
            <input :id="`${id}-endpoint`" v-model="form.endpoint" data-field="endpoint" class="input w-full" placeholder="https://s3.example.com" />
          </div>
          <div>
            <label :for="`${id}-region`" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.backup.s3.region') }}</label>
            <input :id="`${id}-region`" v-model="form.region" class="input w-full" placeholder="auto" />
          </div>
          <div>
            <label :for="`${id}-access-key`" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.backup.s3.accessKeyId') }}</label>
            <input :id="`${id}-access-key`" v-model="form.access_key_id" class="input w-full" autocomplete="off" />
          </div>
          <div>
            <label :for="`${id}-secret`" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.backup.s3.secretAccessKey') }}</label>
            <input :id="`${id}-secret`" v-model="form.secret_access_key" data-field="secret-access-key" type="password" class="input w-full" autocomplete="new-password" :placeholder="secretConfigured ? t('admin.backup.s3.secretConfigured') : ''" />
          </div>
          <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 md:col-span-2">
            <input v-model="form.force_path_style" type="checkbox" />
            <span>{{ t('admin.backup.s3.forcePathStyle') }}</span>
          </label>
        </template>
        <div>
          <label :for="`${id}-public-url`" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.backup.imageStorage.publicBaseUrl') }}</label>
          <input :id="`${id}-public-url`" v-model="form.public_base_url" class="input w-full" :placeholder="t('admin.backup.imageStorage.publicBaseUrlPlaceholder')" />
        </div>
        <div>
          <label :for="`${id}-expiry`" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.backup.imageStorage.presignExpiryHours') }}</label>
          <input :id="`${id}-expiry`" v-model.number="form.presign_expiry_hours" data-field="presign-expiry-hours" type="number" min="1" max="168" step="1" class="input w-full" />
        </div>
        <div>
          <label :for="`${id}-max-size`" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.backup.mediaUploadStorage.maxUploadMB') }}</label>
          <input :id="`${id}-max-size`" v-model.number="maxUploadMB" data-field="max-upload-mb" type="number" min="1" max="1024" step="1" class="input w-full" :aria-invalid="!!sizeInvalid" :aria-describedby="`${id}-size-hint`" />
          <p :id="`${id}-size-hint`" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.backup.mediaUploadStorage.sizeHint') }}</p>
        </div>
      </div>
      <p v-if="validationError" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ validationError }}</p>
      <div class="flex flex-wrap gap-2 pt-1">
        <button type="button" data-action="test" class="btn btn-secondary btn-sm" :disabled="busy || !loaded || !!validationError" @click="testConnection">
          {{ testing ? t('common.loading') : t('admin.backup.s3.testConnection') }}
        </button>
        <button type="button" data-action="save" class="btn btn-primary btn-sm" :disabled="busy || !loaded || !!validationError" @click="save">
          {{ saving ? t('common.loading') : t('common.save') }}
        </button>
      </div>
    </fieldset>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import type { MediaUploadKind, MediaUploadStorageConfig } from '@/api/admin/backup'
import { useAppStore } from '@/stores'
import { isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import type { StepUpController } from '@/composables/useStepUp'

const props = defineProps<{ kind: MediaUploadKind; runStepUp: StepUpController['run'] }>()
const { t } = useI18n()
const appStore = useAppStore()
const id = `media-upload-${props.kind}`
const mib = 1024 * 1024
const form = ref<MediaUploadStorageConfig>({
  enabled: false,
  reuse_backup_s3: false,
  endpoint: '',
  region: 'auto',
  bucket: '',
  access_key_id: '',
  secret_access_key: '',
  force_path_style: false,
  prefix: `uploads/${props.kind}/`,
  public_base_url: '',
  presign_expiry_hours: 24,
  max_upload_bytes: (props.kind === 'images' ? 10 : 50) * mib,
})
const maxUploadMB = ref<number | string>(form.value.max_upload_bytes / mib)
const secretConfigured = ref(false)
const loading = ref(true)
const loaded = ref(false)
const loadError = ref(false)
const saving = ref(false)
const testing = ref(false)
const busy = computed(() => loading.value || saving.value || testing.value)
const sizeInvalid = computed(() => !Number.isSafeInteger(maxUploadMB.value) || Number(maxUploadMB.value) < 1 || Number(maxUploadMB.value) > 1024)
const validationError = computed(() => {
  if (sizeInvalid.value) return t('admin.backup.mediaUploadStorage.invalidSize')
  const expiry = form.value.presign_expiry_hours
  if (!Number.isSafeInteger(expiry) || expiry < 1 || expiry > 168) return t('admin.backup.mediaUploadStorage.invalidExpiry')
  return ''
})

function applyConfig(config: MediaUploadStorageConfig) {
  form.value = { ...form.value, ...config, secret_access_key: '' }
  maxUploadMB.value = form.value.max_upload_bytes / mib
}

function payload(): MediaUploadStorageConfig {
  return { ...form.value, max_upload_bytes: Number(maxUploadMB.value) * mib }
}

function showError(error: unknown) {
  if (isStepUpCancelled(error)) return
  if (isStepUpBlocked(error)) {
    appStore.showError(t(stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN' ? 'stepUp.adminApiKeyForbidden' : 'stepUp.notEnabled'))
    return
  }
  appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
}

async function load() {
  loading.value = true
  loaded.value = false
  loadError.value = false
  try {
    const result = await adminAPI.backup.getMediaUploadStorageConfig(props.kind)
    applyConfig(result.config)
    secretConfigured.value = result.secret_configured
    loaded.value = true
  } catch (error) {
    loadError.value = true
    showError(error)
  } finally {
    loading.value = false
  }
}

async function save() {
  if (busy.value || !loaded.value || validationError.value) return
  const config = payload()
  saving.value = true
  try {
    const saved = await props.runStepUp(() => adminAPI.backup.updateMediaUploadStorageConfig(props.kind, config))
    applyConfig(saved)
    appStore.showSuccess(t('admin.backup.mediaUploadStorage.saved'))
    await load()
  } catch (error) {
    showError(error)
  } finally {
    saving.value = false
  }
}

async function testConnection() {
  if (busy.value || !loaded.value || validationError.value) return
  testing.value = true
  try {
    const result = await adminAPI.backup.testMediaUploadStorageConnection(props.kind, payload())
    if (result.ok) appStore.showSuccess(result.message || t('admin.backup.s3.testSuccess'))
    else appStore.showError(result.message || t('admin.backup.s3.testFailed'))
  } catch (error) {
    showError(error)
  } finally {
    testing.value = false
  }
}

onMounted(load)
</script>
