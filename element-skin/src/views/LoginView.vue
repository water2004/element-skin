<template>
  <div style="max-width:500px; margin:0 auto; padding:40px 20px">
    <el-card>
      <h2>登录</h2>
      <el-form :model="form">
        <el-form-item label="Email">
          <el-input v-model="form.email" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="login">登录</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { reactive } from 'vue'
import axios from 'axios'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

const form = reactive({ email: '', password: '' })
const router = useRouter()

async function login() {
  try {
    // 使用 authserver 登录接口
    const res = await axios.post('/authserver/authenticate', {
      username: form.email,
      password: form.password,
      requestUser: true,
    })

    if (res.data.accessToken) {
      localStorage.setItem('accessToken', res.data.accessToken)
      console.log('accessToken saved')
    }
    if (res.data.token) {
      localStorage.setItem('jwt', res.data.token)
      console.log('jwt saved')
    }

    ElMessage.success('登录成功')

    // 等待一下再跳转，确保 localStorage 保存完成
    setTimeout(() => {
      router.push('/dashboard')
    }, 300)
  } catch (e) {
    ElMessage.error('登录失败: ' + (e.response?.data?.errorMessage || e.message))
  }
}
</script>
