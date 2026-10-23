<template>
  <footer class="footer-container" :class="variant === 'home' ? 'footer-home' : 'footer-standard'">
    <div class="footer-content">
      <div class="footer-row">
        <span v-if="footerText" class="footer-text-item">{{ footerText }}</span>

        <template v-if="filingIcp">
          <span class="footer-separator">|</span>
          <a :href="filingIcpLink || '#'" target="_blank" class="footer-link-item">
            {{ filingIcp }}
          </a>
        </template>

        <template v-if="filingMps">
          <span class="footer-separator">|</span>
          <a :href="filingMpsLink || '#'" target="_blank" class="footer-link-item">
            <img src="https://www.beian.gov.cn/img/ghs.png" class="filing-icon mr-1" />
            {{ filingMps }}
          </a>
        </template>
      </div>
      <div class="footer-credits">
        Powered by <a :href="repoUrl" target="_blank" class="footer-link-item">{{ repoLabel }}</a>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    variant?: 'home' | 'standard'
    footerText?: string
    filingIcp?: string
    filingIcpLink?: string
    filingMps?: string
    filingMpsLink?: string
    repoUrl: string
    repoLabel: string
  }>(),
  {
    variant: 'standard',
    footerText: '',
    filingIcp: '',
    filingIcpLink: '',
    filingMps: '',
    filingMpsLink: '',
  },
)
</script>

<style scoped>
.footer-container {
  width: 100%;
  padding: 8px 20px;
  transition:
    background-color 0.3s ease,
    color 0.3s ease,
    border-color 0.3s ease;
  flex-shrink: 0;
  position: relative;
  z-index: 10;
}

.footer-content {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0;
  text-align: center;
}

.footer-row {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  line-height: 1.2;
}

.footer-link-item {
  font-size: 12px;
  display: inline-flex;
  align-items: center;
  color: inherit;
  opacity: 0.7;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  text-decoration: none;
  padding: 4px 10px !important;
  border-radius: 6px;
  margin: 0 -2px;
}

.footer-link-item:hover {
  opacity: 1;
  text-decoration: none !important;
}

.footer-home .footer-link-item:hover {
  color: #ffffff !important;
  background-color: rgba(255, 255, 255, 0.15) !important;
  text-decoration: underline !important;
}

.footer-standard .footer-link-item:hover {
  color: var(--el-color-primary) !important;
  background-color: var(--color-background-soft) !important;
}

.footer-text-item {
  font-size: 12px;
  opacity: 0.7;
  display: inline-flex;
  align-items: center;
  padding: 4px 10px !important;
}

.footer-separator {
  font-size: 10px;
  margin: 0 5px;
  opacity: 0.6;
  user-select: none;
}

.footer-credits {
  font-size: 11px;
  opacity: 0.6;
  margin-top: 0;
}

.footer-home {
  background: transparent !important;
  color: #ffffff !important;
  border-top: none !important;
  padding: 24px 20px !important;
}

.footer-home .footer-content {
  gap: 2px;
}

.footer-standard {
  border-top: 1px solid var(--color-border);
  background: var(--color-card-background);
  color: var(--color-text-light);
  padding: 8px 20px !important;
}

@media (max-width: 768px) {
  .footer-container {
    padding: 16px 16px 24px;
  }
}
</style>
