package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/chinmay28/homeapps/server/internal/myapps"
)

func (s *Server) handleListMyApps(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.MyApps.List())
}

type myAppBody struct {
	Name *string `json:"name"`
	URL  *string `json:"url"`
}

func decodeBody(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return &myapps.ValidationError{Msg: "the request body must be JSON with \"name\" and \"url\""}
	}
	return nil
}

// handleAddMyApp stores {url, name?}. An app added without a name is called
// what its own manifest calls it, looked up once now, else by its host.
func (s *Server) handleAddMyApp(w http.ResponseWriter, r *http.Request) {
	var body myAppBody
	if err := decodeBody(r, &body); err != nil {
		s.writeMyAppsError(w, err)
		return
	}
	if body.URL == nil {
		s.writeMyAppsError(w, &myapps.ValidationError{Msg: "enter the app's address, for example pi5.local:8123"})
		return
	}
	name := ""
	if body.Name != nil {
		name = *body.Name
	}
	if n, err := myapps.CleanName(name); err == nil && n == "" && s.NameOf != nil {
		if u, err := myapps.NormalizeURL(*body.URL); err == nil {
			ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
			name = s.NameOf(ctx, u)
			cancel()
		}
	}
	app, err := s.MyApps.Add(name, *body.URL)
	if err != nil {
		s.writeMyAppsError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func (s *Server) handleUpdateMyApp(w http.ResponseWriter, r *http.Request) {
	var body myAppBody
	if err := decodeBody(r, &body); err != nil {
		s.writeMyAppsError(w, err)
		return
	}
	if body.Name == nil && body.URL == nil {
		s.writeMyAppsError(w, &myapps.ValidationError{Msg: "nothing to change: send \"name\" and/or \"url\""})
		return
	}
	app, err := s.MyApps.Update(r.PathValue("id"), body.Name, body.URL)
	if err != nil {
		s.writeMyAppsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (s *Server) handleDeleteMyApp(w http.ResponseWriter, r *http.Request) {
	if err := s.MyApps.Delete(r.PathValue("id")); err != nil {
		s.writeMyAppsError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeMyAppsError(w http.ResponseWriter, err error) {
	var ve *myapps.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, ve.Msg)
	case errors.Is(err, myapps.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, myapps.ErrDuplicate):
		writeError(w, http.StatusConflict, err.Error())
	default:
		s.Log.Error("my apps: save failed", "err", err)
		writeError(w, http.StatusInternalServerError, "could not save the app list")
	}
}
