import axios from 'axios'

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '',
  withCredentials: true,
})

export default apiClient
