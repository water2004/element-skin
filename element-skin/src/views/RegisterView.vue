<template>
  <div class="register-container">
    <div class="register-card">
      <div class="register-header">
        <h1>注册账号</h1>
        <p>创建一个新账号来开始使用</p>
      </div>

      <el-form :model="form" :rules="rules" ref="formRef" label-position="top" size="large">
        <el-form-item label="用户名" prop="username">
          <el-input
            v-model="form.username"
            placeholder="请输入用户名"
            :prefix-icon="User"
            @keyup.enter="register"
          />
        </el-form-item>

        <el-form-item label="邮箱地址" prop="email">
          <el-input
            v-model="form.email"
            placeholder="请输入邮箱地址"
            :prefix-icon="Message"
            @keyup.enter="register"
          />
        </el-form-item>

        <el-form-item v-if="emailVerifyEnabled" label="验证码" prop="code">
          <div class="verification-code-row">
            <el-input
              v-model="form.code"
              placeholder="请输入验证码"
              :prefix-icon="Ticket"
              @keyup.enter="register"
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

        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="至少6个字符"
            :prefix-icon="Lock"
            show-password
            @keyup.enter="register"
          />
        </el-form-item>

        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            placeholder="请再次输入密码"
            :prefix-icon="Lock"
            show-password
            @keyup.enter="register"
          />
        </el-form-item>

        <el-form-item label="邀请码 (若需要)" prop="invite">
          <el-input
            v-model="form.invite"
            placeholder="如果需要邀请码，请在此输入"
            :prefix-icon="Ticket"
            @keyup.enter="register"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            @click="register"
            :loading="loading"
            style="width: 100%"
          >
            <el-icon v-if="!loading"><UserFilled /></el-icon>
            {{ loading ? '注册中...' : '注册' }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="register-footer">
        <span>已有账号？</span>
        <el-button link type="primary" @click="$router.push('/login')">
          立即登录
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
import { Message, Lock, Ticket, UserFilled, User } from '@element-plus/icons-vue'

const router = useRouter()
const formRef = ref(null)
const loading = ref(false)

const form = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
  invite: '',
  code: ''
})

const emailVerifyEnabled = ref(false)
const codeLoading = ref(false)
const countdown = ref(0)
let timer = null

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, message: '用户名至少需要3个字符', trigger: 'blur' },
    { max: 20, message: '用户名长度不能超过20个字符', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_\u4e00-\u9fa5]+$/, message: '用户名仅支持中英文、数字和下划线', trigger: 'blur' }
  ],
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
  ],
  code: [
    { required: true, message: '请输入验证码' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
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
    emailVerifyEnabled.value = res.data.email_verify_enabled
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
      type: 'register'
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

async function register() {
  try {
    await formRef.value.validate()
    loading.value = true

    // 在发送前trim邀请码
    const payload = {
      username: form.username,
      email: form.email,
      password: form.password,
      invite: form.invite ? form.invite.trim() : '',
      code: form.code
    }

    const res = await axios.post('/register', payload)
    ElMessage.success('注册成功！即将跳转到登录页面...')

    // 延迟跳转，让用户看到成功消息
    setTimeout(() => {
      router.push('/login')
    }, 1500)
  } catch (e) {
    if (e.response?.data?.detail) {
      ElMessage.error('注册失败: ' + e.response.data.detail)
    } else if (e.message && !e.message.includes('validate')) {
      ElMessage.error('注册失败: ' + e.message)
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.register-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: var(--color-background-hero-light);
  transition: background 0.3s ease;
}

.register-card {
  width: 100%;
  max-width: 440px;
  background: var(--color-card-background);
  border-radius: 16px;
  padding: 40px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  animation: slideUp 0.5s ease-out;
  border: 1px solid var(--color-border);
  transition: background-color 0.3s ease, border-color 0.3s ease, color 0.3s ease;
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

.register-header {
  text-align: center;
  margin-bottom: 32px;
}

.register-header h1 {
  margin: 0 0 8px 0;
  font-size: 28px;
  font-weight: 600;
  color: var(--color-heading);
  transition: color 0.3s ease;
}

.register-header p {
  margin: 0;
  font-size: 14px;
  color: var(--color-text-light);
  transition: color 0.3s ease;
}

.register-footer {
  text-align: center;
  margin-top: 24px;
  color: var(--color-text);
  font-size: 14px;
  transition: color 0.3s ease;
}

:deep(.el-form-item__label) {
  font-weight: 500;
  color: var(--color-text);
  transition: color 0.3s ease;
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
