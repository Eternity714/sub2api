import { defineConfig } from 'vitepress'

export default defineConfig({
  lang: 'zh-CN',
  title: 'Gkotta 文档',
  description: 'Gkotta API 接入指南、客户端配置、计费与常见问题。',
  base: '/docs/',
  cleanUrls: true,
  outDir: '../../backend/internal/web/dist/docs',
  srcExclude: ['README.md'],
  lastUpdated: false,
  head: [['link', { rel: 'icon', href: '/docs/logo.svg' }]],
  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'Gkotta 文档',
    nav: [
      { text: '文档', link: '/' },
      { text: '主站', link: 'https://www.gkotta.bid', target: '_self' },
      { text: '控制台', link: 'https://www.gkotta.bid/login', target: '_self' },
    ],
    sidebar: [
      { text: '文档首页', link: '/' },
      { text: '开始使用', items: [
        { text: '快速开始', link: '/quick-start' },
        { text: 'API 密钥与分组', link: '/api-keys' },
      ] },
      { text: '客户端接入', items: [
        { text: 'Claude Code', link: '/claude-code' },
        { text: 'Codex', link: '/codex' },
        { text: '常用客户端', link: '/clients' },
      ] },
      { text: 'API 参考', items: [
        { text: 'OpenAI 兼容 API', link: '/openai-api' },
        { text: 'Anthropic API', link: '/anthropic-api' },
        { text: 'Gemini API', link: '/gemini-api' },
      ] },
      { text: '账户与支持', items: [
        { text: '计费与用量', link: '/billing' },
        { text: '常见问题与排错', link: '/troubleshooting' },
      ] },
    ],
    outline: { level: [2, 3], label: '在此页面' },
    docFooter: { prev: '上一篇', next: '下一篇' },
    sidebarMenuLabel: '文档目录',
    returnToTopLabel: '返回顶部',
    darkModeSwitchLabel: '主题',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
    notFound: {
      title: '未找到这篇文档',
      quote: '链接可能已变更，请从目录或搜索中查找需要的内容。',
      linkLabel: '返回文档首页',
      linkText: '返回文档首页',
    },
    search: {
      provider: 'local',
      options: {
        translations: {
          button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
          modal: {
            displayDetails: '显示详细信息',
            resetButtonTitle: '清除搜索',
            backButtonTitle: '返回',
            noResultsText: '没有找到相关结果',
            footer: { selectText: '选择', navigateText: '切换', closeText: '关闭' },
          },
        },
      },
    },
    footer: { message: '模型、价格与可用性以控制台实时信息为准。', copyright: 'Gkotta 使用文档' },
  },
})
