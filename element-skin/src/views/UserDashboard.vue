<template>
  <div class="app-container">
    <el-card>
      <h2>个人面板</h2>
      <div v-if="user">
        <p><strong>Email:</strong> {{ user.email }}</p>
        <p><strong>Language:</strong> {{ user.lang }}</p>
        <p><strong>Profiles:</strong></p>
        <el-table :data="user.profiles" style="width:100%">
          <el-table-column prop="name" label="Name" />
          <el-table-column prop="model" label="Model" />
          <el-table-column label="Skin">
            <template #default="{ row }">
              <div v-if="row.skin_hash">
                <img :src="texturesUrl(row.skin_hash)" width="64" />
              </div>
              <div v-else>-</div>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-card>

    <div style="margin-top:16px">
      <upload-skin />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import UploadSkin from '../components/UploadSkin.vue'

const user = ref(null)

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function texturesUrl(hash) {
  if (!hash) return ''
  return (import.meta.env.VITE_API_BASE || '') + '/static/textures/' + hash + '.png'
}

async function fetchMe() {
  try {
    const res = await axios.get('/me', { headers: authHeaders() })
    user.value = res.data
  } catch (e) {
    console.error(e)
  }
}

onMounted(() => {
  fetchMe()
})
</script>
