import { ref } from 'vue'
import { loginStudent, signUpStudent, logoutStudent } from '../services/appApi'
import { useDialog } from './useDialog'

export function useAuth(reloadFn, errorRef, successRef) {
  const { confirm } = useDialog()

  const isSignUpMode = ref(false)
  const loginUsername = ref('')
  const loginPassword = ref('')
  const signupUsername = ref('')
  const signupPassword = ref('')
  const signupClassroomCode = ref('')
  const loginError = ref('')
  const loggingIn = ref(false)

  function toggleAuthMode() {
    isSignUpMode.value = !isSignUpMode.value
    loginError.value = ''
  }

  async function handleLogin() {
    if (!loginUsername.value.trim() || !loginPassword.value.trim()) {
      loginError.value = 'Username and password are required.'
      return
    }
    loginError.value = ''
    loggingIn.value = true
    try {
      const res = await loginStudent(
        loginUsername.value.trim(),
        loginPassword.value.trim()
      )
      if (res.error) {
        loginError.value = res.error
      } else {
        loginUsername.value = ''
        loginPassword.value = ''
        await reloadFn()
        successRef.value = 'Successfully signed in & classroom study profile active!'
        setTimeout(() => (successRef.value = ''), 4000)
      }
    } catch (err) {
      loginError.value = err.message || 'An error occurred during sign in.'
    } finally {
      loggingIn.value = false
    }
  }

  async function handleSignUp() {
    if (!signupUsername.value.trim() || !signupPassword.value.trim() || !signupClassroomCode.value.trim()) {
      loginError.value = 'Username, password, and classroom code are all required.'
      return
    }
    if (signupPassword.value.trim().length < 6) {
      loginError.value = 'Password must be at least 6 characters.'
      return
    }
    loginError.value = ''
    loggingIn.value = true
    try {
      const res = await signUpStudent(
        signupUsername.value.trim(),
        signupPassword.value.trim(),
        signupClassroomCode.value.trim().toUpperCase()
      )
      if (res.error) {
        loginError.value = res.error
      } else {
        signupUsername.value = ''
        signupPassword.value = ''
        signupClassroomCode.value = ''
        isSignUpMode.value = false
        await reloadFn()
        successRef.value = 'Classroom account created & dedicated study profile active!'
        setTimeout(() => (successRef.value = ''), 4000)
      }
    } catch (err) {
      loginError.value = err.message || 'An error occurred during sign up.'
    } finally {
      loggingIn.value = false
    }
  }

  async function handleLogout() {
    const ok = await confirm({
      title: 'Sign Out Profile',
      message: 'Are you sure you want to sign out? This will disable cloud sync for this study profile.',
      confirmText: 'Sign Out',
      cancelText: 'Cancel',
      type: 'warning',
    })
    if (!ok) return
    errorRef.value = ''
    successRef.value = ''
    try {
      const res = await logoutStudent()
      if (res.error) {
        errorRef.value = res.error
      } else {
        await reloadFn()
        successRef.value = 'Signed out profile successfully.'
        setTimeout(() => (successRef.value = ''), 4000)
      }
    } catch (err) {
      errorRef.value = err.message || 'Failed to sign out.'
    }
  }

  return {
    isSignUpMode,
    loginUsername,
    loginPassword,
    signupUsername,
    signupPassword,
    signupClassroomCode,
    loginError,
    loggingIn,
    toggleAuthMode,
    handleLogin,
    handleSignUp,
    handleLogout,
  }
}
