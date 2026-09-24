package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/chand1012/jeb/pkg/config"
	"github.com/chand1012/jeb/pkg/process"
	"github.com/chand1012/jeb/pkg/types"
	"github.com/spf13/cobra"
)

func serveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the jeb HTTP server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conf, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			mux := http.NewServeMux()
			mux.HandleFunc("/v1/systemone", newSystemOneHandler(conf))

			addr := conf.Address()
			fmt.Fprintln(cmd.OutOrStdout(), "Serving /v1/systemone on http://"+addr)
			return (&http.Server{
				Addr:    addr,
				Handler: requestLog(mux, log.New(cmd.OutOrStderr(), "", 0)),
			}).ListenAndServe()
		},
	}
	cmd.Flags().StringP("host", "H", "0.0.0.0", "which network interface are we running on")
	cmd.Flags().IntP("port", "p", 6102, "which port are we running on")
	return cmd
}

// requestLog wraps next and logs one line per request: method, path, status,
// duration, remote address.
func requestLog(next http.Handler, l *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		l.Printf("%s %s %d %s %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond), r.RemoteAddr)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

// newSystemOneHandler serves the Jev System One API: a JevRequest body
// (state + questions) in, a JevResponse (answers + usage) out.
func newSystemOneHandler(conf *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req types.JebRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("decode request: %v", err), http.StatusBadRequest)
			return
		}

		// ponytail: any process error (bad question type or upstream LLM
		// failure) maps to 502; split into 400/5xx if clients need it.
		resp, err := process.Request(req, conf)
		if err != nil {
			http.Error(w, fmt.Sprintf("process request: %v", err), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
