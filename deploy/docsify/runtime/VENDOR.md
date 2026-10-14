# 固定文档运行资源

资源全部随镜像提供，浏览器不请求外部 CDN。升级时一起更新入口文件、资源名、许可证和 `SHA256SUMS`。校验清单的路径相对于本目录。

| 本地文件 | 上游来源 | 许可证 |
| --- | --- | --- |
| `assets/docsify-4.13.1.min.js` | https://cdn.jsdelivr.net/npm/docsify@4.13.1/lib/docsify.min.js | MIT，`assets/docsify-LICENSE.txt` |
| `assets/docsify-4.13.1.css` | https://cdn.jsdelivr.net/npm/docsify@4.13.1/lib/themes/vue.css | MIT，`assets/docsify-LICENSE.txt` |
| `assets/search-4.13.1.min.js` | https://cdn.jsdelivr.net/npm/docsify@4.13.1/lib/plugins/search.min.js | MIT，`assets/docsify-LICENSE.txt` |
| `assets/copy-code-3.0.2.min.js` | https://cdn.jsdelivr.net/npm/docsify-copy-code@3.0.2/dist/docsify-copy-code.min.js | MIT，`assets/copy-code-LICENSE.txt` |
| `assets/mermaid-11.12.0.min.js` | https://cdn.jsdelivr.net/npm/mermaid@11.12.0/dist/mermaid.min.js | MIT，`assets/mermaid-LICENSE.txt` |
| `assets/prism-<language>-1.30.0.min.js` | https://cdn.jsdelivr.net/npm/prismjs@1.30.0/components/prism-<language>.min.js | MIT，`assets/prism-LICENSE.txt` |

Prism language 为 `bash`、`http`、`json`、`powershell`、`python`、`toml`、`yaml`。

Docsify CSS 删除了上游的 Google Fonts `@import`，字体使用系统字体。这是该文件唯一的本地样式修改；站点定制样式放在 `style.css`。清单记录修改后的实际文件 SHA256。

许可证原文来自对应 npm 包的 `LICENSE`（Prism 为 `LICENSE`）。在 Docker Compose 测试容器中可执行 `cd deploy/docsify/runtime && sha256sum -c SHA256SUMS` 验证固定资源。
