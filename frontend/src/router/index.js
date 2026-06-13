import { createRouter, createWebHashHistory } from 'vue-router'

import Dashboard from '../pages/Dashboard.vue'
import Reader from '../pages/Reader.vue'
import Quiz from '../pages/Quiz.vue'
import Flashcards from '../pages/Flashcards.vue'
import Socratic from '../pages/Socratic.vue'
import Tools from '../pages/Tools.vue'
import ToolPlaceholder from '../pages/ToolPlaceholder.vue'
import Settings from '../pages/Settings.vue'
import Notebook from '../pages/Notebook.vue'
import Onboarding from '../pages/Onboarding.vue'
import { isOnboarded } from '../services/appApi'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'dashboard', component: Dashboard },
  { path: '/reader', name: 'reader', component: Reader },
  { path: '/quiz', name: 'quiz', component: Quiz },
  { path: '/flashcards', name: 'flashcards', component: Flashcards },
  {
    path: '/examiner',
    name: 'examiner',
    component: () => import('../pages/WrittenAssessment.vue'),
  },
  { path: '/socratic', name: 'socratic', component: Socratic },
  { path: '/tools', name: 'tools', component: Tools },
  {
    path: '/tools/written-assessment',
    redirect: '/examiner',
    meta: { title: 'Written Assessment' },
  },
  {
    path: '/tools/acronym-generator',
    name: 'acronym-generator',
    component: ToolPlaceholder,
    meta: { title: 'Acronym Generator' },
  },
  {
    path: '/tools/mindmap-generator',
    name: 'mindmap-generator',
    component: ToolPlaceholder,
    meta: { title: 'Mindmap Generator' },
  },
  { path: '/notebooks', name: 'notebooks', component: Notebook },
  { path: '/settings', name: 'settings', component: Settings },
  { path: '/onboarding', name: 'onboarding', component: Onboarding },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach(async (to, from, next) => {
  if (to.path === '/onboarding') {
    next()
    return
  }
  try {
    const res = await isOnboarded()
    if (res && res.onboarded === false) {
      next('/onboarding')
    } else {
      next()
    }
  } catch (err) {
    next()
  }
})

export default router
