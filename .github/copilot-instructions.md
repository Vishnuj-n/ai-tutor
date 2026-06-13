# AI Tutor – Copilot Instructions

## Architecture Rules
- Keep implementation simple and explicit
- Do NOT use LangChain or complex abstractions
- Use OpenAI-compatible APIs with a simple provider interface
- Keep all AI calls stateless

## RAG Rules
- Always scope retrieval to current topic_id
- Enforce token limits strictly

## Go Code Rules
- Avoid unnecessary interfaces
- Use structs only when needed
- Prefer simple functions over large abstractions
- Use pointers only when modifying data

## UX Rules
- No chatbot mode
- Ask AI is contextual (inside reading/review only)

## General
- Optimize for readability over cleverness
- Assume solo developer maintainability