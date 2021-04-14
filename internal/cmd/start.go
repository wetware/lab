package cmd

import (
	"context"
	"embed"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v4"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/lthibault/log"
	"github.com/urfave/cli/v2"
	"github.com/wetware/lab/pkg/sim"
	"golang.org/x/sync/errgroup"
)

//go:embed app/*
var app embed.FS

var upgrader = websocket.Upgrader{
	ReadBufferSize:    1024,
	WriteBufferSize:   1024,
	EnableCompression: true,
}

// Start command
func Start(log *Logger) *cli.Command {
	return &cli.Command{
		Name:  "start",
		Usage: "start a lab server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "addr",
				Aliases: []string{"a"},
				Usage:   "listen address",
				Value:   "localhost:2021",
				EnvVars: []string{"LAB_ADDR"},
			},
			&cli.PathFlag{
				Name:  "src",
				Usage: "source path for web assets",
			},
		},
		Action: func(c *cli.Context) error {
			svr := newServer(log)

			r := chi.NewRouter()
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer r.Body.Close()
					defer io.Copy(io.Discard, r.Body)

					next.ServeHTTP(w, r)
				})
			})

			r.Get("/app/*", http.FileServer(http.FS(app)).ServeHTTP)
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
			})
			r.Route("/events", func(r chi.Router) {
				r.Get("/", svr.ServeNewCluster)
				r.Get("/{clusterID}", svr.ServeCluster)

			})

			l, err := net.Listen("tcp", c.String("addr"))
			if err != nil {
				return cli.Exit(err, 1)
			}
			log = log.WithField("addr", l.Addr().String())
			log.Info("server started")

			s := http.Server{Addr: c.String("addr"), Handler: r}
			if err = s.Serve(l); err != http.ErrServerClosed {
				return cli.Exit(err, 1)
			}

			return nil
		},
	}
}

type server struct {
	log *Logger

	mu sync.RWMutex
	cs map[string]*cluster
}

func newServer(log *Logger) *server {
	return &server{
		log: log,
		cs:  make(map[string]*cluster),
	}
}

func (s *server) ServeNewCluster(w http.ResponseWriter, r *http.Request) {
	c := s.newCluster()
	defer s.stopCluster(c.ID)

	if status, err := s.serveCluster(w, r, c); err != nil {
		http.Error(w, err.Error(), status)
	}
}

func (s *server) ServeCluster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "clusterID"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	c, ok := s.getCluster(id)
	if !ok {
		http.Error(w, "cluster not found", http.StatusNotFound)
	}

	if status, err := s.serveCluster(w, r, c); err != nil {
		http.Error(w, err.Error(), status)
	}
}

func (s *server) serveCluster(w http.ResponseWriter, r *http.Request, c *cluster) (int, error) {
	sub, err := c.Events(r.Context())
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer sub.Close()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer conn.Close()

	// inform the client of the cluster ID.
	if err = conn.WriteJSON(struct{ ClusterID uuid.UUID }{c.ID}); err != nil {
		return http.StatusInternalServerError, err
	}

	g, ctx := errgroup.WithContext(r.Context())
	g.Go(s.handleClusterEvents(ctx, conn, s.log.With(c), sub.Out()))
	g.Go(s.handleUserEvents(ctx, conn, c))

	if err := g.Wait(); err != nil && !errors.Is(err, io.EOF) {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

func (s *server) handleClusterEvents(ctx context.Context, conn *websocket.Conn, l *Logger, events <-chan interface{}) func() error {
	return func() error {
		for {
			select {
			case v, ok := <-events:
				if !ok {
					return io.EOF
				}

				l.With(v.(log.Loggable)).Info("got cluster event")

				if err := conn.WriteJSON(v); err != nil {
					return err
				}

			case <-ctx.Done():
				return io.EOF
			}
		}
	}
}
func (s *server) handleUserEvents(ctx context.Context, conn *websocket.Conn, c *cluster) func() error {

	var (
		log = s.log.With(c)
		ev  UserEvent
	)

	return func() error {
		for {
			if err := conn.ReadJSON(&ev); err != nil {
				return err
			}

			log.With(ev).Info("got user event")

			switch event := ev.Type().(type) {
			case *Spawn:
				for i := 0; i < event.N; i++ {
					id, err := c.Spawn(ctx)
					if err != nil {
						log.WithError(err).Error("failed to spawn peer")
						continue
					}

					s.log.WithField("host", id).Debug("spawned host")
				}

			case *Kill:
				for _, id := range event.Hosts {
					if err := c.Kill(id); err != nil {
						log.WithError(err).WithField("host", id).Error("failed to kill host")
						continue
					}

					log.WithField("host", id).Debug("killed host")
				}
			}
		}
	}
}

func (s *server) newCluster() *cluster {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := uuid.New()
	c := &cluster{
		ID:      u,
		Cluster: sim.NewCluster(s.log.WithField("cluster", u).Logger),
	}

	s.cs[c.ID.String()] = c
	return c
}

func (s *server) getCluster(id uuid.UUID) (*cluster, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.cs[id.String()]
	return c, ok
}

func (s *server) stopCluster(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c := s.cs[id.String()]
	if err := c.Close(); err != nil {
		s.log.With(c).
			WithField("uuid", id).
			WithError(err).
			Error("failed to close cluster")
	}

	delete(s.cs, id.String())
}

type cluster struct {
	ID uuid.UUID
	*sim.Cluster
}

func (c *cluster) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"cluster_id": c.ID,
	}
}

type UserEvent struct {
	Spawn *Spawn `json:"spawn,omitempty"`
	Kill  *Kill  `json:"kill,omitempty"`
}

func (ue UserEvent) Type() log.Loggable {
	if ue.Spawn != nil {
		return ue.Spawn
	}

	if ue.Kill != nil {
		return ue.Kill
	}

	return nil
}

func (ue UserEvent) Loggable() map[string]interface{} {
	return ue.Type().Loggable()
}

type Spawn struct {
	N int `json:"n"`
}

func (s Spawn) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"event": "spawn",
		"n":     s.N,
	}
}

type Kill struct {
	Hosts []peer.ID `json:"hosts"`
}

func (k Kill) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"event": "kill",
		"n":     k.Hosts,
	}
}
