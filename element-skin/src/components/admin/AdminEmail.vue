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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Message, Postcard } from '@element-plus/icons-vue'
import { getAdminSettingsGroup, saveAdminSettingsGroup } from '@/api/admin/settings'
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

async function loadSettings() {
  try {
    const res = await getAdminSettingsGroup('email')
    if (res.data) {
      Object.assign(emailSettings, res.data)
      emailSettings.smtp_password = '' // Don't show password
    }
  } catch {
    ElMessage.error('加载邮件设置失败')
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
