<template>
  <div
    class="flex items-center justify-center min-h-screen p-5 bg-[var(--color-background-hero-light)] dark:bg-[var(--color-background-hero-dark)] transition-[background] duration-300"
  >
    <div
      class="w-full max-w-[440px] bg-[var(--color-card-background)] rounded-[16px] p-10 shadow-[0_8px_32px_rgba(0,0,0,0.1)] animate-slide-up border border-[var(--color-border)] transition-colors"
    >
      <div class="text-center mb-8">
        <h1 class="m-0 mb-2 text-[28px] font-semibold text-[var(--color-heading)]">注册账号</h1>
        <p class="m-0 text-sm text-[var(--color-text-light)]">创建一个新账号来开始使用</p>
      </div>

      <el-skeleton v-if="settingsLoading" :rows="6" animated />

      <div v-else-if="settingsError" class="space-y-4">
        <el-alert
          type="error"
          title="无法加载注册配置"
          description="为避免遗漏验证码、邀请码或邮箱后缀要求，配置加载成功前不能提交注册。"
          :closable="false"
          show-icon
        />
        <el-button type="primary" plain class="w-full" @click="loadRegistrationSettings">
          重新加载
        </el-button>
      </div>

      <el-alert
        v-else-if="!allowRegister"
        type="warning"
        title="本站当前已关闭新用户注册"
        :closable="false"
        show-icon
      />

      <el-form v-else :model="form" :rules="rules" ref="formRef" label-position="top" size="large">
        <el-form-item label="用户名" prop="username">
          <el-input
            v-model="form.username"
            name="display_name"
            autocomplete="nickname"
            placeholder="请输入用户名"
            :prefix-icon="User"
            @keyup.enter="register"
          />
        </el-form-item>

        <el-form-item label="邮箱地址" prop="email">
          <EmailSuffixInput
            v-model="form.email"
            :policy="emailSuffixPolicy"
            name="email"
            type="email"
            autocomplete="username"
            placeholder="请输入邮箱地址"
            @enter="register"
          />
        </el-form-item>

        <el-form-item v-if="emailVerifyEnabled" label="验证码" prop="code">
          <div class="flex gap-3 w-full">
            <el-input
              v-model="form.code"
              name="verification_code"
              autocomplete="one-time-code"
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
              class="h-12 min-w-[120px]"
            >
              {{ countdown > 0 ? `${countdown}s` : '发送验证码' }}
            </el-button>
          </div>
        </el-form-item>

        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            name="password"
            autocomplete="new-password"
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
            name="password_confirmation"
            autocomplete="new-password"
            placeholder="请再次输入密码"
            :prefix-icon="Lock"
            show-password
            @keyup.enter="register"
          />
        </el-form-item>

        <el-form-item v-if="requireInvite" label="邀请码" prop="invite" required>
          <div class="w-full">
            <el-input
              v-model="form.invite"
              name="invite_code"
              autocomplete="off"
              placeholder="请输入管理员提供的邀请码"
              :prefix-icon="Ticket"
              @keyup.enter="register"
            />
            <p class="mt-1.5 mb-0 text-xs text-[var(--color-text-light)]">
              邀请码会按原文校验，请保留其中的空格、大小写和符号。
            </p>
          </div>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="register" :loading="loading" class="w-full">
            <el-icon v-if="!loading"><UserFilled /></el-icon>
            {{ loading ? '注册中...' : '注册' }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="text-center mt-6 text-[var(--color-text)] text-sm transition-colors">
        <span>已有账号？</span>
        <el-button link type="primary" @click="$router.push('/login')"> 立即登录 </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Lock, Ticket, UserFilled, User } from '@element-plus/icons-vue'
import { getPublicSettings } from '@/api/public'
import type { PublicEmailSuffixPolicy } from '@/api/types'
import { sendVerificationCode, register as apiRegister } from '@/api/auth'
import { getErrorMessage } from '@/utils/error'
import { validateForm } from '@/utils/formValidation'
import EmailSuffixInput from '@/components/common/EmailSuffixInput.vue'
import { disabledEmailSuffixPolicy, emailSuffixPolicyError } from '@/utils/emailSuffixPolicy'

const router = useRouter()
const formRef = ref<FormInstance | null>(null)
const loading = ref(false)
const settingsLoading = ref(true)
const settingsError = ref(false)
const allowRegister = ref(false)

const form = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
  invite: '',
  code: '',
})

const emailVerifyEnabled = ref(false)
const requireInvite = ref(false)
const emailSuffixPolicy = ref<PublicEmailSuffixPolicy>({
  ...disabledEmailSuffixPolicy,
})
const codeLoading = ref(false)
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const rules = computed<FormRules>(() => ({
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
    {
      validator: (_rule, value, callback) => {
        const message = emailSuffixPolicyError(String(value || ''), emailSuffixPolicy.value)
        if (message) {
          callback(new Error(message))
          return
        }
        callback()
      },
      trigger: ['blur', 'change'],
    },
  ],
  code: [{ required: true, message: '请输入验证码' }],
  invite: requireInvite.value
    ? [{ required: true, message: '请输入邀请码', trigger: ['blur', 'change'] }]
    : [],
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
}))

async function loadRegistrationSettings() {
  settingsLoading.value = true
  settingsError.value = false
  try {
    const res = await getPublicSettings()
    if (
      typeof res.data.allow_register !== 'boolean' ||
      typeof res.data.require_invite !== 'boolean' ||
      typeof res.data.email_verify_enabled !== 'boolean' ||
      !res.data.email_suffix_policy
    ) {
      throw new Error('registration settings response is incomplete')
    }
    allowRegister.value = res.data.allow_register
    emailVerifyEnabled.value = res.data.email_verify_enabled
    requireInvite.value = res.data.require_invite
    emailSuffixPolicy.value = res.data.email_suffix_policy
    if (!requireInvite.value) {
      form.invite = ''
      formRef.value?.clearValidate('invite')
    }
  } catch (e) {
    console.error('Failed to fetch settings', e)
    settingsError.value = true
  } finally {
    settingsLoading.value = false
  }
}

onMounted(async () => {
  await loadRegistrationSettings()
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
  } catch (e: unknown) {
    ElMessage.error('发送失败: ' + getErrorMessage(e, '请稍后再试'))
  } finally {
    codeLoading.value = false
  }
}

async function register() {
  if (!(await validateForm(formRef.value))) return

  loading.value = true
  try {
    const payload = {
      username: form.username,
      email: form.email,
      password: form.password,
      code: form.code,
      ...(requireInvite.value ? { invite: form.invite } : {}),
    }

    await apiRegister(payload)
    ElMessage.success('注册成功！即将跳转到登录页面...')

    // 延迟跳转，让用户看到成功消息
    setTimeout(() => {
      router.push('/login')
    }, 1500)
  } catch (e: unknown) {
    ElMessage.error('注册失败: ' + getErrorMessage(e, '注册失败'))
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
