package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Server struct {
	mux      *http.ServeMux
	http     *http.Server
	hostname string
	start    time.Time
	version  string
}

func New(hostname, version string, start time.Time) *Server {
	s := &Server{
		hostname: hostname,
		version:  version,
		start:    start,
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/api/execute", s.handleExecute)
	s.mux.HandleFunc("/api/ps", s.handlePS)
	s.mux.HandleFunc("/api/info", s.handleInfo)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/files", s.handleFiles)
	s.mux.HandleFunc("/api/download", s.handleDownload)
	return s
}

func (s *Server) Start(addr string) error {
	s.http = &http.Server{Addr: addr, Handler: s.mux}
	return s.http.ListenAndServe()
}

func (s *Server) Stop() {
	if s.http != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.http.Shutdown(ctx)
	}
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, 405, "Method not allowed")
		return
	}
	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "Invalid JSON")
		return
	}
	result := Execute(req.Command)
	jsonResp(w, 200, result)
}

func (s *Server) handlePS(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, 405, "Method not allowed")
		return
	}
	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "Invalid JSON")
		return
	}
	result := ExecutePS(req.Command)
	jsonResp(w, 200, result)
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, 200, map[string]interface{}{
		"hostname": s.hostname,
		"uptime":   int64(time.Since(s.start).Seconds()),
		"version":  s.version,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, 200, map[string]interface{}{
		"status": "ok",
		"uptime": int64(time.Since(s.start).Seconds()),
	})
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, 405, "Method not allowed")
		return
	}
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = "C:\\"
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		jsonError(w, 500, fmt.Sprintf("ReadDir error: %v", err))
		return
	}
	type Entry struct {
		Name    string `json:"name"`
		IsDir   bool   `json:"is_dir"`
		Size    int64  `json:"size"`
		ModTime string `json:"mod_time"`
	}
	var list []Entry
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		modTime := ""
		if info != nil {
			size = info.Size()
			modTime = info.ModTime().Format(time.RFC3339)
		}
		list = append(list, Entry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    size,
			ModTime: modTime,
		})
	}
	jsonResp(w, 200, list)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, 405, "Method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		jsonError(w, 400, "Missing path parameter")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		jsonError(w, 404, fmt.Sprintf("File error: %v", err))
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(path)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

func jsonResp(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	jsonResp(w, status, map[string]string{"error": msg})
}
