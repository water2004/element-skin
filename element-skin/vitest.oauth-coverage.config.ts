import { defineConfig, mergeConfig } from 'vitest/config'
import baseConfig from './vitest.config'

export default mergeConfig(
  baseConfig,
  defineConfig({
    test: {
      coverage: {
        provider: 'v8',
        include: [
          'src/components/dashboard/oauth/DashboardOAuthAppForm.vue',
          'src/components/dashboard/oauth/DashboardOAuthApps.vue',
          'src/components/dashboard/oauth/oauthAppFormState.ts',
        ],
        reporter: ['text'],
        thresholds: {
          statements: 80,
          branches: 75,
          functions: 80,
          lines: 80,
        },
      },
    },
  }),
)
