<template>
  <div class="register-container bg-gradient-purple">
    <div class="register-card">
      <div class="register-header">
        <h1>注册账号</h1>
        <p>创建一个新账号来开始使用</p>
      </div>

      <el-form :model="form" :rules="rules" ref="formRef" label-position="top" size="large">
        <el-form-item label="邮箱地址" prop="email">
          <el-input
            v-model="form.email"
            placeholder="请输入邮箱地址"
            :prefix-icon="Message"
            @keyup.enter="register"
          />
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
import { Message, Lock, Ticket, UserFilled } from '@element-plus/icons-vue'

const router = useRouter()
const formRef = ref(null)
const loading = ref(false)

const form = reactive({
  email: '',
  password: '',
  confirmPassword: '',
  invite: ''
})

const rules = {
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
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

async function register() {
  try {
    await formRef.value.validate()
    loading.value = true

    // 在发送前trim邀请码
    const payload = {
      email: form.email,
      password: form.password,
      invite: form.invite ? form.invite.trim() : ''
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
}

.register-card {
  width: 100%;
  max-width: 440px;
  background: #fff;
  border-radius: 16px;
  padding: 40px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  animation: slideUp 0.5s ease-out;
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
  color: #303133;
}

.register-header p {
  margin: 0;
  font-size: 14px;
  color: #909399;
}

.register-footer {
  text-align: center;
  margin-top: 24px;
  color: #606266;
  font-size: 14px;
}

:deep(.el-form-item__label) {
  font-weight: 500;
  color: #606266;
}

:deep(.el-input__inner) {
  height: 44px;
}
</style>
