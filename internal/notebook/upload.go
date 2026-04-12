package notebook

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	pdfreader "github.com/ledongthuc/pdf"
)

// UploadConfig holds paths and limits for file uploads
type UploadConfig struct {
	UploadDir   string
	MaxFileSize int64 // in bytes
}

// Service handles notebook file uploads and storage
type Service struct {
	config     UploadConfig
	readFile   func(string) ([]byte, error)
	openPDF    func(string) (*os.File, *pdfreader.Reader, error)
	extractPDF func(string, *ExtractedDocument) error
}

const (
	ChunkWordWindow  = 180
	ChunkWordOverlap = 40
)

// Option customizes Service dependencies for testing and advanced setups.
type Option func(*Service)

// WithReadFileFunc overrides the file reader dependency.
func WithReadFileFunc(fn func(string) ([]byte, error)) Option {
	return func(s *Service) {
		if fn != nil {
			s.readFile = fn
		}
	}
}

// WithOpenPDFFunc overrides the PDF opener dependency.
func WithOpenPDFFunc(fn func(string) (*os.File, *pdfreader.Reader, error)) Option {
	return func(s *Service) {
		if fn != nil {
			s.openPDF = fn
		}
	}
}

// WithExtractPDFFunc overrides PDF extraction logic.
func WithExtractPDFFunc(fn func(string, *ExtractedDocument) error) Option {
	return func(s *Service) {
		if fn != nil {
			s.extractPDF = fn
		}
	}
}

// NewService creates a new notebook service
func NewService(uploadDir string, opts ...Option) *Service {
	// Ensure directory exists
	_ = os.MkdirAll(uploadDir, 0o755) // ignore error, non-fatal
	s := &Service{
		config: UploadConfig{
			UploadDir:   uploadDir,
			MaxFileSize: 50 * 1024 * 1024, // 50MB default
		},
		readFile: os.ReadFile,
		openPDF:  pdfreader.Open,
	}
	s.extractPDF = s.extractPDFDocument

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// UploadResult contains info about uploaded file
type UploadResult struct {
	ID       string
	FileName string
	FilePath string
	FileType string
	Size     int64
}

// ExtractedSection is a normalized content section from an uploaded notebook.
type ExtractedSection struct {
	Heading string
	Text    string
	PageNum int
}

// ExtractedDocument represents normalized notebook content ready for chunking.
type ExtractedDocument struct {
	Title     string
	PageCount int
	WordCount int
	Sections  []ExtractedSection
}

// ParentRecord is a parent section row prepared for DB insertion.
type ParentRecord struct {
	ID         string
	Heading    string
	Content    string
	OrderIndex int
}

// ChunkRecord is a chunk row prepared for DB insertion and notebook linking.
type ChunkRecord struct {
	ID         string
	ParentID   string
	Text       string
	TokenCount int
	PageNum    int
}

// IngestionData is deterministic relational data derived from notebook content.
type IngestionData struct {
	Parents []ParentRecord
	Chunks  []ChunkRecord
}

// SaveUploadedFile saves an uploaded file and returns metadata
// fileData is the raw file bytes, fileName is the user-provided name
func (s *Service) SaveUploadedFile(fileData []byte, fileName string) (*UploadResult, error) {
	// Determine file type from extension
	ext := strings.ToLower(filepath.Ext(fileName))
	fileType := strings.TrimPrefix(ext, ".")

	// Validate file type
	validTypes := map[string]bool{
		"pdf": true,
		"txt": true,
		"md":  true,
	}
	if !validTypes[fileType] {
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}

	// Check file size
	if int64(len(fileData)) > s.config.MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", len(fileData), s.config.MaxFileSize)
	}

	// Generate unique ID and filename
	id := uuid.New().String()
	uniqueFileName := fmt.Sprintf("%s_%s%s", id, sanitizeFileName(fileName), ext)
	filePath := filepath.Join(s.config.UploadDir, uniqueFileName)

	// Write file to disk
	if err := os.WriteFile(filePath, fileData, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &UploadResult{
		ID:       id,
		FileName: fileName,
		FilePath: filePath,
		FileType: fileType,
		Size:     int64(len(fileData)),
	}, nil
}

// GetFilePath returns the full path to a notebook file
func (s *Service) GetFilePath(notebookID string) (string, error) {
	// For now, we'd need to look this up in DB
	// This is a placeholder - actual implementation would query DB
	path := filepath.Join(s.config.UploadDir, notebookID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", notebookID)
	}
	return path, nil
}

// DeleteFile removes a notebook file from disk
func (s *Service) DeleteFile(filePath string) error {
	// Ensure path is within upload directory (security check)
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}

	absUploadDir, err := filepath.Abs(s.config.UploadDir)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(absPath, absUploadDir) {
		return fmt.Errorf("invalid file path: outside upload directory")
	}

	return os.Remove(absPath)
}

// ExtractDocument loads and normalizes notebook text content for ingestion.
func (s *Service) ExtractDocument(filePath string, fileType string) (*ExtractedDocument, error) {
	fileType = strings.ToLower(fileType)

	doc := &ExtractedDocument{
		Title: filepath.Base(filePath),
	}

	switch fileType {
	case "txt":
		raw, err := s.readFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read txt file: %w", err)
		}
		content := normalizeWhitespace(string(raw))
		if content == "" {
			return nil, fmt.Errorf("document has no readable content")
		}
		doc.PageCount = 1
		doc.WordCount = len(strings.Fields(content))
		doc.Sections = []ExtractedSection{{
			Heading: "Document",
			Text:    content,
			PageNum: 1,
		}}

	case "md":
		raw, err := s.readFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read markdown file: %w", err)
		}
		sections := splitMarkdownSections(string(raw))
		if len(sections) == 0 {
			return nil, fmt.Errorf("document has no readable content")
		}
		doc.PageCount = 1
		doc.Sections = sections
		for _, section := range sections {
			doc.WordCount += len(strings.Fields(section.Text))
		}

	case "pdf":
		if err := s.extractPDF(filePath, doc); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}

	if doc.PageCount <= 0 {
		doc.PageCount = 1
	}

	return doc, nil
}

// BuildIngestionData creates deterministic parent/chunk records for DB transaction inserts.
func (s *Service) BuildIngestionData(notebookID string, doc *ExtractedDocument) (*IngestionData, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is nil")
	}
	if len(doc.Sections) == 0 {
		return nil, fmt.Errorf("document has no sections to ingest")
	}

	data := &IngestionData{
		Parents: make([]ParentRecord, 0, len(doc.Sections)),
		Chunks:  make([]ChunkRecord, 0),
	}

	for sectionIndex, section := range doc.Sections {
		if section.Text == "" {
			continue
		}

		parentID := fmt.Sprintf("nbp_%s_%d", notebookID, sectionIndex+1)
		heading := strings.TrimSpace(section.Heading)
		if heading == "" {
			heading = fmt.Sprintf("Section %d", sectionIndex+1)
		}

		data.Parents = append(data.Parents, ParentRecord{
			ID:         parentID,
			Heading:    heading,
			Content:    section.Text,
			OrderIndex: sectionIndex + 1,
		})

		chunks := SplitIntoWordChunks(section.Text, ChunkWordWindow, ChunkWordOverlap)
		for chunkIndex, chunkText := range chunks {
			chunkID := fmt.Sprintf("nbc_%s_%d_%d", notebookID, sectionIndex+1, chunkIndex+1)
			data.Chunks = append(data.Chunks, ChunkRecord{
				ID:         chunkID,
				ParentID:   parentID,
				Text:       chunkText,
				TokenCount: len(strings.Fields(chunkText)),
				PageNum:    section.PageNum,
			})
		}
	}

	if len(data.Chunks) == 0 {
		return nil, fmt.Errorf("document produced no chunks after normalization")
	}

	return data, nil
}

// sanitizeFileName removes potentially dangerous characters
func sanitizeFileName(name string) string {
	// Remove extension for processing
	name = strings.TrimSuffix(name, filepath.Ext(name))
	// Replace spaces and special chars
	name = strings.Map(func(r rune) rune {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '_'
		}
	}, name)
	return name
}

// FileMetadata represents extracted metadata from file
type FileMetadata struct {
	PageCount int
	WordCount int
	Title     string
}

// ExtractMetadata returns metadata derived from normalized extraction output.
func (s *Service) ExtractMetadata(filePath string, fileType string) (*FileMetadata, error) {
	fileType = strings.ToLower(fileType)
	title := filepath.Base(filePath)

	switch fileType {
	case "txt", "md":
		raw, err := s.readFile(filePath)
		if err != nil {
			return nil, err
		}
		wordCount := len(strings.Fields(normalizeWhitespace(string(raw))))
		return &FileMetadata{Title: title, PageCount: 1, WordCount: wordCount}, nil
	case "pdf":
		file, reader, err := s.openPDF(filePath)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = file.Close()
		}()

		pageCount := reader.NumPage()
		if pageCount <= 0 {
			pageCount = 1
		}

		// Lightweight metadata path for PDFs: avoid full text extraction.
		return &FileMetadata{Title: title, PageCount: pageCount, WordCount: 0}, nil
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}

}

func (s *Service) extractPDFDocument(filePath string, doc *ExtractedDocument) error {
	file, reader, err := s.openPDF(filePath)
	if err != nil {
		return fmt.Errorf("failed to read pdf: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	totalPages := reader.NumPage()
	doc.PageCount = totalPages

	for pageIndex := 1; pageIndex <= totalPages; pageIndex++ {
		page := reader.Page(pageIndex)
		if page.V.IsNull() {
			continue
		}
		text, pageErr := page.GetPlainText(nil)
		if pageErr != nil {
			return fmt.Errorf("failed to read pdf page %d: %w", pageIndex, pageErr)
		}

		normalized := normalizeWhitespace(text)
		if normalized == "" {
			continue
		}

		doc.WordCount += len(strings.Fields(normalized))
		doc.Sections = append(doc.Sections, ExtractedSection{
			Heading: fmt.Sprintf("Page %d", pageIndex),
			Text:    normalized,
			PageNum: pageIndex,
		})
	}

	if len(doc.Sections) > 0 {
		return nil
	}

	plainReader, plainErr := reader.GetPlainText()
	if plainErr != nil {
		return fmt.Errorf("pdf did not contain extractable text: %w", plainErr)
	}

	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, plainReader); copyErr != nil {
		return fmt.Errorf("failed to read plain pdf text: %w", copyErr)
	}

	normalized := normalizeWhitespace(buf.String())
	if normalized == "" {
		return fmt.Errorf("pdf did not contain extractable text")
	}

	doc.WordCount = len(strings.Fields(normalized))
	doc.Sections = []ExtractedSection{{
		Heading: "Document",
		Text:    normalized,
		PageNum: 1,
	}}
	if doc.PageCount == 0 {
		doc.PageCount = 1
	}

	return nil
}

func splitMarkdownSections(content string) []ExtractedSection {
	lines := strings.Split(content, "\n")
	sections := make([]ExtractedSection, 0)
	currentHeading := "Document"
	var body []string

	flush := func() {
		joined := normalizeWhitespace(strings.Join(body, "\n"))
		if joined != "" {
			sections = append(sections, ExtractedSection{
				Heading: currentHeading,
				Text:    joined,
				PageNum: 1,
			})
		}
		body = body[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			flush()
			heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if heading == "" {
				heading = "Section"
			}
			currentHeading = heading
			continue
		}
		body = append(body, line)
	}
	flush()

	if len(sections) == 0 {
		normalized := normalizeWhitespace(content)
		if normalized != "" {
			sections = append(sections, ExtractedSection{
				Heading: "Document",
				Text:    normalized,
				PageNum: 1,
			})
		}
	}

	return sections
}

func SplitIntoWordChunks(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = ChunkWordWindow
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 2
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	stride := chunkSize - overlap
	chunks := make([]string, 0)

	for start := 0; start < len(words); start += stride {
		end := start + chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunk := strings.Join(words[start:end], " ")
		chunk = normalizeWhitespace(chunk)
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		if end == len(words) {
			break
		}
	}

	return chunks
}

func normalizeWhitespace(input string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(input), " "))
}
