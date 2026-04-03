import { defineConfig } from 'vitepress'
/**
 * npm add -D vitepress@next
 * npm i vitepress-mermaid-renderer
 * npm run docs:build
 * npm run docs:preview
 */
export default defineConfig({
  title: 'sample_repo',
  ignoreDeadLinks: true,
  themeConfig: {
    sidebar: [
      {
        text: 'Modules',
        items: [
          { text: 'api', link: '/modules/api.md' },
          { text: 'auth', link: '/modules/auth.md' },
          { text: 'cmd', link: '/modules/cmd.md' },
          { text: 'db', link: '/modules/db.md' },
        ],
      },
      {
        text: 'Shared',
        items: [
          { text: 'errors', link: '/shared/errors.md' },
          { text: 'logger', link: '/shared/logger.md' },
        ],
      },
    ],
  },
})