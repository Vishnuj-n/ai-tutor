import { ref } from 'vue'
import {
  getProfiles,
  createProfile,
  updateProfile,
  deleteProfile,
  getNotebooks,
  assignNotebookToProfile,
} from '../services/appApi'
import { useDialog } from './useDialog'

export function useProfiles(errorRef) {
  const { confirm, alert } = useDialog()
  const profiles = ref([])
  const notebooks = ref([])

  const showAddModal = ref(false)

  const showEditModal = ref(false)
  const editProfileId = ref('')
  const editProfileName = ref('')
  const editProfileDeadline = ref('')

  function openEditModal(profile) {
    editProfileId.value = profile.id
    editProfileName.value = profile.name
    const dateObj = new Date(profile.deadline_at * 1000)
    editProfileDeadline.value = dateObj.toISOString().split('T')[0]
    showEditModal.value = true
  }

  function closeEditModal() {
    showEditModal.value = false
    editProfileId.value = ''
    editProfileName.value = ''
    editProfileDeadline.value = ''
  }

  async function handleAddProfile({ name, deadline }) {
    try {
      const res = await createProfile(name, deadline)
      if (res.error) {
        await alert('Profile Error', res.error)
        return
      }
      showAddModal.value = false
      await loadProfiles()
      return true
    } catch (err) {
      await alert('Error', err.message || 'Failed to create profile')
      return false
    }
  }

  async function handleUpdateProfile({ name, deadline }) {
    try {
      const res = await updateProfile(editProfileId.value, name, deadline)
      if (res.error) {
        await alert('Profile Error', res.error)
        return
      }
      closeEditModal()
      await loadProfiles()
      return true
    } catch (err) {
      await alert('Error', err.message || 'Failed to update profile')
      return false
    }
  }

  async function handleDeleteProfile(id) {
    const ok = await confirm({
      title: 'Delete Profile',
      message: 'Are you sure you want to delete this profile? Associated books will become unassigned.',
      confirmText: 'Delete Profile',
      cancelText: 'Cancel',
      type: 'danger',
    })
    if (!ok) return false

    try {
      const res = await deleteProfile(id)
      if (res.error) {
        await alert('Delete Failed', res.error)
        return false
      }
      await loadProfiles()
      return true
    } catch (err) {
      await alert('Error', err.message || 'Failed to delete profile')
      return false
    }
  }

  async function handleAssignProfile(notebookID, profileID) {
    try {
      const res = await assignNotebookToProfile(notebookID, profileID)
      if (res.error) {
        await alert('Assignment Error', res.error)
        return false
      }
      await loadNotebooks()
      return true
    } catch (err) {
      await alert('Error', err.message || 'Failed to assign profile')
      return false
    }
  }

  async function loadProfiles() {
    const res = await getProfiles()
    if (res.error) {
      errorRef.value = res.error
      return
    }
    profiles.value = res.profiles || []
  }

  async function loadNotebooks() {
    const res = await getNotebooks()
    if (res.error) {
      errorRef.value = res.error
      return
    }
    notebooks.value = res || []
  }

  function formatUnixDate(unix) {
    if (!unix) return 'N/A'
    const d = new Date(unix * 1000)
    return d.toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' })
  }

  return {
    profiles,
    notebooks,
    showAddModal,
    showEditModal,
    editProfileName,
    editProfileDeadline,
    openEditModal,
    closeEditModal,
    handleAddProfile,
    handleUpdateProfile,
    handleDeleteProfile,
    handleAssignProfile,
    loadProfiles,
    loadNotebooks,
    formatUnixDate,
  }
}
