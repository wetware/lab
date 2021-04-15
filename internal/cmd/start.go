package cmd

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"

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
			svr, err := newServer(log)
			if err != nil {
				return err
			}

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
			r.Get("/events", svr.ServeHTTP)
			r.Route("/node", func(r chi.Router) {
				r.Post("/", func(w http.ResponseWriter, r *http.Request) {
					id, err := svr.c.Spawn(r.Context())
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}

					if err = json.NewEncoder(w).Encode(struct {
						ID peer.ID `json:"id"`
					}{id}); err != nil {
						log.WithError(err).Error("failed to return id of newly spawned host")
					}
				})

				r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
					// TODO:  kill
					http.Error(w, "NOT IMPLEMENTED", http.StatusNotImplemented)
				})
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
	c   *cluster

	// mu sync.RWMutex
	// cs map[string]*cluster
}

func newServer(log *Logger) (*server, error) {
	u := uuid.New()
	sc, err := sim.NewCluster(log.WithField("cluster", u).Logger)
	if err != nil {
		return nil, err
	}

	c := &cluster{ID: u, Cluster: sc}

	return &server{
		log: log.With(c),
		c:   c,
	}, nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := s.serveHTTP(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *server) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.c.Events(r.Context())
	if err != nil {
		return err
	}
	defer sub.Close()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	// send the initial graph to the client
	if err := s.sendGraph(conn); err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(r.Context())
	g.Go(s.handleClusterEvents(ctx, conn, sub.Out()))
	g.Go(s.handleUserEvents(ctx, conn))

	if err := g.Wait(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	return nil
}

func (s *server) sendGraph(conn *websocket.Conn) error {
	s.log.Warn("stub call to sendGraph (loading from JSON file)")

	b, err := app.ReadFile("app/graph.json")
	if err != nil {
		return err
	}

	g := Graph{Cluster: s.c.ID}
	if err = json.Unmarshal(b, &g); err != nil {
		return err
	}

	return conn.WriteJSON(SimulationChanged{Graph: &g})
}

func (s *server) handleClusterEvents(ctx context.Context, conn *websocket.Conn, events <-chan interface{}) func() error {
	return func() error {
		for {
			select {
			case v, ok := <-events:
				if !ok {
					return io.EOF
				}

				if err := s.sendStep(conn, v.(sim.StateChanged)); err != nil {
					return err
				}

			case <-ctx.Done():
				return io.EOF
			}
		}
	}
}

func (s *server) sendStep(conn *websocket.Conn, ev sim.StateChanged) error {
	return conn.WriteJSON(SimulationChanged{
		Step: &ev,
	})
}

func (s *server) handleUserEvents(ctx context.Context, conn *websocket.Conn) func() error {
	var ev UserEvent

	return func() error {
		for {
			if err := conn.ReadJSON(&ev); err != nil {
				return err
			}

			s.log.With(ev).Info("got user event")

			switch event := ev.Type().(type) {
			case *Spawn:
				for i := 0; i < event.N; i++ {
					id, err := s.c.Spawn(ctx)
					if err != nil {
						s.log.WithError(err).Error("failed to spawn peer")
						continue
					}

					s.log.WithField("host", id).Debug("spawned host")
				}

			case *Kill:
				for _, id := range event.Hosts {
					if err := s.c.Kill(id); err != nil {
						s.log.WithError(err).WithField("host", id).Error("failed to kill host")
						continue
					}

					s.log.WithField("host", id).Debug("killed host")
				}
			}
		}
	}
}

// func (s *server) newCluster() (*cluster, error) {
// 	u := uuid.New()

// 	sc, err := sim.NewCluster(s.log.WithField("cluster", u).Logger)
// 	if err != nil {
// 		return nil, err
// 	}

// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	c := &cluster{
// 		ID:      u,
// 		Cluster: sc,
// 	}

// 	s.cs[c.ID.String()] = c
// 	return c, nil
// }

// func (s *server) getCluster(id uuid.UUID) (*cluster, bool) {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()

// 	c, ok := s.cs[id.String()]
// 	return c, ok
// }

// func (s *server) stopCluster(id uuid.UUID) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	c := s.cs[id.String()]
// 	if err := c.Close(); err != nil {
// 		s.log.With(c).
// 			WithField("uuid", id).
// 			WithError(err).
// 			Error("failed to close cluster")
// 	}

// 	delete(s.cs, id.String())
// }

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
