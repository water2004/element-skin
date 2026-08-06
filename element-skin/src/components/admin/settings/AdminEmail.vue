<template>
  <div class="max-w-[900px] mx-auto py-5 animate-fade-in">
    <PageHeader title="邮件服务设置" subtitle="配置 SMTP 服务器以启用注册验证、找回密码等通知功能">
      <template #icon><Message /></template>
      <template #actions>
        <el-button type="primary" :icon="Refresh" @click="loadSettings" plain class="hover-lift">
          刷新配置
        </el-button>
      </template>
    </PageHeader>

    <UiCard shadow="never">
      <template #header>
        <div class="flex justify-between items-center">
          <div class="flex items-center gap-2 font-semibold text-[var(--color-heading)]">
            <el-icon><Postcard /></el-icon>
            <span>SMTP 与验证配置</span>
          </div>
          <el-button
            type="primary"
            size="small"
            @click="saveSettings"
            :loading="saving"
            class="hover-lift"
            >保存配置</el-button
          >
        </div>
      </template>

      <el-form label-position="top" :model="emailSettings">
        <div class="py-2">
          <div
            class="text-sm font-semibold text-[var(--color-text-light)] mb-5 pl-3 border-l-4 border-l-[var(--el-color-primary)]"
          >
            验证功能
          </div>
          <el-row :gutter="40">
            <el-col :xs="24" :sm="12">
              <el-form-item label="启用邮件验证">
                <el-switch v-model="emailSettings.email_verify_enabled" />
                <p class="text-xs text-[var(--color-text-light)] leading-normal mt-1">
                  开启后，用户注册和重置密码时必须通过邮件验证码确认身份。
                </p>
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12" v-if="emailSettings.email_verify_enabled">
              <el-form-item label="验证码有效期 (秒)">
                <el-input-number v-model="emailSettings.email_verify_ttl" :min="60" :step="60" />
              </el-form-item>
            </el-col>
          </el-row>
        </div>

        <el-divider />

        <div class="py-2">
          <div
            class="text-sm font-semibold text-[var(--color-text-light)] mb-5 pl-3 border-l-4 border-l-[var(--el-color-primary)]"
          >
            SMTP 服务器
          </div>
          <el-row :gutter="20">
            <el-col :xs="24" :sm="18">
              <el-form-item label="服务器地址">
                <el-input v-model="emailSettings.smtp_host" placeholder="smtp.example.com" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="6">
              <el-form-item label="端口">
                <el-input v-model="emailSettings.smtp_port" placeholder="465" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="20">
            <el-col :xs="24" :sm="12">
              <el-form-item label="用户名 (通常为邮箱地址)">
                <el-input v-model="emailSettings.smtp_user" placeholder="user@example.com" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="密码 / 授权码">
                <el-input
                  v-model="emailSettings.smtp_password"
                  type="password"
                  show-password
                  placeholder="留空则不修改原有密码"
                />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="20">
            <el-col :xs="24" :sm="12">
              <el-form-item label="使用 SSL/TLS 加密">
                <el-switch v-model="emailSettings.smtp_ssl" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="发件人显示名称">
                <el-input
                  v-model="emailSettings.smtp_sender"
                  placeholder="SkinServer <no-reply@example.com>"
                />
                <p class="text-xs text-[var(--color-text-light)] leading-normal mt-1">
                  发件人在邮件客户端中显示的名称及回复地址。
                </p>
              </el-form-item>
            </el-col>
          </el-row>
        </div>
      </el-form>
    </UiCard>

    <UiCard class="mt-6" shadow="never">
      <template #header>
        <div class="flex justify-between items-center">
          <div class="flex items-center gap-2 font-semibold text-[var(--color-heading)]">
            <el-icon><Filter /></el-icon>
            <span>账户邮箱后缀策略</span>
          </div>
          <el-button
            type="primary"
            size="small"
            :loading="policySaving"
            class="hover-lift"
            @click="saveEmailSuffixPolicy"
          >
            保存策略
          </el-button>
        </div>
      </template>

      <el-form label-position="top">
        <el-form-item label="名单模式">
          <el-radio-group v-model="emailSuffixPolicy.mode">
            <el-radio-button value="disabled">关闭名单过滤</el-radio-button>
            <el-radio-button value="allowlist">仅允许白名单</el-radio-button>
            <el-radio-button value="denylist">拒绝黑名单</el-radio-button>
          </el-radio-group>
          <p class="text-xs text-[var(--color-text-light)] leading-normal mt-2">
            仅影响新用户注册和修改账户邮箱；找回已有账户密码不受名单限制。
          </p>
        </el-form-item>

        <el-divider />

        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div>
            <div class="mb-3 font-medium text-[var(--color-heading)]">白名单后缀</div>
            <div class="flex gap-2 mb-3">
              <el-input
                v-model="allowlistDraft"
                placeholder="例如 @qq.com"
                @keyup.enter="addSuffix('allowlist')"
              />
              <el-button @click="addSuffix('allowlist')">添加</el-button>
            </div>
            <div class="flex flex-wrap gap-2 min-h-8">
              <el-tag
                v-for="suffix in emailSuffixPolicy.allowlist"
                :key="suffix"
                closable
                @close="removeSuffix('allowlist', suffix)"
              >
                {{ suffix }}
              </el-tag>
              <span
                v-if="emailSuffixPolicy.allowlist.length === 0"
                class="text-sm text-[var(--color-text-light)]"
              >
                尚未添加白名单后缀
              </span>
            </div>
          </div>

          <div>
            <div class="mb-3 font-medium text-[var(--color-heading)]">黑名单后缀</div>
            <div class="flex gap-2 mb-3">
              <el-input
                v-model="denylistDraft"
                placeholder="例如 @example.com"
                @keyup.enter="addSuffix('denylist')"
              />
              <el-button @click="addSuffix('denylist')">添加</el-button>
            </div>
            <div class="flex flex-wrap gap-2 min-h-8">
              <el-tag
                v-for="suffix in emailSuffixPolicy.denylist"
                :key="suffix"
                closable
                type="danger"
                @close="removeSuffix('denylist', suffix)"
              >
                {{ suffix }}
              </el-tag>
              <span
                v-if="emailSuffixPolicy.denylist.length === 0"
                class="text-sm text-[var(--color-text-light)]"
              >
                尚未添加黑名单后缀
              </span>
            </div>
          </div>
        </div>
      </el-form>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Message, Postcard, Filter } from '@element-plus/icons-vue'
import {
  getAdminEmailSuffixPolicy,
  getAdminSettingsGroup,
  putAdminEmailSuffixPolicy,
  saveAdminSettingsGroup,
} from '@/api/admin/settings'
import type { EmailSuffixPolicy } from '@/api/types'
import PageHeader from '@/components/common/PageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'

const emailSettings = reactive({
  email_verify_enabled: false,
  email_verify_ttl: 300,
  smtp_host: '',
  smtp_port: '465',
  smtp_user: '',
  smtp_password: '',
  smtp_ssl: true,
  smtp_sender: '',
})

const saving = ref(false)
const policySaving = ref(false)
const allowlistDraft = ref('')
const denylistDraft = ref('')
const emailSuffixPolicy = reactive<EmailSuffixPolicy>({
  mode: 'disabled',
  allowlist: [],
  denylist: [],
})

async function loadSettings() {
  try {
    const [settingsResponse, policyResponse] = await Promise.all([
      getAdminSettingsGroup('email'),
      getAdminEmailSuffixPolicy(),
    ])
    if (settingsResponse.data) {
      Object.assign(emailSettings, settingsResponse.data)
      emailSettings.smtp_password = '' // Don't show password
    }
    Object.assign(emailSuffixPolicy, policyResponse.data)
  } catch {
    ElMessage.error('加载邮件设置失败')
  }
}

function normalizeSuffix(value: string) {
  const trimmed = value.trim().toLowerCase()
  return trimmed.startsWith('@') ? trimmed : `@${trimmed}`
}

function addSuffix(list: 'allowlist' | 'denylist') {
  const draft = list === 'allowlist' ? allowlistDraft : denylistDraft
  const suffix = normalizeSuffix(draft.value)
  if (!/^@[^\s@]+\.[^\s@]+$/.test(suffix)) {
    ElMessage.warning('请输入有效的邮箱后缀，例如 @qq.com')
    return
  }
  if (emailSuffixPolicy[list].includes(suffix)) {
    ElMessage.warning('该邮箱后缀已经存在')
    return
  }
  emailSuffixPolicy[list].push(suffix)
  emailSuffixPolicy[list].sort()
  draft.value = ''
}

function removeSuffix(list: 'allowlist' | 'denylist', suffix: string) {
  emailSuffixPolicy[list] = emailSuffixPolicy[list].filter((item) => item !== suffix)
}

async function saveEmailSuffixPolicy() {
  if (emailSuffixPolicy.mode === 'allowlist' && emailSuffixPolicy.allowlist.length === 0) {
    ElMessage.warning('启用白名单前至少需要添加一个邮箱后缀')
    return
  }
  policySaving.value = true
  try {
    await putAdminEmailSuffixPolicy({
      mode: emailSuffixPolicy.mode,
      allowlist: [...emailSuffixPolicy.allowlist],
      denylist: [...emailSuffixPolicy.denylist],
    })
    ElMessage.success('邮箱后缀策略已保存')
    const response = await getAdminEmailSuffixPolicy()
    Object.assign(emailSuffixPolicy, response.data)
  } catch {
    ElMessage.error('保存邮箱后缀策略失败')
  } finally {
    policySaving.value = false
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await saveAdminSettingsGroup('email', emailSettings)
    ElMessage.success('设置已保存')
    emailSettings.smtp_password = '' // Clear password field after save
  } catch {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>
