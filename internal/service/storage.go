package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	ISOStorageDir = "./data/isos"
)

// StorageService handles file storage operations
type StorageService struct {
	storageDir string
}

// NewStorageService creates a new instance of StorageService
func NewStorageService() (*StorageService, error) {
	service := &StorageService{
		storageDir: ISOStorageDir,
	}
	
	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(service.storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	
	return service, nil
}

// SaveISO saves an ISO file from an io.Reader to the storage directory
// Uses io.Copy for streaming to avoid loading the entire file into RAM
func (s *StorageService) SaveISO(filename string, reader io.Reader) (string, error) {
	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(filename), ".iso") {
		return "", fmt.Errorf("invalid file extension. Only .iso files are allowed")
	}
	
	// Generate safe filename
	safeFilename := filepath.Base(filename)
	
	// Create full file path
	filePath := filepath.Join(s.storageDir, safeFilename)
	
	// Create the file
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	// Stream copy from reader to file (this avoids loading entire file into RAM)
	_, err = io.Copy(file, reader)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}
	
	return filePath, nil
}

// ListISOs returns a list of ISO files in the storage directory
func (s *StorageService) ListISOs() ([]string, error) {
	var isos []string
	
	entries, err := os.ReadDir(s.storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage directory: %w", err)
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".iso") {
			isos = append(isos, entry.Name())
		}
	}
	
	return isos, nil
}

// GetISOPath returns the full path for an ISO file
func (s *StorageService) GetISOPath(filename string) string {
	return filepath.Join(s.storageDir, filepath.Base(filename))
}

// GetISOInfo returns information about an ISO file
func (s *StorageService) GetISOInfo(filename string) (os.FileInfo, error) {
	filePath := s.GetISOPath(filename)
	return os.Stat(filePath)
}

// DeleteISO removes an ISO file from the storage
func (s *StorageService) DeleteISO(filename string) error {
	filePath := s.GetISOPath(filename)
	return os.Remove(filePath)
}

// ISOFileInfo represents information about an ISO file
type ISOFileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}

// SaveISOWithInfo saves an ISO file and returns detailed information about it
func (s *StorageService) SaveISOWithInfo(filename string, reader io.Reader) (*ISOFileInfo, error) {
	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(filename), ".iso") {
		return nil, fmt.Errorf("invalid file extension. Only .iso files are allowed")
	}

	// Generate safe filename
	safeFilename := filepath.Base(filename)

	// Create full file path
	filePath := filepath.Join(s.storageDir, safeFilename)

	// Create the file
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Stream copy from reader to file (this avoids loading entire file into RAM)
	bytesWritten, err := io.Copy(file, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	return &ISOFileInfo{
		Name: safeFilename,
		Size: bytesWritten,
		Path: filePath,
	}, nil
}

// SaveISOFromReader is an alias for SaveISOWithInfo to maintain compatibility
func (s *StorageService) SaveISOFromReader(filename string, reader io.Reader) (string, error) {
	result, err := s.SaveISOWithInfo(filename, reader)
	if err != nil {
		return "", err
	}
	return result.Path, nil
}