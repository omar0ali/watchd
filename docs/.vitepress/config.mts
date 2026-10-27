import { defineConfig } from 'vitepress'

const repo = 'https://github.com/omar0ali/watchd'

export default defineConfig({
  title: 'watchd',
  description: 'Run, watch and supervise every process of your projects from one Lua config.',
  base: '/watchd/',
  cleanUrls: true,
  lastUpdated: true,
  head: [['link', { rel: 'icon', type: 'image/svg+xml', href: '/watchd/logo.svg' }]],

  themeConfig: {
    logo: '/logo.svg',

    nav: [
      { text: 'Guide', link: '/guide/introduction', activeMatch: '/guide/' },
      { text: 'Configuration', link: '/config/', activeMatch: '/config/' },
      { text: 'Reference', link: '/reference/cli', activeMatch: '/reference/' },
      {
        text: 'Links',
        items: [
          { text: 'Releases', link: `${repo}/releases` },
          { text: 'Report an issue', link: `${repo}/issues` },
          { text: 'Contributing', link: '/contributing' },
        ],
      },
    ],

    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Introduction', link: '/guide/introduction' },
          { text: 'Installation', link: '/guide/installation' },
          { text: 'Getting started', link: '/guide/getting-started' },
          { text: 'Working with many projects', link: '/guide/projects' },
          { text: 'The dashboard', link: '/guide/dashboard' },
          { text: 'Recipes', link: '/guide/recipes' },
        ],
      },
      {
        text: 'Configuration',
        items: [
          { text: 'Overview', link: '/config/' },
          { text: 'Apps', link: '/config/apps' },
          { text: 'Health checks', link: '/config/healthchecks' },
          { text: 'Watching files', link: '/config/watching' },
          { text: 'Environment', link: '/config/environment' },
          { text: 'Hooks', link: '/config/hooks' },
          { text: 'Lua in configs', link: '/config/lua' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'CLI commands', link: '/reference/cli' },
          { text: 'How supervision works', link: '/reference/supervision' },
          { text: 'Security', link: '/reference/security' },
          { text: 'Running as a service', link: '/reference/service' },
          { text: 'Troubleshooting', link: '/reference/troubleshooting' },
        ],
      },
      {
        text: 'Project',
        items: [{ text: 'Contributing', link: '/contributing' }],
      },
    ],

    socialLinks: [{ icon: 'github', link: repo }],
    search: { provider: 'local' },
    outline: { level: [2, 3] },
    editLink: {
      pattern: `${repo}/edit/main/docs/:path`,
      text: 'Edit this page on GitHub',
    },
    footer: {
      message: 'A process supervisor for local development, configured in Lua.',
      copyright: `<a href="${repo}">github.com/omar0ali/watchd</a>`,
    },
  },
})
