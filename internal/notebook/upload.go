package notebook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// UploadConfig holds paths and limits for file uploads
type UploadConfig struct {
	UploadDir   string
	MaxFileSize int64 // in bytes
}

// Service handles notebook file uploads and storage
type Service struct {
	config UploadConfig
}

// NewService creates a new notebook service
func NewService(uploadDir string) *Service {
	// Ensure directory exists
	_ = os.MkdirAll(uploadDir, 0o755) // ignore error, non-fatal
	return &Service{
		config: UploadConfig{
			UploadDir:   uploadDir,
			MaxFileSize: 50 * 1024 * 1024, // 50MB default
		},
	}
}

// UploadResult contains info about uploaded file
type UploadResult struct {
	ID       string
	FileName string
	FilePath string
	FileType string
	Size     int64
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

// ExtractMetadata returns basic metadata about a file (placeholder for PDF parsing)
func (s *Service) ExtractMetadata(filePath string, fileType string) (*FileMetadata, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	meta := &FileMetadata{
		Title: fileInfo.Name(),
	}

	// For PDF files, this would use a PDF library to extract page count
	// For now, return basic metadata
	if fileType == "pdf" {
		// TODO: use PDF library to extract page count
		meta.PageCount = 1 // placeholder
	}

	return meta, nil
}
