<template>
  <div>
    <el-card>
      <div style="display:flex;justify-content:space-between;align-items:center;">
        <h3>用户列表</h3>
        <div>
          <el-button type="primary" @click="fetchUsers">刷新</el-button>
        </div>
      </div>
      <el-table :data="users" style="width:100%" v-if="users.length">
        <el-table-column prop="email" label="Email" />
        <el-table-column prop="lang" label="Language" />
        <el-table-column prop="is_admin" label="Admin" :formatter="(row)=>row.is_admin? 'Yes':'No'" />
        <el-table-column label="操作">
          <template #default="{ row }">
            <el-button size="small" @click="resetPassword(row)">重置密码</el-button>
            <el-button size="small" type="danger" @click="deleteUser(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:12px; text-align:right">
        <el-pagination background :page-size="perPage" :current-page.sync="page" :total="total" @current-change="onPageChange" />
      </div>
    </el-card>

    <el-card style="margin-top:16px;">
      <div style="display:flex;justify-content:space-between;align-items:center;">
        <h3>邀请码</h3>
        <el-button type="primary" @click="genInvite">生成邀请码</el-button>
      </div>
      <div v-if="invites.length" style="margin-top:8px">
        <div v-for="inv in invites" :key="inv.code">{{ inv.code }} - used_by: {{ inv.used_by || '-' }}</div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'

const users = ref([])
const invites = ref([])
const page = ref(1)
const perPage = ref(20)
const total = ref(0)

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

async function fetchUsers() {
  const res = await axios.get('/admin/users/list', { params: { page: page.value, per_page: perPage.value }, headers: authHeaders() })
  users.value = res.data.items
  total.value = res.data.total
}

async function fetchInvites() {
  const res = await axios.get('/admin/invites', { headers: authHeaders() })
  invites.value = res.data
}

async function genInvite() {
  const res = await axios.post('/admin/invite/generate', {}, { headers: authHeaders() })
  invites.value.unshift(res.data)
}

async function resetPassword(row) {
  const np = prompt('输入新密码（将直接设置）')
  if (!np) return
  await axios.post('/admin/users/reset-password', { user_id: row.id, new_password: np }, { headers: authHeaders() })
  alert('已重置')
}

async function deleteUser(row) {
  if (!confirm('确认删除用户 ' + row.email + ' ?')) return
  await axios.post('/admin/users/delete', { user_id: row.id }, { headers: authHeaders() })
  fetchUsers()
}

function onPageChange(p) {
  page.value = p
  fetchUsers()
}

onMounted(() => {
  fetchUsers()
  fetchInvites()
})
</script>
