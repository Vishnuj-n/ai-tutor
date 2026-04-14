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

export function getReaderTopicBundle(topicID) {
  return appBridge().GetReaderTopicBundle(topicID)
}

export function getAvailableTopics() {
  return appBridge().GetAvailableTopics()
}

export function askAI(topicID, question) {
  return appBridge().AskAI(topicID, question)
}

export function explainReaderSection(sectionID, question = '') {
  return appBridge().ExplainReaderSection(sectionID, question)
}

export function getTodayPlan() {
  return appBridge().GetTodayPlan()
}

export function generateQuiz(topicID) {
  return appBridge().GenerateQuiz(topicID)
}

export function scoreAnswer(questionID, userAnswer) {
  return appBridge().ScoreAnswer(questionID, userAnswer)
}

export function generateFlashcards(topicID) {
  return appBridge().GenerateFlashcards(topicID)
}

export function getFlashcards(topicID, dueOnly = true) {
  return appBridge().GetFlashcards(topicID, dueOnly)
}

export function recordFlashcardReview(cardID, rating) {
  return appBridge().RecordFlashcardReview(cardID, rating)
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

export function deleteNotebook(notebookID) {
  return appBridge().DeleteNotebook(notebookID)
}
