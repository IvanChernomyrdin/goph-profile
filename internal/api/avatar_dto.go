package api

import "io"

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
