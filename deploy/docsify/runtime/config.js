/* Docsify's content base is distinct from its history route prefix. The edge
 * preserves /docs and all tutorial links explicitly use that namespace. */
window.$docsify = {
  name: 'Gkotta 文档',
  nameLink: '/docs/',
  logo: '/docs/logo.svg',
  routerMode: 'history',
  basePath: '/',
  homepage: '/docs/index.md',
  loadSidebar: '_sidebar.md',
  loadNavbar: '_navbar.md',
  alias: {
    '/docs/?': '/docs/index.md',
    '/docs/scenarios/?': '/docs/scenarios/index.md',
    '/docs/.*/_sidebar.md': '/docs/_sidebar.md',
    '/docs/.*/_navbar.md': '/docs/_navbar.md',
  },
  notFoundPage: '/docs/_404.md',
  auto2top: true,
  subMaxLevel: 3,
  mergeNavbar: true,
  noEmoji: true,
  executeScript: false,
  themeColor: '#0077c2',
  search: {
    paths: 'auto',
    maxAge: 1,
    placeholder: '搜索文档',
    noData: '没有找到相关结果',
    depth: 3,
    namespace: 'gkotta-docsify',
  },
  copyCode: {
    buttonText: '复制',
    errorText: '复制失败',
    successText: '已复制',
  },
  plugins: [function (hook) {
    hook.doneEach(function () {
      var diagrams = [];
      document.querySelectorAll('#main pre[data-lang="mermaid"]').forEach(function (block) {
        var source = block.querySelector('code');
        if (!source) return;
        var diagram = document.createElement('div');
        diagram.className = 'mermaid';
        diagram.textContent = source.textContent;
        block.replaceWith(diagram);
        diagrams.push(diagram);
      });
      if (diagrams.length) {
        mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: 'default' });
        mermaid.run({ nodes: diagrams }).catch(function (error) {
          console.error('文档图表渲染失败', error);
        });
      }
    });
  }],
};
