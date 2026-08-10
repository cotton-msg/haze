import { createRouter, createWebHistory } from "vue-router"

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", redirect: "/chats" },
    {
      path: "/login",
      name: "login",
      component: () => import("../pages/LoginPage.vue"),
    },
    {
      path: "/auth/callback",
      name: "auth-callback",
      component: () => import("../pages/AuthCallback.vue"),
    },
    {
      path: "/chats",
      name: "chats",
      component: () => import("../pages/ChatLayout.vue"),
      meta: { requiresAuth: true },
    },
    {
      path: "/chats/:id",
      name: "chat",
      component: () => import("../pages/ChatLayout.vue"),
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach((to) => {
  const token = localStorage.getItem("access_token")
  if (to.meta.requiresAuth && !token) {
    return "/login"
  }
})

export default router
