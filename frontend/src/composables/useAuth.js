import { ref } from 'vue'
import { loginStudent, logoutStudent } from '../services/appApi'

export function useAuth(reloadFn, errorRef, successRef) {
  const loginUsername = ref('')
  const loginPassword = ref('')
  const loginClassroomCode = ref('')
  const loginError = ref('')
  const loggingIn = ref(false)

  async function handleLogin() {
    if (
      !loginUsername.value.trim() ||
      !loginPassword.value.trim() ||
      !loginClassroomCode.value.trim()
    ) {
      loginError.value = 'All fields are required.'
      return
    }
    loginError.value = ''
    loggingIn.value = true
    try {
      const res = await loginStudent(
        loginUsername.value.trim(),
        loginPassword.value.trim(),
        loginClassroomCode.value.trim().toUpperCase()
      )
      if (res.error) {
        loginError.value = res.error
      } else {
        loginUsername.value = ''
        loginPassword.value = ''
        loginClassroomCode.value = ''
        await reloadFn()
        successRef.value = 'Successfully signed in and cloud sync enabled!'
        setTimeout(() => (successRef.value = ''), 4000)
      }
    } catch (err) {
      loginError.value = err.message || 'An error occurred during sign in.'
    } finally {
      loggingIn.value = false
    }
  }

  async function handleLogout() {
    if (!confirm('Are you sure you want to sign out? This will disable cloud sync.')) return
    errorRef.value = ''
    successRef.value = ''
    try {
      const res = await logoutStudent()
      if (res.error) {
        errorRef.value = res.error
      } else {
        await reloadFn()
        successRef.value = 'Signed out successfully.'
        setTimeout(() => (successRef.value = ''), 4000)
      }
    } catch (err) {
      errorRef.value = err.message || 'Failed to sign out.'
    }
  }

  return {
    loginUsername,
    loginPassword,
    loginClassroomCode,
    loginError,
    loggingIn,
    handleLogin,
    handleLogout,
  }
}
