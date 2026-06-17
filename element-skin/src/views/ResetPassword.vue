<template>
  <div
    class="flex items-center justify-center min-h-screen p-5 bg-[var(--color-background-hero-light)] dark:bg-[var(--color-background-hero-dark)] transition-[background] duration-300"
  >
    <div
      class="w-full max-w-[440px] bg-[var(--color-card-background)] rounded-[16px] p-10 shadow-[0_8px_32px_rgba(0,0,0,0.1)] animate-slide-up border border-[var(--color-border)]"
    >
      <div class="text-center mb-8">
        <h1 class="m-0 mb-2 text-[28px] font-semibold text-[var(--color-heading)]">重置密码</h1>
        <p class="m-0 text-sm text-[var(--color-text-light)]">输入您的邮箱并获取验证码以重置密码</p>
      </div>

      <el-form :model="form" :rules="rules" ref="formRef" label-position="top" size="large">
        <el-form-item label="邮箱地址" prop="email">
          <el-input v-model="form.email" placeholder="请输入邮箱地址" :prefix-icon="Message" />
        </el-form-item>

        <el-form-item label="验证码" prop="code">
          <div class="flex gap-3 w-full">
            <el-input v-model="form.code" placeholder="请输入验证码" :prefix-icon="Ticket" />
            <el-button
              type="primary"
              plain
              :disabled="countdown > 0"
              :loading="codeLoading"
              @click="sendCode"
              class="h-12 min-w-[120px]"
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
          <el-button type="primary" @click="resetPassword" :loading="loading" class="w-full">
            {{ loading ? '提交中...' : '重置密码' }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="text-center mt-6">
        <el-button link type="primary" @click="$router.push('/login')"> 返回登录 </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Message, Lock, Ticket } from '@element-plus/icons-vue'
import { getPublicSettings } from '@/api/public'
import { sendVerificationCode, resetPassword as apiResetPassword } from '@/api/auth'
import { getErrorMessage, isValidationError } from '@/utils/error'

const router = useRouter()
const formRef = ref<FormInstance | null>(null)
const loading = ref(false)
const codeLoading = ref(false)
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const form = reactive({
  email: '',
  code: '',
  password: '',
  confirmPassword: '',
})

const rules: FormRules = {
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' },
  ],
  code: [{ required: true, message: '请输入验证码' }],
  password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
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
    if (!formRef.value) return
    await formRef.value.validateField('email')
  } catch {
    ElMessage.warning('请先输入有效的邮箱地址')
    return
  }

  try {
    codeLoading.value = true
    await sendVerificationCode({
      email: form.email,
      type: 'reset',
    })
    ElMessage.success('验证码已发送到您的邮箱')

    countdown.value = 60
    timer = setInterval(() => {
      countdown.value--
      if (countdown.value <= 0 && timer) {
        clearInterval(timer)
      }
    }, 1000)
  } catch (e: unknown) {
    ElMessage.error('发送失败: ' + getErrorMessage(e, '请稍后再试'))
  } finally {
    codeLoading.value = false
  }
}

async function resetPassword() {
  try {
    if (!formRef.value) return
    await formRef.value.validate()
    loading.value = true

    await apiResetPassword({
      email: form.email,
      password: form.password,
      code: form.code,
    })

    ElMessage.success('密码重置成功！请使用新密码登录。')
    setTimeout(() => {
      router.push('/login')
    }, 1500)
  } catch (e: unknown) {
    if (!isValidationError(e)) {
      ElMessage.error('重置失败: ' + getErrorMessage(e, '重置失败'))
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
}

:deep(.el-input__wrapper) {
  height: 48px;
}
</style>
