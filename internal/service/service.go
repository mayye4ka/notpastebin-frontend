package service

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"

	pbapi "github.com/mayye4ka/notpastebin/pkg/api/go"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const hashLen = 32

type Service struct {
	httpPort int
	client   pbapi.NotPasteBinClient
	tmpl     *template.Template
	server   *http.Server
	logger   zerolog.Logger
	siteUrl  string
}

func New(httpPort int, backendClient pbapi.NotPasteBinClient, logger zerolog.Logger, siteUrl string) (*Service, error) {
	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		return nil, fmt.Errorf("parse template")
	}
	return &Service{
		httpPort: httpPort,
		client:   backendClient,
		tmpl:     t,
		siteUrl:  siteUrl,
	}, nil
}

func (s *Service) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/style.css", s.cssHandler)
	mux.HandleFunc("/create", s.createNoteHandler)
	mux.HandleFunc("/delete/", s.deleteNoteHandler)
	mux.HandleFunc("/update/", s.updateNoteHandler)
	mux.HandleFunc("/note/", s.readNoteHandler)
	mux.HandleFunc("/edit/", s.editNoteHandler)
	mux.HandleFunc("/", s.mainPageHandler)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.httpPort),
		Handler: mux,
	}
	return s.server.ListenAndServe()
}

func (s *Service) readNoteHandler(w http.ResponseWriter, r *http.Request) {
	hash, err := extractHash(r.URL.Path, "note")
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, err.Error())
		return
	}

	resp, err := s.client.GetNote(r.Context(), &pbapi.GetNoteRequest{
		Hash: hash,
	})
	if err != nil {
		e, ok := status.FromError(err)
		if ok && e.Code() == codes.NotFound {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, "note not found")
			return
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "internal error")
			s.logger.Err(err).Msg("got internal error from backend")
			return
		}
	}
	if resp.IsAdmin {
		http.Redirect(w, r, fmt.Sprintf("/edit/%s", hash), http.StatusTemporaryRedirect)
	}
	err = s.tmpl.Execute(w, TemplateData{
		IsReadPage: true,
		NoteText:   resp.Text,
		ReaderUrl:  s.createReaderUrl(resp.ReaderHash),
	})
	if err != nil {
		s.logger.Err(err).Msg("execute template error")
	}
}

func (s *Service) editNoteHandler(w http.ResponseWriter, r *http.Request) {
	hash, err := extractHash(r.URL.Path, "edit")
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, err.Error())
		return
	}

	resp, err := s.client.GetNote(r.Context(), &pbapi.GetNoteRequest{
		Hash: hash,
	})
	if err != nil {
		e, ok := status.FromError(err)
		if ok && e.Code() == codes.NotFound {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, "note not found")
			return
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "internal error")
			s.logger.Err(err).Msg("got internal error from backend")
			return
		}
	}
	if !resp.IsAdmin {
		http.Redirect(w, r, fmt.Sprintf("/note/%s", resp.ReaderHash), http.StatusTemporaryRedirect)
	}
	err = s.tmpl.Execute(w, TemplateData{
		IsEditPage: true,
		NoteText:   resp.Text,
		ReaderUrl:  s.createReaderUrl(resp.ReaderHash),
		AdminHash:  hash,
	})
	if err != nil {
		s.logger.Err(err).Msg("execute template error")
	}
}

func (s *Service) mainPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "" {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "not found")
		return
	}
	err := s.tmpl.Execute(w, TemplateData{
		IsMainPage: true,
	})
	if err != nil {
		s.logger.Err(err).Msg("execute template error")
	}
}

func (s *Service) createNoteHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
		s.logger.Err(err).Msg("parse form error")
		return
	}
	resp, err := s.client.CreateNote(r.Context(), &pbapi.CreateNoteRequest{
		Text: r.Form["text"][0],
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
		s.logger.Err(err).Msg("create note error")
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/edit/%s", resp.AdminHash), http.StatusTemporaryRedirect)
}

func (s *Service) updateNoteHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
		s.logger.Err(err).Msg("parse form error")
		return
	}
	hash, err := extractHash(r.URL.Path, "update")
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, err.Error())
		return
	}
	_, err = s.client.UpdateNote(r.Context(), &pbapi.UpdateNoteRequest{
		Text:      r.Form["text"][0],
		AdminHash: hash,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
		s.logger.Err(err).Msg("update note error")
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/edit/%s", hash), http.StatusTemporaryRedirect)
}

func (s *Service) deleteNoteHandler(w http.ResponseWriter, r *http.Request) {
	hash, err := extractHash(r.URL.Path, "delete")
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, err.Error())
		return
	}
	_, err = s.client.DeleteNote(r.Context(), &pbapi.DeleteNoteRequest{
		AdminHash: hash,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
		s.logger.Err(err).Msg("delete note error")
		return
	}
	http.Redirect(w, r, ("/"), http.StatusTemporaryRedirect)
}

func (s *Service) cssHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/style.css")
}

func (s *Service) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Service) createReaderUrl(hash string) string {
	return fmt.Sprintf("%s/note/%s", s.siteUrl, hash)
}

func extractHash(path, prefixPart string) (string, error) {
	if path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	path = path[len(prefixPart)+2:]
	if len(path) != hashLen {
		return "", errors.New("invalid hash")
	}
	return path, nil
}

type TemplateData struct {
	IsMainPage bool
	IsEditPage bool
	IsReadPage bool
	NoteText   string
	ReaderUrl  string
	AdminHash  string
}
