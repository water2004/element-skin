<template>
  <div class="flex items-center justify-center min-h-screen p-5 bg-screen-gradient transition-bg">
    <div
      class="w-full max-w-440 bg-card rounded-3xl p-10 shadow-lg-soft animate-slide-up border transition-colors"
    >
      <div class="text-center mb-8">
        <h1 class="m-0 mb-2 text-28 font-semibold text-heading">注册账号</h1>
        <p class="m-0 text-sm text-light">创建一个新账号来开始使用</p>
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
          <div class="flex gap-3 w-full">
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
              class="h-12 min-w-120"
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
          <el-button type="primary" @click="register" :loading="loading" class="w-full">
            <el-icon v-if="!loading"><UserFilled /></el-icon>
            {{ loading ? '注册中...' : '注册' }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="text-center mt-6 text-body text-sm transition-colors">
        <span>已有账号？</span>
        <el-button link type="primary" @click="$router.push('/login')"> 立即登录 </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Message, Lock, Ticket, UserFilled, User } from '@element-plus/icons-vue'
import { getPublicSettings } from '@/api/public'
import { sendVerificationCode, register as apiRegister } from '@/api/auth'

const router = useRouter()
const formRef = ref<FormInstance | null>(null)
const loading = ref(false)

const form = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
  invite: '',
  code: '',
})

const emailVerifyEnabled = ref(false)
const codeLoading = ref(false)
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const rules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, message: '用户名至少需要3个字符', trigger: 'blur' },
    { max: 20, message: '用户名长度不能超过20个字符', trigger: 'blur' },
    {
      pattern: /^[a-zA-Z0-9_\u4e00-\u9fa5]+$/,
      message: '用户名仅支持中英文、数字和下划线',
      trigger: 'blur',
    },
  ],
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' },
  ],
  code: [{ required: true, message: '请输入验证码' }],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少需要6个字符', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (value !== form.password) {
          callback(new Error('两次输入的密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur',
    },
  ],
}

onMounted(async () => {
  try {
    const res = await getPublicSettings()
    emailVerifyEnabled.value = res.data.email_verify_enabled ?? false
  } catch (e) {
    console.error('Failed to fetch settings', e)
  }
})

async function sendCode() {
  try {
    if (!formRef.value) return
    await formRef.value.validateField('email')
  } catch (e) {
    ElMessage.warning('请先输入有效的邮箱地址')
    return
  }

  try {
    codeLoading.value = true
    await sendVerificationCode({
      email: form.email,
      type: 'register',
    })
    ElMessage.success('验证码已发送到您的邮箱')

    countdown.value = 60
    timer = setInterval(() => {
      countdown.value--
      if (countdown.value <= 0 && timer) {
        clearInterval(timer)
      }
    }, 1000)
  } catch (e: any) {
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
    if (!formRef.value) return
    await formRef.value.validate()
    loading.value = true

    // 在发送前trim邀请码
    const payload = {
      username: form.username,
      email: form.email,
      password: form.password,
      invite: form.invite ? form.invite.trim() : '',
      code: form.code,
    }

    await apiRegister(payload)
    ElMessage.success('注册成功！即将跳转到登录页面...')

    // 延迟跳转，让用户看到成功消息
    setTimeout(() => {
      router.push('/login')
    }, 1500)
  } catch (e: any) {
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
:deep(.el-form-item__label) {
  font-weight: 500;
  color: var(--color-text);
  transition: color 0.3s ease;
}

:deep(.el-input__wrapper) {
  height: 48px;
}
</style>
