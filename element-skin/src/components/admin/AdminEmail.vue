<template>
  <div class="settings-section">
    <div class="section-header">
      <h2>邮件服务设置</h2>
      <el-button type="primary" @click="loadSettings">
        <el-icon><Refresh /></el-icon>
        刷新
      </el-button>
    </div>

    <el-card class="settings-card">
      <el-form 
        :label-width="labelPosition === 'top' ? 'auto' : '140px'" 
        :model="emailSettings" 
        :label-position="labelPosition"
      >
        <el-divider content-position="left">验证功能开关</el-divider>
        <el-form-item label="启用邮件验证">
          <el-switch v-model="emailSettings.email_verify_enabled" />
          <el-text size="small" type="info" style="margin-left: 12px">开启后，注册和重置密码需要邮件验证码</el-text>
        </el-form-item>
        <el-form-item label="验证码有效期" v-if="emailSettings.email_verify_enabled">
          <el-input v-model="emailSettings.email_verify_ttl" type="number">
            <template #suffix>秒</template>
          </el-input>
        </el-form-item>

        <el-divider content-position="left">SMTP 服务器配置</el-divider>
        <el-form-item label="SMTP 服务器地址">
          <el-input v-model="emailSettings.smtp_host" placeholder="smtp.example.com" />
        </el-form-item>
        <el-form-item label="SMTP 端口">
          <el-input v-model="emailSettings.smtp_port" placeholder="465" />
        </el-form-item>
        <el-form-item label="SMTP 用户名">
          <el-input v-model="emailSettings.smtp_user" placeholder="user@example.com" />
        </el-form-item>
        <el-form-item label="SMTP 密码">
          <el-input v-model="emailSettings.smtp_password" type="password" show-password placeholder="留空则不修改" />
        </el-form-item>
        <el-form-item label="使用 SSL/TLS">
          <el-switch v-model="emailSettings.smtp_ssl" />
        </el-form-item>
        <el-form-item label="发件人信息">
          <el-input v-model="emailSettings.smtp_sender" placeholder="SkinServer <no-reply@example.com>" />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="saveSettings" size="large">
            <el-icon><Check /></el-icon>
            保存配置
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { Refresh, Check } from '@element-plus/icons-vue'

const emailSettings = ref({
  email_verify_enabled: false,
  email_verify_ttl: 300,
  smtp_host: '',
  smtp_port: '465',
  smtp_user: '',
  smtp_password: '',
  smtp_ssl: true,
  smtp_sender: ''
})

const windowWidth = ref(window.innerWidth)
const labelPosition = computed(() => windowWidth.value < 992 ? 'top' : 'right')

function handleResize() {
  windowWidth.value = window.innerWidth
}

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

async function loadSettings() {
  try {
    const res = await axios.get('/admin/settings', { headers: authHeaders() })
    if (res.data) {
      // Pick only email related settings
      emailSettings.value.email_verify_enabled = res.data.email_verify_enabled
      emailSettings.value.email_verify_ttl = res.data.email_verify_ttl
      emailSettings.value.smtp_host = res.data.smtp_host
      emailSettings.value.smtp_port = res.data.smtp_port
      emailSettings.value.smtp_user = res.data.smtp_user
      emailSettings.value.smtp_ssl = res.data.smtp_ssl
      emailSettings.value.smtp_sender = res.data.smtp_sender
      // Password is not returned by API
    }
  } catch (e) {
    console.error('Load settings error:', e)
    ElMessage.error('加载设置失败')
  }
}

async function saveSettings() {
  try {
    await axios.post('/admin/settings', emailSettings.value, { headers: authHeaders() })
    ElMessage.success('保存成功')
  } catch (e) {
    ElMessage.error('保存失败: ' + (e.response?.data?.detail || e.message))
  }
}

onMounted(() => {
  loadSettings()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.settings-section {
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.settings-card {
  width: 100%;
  max-width: 800px;
  padding: 30px;
  animation: cardSlideIn 0.5s cubic-bezier(0.4, 0, 0.2, 1) 0.1s backwards;
  background: var(--color-card-background);
  border: 1px solid var(--color-border);
}

.settings-card :deep(.el-form-item__label) {
  color: var(--color-text);
}

.settings-card :deep(.el-divider__text) {
  background-color: var(--color-card-background);
  color: var(--color-heading);
}

.settings-card .el-form {
  max-width: 600px;
  margin: 0 auto;
}

@media (max-width: 768px) {
  .settings-card {
    padding: 15px;
  }
}
</style>