package api

import "net/http"

func (h *Handler) UploadAvatar(w http.ResponseWriter, r *http.Request)      {}
func (h *Handler) GetAvatar(w http.ResponseWriter, r *http.Request)         {}
func (h *Handler) GetUserAvatar(w http.ResponseWriter, r *http.Request)     {}
func (h *Handler) DeleteAvatar(w http.ResponseWriter, r *http.Request)      {}
func (h *Handler) DeleteUserAvatar(w http.ResponseWriter, r *http.Request)  {}
func (h *Handler) GetAvatarMetadata(w http.ResponseWriter, r *http.Request) {}
func (h *Handler) GetUserAvatars(w http.ResponseWriter, r *http.Request)    {}
