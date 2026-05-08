package ingress

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

type Dispatcher interface {
	Dispatch(ctx context.Context, req ir.Request) (ir.Response, error)
}

type StreamDispatcher interface {
	Stream(ctx context.Context, req ir.Request) (<-chan ir.Event, error)
}

type ModelProvider interface {
	Models() []string
}

type AuthFunc func(*http.Request) (bridgeKeyID string, err error)
type LimitFunc func(*http.Request, string) error
type BudgetFunc func(*http.Request, string, ir.Request) error
type ObserveFunc func(*http.Request, ir.Request, ir.Response, error, time.Duration)

type Server struct {
	dispatcher Dispatcher
	auth       AuthFunc
	rateLimit  LimitFunc
	budget     BudgetFunc
	observe    ObserveFunc
	modelIDs   []string
	mux        *http.ServeMux
}

func New(dispatcher Dispatcher) *Server {
	s := &Server{dispatcher: dispatcher, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) WithAuth(fn AuthFunc) *Server        { s.auth = fn; return s }
func (s *Server) WithRateLimit(fn LimitFunc) *Server  { s.rateLimit = fn; return s }
func (s *Server) WithBudget(fn BudgetFunc) *Server    { s.budget = fn; return s }
func (s *Server) WithObserver(fn ObserveFunc) *Server { s.observe = fn; return s }
func (s *Server) WithModels(models []string) *Server {
	s.modelIDs = append([]string(nil), models...)
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) HTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       600 * time.Second,
		WriteTimeout:      600 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	s.mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if s.dispatcher == nil {
			http.Error(w, "router not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	s.mux.HandleFunc("/v1/chat/completions", s.openAIChat)
	s.mux.HandleFunc("/v1/models", s.models)
	s.mux.HandleFunc("/v1/messages", s.anthropicMessages)
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) (string, bool) {
	if s.auth == nil {
		return "", true
	}
	keyID, err := s.auth(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return "", false
	}
	if s.rateLimit != nil {
		if err := s.rateLimit(r, keyID); err != nil {
			writeError(w, http.StatusTooManyRequests, err.Error())
			return "", false
		}
	}
	return keyID, true
}

func (s *Server) dispatch(w http.ResponseWriter, r *http.Request, req ir.Request, encode func(ir.Response) ([]byte, error), stream func(ir.Response, <-chan ir.Event) ([]byte, error)) {
	start := time.Now()
	if s.budget != nil {
		if err := s.budget(r, req.BridgeKeyID, req); err != nil {
			writeError(w, http.StatusPaymentRequired, err.Error())
			return
		}
	}
	if s.dispatcher == nil {
		writeError(w, http.StatusServiceUnavailable, "router not ready")
		return
	}
	if req.Stream && stream != nil {
		streamer, ok := s.dispatcher.(StreamDispatcher)
		if !ok {
			err := errors.New("streaming not supported")
			s.observeDispatch(r, req, ir.Response{}, err, start)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		events, err := streamer.Stream(r.Context(), req)
		if err != nil {
			s.observeDispatch(r, req, ir.Response{}, err, start)
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		raw, err := stream(ir.Response{ID: req.ID, UpstreamModel: req.ModelAlias}, events)
		if err != nil {
			s.observeDispatch(r, req, ir.Response{}, err, start)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.observeDispatch(r, req, ir.Response{ID: req.ID, UpstreamModel: req.ModelAlias, StopReason: ir.StopEndTurn}, nil, start)
		writeSSE(w, raw)
		return
	}
	resp, err := s.dispatcher.Dispatch(r.Context(), req)
	if err != nil {
		s.observeDispatch(r, req, ir.Response{}, err, start)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	resp.LatencyMs = time.Since(start).Milliseconds()
	s.observeDispatch(r, req, resp, nil, start)
	raw, err := encode(resp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

func (s *Server) observeDispatch(r *http.Request, req ir.Request, resp ir.Response, err error, start time.Time) {
	if s.observe == nil {
		return
	}
	s.observe(r, req, resp, err, time.Since(start))
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": message, "status": status}})
}
