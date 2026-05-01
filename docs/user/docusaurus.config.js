// @ts-check
// `@type` JSDoc annotations allow editor autocompletion and type checking
// (when paired with `@ts-check`).
// There are various equivalent ways to declare your Docusaurus config.
// See: https://docusaurus.io/docs/api/docusaurus-config

import {themes as prismThemes} from 'prism-react-renderer';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'MCPSmithy',
  tagline: 'Model Context Protocol toolkit for any project',
  favicon: 'img/logo.svg',

  // Future flags, see https://docusaurus.io/docs/api/docusaurus-config#future
  future: {
    v4: true, // Improve compatibility with the upcoming Docusaurus v4
  },

  // Set the production url of your site here
  url: 'https://iorubs.github.io',
  baseUrl: '/mcpsmithy/',

  // GitHub pages deployment config.
  organizationName: 'iorubs',
  projectName: 'mcpsmithy',

  onBrokenLinks: 'throw',

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          path: '.',
          routeBasePath: '/',
          sidebarPath: './sidebars.js',
          exclude: [
            'node_modules/**',
            'src/**',
            'static/**',
            'Dockerfile',
            '.dockerignore',
            'package*.json',
            'docusaurus.config.js',
            'sidebars.js',
          ],
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      // Replace with your project's social card
      image: 'img/docusaurus-social-card.jpg',
      colorMode: {
        respectPrefersColorScheme: true,
      },
      navbar: {
        title: 'Smithy',
        logo: {
          alt: 'smithy',
          src: 'img/logo.svg',
          href: 'https://iorubs.github.io/smithy/',
          target: '_self',
        },
        items: [
          {
            href: 'https://iorubs.github.io/smithy-cli/',
            label: 'CLI',
            position: 'left',
            target: '_self',
          },
          {
            to: '/',
            label: 'MCPSmithy',
            position: 'left',
          },
          {
            href: 'https://iorubs.github.io/agentsmithy/',
            label: 'AgentSmithy',
            position: 'left',
            target: '_self',
          },
          {
            href: 'https://github.com/iorubs/mcpsmithy',
            position: 'right',
            label: 'GitHub',
            className: 'header-github-link',
          },
        ],
      },
      footer: {
        style: 'dark',
        copyright: `Copyright © ${new Date().getFullYear()} MCPSmithy`,
      },
      prism: {
        theme: prismThemes.github,
        darkTheme: prismThemes.dracula,
      },
    }),
};

export default config;
