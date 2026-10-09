import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import AppHeader from '../AppHeader.vue'
import AppSidebar from '../AppSidebar.vue'
import HomeView from '@/views/HomeView.vue'
import KeyUsageView from '@/views/KeyUsageView.vue'

const { appStore, authStore, adminSettingsStore, route, router } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'GKotta',
    siteLogo: '',
    siteVersion: '1.0.0',
    docUrl: '',
    contactInfo: '',
    backendModeEnabled: false,
    publicSettingsLoaded: true,
    sidebarCollapsed: false,
    mobileOpen: false,
    sidebarScrollTop: 0,
    setMobileOpen: vi.fn(),
    fetchPublicSettings: vi.fn(),
  },
  authStore: {
    user: null,
    isAuthenticated: false,
    isAdmin: false,
    isSimpleMode: false,
    checkAuth: vi.fn(),
  },
  adminSettingsStore: {
    customMenuItems: [],
    opsMonitoringEnabled: false,
    paymentEnabled: false,
    fetch: vi.fn(),
  },
  route: { name: 'Dashboard', path: '/dashboard', params: {}, meta: {} },
  router: { push: vi.fn() },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
  useAdminSettingsStore: () => adminSettingsStore,
  useOnboardingStore: () => ({ isCurrentStep: () => false }),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => adminSettingsStore }))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({
    canUseBatchImage: { value: false },
    refreshBatchImageAccess: vi.fn().mockResolvedValue(false),
  }),
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRouter: () => router,
  useRoute: () => route,
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } }),
}))

const global = {
  stubs: {
    RouterLink: RouterLinkStub,
    VersionBadge: true,
    LocaleSwitcher: true,
    AnnouncementBell: true,
    SubscriptionProgressMini: true,
  },
}

describe('documentation entries', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = { doc_url: 'https://outdated.example.com/docs' }
    appStore.docUrl = 'https://outdated.example.com/docs'
    appStore.backendModeEnabled = false
    appStore.sidebarCollapsed = false
    appStore.mobileOpen = false
    appStore.setMobileOpen.mockClear()
    authStore.isAdmin = false
    authStore.isSimpleMode = false
    route.name = 'Dashboard'
    route.path = '/dashboard'
    localStorage.clear()
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it.each([
    ['home', HomeView, 'home-docs'],
    ['app header', AppHeader, 'header-docs'],
    ['key usage', KeyUsageView, 'key-usage-docs'],
  ] as const)('links the %s entry to the built-in docs', (_label, component, testId) => {
    const wrapper = mount(component, { global })
    const link = wrapper.get(`[data-testid="${testId}"]`)
    expect(link.element.tagName).toBe('A')
    expect(link.attributes('href')).toBe('/docs')
    expect(link.attributes('target')).toBeUndefined()
    expect(link.attributes('aria-label')).toBeTruthy()
    wrapper.unmount()
  })

  it('shows the compact home docs entry even without a configured document URL', () => {
    appStore.docUrl = ''
    appStore.cachedPublicSettings = { compact_home_enabled: true }
    const wrapper = mount(HomeView, { global })
    expect(wrapper.get('[data-testid="home-docs"]').attributes('href')).toBe('/docs')
    wrapper.unmount()
  })

  it.each([
    { isAdmin: false, isSimpleMode: false, backendMode: false },
    { isAdmin: false, isSimpleMode: true, backendMode: false },
    { isAdmin: true, isSimpleMode: true, backendMode: false },
    { isAdmin: false, isSimpleMode: false, backendMode: true },
    { isAdmin: true, isSimpleMode: false, backendMode: true },
  ])('shows sidebar documentation in mode %j', ({ isAdmin, isSimpleMode, backendMode }) => {
    authStore.isAdmin = isAdmin
    authStore.isSimpleMode = isSimpleMode
    appStore.backendModeEnabled = backendMode
    const wrapper = mount(AppSidebar, { global })
    expect(wrapper.get('[data-testid="sidebar-docs"]').attributes('href')).toBe('/docs')
    wrapper.unmount()
  })

  it('keeps the collapsed entry accessible', () => {
    appStore.sidebarCollapsed = true
    const wrapper = mount(AppSidebar, { global })
    const link = wrapper.get('[data-testid="sidebar-docs"]')
    expect(link.attributes('href')).toBe('/docs')
    expect(link.attributes('aria-label')).toBe('nav.docs')
    wrapper.unmount()
  })

  it('closes the mobile sidebar without intercepting browser navigation', () => {
    vi.useFakeTimers()
    appStore.mobileOpen = true
    const wrapper = mount(AppSidebar, { global })
    const click = new Event('click', { bubbles: true, cancelable: true })
    wrapper.get('[data-testid="sidebar-docs"]').element.dispatchEvent(click)
    expect(click.defaultPrevented).toBe(false)
    vi.advanceTimersByTime(150)
    expect(appStore.setMobileOpen).toHaveBeenCalledWith(false)
    wrapper.unmount()
  })
})
