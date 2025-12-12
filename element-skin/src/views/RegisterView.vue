<template>
  <el-card>
    <h2>注册</h2>
    <el-form :model="form">
      <el-form-item label="Email">
        <el-input v-model="form.email" />
      </el-form-item>
      <el-form-item label="密码">
        <el-input v-model="form.password" type="password" />
      </el-form-item>
      <el-form-item label="邀请码 (若需要)">
        <el-input v-model="form.invite" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="register">注册</el-button>
      </el-form-item>
    </el-form>
  </el-card>
</template>

<script setup>
import { reactive } from 'vue'
import axios from 'axios'

const form = reactive({ email: '', password: '', invite: '' })

async function register() {
  try {
    const res = await axios.post('/register', { ...form })
    alert('注册成功: ' + res.data.id)
  } catch (e) {
    alert('注册失败: ' + (e.response?.data?.detail || e.message))
  }
}
</script>
