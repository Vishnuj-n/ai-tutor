// Centralized Wails bridge helpers make page-level code easier to debug.
function appBridge() {
  const bridge = window?.go?.main?.App
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

export function askSocratic(notebookID, topicID, question) {
  return appBridge().AskSocratic(notebookID, topicID, question)
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

export function getTodayPlan() {
  return appBridge().GetTodayPlan()
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

export function updateUserSettings(
  maxFlashcards,
  startTime,
  endTime,
  remindersEnabled,
  activeProfileID,
  skipToReading,
  syncURL,
  apiToken,
  theme,
  ragEnabled,
  ragNotebookChapter,
  ragEntireNotebook,
  ragQueueStudy,
  defaultRemedialStrategy
) {
  return appBridge().UpdateUserSettings(
    maxFlashcards,
    startTime,
    endTime,
    remindersEnabled,
    activeProfileID,
    skipToReading,
    syncURL,
    apiToken,
    theme,
    ragEnabled,
    ragNotebookChapter,
    ragEntireNotebook,
    ragQueueStudy,
    defaultRemedialStrategy
  )
}

export function getRemedialStrategy() {
  return appBridge().GetRemedialStrategy()
}

export function setRemedialStrategy(strategy) {
  return appBridge().SetRemedialStrategy(strategy)
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

export function getAppEnv() {
  return appBridge().GetAppEnv()
}

export function getTopicSectionsContent(topicID, notebookID) {
  return appBridge().GetTopicSectionsContent(topicID, notebookID)
}

export function devForceSocraticRescue(notebookID, topicID) {
  return appBridge().DevForceSocraticRescue(notebookID, topicID)
}

export function devForceFlashcardSync(notebookID) {
  return appBridge().DevForceFlashcardSync(notebookID)
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
