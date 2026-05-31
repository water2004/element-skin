<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-header">
        <h1>欢迎回来</h1>
        <p>登录您的账号</p>
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
          <el-button
            type="primary"
            @click="login"
            :loading="loading"
            style="width: 100%"
          >
            <el-icon v-if="!loading"><Right /></el-icon>
            {{ loading ? '登录中...' : '登录' }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="login-actions" v-if="emailVerifyEnabled">
        <el-button link type="info" @click="$router.push('/reset-password')">
          忘记密码？
        </el-button>
      </div>

      <div class="login-footer">
        <span>还没有账号？</span>
        <el-button link type="primary" @click="$router.push('/register')">
          立即注册
        </el-button>
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

const router = useRouter()
const fetchMe = inject<() => Promise<void>>('fetchMe')
const formRef = ref<FormInstance | null>(null)
const loading = ref(false)

const form = reactive({
  email: '',
  password: ''
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
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' }
  ]
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
  } catch (e: any) {
    if (e.response?.data?.detail) {
      ElMessage.error('登录失败: ' + e.response.data.detail)
    } else if (e.message && !e.message.includes('validate')) {
      ElMessage.error('登录失败: ' + e.message)
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: var(--color-background-hero-light);
  transition: background 0.3s ease;
}

.login-card {
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

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.login-header h1 {
  margin: 0 0 8px 0;
  font-size: 28px;
  font-weight: 600;
  color: var(--color-heading);
  transition: color 0.3s ease;
}

.login-header p {
  margin: 0;
  font-size: 14px;
  color: var(--color-text-light);
  transition: color 0.3s ease;
}

.login-actions {
  text-align: right;
  margin-top: -12px;
  margin-bottom: 20px;
}

.login-footer {
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
</style>
