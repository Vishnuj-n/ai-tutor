// Centralized Wails bridge helpers make page-level code easier to debug.
function appBridge() {
  const bridge = window?.go?.app?.App || window?.go?.main?.App
  if (!bridge) {
    throw new Error('Wails backend bridge unavailable')
  }
  return bridge
}

export function getReaderTopicBundle(topicID, notebookID = '') {
  return appBridge().GetReaderTopicBundle(topicID, notebookID)
}

export function getAvailableTopics() {
  return appBridge().GetAvailableTopics()
}

export function askSocratic(notebookID, topicID, question, conversationHistory = []) {
  return appBridge().AskSocratic(notebookID, topicID, question, conversationHistory)
}

export function askReaderAI(
  topicID,
  notebookID,
  question,
  scope,
  currentPage,
  chapterStartPage,
  chapterEndPage
) {
  return appBridge().AskReaderAI(
    topicID,
    notebookID || '',
    question,
    scope,
    currentPage || 0,
    chapterStartPage || 0,
    chapterEndPage || 0
  )
}

export function activateTask(taskID) {
  return appBridge().ActivateTask(taskID)
}

export function initializeReadingSession(taskID, notebookID, topicID, startPage, endPage) {
  return appBridge().InitializeReadingSession(
    taskID,
    notebookID || '',
    topicID || '',
    startPage || 0,
    endPage || 0
  )
}

export async function completeReading(taskID) {
  console.warn('[COMPLETE_SESSION] appApi.completeReading request', { taskID })
  try {
    const response = await appBridge().CompleteReading(taskID)
    console.warn('[COMPLETE_SESSION] appApi.completeReading raw backend response', response)
    return response
  } catch (err) {
    console.error('[COMPLETE_SESSION] appApi.completeReading thrown error', err)
    throw err
  }
}

export function getTask(taskID) {
  return appBridge().GetTask(taskID)
}

export function GetTaskContext(taskID) {
  return appBridge().GetTaskContext(taskID)
}

export function generateQuizForPageRange(notebookID, startPage, endPage) {
  return appBridge().GenerateQuizForPageRange(notebookID, startPage, endPage)
}

export function submitQuizAttempt(taskID, answers) {
  return appBridge().SubmitQuizAttempt(taskID, answers)
}

export function generateFlashcardsForQuizTask(taskID) {
  return appBridge().GenerateFlashcardsForQuizTask(taskID)
}

export function retryFlashcardGeneration(taskID) {
  return appBridge().RetryFlashcardGeneration(taskID)
}

export function getTodayPlan() {
  return appBridge().GetTodayPlan()
}

export function getDashboardOverview(timezoneOffsetMinutes) {
  return appBridge().GetDashboardOverview(timezoneOffsetMinutes)
}

// Comprehensive Mode endpoints (Phase 1)
export function generateManualFlashcards(notebookID, startPage, endPage) {
  return appBridge().GenerateManualFlashcards(notebookID, startPage, endPage)
}

export function generateComprehensiveExam(notebookID, startPage, endPage) {
  return appBridge().GenerateComprehensiveExam(notebookID, startPage, endPage)
}

export function scoreShortAnswer(questionID, userAnswer) {
  return appBridge().ScoreShortAnswer(questionID, userAnswer)
}

export function getReviewSession(taskID, notebookID = '') {
  return appBridge().GetReviewSession(taskID, notebookID)
}

export function recordCardReview(taskID, cardID, rating) {
  return appBridge().RecordCardReview(taskID, cardID, rating)
}

export function completeReviewSession(taskID) {
  return appBridge().CompleteReviewSession(taskID)
}

export function suspendFlashcard(taskID, cardID) {
  return appBridge().SuspendFlashcard(taskID, cardID)
}

export function forceDueFlashcardsNow() {
  return appBridge().ForceDueFlashcardsNow()
}

export function getNotebooks(topicID = '', profileID = '') {
  return appBridge().GetNotebooks(topicID, profileID)
}

export function getNotebookTopicTree() {
  return appBridge().GetNotebookTopicTree()
}

export function uploadNotebook(fileBytes, fileName) {
  return appBridge().UploadNotebook(fileBytes, fileName)
}

export function uploadNotebookFromPath(filePath) {
  return appBridge().UploadNotebookFromPath(filePath)
}

export function draftNotebookSyllabus(notebookID, regenerate = false) {
  return appBridge().DraftNotebookSyllabus(notebookID, regenerate)
}

export function aiCleanupNotebookSyllabus(notebookID) {
  return appBridge().AICleanupNotebookSyllabus(notebookID)
}

export function confirmNotebookSyllabus(notebookID, chapters) {
  return appBridge().ConfirmNotebookSyllabus(notebookID, chapters)
}

export function updateNotebookTitle(notebookID, title) {
  return appBridge().UpdateNotebookTitle(notebookID, title)
}

export function updateNotebookPriority(notebookID, priority) {
  return appBridge().UpdateNotebookPriority(notebookID, priority)
}

export function deleteNotebook(notebookID) {
  return appBridge().DeleteNotebook(notebookID)
}

export function getUserSettings() {
  return appBridge().GetUserSettings()
}

export function updateUserSettings(settings) {
  return appBridge().UpdateUserSettings(settings)
}

export function trackAnalyticsEvent(eventType, fileHash, pageNumber, metadata) {
  const metaStr = typeof metadata === 'object' ? JSON.stringify(metadata) : metadata || ''
  return appBridge().TrackAnalyticsEvent(eventType, fileHash || '', pageNumber || 0, metaStr)
}

export function getLLMSettings() {
  return appBridge().GetLLMSettings()
}

export function getLLMProviderPreset(provider) {
  return appBridge().GetLLMProviderPreset(provider)
}

export function updateLLMSettings(settings) {
  return appBridge().UpdateLLMSettings(settings)
}

export function saveLLMAPIKey(tier, key) {
  return appBridge().SaveLLMAPIKey(tier, key)
}

export function deleteLLMAPIKey(tier) {
  return appBridge().DeleteLLMAPIKey(tier)
}

export function initializeRAG() {
  return appBridge().InitializeRAG()
}

export function getProfiles() {
  return appBridge().GetProfiles()
}

export function createProfile(name, deadlineStr) {
  return appBridge().CreateProfile(name, deadlineStr)
}

export function updateProfile(id, name, deadlineStr) {
  return appBridge().UpdateProfile(id, name, deadlineStr)
}

export function deleteProfile(id) {
  return appBridge().DeleteProfile(id)
}

export function assignNotebookToProfile(notebookID, profileID) {
  return appBridge().AssignNotebookToProfile(notebookID, profileID)
}

export function updateNotebookStudyStatus(notebookID, studyStatus) {
  return appBridge().UpdateNotebookStudyStatus(notebookID, studyStatus)
}

export function isOnboarded() {
  return appBridge().IsOnboarded()
}

export function triggerCloudSync() {
  return appBridge().TriggerCloudSync()
}

export function getProfileDailyPace(profileID) {
  return appBridge().GetProfileDailyPace(profileID)
}

export function completeSocraticRescue(taskID) {
  return appBridge().CompleteSocraticRescue(taskID)
}

export function completeMilestoneExam(taskID) {
  return appBridge().CompleteMilestoneExam(taskID)
}

export function getFlashcardDueTimeline(timezoneOffsetMinutes) {
  return appBridge().GetFlashcardDueTimeline(timezoneOffsetMinutes)
}

export function getAppEnv() {
  return appBridge().GetAppEnv()
}

export function loginStudent(username, password) {
  return appBridge().LoginStudent(username, password)
}

export function signUpStudent(username, password, classroomCode) {
  return appBridge().SignUpStudent(username, password, classroomCode)
}

export function logoutStudent() {
  return appBridge().LogoutStudent()
}

export function getCloudConfig() {
  return appBridge().GetCloudConfig()
}

export function getTopicSectionsContent(topicID, notebookID) {
  return appBridge().GetTopicSectionsContent(topicID, notebookID)
}

export function devForceSocraticRescue(notebookID, topicID) {
  return appBridge().DevForceSocraticRescue(notebookID, topicID)
}

export function devForceFlashcardGenerate(notebookID) {
  return appBridge().DevForceFlashcardGenerate(notebookID)
}

export function checkForUpdates() {
  return appBridge().CheckForUpdates()
}

export function openRepoURL() {
  return appBridge().OpenRepoURL()
}

export function logFrontendEvent(level, component, event, details = '') {
  try {
    const bridge = window?.go?.main?.App
    if (bridge && bridge.LogFrontendEvent) {
      const detailsStr = typeof details === 'string' ? details : JSON.stringify(details)
      bridge.LogFrontendEvent(level, component, event, detailsStr)
    }
  } catch (err) {
    console.error('Failed to forward log to backend:', err)
  }
}
