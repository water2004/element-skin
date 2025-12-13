import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import AdminView from '../views/AdminView.vue'
import RegisterView from '../views/RegisterView.vue'
import LoginView from '../views/LoginView.vue'
import UserDashboard from '../views/UserDashboard.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/login',
      name: 'login',
      component: LoginView,
    },
    {
      path: '/register',
      name: 'register',
      component: RegisterView,
    },
    {
      path: '/admin',
      name: 'admin',
      component: AdminView,
      redirect: '/admin/settings',
      children: [
        {
          path: 'settings',
          name: 'admin-settings',
          component: AdminView,
        },
        {
          path: 'users',
          name: 'admin-users',
          component: AdminView,
        },
        {
          path: 'invites',
          name: 'admin-invites',
          component: AdminView,
        },
      ],
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: UserDashboard,
      redirect: '/dashboard/wardrobe',
      children: [
        {
          path: 'wardrobe',
          name: 'wardrobe',
          component: UserDashboard,
        },
        {
          path: 'roles',
          name: 'roles',
          component: UserDashboard,
        },
        {
          path: 'profile',
          name: 'profile',
          component: UserDashboard,
        },
      ],
    },
    {
      path: '/about',
      name: 'about',
      // route level code-splitting
      // this generates a separate chunk (About.[hash].js) for this route
      // which is lazy-loaded when the route is visited.
      component: () => import('../views/AboutView.vue'),
    },
  ],
})

export default router
