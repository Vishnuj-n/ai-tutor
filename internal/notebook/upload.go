package notebook

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"ai-tutor/internal/embeddings"

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
	DefaultSemanticChunkTargetWords = 150
	semanticChunkLowerBoundWords    = 100
	semanticChunkUpperBoundWords    = 200
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

// SaveUploadedFile saves an uploaded file and returns metadata
// fileData is the raw file bytes, fileName is the user-provided name
func (s *Service) SaveUploadedFile(fileData []byte, fileName string) (*UploadResult, error) {
	ext, fileType, err := validateUploadFileType(fileName)
	if err != nil {
		return nil, err
	}

	// Check file size
	if int64(len(fileData)) > s.config.MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", len(fileData), s.config.MaxFileSize)
	}

	id, filePath := s.buildUploadPath(fileName, ext)

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

// SaveUploadedFileFromPath copies a user-selected local file into notebook storage.
func (s *Service) SaveUploadedFileFromPath(sourcePath string) (*UploadResult, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return nil, fmt.Errorf("file path is required")
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to access file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("file path points to a directory")
	}
	if info.Size() > s.config.MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), s.config.MaxFileSize)
	}

	fileName := filepath.Base(sourcePath)
	ext, fileType, err := validateUploadFileType(fileName)
	if err != nil {
		return nil, err
	}

	id, destinationPath := s.buildUploadPath(fileName, ext)

	source, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = source.Close() }()
	fi, err := source.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat source file: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("source file is not a regular file")
	}
	if fi.Size() > s.config.MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", fi.Size(), s.config.MaxFileSize)
	}

	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() { _ = destination.Close() }()

	copied, err := io.CopyN(destination, source, fi.Size())
	if err != nil {
		_ = destination.Close()
		_ = os.Remove(destinationPath)
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}
	if copied != fi.Size() {
		_ = destination.Close()
		_ = os.Remove(destinationPath)
		return nil, fmt.Errorf("failed to copy file: copied %d bytes, expected %d", copied, fi.Size())
	}

	return &UploadResult{
		ID:       id,
		FileName: fileName,
		FilePath: destinationPath,
		FileType: fileType,
		Size:     fi.Size(),
	}, nil
}

func (s *Service) buildUploadPath(fileName, ext string) (string, string) {
	id := uuid.New().String()
	uniqueFileName := fmt.Sprintf("%s_%s%s", id, sanitizeFileName(fileName), ext)
	filePath := filepath.Join(s.config.UploadDir, uniqueFileName)
	return id, filePath
}

func validateUploadFileType(fileName string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	fileType := strings.TrimPrefix(ext, ".")
	validTypes := map[string]bool{
		"pdf": true,
		"txt": true,
		"md":  true,
	}
	if !validTypes[fileType] {
		return "", "", fmt.Errorf("unsupported file type: %s", fileType)
	}
	return ext, fileType, nil
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
		content := embeddings.NormalizeWhitespace(string(raw))
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
		content := string(raw)
		if strings.TrimSpace(content) == "" {
			return nil, fmt.Errorf("document has no readable content")
		}
		// Split markdown by headings to create sections
		sections := splitMarkdownByHeadings(content)
		if len(sections) == 0 {
			// Fallback to single section if no headings found
			doc.PageCount = 1
			doc.WordCount = len(strings.Fields(content))
			doc.Sections = []ExtractedSection{{
				Heading: "Document",
				Text:    content,
				PageNum: 1,
			}}
		} else {
			doc.PageCount = len(sections)
			doc.WordCount = 0
			doc.Sections = make([]ExtractedSection, len(sections))
			for i, sec := range sections {
				doc.Sections[i] = ExtractedSection{
					Heading: sec.Heading,
					Text:    sec.Text,
					PageNum: i + 1,
				}
				doc.WordCount += len(strings.Fields(sec.Text))
			}
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

// ExtractDocumentSample extracts only a lightweight sample of the document for syllabus drafting.
// This is much faster than full extraction as it only reads a small subset of pages.
func (s *Service) ExtractDocumentSample(filePath string, fileType string, maxPages int) (*ExtractedDocument, error) {
	fileType = strings.ToLower(fileType)
	if maxPages <= 0 {
		return nil, fmt.Errorf("max pages must be positive")
	}

	doc := &ExtractedDocument{
		Title: filepath.Base(filePath),
	}

	switch fileType {
	case "txt":
		raw, err := s.readFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read txt file: %w", err)
		}
		content := embeddings.NormalizeWhitespace(string(raw))
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
		content := string(raw)
		if strings.TrimSpace(content) == "" {
			return nil, fmt.Errorf("document has no readable content")
		}
		sections := splitMarkdownByHeadings(content)
		if len(sections) == 0 {
			doc.PageCount = 1
			doc.Sections = []ExtractedSection{{
				Heading: "Document",
				Text:    content,
				PageNum: 1,
			}}
		} else {
			limit := min(maxPages, len(sections))
			doc.PageCount = len(sections)
			doc.Sections = make([]ExtractedSection, limit)
			for i := 0; i < limit; i++ {
				doc.Sections[i] = ExtractedSection{
					Heading: sections[i].Heading,
					Text:    sections[i].Text,
					PageNum: i + 1,
				}
			}
		}

	case "pdf":
		if err := s.extractPDFSample(filePath, doc, maxPages); err != nil {
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
		wordCount := len(strings.Fields(embeddings.NormalizeWhitespace(string(raw))))
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

func (s *Service) extractPDFSample(filePath string, doc *ExtractedDocument, maxPages int) error {
	file, reader, err := s.openPDF(filePath)
	if err != nil {
		return fmt.Errorf("failed to read pdf: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	totalPages := reader.NumPage()
	doc.PageCount = totalPages

	// Extract only a sample of pages for syllabus drafting
	limit := min(maxPages, totalPages)

	for pageIndex := 1; pageIndex <= limit; pageIndex++ {
		page := reader.Page(pageIndex)
		if page.V.IsNull() {
			continue
		}
		text, pageErr := page.GetPlainText(nil)
		if pageErr != nil {
			return fmt.Errorf("failed to read pdf page %d: %w", pageIndex, pageErr)
		}

		normalized := embeddings.NormalizeWhitespace(text)
		if normalized == "" {
			continue
		}

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

	normalized := embeddings.NormalizeWhitespace(buf.String())
	if normalized == "" {
		return fmt.Errorf("pdf did not contain extractable text")
	}
	doc.Sections = append(doc.Sections, ExtractedSection{
		Heading: "Document",
		Text:    normalized,
		PageNum: 1,
	})

	return nil
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

		normalized := embeddings.NormalizeWhitespace(text)
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

	normalized := embeddings.NormalizeWhitespace(buf.String())
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

type wordSpan struct {
	start int
	end   int
	text  string
}

// SplitPageIntoSemanticChunks splits page-local text near semantic boundaries around targetWords.
// It never crosses page boundaries because callers provide one page body at a time.
func SplitPageIntoSemanticChunks(text string, targetWords int) []string {
	if targetWords <= 0 {
		targetWords = DefaultSemanticChunkTargetWords
	}

	spans := tokenizeWordSpans(text)
	if len(spans) == 0 {
		return nil
	}

	chunks := make([]string, 0)
	for start := 0; start < len(spans); {
		if len(spans)-start <= targetWords {
			chunk := embeddings.NormalizeWhitespace(text[spans[start].start:spans[len(spans)-1].end])
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
			break
		}

		lower := start + semanticChunkLowerBoundWords
		if lower <= start {
			lower = start + 1
		}
		if lower > len(spans) {
			lower = len(spans)
		}

		upper := start + semanticChunkUpperBoundWords
		if upper > len(spans) {
			upper = len(spans)
		}
		if upper < lower {
			upper = lower
		}

		bestEnd := -1
		bestDistance := math.MaxInt32
		for end := lower; end <= upper; end++ {
			if !isPreferredBoundary(text, spans, end) {
				continue
			}
			distance := absInt((end - start) - targetWords)
			if distance < bestDistance {
				bestDistance = distance
				bestEnd = end
			}
		}

		if bestEnd < 0 {
			bestEnd = start + targetWords
			if bestEnd > len(spans) {
				bestEnd = len(spans)
			}
		}

		chunk := embeddings.NormalizeWhitespace(text[spans[start].start:spans[bestEnd-1].end])
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		start = bestEnd
	}

	return chunks
}

func tokenizeWordSpans(text string) []wordSpan {
	spans := make([]wordSpan, 0)
	i := 0
	for i < len(text) {
		for i < len(text) && isWhitespaceByte(text[i]) {
			i++
		}
		if i >= len(text) {
			break
		}
		start := i
		for i < len(text) && !isWhitespaceByte(text[i]) {
			i++
		}
		end := i
		spans = append(spans, wordSpan{
			start: start,
			end:   end,
			text:  text[start:end],
		})
	}
	return spans
}

func isWhitespaceByte(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t' || b == '\f' || b == '\v'
}

func isPreferredBoundary(text string, spans []wordSpan, end int) bool {
	if end <= 0 || end > len(spans) {
		return false
	}
	if end == len(spans) {
		return true
	}

	prev := spans[end-1]
	next := spans[end]
	if hasTerminalPeriod(prev.text) {
		return true
	}
	gap := text[prev.end:next.start]
	return strings.Contains(gap, "\n")
}

func hasTerminalPeriod(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	i := len(token) - 1
	for i >= 0 {
		switch token[i] {
		case '"', '\'', ')', ']', '}':
			i--
			continue
		}
		break
	}
	if i < 0 {
		return false
	}
	return token[i] == '.'
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

type markdownSection struct {
	Heading string
	Text    string
}

// splitMarkdownByHeadings splits markdown content by headings (#, ##, ###, etc.)
func splitMarkdownByHeadings(content string) []markdownSection {
	lines := strings.Split(content, "\n")
	sections := make([]markdownSection, 0)
	var currentHeading string
	var currentText strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check if line is a heading (starts with #)
		if strings.HasPrefix(trimmed, "#") {
			// Save previous section if it has content
			if currentHeading != "" || currentText.Len() > 0 {
				sections = append(sections, markdownSection{
					Heading: currentHeading,
					Text:    strings.TrimSpace(currentText.String()),
				})
			}
			// Start new section
			currentHeading = strings.TrimLeft(trimmed, "# ")
			currentHeading = strings.TrimSpace(currentHeading)
			currentText.Reset()
		} else {
			currentText.WriteString(line)
			currentText.WriteString("\n")
		}
	}

	// Add final section
	if currentHeading != "" || currentText.Len() > 0 {
		sections = append(sections, markdownSection{
			Heading: currentHeading,
			Text:    strings.TrimSpace(currentText.String()),
		})
	}

	return sections
}
