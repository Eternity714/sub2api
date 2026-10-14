import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import BackupView from '../BackupView.vue'

const {
  getS3Config,
  getImageStorageConfig,
  getMediaUploadStorageConfig,
  updateMediaUploadStorageConfig,
  testMediaUploadStorageConnection,
  stepUpRun,
  getSchedule,
  updateSchedule,
  deleteBackup,
  listBackups,
  getDownloadURL,
} = vi.hoisted(() => ({
  getS3Config: vi.fn(),
  getImageStorageConfig: vi.fn(),
  getMediaUploadStorageConfig: vi.fn(),
  updateMediaUploadStorageConfig: vi.fn(),
  testMediaUploadStorageConnection: vi.fn(),
  stepUpRun: vi.fn((fn: () => unknown) => fn()),
  getSchedule: vi.fn(),
  updateSchedule: vi.fn(),
  deleteBackup: vi.fn(),
  listBackups: vi.fn(),
  getDownloadURL: vi.fn(),
}))

vi.mock('@/api', () => ({
  adminAPI: {
    backup: {
      getS3Config,
      updateS3Config: vi.fn(),
      testS3Connection: vi.fn(),
      getImageStorageConfig,
      updateImageStorageConfig: vi.fn(),
      testImageStorageConnection: vi.fn(),
      getMediaUploadStorageConfig,
      updateMediaUploadStorageConfig,
      testMediaUploadStorageConnection,
      getSchedule,
      updateSchedule,
      createBackup: vi.fn(),
      listBackups,
      getBackup: vi.fn(),
      deleteBackup,
      getDownloadURL,
      restoreBackup: vi.fn(),
    },
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn(),
  }),
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ run: stepUpRun }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => '',
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params?.index !== undefined ? `${key}:${params.index}` : params?.day !== undefined ? `${key}:${params.day}` : key,
  }),
}))

const baseRecord = (id: string, parts?: unknown[]) => ({
  id,
  status: 'completed',
  backup_type: 'postgres',
  file_name: `${id}.sql.gz`,
  s3_key: `backups/${id}.sql.gz`,
  parts,
  size_bytes: 10,
  triggered_by: 'manual',
  started_at: '2026-08-09T00:00:00Z',
})

const wrappers: ReturnType<typeof mount>[] = []

function mountBackupView() {
  const wrapper = mount(BackupView, {
    global: {
      stubs: {
        TotpStepUpDialog: true,
      },
    },
  })
  wrappers.push(wrapper)
  return wrapper
}

describe('admin BackupView', () => {
  beforeEach(() => {
    getS3Config.mockResolvedValue({})
    getImageStorageConfig.mockResolvedValue({ config: {}, secret_configured: false })
    const mediaConfigs = new Map<string, { config: object; secret_configured: boolean }>()
    getMediaUploadStorageConfig.mockReset().mockImplementation((kind: string) => Promise.resolve(mediaConfigs.get(kind) || { config: {
      enabled: false, reuse_backup_s3: false, endpoint: '', region: 'auto', bucket: '',
      access_key_id: '', force_path_style: false, prefix: `uploads/${kind}/`,
      public_base_url: '', presign_expiry_hours: 24,
      max_upload_bytes: kind === 'images' ? 10485760 : 52428800,
    }, secret_configured: false }))
    updateMediaUploadStorageConfig.mockReset().mockImplementation((kind: string, config: Record<string, unknown>) => {
      const { secret_access_key, ...publicConfig } = config
      mediaConfigs.set(kind, { config: publicConfig, secret_configured: Boolean(secret_access_key) || !!mediaConfigs.get(kind)?.secret_configured })
      return Promise.resolve(publicConfig)
    })
    testMediaUploadStorageConnection.mockReset().mockResolvedValue({ ok: true, message: 'connected' })
    stepUpRun.mockClear()
    getSchedule.mockResolvedValue({ enabled: false, cron_expr: '', retain_days: 14, retain_count: 10 })
    updateSchedule.mockReset().mockResolvedValue({})
    deleteBackup.mockReset().mockResolvedValue(undefined)
    listBackups.mockResolvedValue({ items: [] })
    getDownloadURL.mockReset()
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
  })

  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('三类上传存储独立配置，只有打开复用开关才隐藏该类 S3 凭证', async () => {
    const wrapper = mountBackupView()
    await flushPromises()
    for (const kind of ['images', 'videos', 'audios']) {
      const card = wrapper.get(`[data-testid="media-upload-storage-${kind}"]`)
      expect((card.get('[data-field="reuse-backup-s3"]').element as HTMLInputElement).checked).toBe(false)
      expect(card.find('[data-field="endpoint"]').exists()).toBe(true)
    }
    const images = wrapper.get('[data-testid="media-upload-storage-images"]')
    await images.get('[data-field="reuse-backup-s3"]').setValue(true)
    expect(images.find('[data-field="endpoint"]').exists()).toBe(false)
    expect(images.find('[data-field="secret-access-key"]').exists()).toBe(false)
    expect(images.find('[data-field="bucket"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="media-upload-storage-videos"]').find('[data-field="endpoint"]').exists()).toBe(true)
  })

  it('上传上限按 MB 编辑并按字节独立保存，通过现有二次认证保护', async () => {
    const wrapper = mountBackupView()
    await flushPromises()
    const images = wrapper.get('[data-testid="media-upload-storage-images"]')
    const videos = wrapper.get('[data-testid="media-upload-storage-videos"]')
    expect((images.get('[data-field="max-upload-mb"]').element as HTMLInputElement).value).toBe('10')
    expect((videos.get('[data-field="max-upload-mb"]').element as HTMLInputElement).value).toBe('50')
    await videos.get('[data-field="max-upload-mb"]').setValue(75)
    await videos.get('[data-field="prefix"]').setValue('custom/videos/')
    await videos.get('[data-action="save"]').trigger('click')
    await flushPromises()
    expect(stepUpRun).toHaveBeenCalledTimes(1)
    expect(updateMediaUploadStorageConfig).toHaveBeenCalledTimes(1)
    expect(updateMediaUploadStorageConfig).toHaveBeenCalledWith('videos', expect.objectContaining({
      max_upload_bytes: 78643200, prefix: 'custom/videos/', reuse_backup_s3: false,
    }))
    expect((images.get('[data-field="max-upload-mb"]').element as HTMLInputElement).value).toBe('10')
    expect((videos.get('[data-field="max-upload-mb"]').element as HTMLInputElement).value).toBe('75')
  })

  it('非法文件上限和链接有效期阻止保存与连接测试', async () => {
    const wrapper = mountBackupView()
    await flushPromises()
    const card = wrapper.get('[data-testid="media-upload-storage-audios"]')
    for (const value of ['0', '-1', '1025', '1.5', '']) {
      await card.get('[data-field="max-upload-mb"]').setValue(value)
      expect(card.get('[data-action="save"]').attributes('disabled')).toBeDefined()
      expect(card.get('[data-action="test"]').attributes('disabled')).toBeDefined()
      expect(card.find('[role="alert"]').exists()).toBe(true)
    }
    await card.get('[data-field="max-upload-mb"]').setValue(1024)
    await card.get('[data-field="presign-expiry-hours"]').setValue(169)
    expect(card.get('[data-action="save"]').attributes('disabled')).toBeDefined()
    await card.get('[data-field="presign-expiry-hours"]').setValue(24)
    expect(card.get('[data-action="save"]').attributes('disabled')).toBeUndefined()
    expect(updateMediaUploadStorageConfig).not.toHaveBeenCalled()
    expect(testMediaUploadStorageConnection).not.toHaveBeenCalled()
  })

  it('已保存的密钥只提示已配置，更新后清空密钥输入', async () => {
    getMediaUploadStorageConfig.mockResolvedValue({ config: {
      enabled: true, reuse_backup_s3: false, endpoint: 'https://s3.example.test', region: 'auto',
      bucket: 'media', access_key_id: 'key', force_path_style: false, prefix: 'uploads/',
      public_base_url: '', presign_expiry_hours: 24, max_upload_bytes: 10485760,
    }, secret_configured: true })
    const wrapper = mountBackupView()
    await flushPromises()
    const card = wrapper.get('[data-testid="media-upload-storage-images"]')
    const secret = card.get('[data-field="secret-access-key"]')
    expect((secret.element as HTMLInputElement).value).toBe('')
    expect(secret.attributes('placeholder')).toBe('admin.backup.s3.secretConfigured')
    await secret.setValue('new-secret')
    await card.get('[data-action="save"]').trigger('click')
    await flushPromises()
    expect(updateMediaUploadStorageConfig).toHaveBeenCalledWith('images', expect.objectContaining({ secret_access_key: 'new-secret' }))
    expect((secret.element as HTMLInputElement).value).toBe('')
  })

  it('保存后重新读取独立密钥状态，取消复用不会错误提示已配置', async () => {
    const config = {
      enabled: false, reuse_backup_s3: false, endpoint: 'https://s3.example.test', region: 'auto',
      bucket: 'media', access_key_id: 'key', force_path_style: false, prefix: 'uploads/images/',
      public_base_url: '', presign_expiry_hours: 24, max_upload_bytes: 10485760,
    }
    getMediaUploadStorageConfig.mockResolvedValue({ config, secret_configured: true })
    const wrapper = mountBackupView()
    await flushPromises()
    const card = wrapper.get('[data-testid="media-upload-storage-images"]')
    expect(card.get('[data-field="secret-access-key"]').attributes('placeholder')).toBe('admin.backup.s3.secretConfigured')
    await card.get('[data-field="reuse-backup-s3"]').setValue(true)
    getMediaUploadStorageConfig.mockResolvedValue({ config: { ...config, reuse_backup_s3: true }, secret_configured: false })
    await card.get('[data-action="save"]').trigger('click')
    await flushPromises()
    await card.get('[data-field="reuse-backup-s3"]').setValue(false)
    expect(card.get('[data-field="secret-access-key"]').attributes('placeholder')).toBe('')
  })

  it('连接测试使用当前类别的未保存配置，并保留独立的其他表单', async () => {
    const wrapper = mountBackupView()
    await flushPromises()
    const card = wrapper.get('[data-testid="media-upload-storage-audios"]')
    await card.get('[data-field="bucket"]').setValue('audio-files')
    await card.get('[data-field="max-upload-mb"]').setValue(100)
    await card.get('[data-action="test"]').trigger('click')
    await flushPromises()
    expect(testMediaUploadStorageConnection).toHaveBeenCalledTimes(1)
    expect(testMediaUploadStorageConnection).toHaveBeenCalledWith('audios', expect.objectContaining({ bucket: 'audio-files', max_upload_bytes: 104857600 }))
    expect(updateMediaUploadStorageConfig).not.toHaveBeenCalled()
    expect((wrapper.get('[data-testid="media-upload-storage-images"] [data-field="bucket"]').element as HTMLInputElement).value).toBe('')
  })

  it.each(['backup', 'restore'])('does not resume %s polling after navigation during initial loading', async (operation) => {
    vi.useFakeTimers()
    let finish!: (value: object) => void
    listBackups.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const wrapper = mountBackupView()
    try {
      await flushPromises()
      wrapper.unmount()
      finish({ items: [{ ...baseRecord('pending'),
        status: operation === 'backup' ? 'running' : 'completed',
        restore_status: operation === 'restore' ? 'running' : undefined,
      }] })
      await flushPromises()
      expect(vi.getTimerCount()).toBe(0)
    } finally {
      wrapper.unmount()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('显示分卷数并在下载时列出每个分卷链接', async () => {
    listBackups.mockResolvedValue({
      items: [baseRecord('split', [{ index: 1 }, { index: 2 }, { index: 3 }])],
    })
    getDownloadURL.mockResolvedValue({
      parts: [
        { index: 1, size_bytes: 5, url: 'https://example.test/part-1' },
        { index: 2, size_bytes: 6, url: 'https://example.test/part-2' },
        { index: 3, size_bytes: 7, url: 'https://example.test/part-3' },
      ],
    })

    const wrapper = mountBackupView()
    await flushPromises()

    expect(wrapper.text()).toContain('3')
    const downloadButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.backup.actions.download'),
    )
    expect(downloadButton).toBeDefined()
    await downloadButton!.trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('admin.backup.actions.partLabel:1')
    expect(document.body.textContent).toContain('admin.backup.actions.partLabel:3')
    expect(document.body.querySelector('a[href="https://example.test/part-2"]')).not.toBeNull()
  })

  it('旧单文件记录仍使用单个下载地址', async () => {
    listBackups.mockResolvedValue({ items: [baseRecord('legacy')] })
    getDownloadURL.mockResolvedValue({ url: 'https://example.test/legacy.sql.gz' })

    const wrapper = mountBackupView()
    await flushPromises()
    const downloadButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.backup.actions.download'),
    )
    await downloadButton!.trigger('click')
    await flushPromises()

    expect(getDownloadURL).toHaveBeenCalledWith('legacy')
    expect(document.body.textContent).not.toContain('admin.backup.actions.downloadParts')
  })

  it('运行中的备份不显示删除入口', async () => {
    listBackups.mockResolvedValue({
      items: [{ ...baseRecord('running'), status: 'running', progress: 'uploading' }],
    })

    const wrapper = mountBackupView()
    await flushPromises()

    expect(wrapper.find('tbody tr td:nth-child(5)').text()).toBe('-')
    expect(wrapper.findAll('button').some(button => button.text() === 'common.delete')).toBe(false)
  })

  it('兼容旧配置并保留 0 值，不自动启用月度归档', async () => {
    getSchedule.mockResolvedValue({ enabled: true, cron_expr: '0 4 * * *', retain_days: 0, retain_count: 0 })
    const wrapper = mountBackupView()
    await flushPromises()
    expect((wrapper.get('[data-testid="backup-retain-days"]').element as HTMLInputElement).value).toBe('0')
    expect((wrapper.get('[data-testid="backup-retain-count"]').element as HTMLInputElement).value).toBe('0')
    expect((wrapper.get('[data-testid="archive-enabled"]').element as HTMLInputElement).checked).toBe(false)
    await wrapper.get('[data-testid="backup-schedule"] .btn-primary').trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenCalledWith(expect.objectContaining({ retain_days: 0, retain_count: 0, monthly_archive: expect.objectContaining({ enabled: false }) }))
  })

  it('多选日期，手填份数；永久保留隐藏数量并在取消后恢复', async () => {
    const wrapper = mountBackupView()
    await flushPromises()
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    await wrapper.get('#backup-archive-dates').trigger('click')
    await wrapper.get('#backup-archive-options input[value="15"]').setValue(true)
    await wrapper.get('#backup-archive-options input[value="last"]').setValue(true)
    expect(wrapper.get('#backup-archive-dates').attributes('aria-expanded')).toBe('true')
    expect(wrapper.find('[data-testid="archive-count"]').exists()).toBe(false)
    await wrapper.get('[data-testid="archive-forever"]').setValue(false)
    await wrapper.get('[data-testid="archive-count"]').setValue(17)
    await wrapper.get('[data-testid="archive-forever"]').setValue(true)
    expect(wrapper.find('[data-testid="archive-count"]').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'BackupArchiveSettings' }).text()).not.toContain('admin.backup.archive.copies')
    await wrapper.get('[data-testid="archive-forever"]').setValue(false)
    expect((wrapper.get('[data-testid="archive-count"]').element as HTMLInputElement).value).toBe('17')
    await wrapper.get('[data-testid="backup-schedule"] .btn-primary').trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: true, days: [1, 15], include_month_end: true, retain_count: 17 } }))
    await wrapper.get('[data-testid="archive-forever"]').setValue(true)
    await wrapper.get('[data-testid="backup-schedule"] .btn-primary').trigger('click')
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: expect.objectContaining({ retain_count: 0 }) }))
  })

  it('空日期和非法份数不可保存，输入 0 不会误选永久保留', async () => {
    const wrapper = mountBackupView()
    await flushPromises()
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    await wrapper.get('#backup-archive-dates').trigger('click')
    await wrapper.get('#backup-archive-options input[value="1"]').setValue(false)
    const save = wrapper.get('[data-testid="backup-schedule"] .btn-primary')
    expect(save.attributes('disabled')).toBeDefined()
    await wrapper.get('#backup-archive-options input[value="15"]').setValue(true)
    await wrapper.get('[data-testid="archive-forever"]').setValue(false)
    for (const value of ['0', '-1', '1.5', '']) {
      await wrapper.get('[data-testid="archive-count"]').setValue(value)
      expect(save.attributes('disabled')).toBeDefined()
      expect((wrapper.get('[data-testid="archive-forever"]').element as HTMLInputElement).checked).toBe(false)
    }
    await save.trigger('click')
    expect(updateSchedule).not.toHaveBeenCalled()
  })

  it('关闭月度归档后不提交隐藏的归档参数，重新启用时保留编辑值', async () => {
    getSchedule.mockResolvedValue({ enabled: true, cron_expr: '0 4 * * *', retain_days: 14, retain_count: 10,
      monthly_archive: { enabled: true, days: [1, 15], include_month_end: false, retain_count: 12 } })
    const wrapper = mountBackupView()
    await flushPromises()
    await wrapper.get('[data-testid="archive-count"]').setValue(1)
    await wrapper.get('[data-testid="archive-enabled"]').setValue(false)
    expect(wrapper.find('[data-testid="archive-count"]').exists()).toBe(false)
    const save = wrapper.get('[data-testid="backup-schedule"] .btn-primary')
    await save.trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: false, days: [1, 15], include_month_end: false, retain_count: 12 } }))
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    expect((wrapper.get('[data-testid="archive-count"]').element as HTMLInputElement).value).toBe('1')
    // A hidden invalid count neither blocks saving nor reaches the request.
    await wrapper.get('[data-testid="archive-count"]').setValue('')
    expect(save.attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="archive-enabled"]').setValue(false)
    expect(save.attributes('disabled')).toBeUndefined()
    await save.trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: false, days: [1, 15], include_month_end: false, retain_count: 12 } }))
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    await wrapper.get('[data-testid="archive-count"]').setValue(1)
    await save.trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: true, days: [1, 15], include_month_end: false, retain_count: 1 } }))
  })

  it('加载归档设置并支持键盘关闭日期选择器', async () => {
    getSchedule.mockResolvedValue({ enabled: true, cron_expr: '0 4 * * *', retain_days: 14, retain_count: 10,
      monthly_archive: { enabled: true, days: [1, 15], include_month_end: true, retain_count: 23 } })
    const wrapper = mountBackupView()
    await flushPromises()
    expect((wrapper.get('[data-testid="archive-count"]').element as HTMLInputElement).value).toBe('23')
    await wrapper.get('#backup-archive-dates').trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()
    expect(wrapper.find('#backup-archive-options').exists()).toBe(true)
    expect((wrapper.get('#backup-archive-options input[value="15"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('#backup-archive-options').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('#backup-archive-options').exists()).toBe(false)
  })

  it('归档删除需要单独确认，有限归档不会显示永不过期', async () => {
    listBackups.mockResolvedValue({ items: [{ ...baseRecord('archived'), monthly_archive: { dates: ['2026-09-01', '2026-09-15'], retain_count: 12 } }] })
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const wrapper = mountBackupView()
    await flushPromises()
    expect(wrapper.text()).toContain('admin.backup.archive.badge')
    expect(wrapper.get('tbody tr td:nth-child(6)').text()).toBe('admin.backup.archive.retainLatest')
    const button = wrapper.findAll('button').find(button => button.text() === 'common.delete')!
    await button.trigger('click')
    expect(confirm).toHaveBeenCalledWith('admin.backup.archive.deleteConfirm')
    expect(deleteBackup).not.toHaveBeenCalled()
    confirm.mockReturnValue(true)
    await button.trigger('click')
    await flushPromises()
    expect(deleteBackup).toHaveBeenCalledWith('archived', true)
  })
})
