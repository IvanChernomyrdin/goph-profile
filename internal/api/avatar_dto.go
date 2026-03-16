package api

import "io"

// загрузка аватарок
type UploadAvatarInput struct {
	UserID      string
	FileName    string
	ContentType string
	Size        int64
	File        io.Reader
}

type UploadAvatarResult struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	URL       string `json:"url"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// получение аватарок
type GetAvatarResult struct {
	ID        string
	UserID    string
	FileName  string
	MimeType  string
	SizeBytes int64
	Reader    io.ReadCloser
}

type AvatarItem struct {
	ID               string            `json:"id"`
	UserID           string            `json:"user_id"`
	FileName         string            `json:"file_name"`
	MimeType         string            `json:"mime_type"`
	SizeBytes        int64             `json:"size_bytes"`
	UploadStatus     string            `json:"upload_status"`
	ProcessingStatus string            `json:"processing_status"`
	CreatedAt        string            `json:"created_at"`
	UpdatedAt        string            `json:"updated_at"`
	URL              string            `json:"url"`
	IsCurrent        bool              `json:"is_current"`
	ThumbnailURLs    map[string]string `json:"thumbnail_urls,omitempty"`
}
