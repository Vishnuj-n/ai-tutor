// Centralized Wails bridge helpers make page-level code easier to debug.
function appBridge() {
  const bridge = window?.go?.main?.App
  if (!bridge) {
    throw new Error('Wails backend bridge unavailable')
  }
  return bridge
}

export function getTopicContent(topicID) {
  return appBridge().GetTopicContent(topicID)
}

export function getReaderTopicBundle(topicID, notebookID = '') {
  return appBridge().GetReaderTopicBundle(topicID, notebookID)
}

export function getAvailableTopics() {
  return appBridge().GetAvailableTopics()
}

export function askAI(topicID, question) {
  return appBridge().AskAI(topicID, question)
}

export function askSocratic(topicID, question) {
  return appBridge().AskSocratic(topicID, question)
}

export function explainReaderSection(sectionID, question = '') {
  return appBridge().ExplainReaderSection(sectionID, question)
}

export function askReaderAI(topicID, notebookID, question, scope, currentPage, chapterStartPage, chapterEndPage) {
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

export function getReadingTask(taskID) {
  return appBridge().GetReadingTask(taskID)
}

export function initializeReadingSession(taskID, notebookID, topicID, startPage, endPage) {
  return appBridge().InitializeReadingSession(taskID, notebookID || '', topicID || '', startPage || 0, endPage || 0)
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

export function generateQuizForPageRange(notebookID, startPage, endPage) {
  return appBridge().GenerateQuizForPageRange(notebookID, startPage, endPage)
}

export function generateQuizSync(topicID, chunkIDs) {
  return appBridge().GenerateQuizSync(topicID, chunkIDs)
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

export function getDailyStudySettings() {
  return appBridge().GetDailyStudySettings()
}

export function updateDailyStudyMinutes(minutes) {
  return appBridge().UpdateDailyStudyMinutes(minutes)
}

// Comprehensive Mode endpoints (Phase 1)
export function generateMarathonQuiz(notebookID, startPage, endPage) {
  return appBridge().GenerateMarathonQuiz(notebookID, startPage, endPage)
}

export function generateMarathonFlashcards(notebookID, startPage, endPage) {
  return appBridge().GenerateMarathonFlashcards(notebookID, startPage, endPage)
}

export function generateComprehensiveExam(notebookID, startPage, endPage) {
  return appBridge().GenerateComprehensiveExam(notebookID, startPage, endPage)
}


export function generateShortAnswerPrompt(topicID) {
  return appBridge().GenerateShortAnswerPrompt(topicID)
}

export function scoreShortAnswer(questionID, userAnswer) {
  return appBridge().ScoreShortAnswer(questionID, userAnswer)
}

export function logReview(topicID, activityType, referenceID, sourceChunkID, score) {
  return appBridge().LogReview(topicID, activityType, referenceID, sourceChunkID, score)
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

export function getNotebooks(topicID = '') {
  return appBridge().GetNotebooks(topicID)
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
