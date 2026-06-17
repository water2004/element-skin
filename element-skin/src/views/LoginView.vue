<template>
  <div
    class="flex items-center justify-center min-h-screen p-5 bg-[var(--color-background-hero-light)] dark:bg-[var(--color-background-hero-dark)] transition-[background] duration-300"
  >
    <div
      class="w-full max-w-[440px] bg-[var(--color-card-background)] rounded-[16px] p-10 shadow-[0_8px_32px_rgba(0,0,0,0.1)] animate-slide-up border border-[var(--color-border)] transition-colors"
    >
      <div class="text-center mb-8">
        <h1 class="m-0 mb-2 text-[28px] font-semibold text-[var(--color-heading)]">欢迎回来</h1>
        <p class="m-0 text-sm text-[var(--color-text-light)]">登录您的账号</p>
      </div>

      <el-form :model="form" :rules="rules" ref="formRef" label-position="top" size="large">
        <el-form-item label="邮箱地址" prop="email">
          <el-input
            v-model="form.email"
            placeholder="请输入邮箱地址"
            :prefix-icon="Message"
            @keyup.enter="login"
          />
        </el-form-item>

        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="请输入密码"
            :prefix-icon="Lock"
            show-password
            @keyup.enter="login"
          />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="login" :loading="loading" class="w-full">
            <el-icon v-if="!loading"><Right /></el-icon>
            {{ loading ? '登录中...' : '登录' }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="text-right -mt-3 mb-5" v-if="emailVerifyEnabled">
        <el-button link type="info" @click="$router.push('/reset-password')">
          忘记密码？
        </el-button>
      </div>

      <div class="text-center mt-6 text-[var(--color-text)] text-sm transition-colors">
        <span>还没有账号？</span>
        <el-button link type="primary" @click="$router.push('/register')"> 立即注册 </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, inject, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Message, Lock, Right } from '@element-plus/icons-vue'
import { getPublicSettings } from '@/api/public'
import { siteLogin } from '@/api/auth'
import { getErrorMessage, isValidationError } from '@/utils/error'

const router = useRouter()
const fetchMe = inject<() => Promise<void>>('fetchMe')
const formRef = ref<FormInstance | null>(null)
const loading = ref(false)

const form = reactive({
  email: '',
  password: '',
})

const emailVerifyEnabled = ref(false)

onMounted(async () => {
  try {
    const res = await getPublicSettings()
    emailVerifyEnabled.value = res.data.email_verify_enabled ?? false
  } catch (e) {
    console.error('Failed to fetch settings', e)
  }
})

const rules: FormRules = {
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' },
  ],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function login() {
  try {
    if (!formRef.value) return
    await formRef.value.validate()
    loading.value = true

    // 使用站点登录接口（token 自动存入 HttpOnly Cookie）
    await siteLogin({
      email: form.email,
      password: form.password,
    })

    // Cookie 已设置，刷新顶栏共享的登录状态，避免必须整页刷新
    if (fetchMe) await fetchMe()

    ElMessage.success('登录成功！')
    router.push('/dashboard')
  } catch (e: unknown) {
    if (!isValidationError(e)) {
      ElMessage.error('登录失败: ' + getErrorMessage(e, '登录失败'))
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
