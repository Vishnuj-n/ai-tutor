import { createRouter, createWebHashHistory } from 'vue-router'

import Dashboard from '../pages/Dashboard.vue'
import Reader from '../pages/Reader.vue'
import Quiz from '../pages/Quiz.vue'
import Flashcards from '../pages/Flashcards.vue'
import Socratic from '../pages/Socratic.vue'
import Settings from '../pages/Settings.vue'
import Notebook from '../pages/Notebook.vue'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'dashboard', component: Dashboard },
  { path: '/reader', name: 'reader', component: Reader },
  { path: '/quiz', name: 'quiz', component: Quiz },
  { path: '/flashcards', name: 'flashcards', component: Flashcards },
  { path: '/socratic', name: 'socratic', component: Socratic },
  { path: '/notebooks', name: 'notebooks', component: Notebook },
  { path: '/settings', name: 'settings', component: Settings },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router
