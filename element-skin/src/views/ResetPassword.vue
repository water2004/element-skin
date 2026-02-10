<template>
  <div class="reset-container">
    <div class="reset-card">
      <div class="reset-header">
        <h1>重置密码</h1>
        <p>输入您的邮箱并获取验证码以重置密码</p>
      </div>

      <el-form :model="form" :rules="rules" ref="formRef" label-position="top" size="large">
        <el-form-item label="邮箱地址" prop="email">
          <el-input
            v-model="form.email"
            placeholder="请输入邮箱地址"
            :prefix-icon="Message"
          />
        </el-form-item>

        <el-form-item label="验证码" prop="code">
          <div class="verification-code-row">
            <el-input
              v-model="form.code"
              placeholder="请输入验证码"
              :prefix-icon="Ticket"
            />
            <el-button 
              type="primary"
              plain
              :disabled="countdown > 0" 
              :loading="codeLoading"
              @click="sendCode"
              class="code-btn"
            >
              {{ countdown > 0 ? `${countdown}s` : '发送验证码' }}
            </el-button>
          </div>
        </el-form-item>

        <el-form-item label="新密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="至少6个字符"
            :prefix-icon="Lock"
            show-password
          />
        </el-form-item>

        <el-form-item label="确认新密码" prop="confirmPassword">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            placeholder="请再次输入新密码"
            :prefix-icon="Lock"
            show-password
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            @click="resetPassword"
            :loading="loading"
            style="width: 100%"
          >
            {{ loading ? '提交中...' : '重置密码' }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="reset-footer">
        <el-button link type="primary" @click="$router.push('/login')">
          返回登录
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import axios from 'axios'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Message, Lock, Ticket } from '@element-plus/icons-vue'

const router = useRouter()
const formRef = ref(null)
const loading = ref(false)
const codeLoading = ref(false)
const countdown = ref(0)
let timer = null

const form = reactive({
  email: '',
  code: '',
  password: '',
  confirmPassword: ''
})

const rules = {
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
  ],
  code: [
    { required: true, message: '请输入验证码' }
  ],
  password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码至少需要6个字符', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== form.password) {
          callback(new Error('两次输入的密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

import { onMounted } from 'vue'

onMounted(async () => {
  try {
    const res = await axios.get('/public/settings')
    if (!res.data.email_verify_enabled) {
      ElMessage.warning('密码重置功能未开启')
      router.push('/login')
    }
  } catch (e) {
    console.error('Failed to fetch settings', e)
  }
})

async function sendCode() {
  try {
    await formRef.value.validateField('email')
  } catch (e) {
    ElMessage.warning('请先输入有效的邮箱地址')
    return
  }

  try {
    codeLoading.value = true
    await axios.post('/send-verification-code', {
      email: form.email,
      type: 'reset'
    })
    ElMessage.success('验证码已发送到您的邮箱')
    
    countdown.value = 60
    timer = setInterval(() => {
      countdown.value--
      if (countdown.value <= 0) {
        clearInterval(timer)
      }
    }, 1000)
  } catch (e) {
    if (e.response?.data?.detail) {
      ElMessage.error('发送失败: ' + e.response.data.detail)
    } else {
      ElMessage.error('发送失败，请稍后再试')
    }
  } finally {
    codeLoading.value = false
  }
}

async function resetPassword() {
  try {
    await formRef.value.validate()
    loading.value = true

    await axios.post('/reset-password', {
      email: form.email,
      password: form.password,
      code: form.code
    })

    ElMessage.success('密码重置成功！请使用新密码登录。')
    setTimeout(() => {
      router.push('/login')
    }, 1500)
  } catch (e) {
    if (e.response?.data?.detail) {
      ElMessage.error('重置失败: ' + e.response.data.detail)
    } else if (e.message && !e.message.includes('validate')) {
      ElMessage.error('重置失败: ' + e.message)
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.reset-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: var(--color-background-hero-light);
  transition: background 0.3s ease;
}

.reset-card {
  width: 100%;
  max-width: 440px;
  background: var(--color-card-background);
  border-radius: 16px;
  padding: 40px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  animation: slideUp 0.5s ease-out;
  border: 1px solid var(--color-border);
}

@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.reset-header {
  text-align: center;
  margin-bottom: 32px;
}

.reset-header h1 {
  margin: 0 0 8px 0;
  font-size: 28px;
  font-weight: 600;
  color: var(--color-heading);
}

.reset-header p {
  margin: 0;
  font-size: 14px;
  color: var(--color-text-light);
}

.reset-footer {
  text-align: center;
  margin-top: 24px;
}

:deep(.el-form-item__label) {
  font-weight: 500;
  color: var(--color-text);
}

:deep(.el-input__wrapper) {
  height: 48px;
}

.verification-code-row {
  display: flex;
  gap: 12px;
  width: 100%;
}

.code-btn {
  height: 48px;
  min-width: 120px;
}
</style>
