import { createRouter, createWebHashHistory } from 'vue-router';
import Login from '../pages/Login.vue';
import Overview from '../pages/Overview.vue';
import Students from '../pages/Students.vue';
import Assignments from '../pages/Assignments.vue';

const routes = [
  { path: '/', redirect: '/overview' },
  { path: '/login', name: 'login', component: Login },
  { path: '/overview', name: 'overview', component: Overview },
  { path: '/students', name: 'students', component: Students },
  { path: '/assignments', name: 'assignments', component: Assignments }
];

const router = createRouter({
  history: createWebHashHistory(),
  routes
});

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('session_token') || sessionStorage.getItem('session_token');
  const cls = localStorage.getItem('classroom_code') || sessionStorage.getItem('classroom_code');
  const isAuthenticated = Boolean(token && cls);

  if (to.path !== '/login' && !isAuthenticated) {
    next('/login');
  } else if (to.path === '/login' && isAuthenticated) {
    next('/overview');
  } else {
    next();
  }
});

export default router;
