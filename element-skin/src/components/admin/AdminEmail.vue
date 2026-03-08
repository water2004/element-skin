<template>
  <div class="settings-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div class="page-header-icon"><Message /></div>
        <div class="page-header-text">
          <h2>邮件服务设置</h2>
          <p class="subtitle">配置 SMTP 服务器以启用注册验证、找回密码等通知功能</p>
        </div>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" :icon="Refresh" @click="loadSettings" plain class="hover-lift">
          刷新配置
        </el-button>
      </div>
    </div>

    <el-card class="surface-card" shadow="never">
      <template #header>
        <div class="card-header-flex">
          <div class="title-group">
            <el-icon><Postcard /></el-icon>
            <span>SMTP 与验证配置</span>
          </div>
          <el-button type="primary" size="small" @click="saveSettings" :loading="saving" class="hover-lift">保存配置</el-button>
        </div>
      </template>

      <el-form label-position="top" :model="emailSettings">
        <div class="settings-group">
          <div class="group-title">验证功能</div>
          <el-row :gutter="40">
            <el-col :xs="24" :sm="12">
              <el-form-item label="启用邮件验证">
                <el-switch v-model="emailSettings.email_verify_enabled" />
                <p class="hint-text">开启后，用户注册和重置密码时必须通过邮件验证码确认身份。</p>
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

        <div class="settings-group">
          <div class="group-title">SMTP 服务器</div>
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
                <el-input v-model="emailSettings.smtp_password" type="password" show-password placeholder="留空则不修改原有密码" />
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
                <el-input v-model="emailSettings.smtp_sender" placeholder="SkinServer <no-reply@example.com>" />
                <p class="hint-text">发件人在邮件客户端中显示的名称及回复地址。</p>
              </el-form-item>
            </el-col>
          </el-row>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { Refresh, Message, Postcard } from '@element-plus/icons-vue'

const emailSettings = reactive({
  email_verify_enabled: false,
  email_verify_ttl: 300,
  smtp_host: '',
  smtp_port: '465',
  smtp_user: '',
  smtp_password: '',
  smtp_ssl: true,
  smtp_sender: ''
})

const saving = ref(false)
const authHeaders = () => ({ Authorization: 'Bearer ' + localStorage.getItem('jwt') })

async function loadSettings() {
  try {
    const res = await axios.get('/admin/settings/email', { headers: authHeaders() })
    if (res.data) {
      Object.assign(emailSettings, res.data)
      emailSettings.smtp_password = '' // Don't show password
    }
  } catch (e) {
    ElMessage.error('加载邮件设置失败')
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await axios.post('/admin/settings/email', emailSettings, { headers: authHeaders() })
    ElMessage.success('设置已保存')
    emailSettings.smtp_password = '' // Clear password field after save
  } catch (e) {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<style scoped>
@import "@/assets/styles/animations.css";
@import "@/assets/styles/layout.css";
@import "@/assets/styles/cards.css";
@import "@/assets/styles/headers.css";
@import "@/assets/styles/buttons.css";

.settings-section {
  max-width: 900px;
  margin: 0 auto;
  padding: 20px 0;
}

.card-header-flex { display: flex; justify-content: space-between; align-items: center; }
.card-header-flex .title-group { display: flex; align-items: center; gap: 8px; font-weight: 600; color: var(--color-heading); }

.settings-group { padding: 10px 0; }
.group-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-light);
  margin-bottom: 20px;
  border-left: 4px solid var(--el-color-primary);
  padding-left: 12px;
}

.hint-text { font-size: 12px; color: var(--color-text-light); line-height: 1.5; margin-top: 4px; }
</style>