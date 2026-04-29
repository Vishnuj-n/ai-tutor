package notebook

import (
	"fmt"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
)

// BuildTopicGroupsFromChapters builds topic groups and chunks from document chapters.
func BuildTopicGroupsFromChapters(notebookID string, doc *ExtractedDocument, topicIDs []string, chapters []models.SyllabusChapterDraft) ([]db.NotebookTopicIngestionGroup, []models.Chunk) {
	if doc == nil || len(doc.Sections) == 0 || len(topicIDs) == 0 || len(chapters) == 0 || len(topicIDs) != len(chapters) {
		return nil, nil
	}

	builders := make([]*topicGroupBuilder, len(topicIDs))
	for i := range topicIDs {
		builders[i] = &topicGroupBuilder{topicID: topicIDs[i]}
	}

	allChunks := make([]models.Chunk, 0)
	for sectionIndex, section := range doc.Sections {
		sectionText := strings.TrimSpace(section.Text)
		if sectionText == "" {
			continue
		}
		page := section.PageNum
		if page <= 0 {
			page = 1
		}

		topicIdx := chapterIndexForPage(page, chapters)
		if topicIdx < 0 {
			continue
		}

		builder := builders[topicIdx]
		builder.order++
		parentID := fmt.Sprintf("nbp_%s_%02d_%04d", notebookID, topicIdx+1, builder.order)
		heading := strings.TrimSpace(section.Heading)
		if heading == "" {
			heading = fmt.Sprintf("Section %d", sectionIndex+1)
		}

		builder.parents = append(builder.parents, db.NotebookParentInput{
			ID:         parentID,
			Heading:    heading,
			Content:    sectionText,
			OrderIndex: builder.order,
		})

		chunkTexts := SplitPageIntoSemanticChunks(sectionText, DefaultSemanticChunkTargetWords)
		for chunkIndex, chunkText := range chunkTexts {
			chunkID := fmt.Sprintf("nbc_%s_%02d_%04d_%03d", notebookID, topicIdx+1, builder.order, chunkIndex+1)
			builder.chunks = append(builder.chunks, db.NotebookChunkInput{
				ID:         chunkID,
				ParentID:   parentID,
				Text:       chunkText,
				TokenCount: len(strings.Fields(chunkText)),
				PageNum:    page,
			})
			allChunks = append(allChunks, models.Chunk{
				ID:              chunkID,
				TopicID:         builder.topicID,
				ParentID:        parentID,
				Text:            chunkText,
				PageNum:         page,
				ImportanceScore: 0,
				WeaknessScore:   0,
			})
		}
	}

	groups := make([]db.NotebookTopicIngestionGroup, 0, len(builders))
	for _, builder := range builders {
		if len(builder.chunks) == 0 {
			continue
		}
		groups = append(groups, db.NotebookTopicIngestionGroup{
			TopicID: builder.topicID,
			Parents: builder.parents,
			Chunks:  builder.chunks,
		})
	}

	return groups, allChunks
}

// chapterIndexForPage finds the chapter index containing the given page.
func chapterIndexForPage(page int, chapters []models.SyllabusChapterDraft) int {
	for i, ch := range chapters {
		if page >= ch.StartPage && page <= ch.EndPage {
			return i
		}
	}
	if len(chapters) == 0 {
		return -1
	}
	if page < chapters[0].StartPage {
		return 0
	}
	last := chapters[len(chapters)-1]
	if page > last.EndPage {
		return len(chapters) - 1
	}
	return -1
}

// topicGroupBuilder builds topic groups during ingestion.
type topicGroupBuilder struct {
	topicID string
	parents []db.NotebookParentInput
	chunks  []db.NotebookChunkInput
	order   int
}
