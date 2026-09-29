package golang

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A service and a command-line tool, each written the way Go projects are,
// fed through the pack file by file and then resolved the way the engine
// does. These are the facts the UI's Go profiles read: which unit is which
// kind, what it belongs to, which modules it uses, and which unit it reaches.

// The files of a gin service: handlers -> service -> store -> models, with
// middleware beside them and a gRPC server as a second entry point.
var service = map[string]string{
	"cmd/server/main.go": `package main

import (
	"log"

	"github.com/acme/shop/internal/handlers"
	"github.com/acme/shop/internal/store"
	"github.com/gin-gonic/gin"
)

func init() { log.SetFlags(0) }

func main() {
	r := gin.Default()
	handlers.Register(r, store.New(nil))
	log.Fatal(r.Run())
}
`,
	"internal/handlers/orders.go": `package handlers

import (
	"net/http"

	"github.com/acme/shop/internal/middleware"
	"github.com/acme/shop/internal/models"
	"github.com/acme/shop/internal/service"
	"github.com/acme/shop/internal/store"
	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	svc *service.OrderService
}

func NewOrderHandler(svc *service.OrderService) *OrderHandler { return &OrderHandler{svc: svc} }

func Register(r *gin.Engine, st *store.OrderStore) {
	h := NewOrderHandler(service.NewOrderService(st))
	r.Use(middleware.RequestLogger())
	r.GET("/orders/:id", h.Get)
	r.POST("/orders", h.Create)
	r.GET("/health", healthHandler())
}

func (h *OrderHandler) Get(c *gin.Context) {
	o, err := h.svc.Find(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, o)
}

func (h *OrderHandler) Create(c *gin.Context) {
	var req models.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, h.svc.Place(c.Request.Context(), req))
}

// A handler written as a closure returned from a function. The function is
// the unit; what the closure uses is the function's.
func healthHandler() gin.HandlerFunc {
	return func(c *gin.Context) { c.String(http.StatusOK, "ok") }
}
`,
	"internal/middleware/logging.go": `package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("%s %s", c.Request.URL.Path, time.Since(start))
	}
}
`,
	"internal/service/orders.go": `package service

import (
	"context"

	"github.com/acme/shop/internal/models"
	"github.com/acme/shop/internal/store"
)

// A repository the service depends on, so the store can be swapped out.
type OrderRepository interface {
	Get(ctx context.Context, id string) (*models.Order, error)
	Save(ctx context.Context, o *models.Order) error
}

type OrderService struct {
	repo OrderRepository
}

func NewOrderService(st *store.OrderStore) *OrderService { return &OrderService{repo: st} }

func (s *OrderService) Find(ctx context.Context, id string) (*models.Order, error) {
	return s.repo.Get(ctx, id)
}

func (s *OrderService) Place(ctx context.Context, req models.CreateOrderRequest) *models.Order {
	o := &models.Order{Customer: req.Customer, Total: req.Total}
	_ = s.repo.Save(ctx, o)
	return o
}
`,
	"internal/store/orders.go": `package store

import (
	"context"
	"database/sql"

	"github.com/acme/shop/internal/models"
	"github.com/redis/go-redis/v9"
)

type OrderStore struct {
	db    *sql.DB
	cache *redis.Client
}

func New(db *sql.DB) *OrderStore { return &OrderStore{db: db} }

func (s *OrderStore) Get(ctx context.Context, id string) (*models.Order, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id, customer, total FROM orders WHERE id = ?", id)
	var o models.Order
	return &o, row.Scan(&o.ID, &o.Customer, &o.Total)
}

func (s *OrderStore) Save(ctx context.Context, o *models.Order) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO orders VALUES (?, ?, ?)", o.ID, o.Customer, o.Total)
	return err
}

// A value receiver on the same type is the same owner.
func (s OrderStore) Ping(ctx context.Context) error { return s.cache.Ping(ctx).Err() }
`,
	"internal/models/order.go": `package models

import "time"

type Timestamps struct {
	CreatedAt time.Time ` + "`json:\"created_at\" db:\"created_at\"`" + `
}

type Order struct {
	Timestamps
	ID       string  ` + "`json:\"id\" db:\"id\"`" + `
	Customer string  ` + "`json:\"customer\" db:\"customer\"`" + `
	Total    float64 ` + "`json:\"total\" db:\"total\"`" + `
}

type CreateOrderRequest struct {
	Customer string  ` + "`json:\"customer\" binding:\"required\"`" + `
	Total    float64 ` + "`json:\"total\"`" + `
}

// A struct tagged json that is configuration, not a model.
type Config struct {
	Addr string ` + "`json:\"addr\" yaml:\"addr\"`" + `
	DSN  string ` + "`json:\"dsn\" yaml:\"dsn\"`" + `
}
`,
	"internal/grpc/server.go": `package grpc

import (
	"context"

	"github.com/acme/shop/internal/service"
	pb "github.com/acme/shop/internal/grpc/pb"
	"google.golang.org/grpc"
)

type Server struct {
	pb.UnimplementedOrdersServer
	svc *service.OrderService
}

func NewServer(svc *service.OrderService) *grpc.Server {
	s := grpc.NewServer()
	pb.RegisterOrdersServer(s, &Server{svc: svc})
	return s
}

func (s *Server) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.Order, error) {
	o, err := s.svc.Find(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.Order{Id: o.ID}, nil
}
`,
	"internal/grpc/pb/orders.pb.go": `// Code generated by protoc-gen-go. DO NOT EDIT.
package pb

import "context"

type Order struct {
	Id string ` + "`protobuf:\"bytes,1,opt,name=id\" json:\"id,omitempty\"`" + `
}

type GetOrderRequest struct {
	Id string ` + "`protobuf:\"bytes,1,opt,name=id\" json:\"id,omitempty\"`" + `
}

type UnimplementedOrdersServer struct{}

func (UnimplementedOrdersServer) GetOrder(context.Context, *GetOrderRequest) (*Order, error) { return nil, nil }

func RegisterOrdersServer(s interface{}, srv interface{}) {}
`,
	"internal/store/orders_test.go": `package store

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func init() {}

func TestGet(t *testing.T) {
	db, _, _ := sqlmock.New()
	_ = New(db)
}
`,
}

// A cobra tool: commands -> logic -> readers and writers -> config.
var cli = map[string]string{
	"main.go": `package main

import "github.com/acme/tidy/cmd"

func main() { cmd.Execute() }
`,
	"cmd/root.go": `package cmd

import (
	"os"

	"github.com/acme/tidy/internal/config"
	"github.com/acme/tidy/internal/tidy"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{Use: "tidy"}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use: "run",
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := config.Load(args[0])
			if err != nil {
				return err
			}
			return tidy.Run(cfg)
		},
	}
}

func init() { rootCmd.AddCommand(newRunCmd()) }
`,
	"internal/tidy/run.go": `package tidy

import (
	"github.com/acme/tidy/internal/config"
	"github.com/acme/tidy/internal/io"
)

type Runner struct{ cfg *config.Config }

func NewRunner(cfg *config.Config) *Runner { return &Runner{cfg: cfg} }

func Run(cfg *config.Config) error { return NewRunner(cfg).Run() }

func (r *Runner) Run() error {
	lines, err := io.NewFileReader(r.cfg.Input).ReadAll()
	if err != nil {
		return err
	}
	return io.NewFileWriter(r.cfg.Output).WriteAll(Dedupe(lines))
}

// A generic function.
func Dedupe[T comparable](xs []T) []T {
	seen := map[T]bool{}
	var out []T
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
`,
	"internal/io/files.go": `package io

import (
	"bufio"
	"os"
)

type FileReader struct{ path string }
type FileWriter struct{ path string }

func NewFileReader(path string) *FileReader { return &FileReader{path: path} }
func NewFileWriter(path string) *FileWriter { return &FileWriter{path: path} }

func (r *FileReader) ReadAll() ([]string, error) {
	f, err := os.Open(r.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out, sc.Err()
}

func (w *FileWriter) WriteAll(lines []string) error {
	f, err := os.Create(w.path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, l := range lines {
		f.WriteString(l + "\n")
	}
	return nil
}

// A generic type and its methods, on a pointer receiver.
type Buffer[T any] struct{ items []T }

func (b *Buffer[T]) Push(x T) { b.items = append(b.items, x) }
func (b *Buffer[T]) Len() int  { return len(b.items) }
`,
	"internal/config/config.go": `package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Input  string ` + "`yaml:\"input\" json:\"input\"`" + `
	Output string ` + "`yaml:\"output\" json:\"output\"`" + `
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	return &c, yaml.Unmarshal(b, &c)
}
`,
}

type project struct {
	units  map[string]*unit.Unit
	edges  map[string][]string // from -> to
	raw    map[string][]string // file -> raw imports
	merged []*unit.Unit
}

// Every file through the pack, then merged and resolved the way the engine
// does once it has the whole codebase.
func analyseProject(t *testing.T, files map[string]string) *project {
	t.Helper()
	var all []*unit.Unit
	raw := map[string][]string{}
	for path, src := range files {
		res := analyse(t, path, src)
		all = append(all, res.Units...)
		for _, s := range res.Snippets {
			if s.Type == file.ImportRaw {
				raw[path] = append(raw[path], s.Value)
			}
		}
	}
	merged := unit.Merge(all)
	p := &project{units: unitsByID(merged), edges: map[string][]string{}, raw: raw, merged: merged}
	for _, c := range unit.Connections(merged) {
		p.edges[c.From] = append(p.edges[c.From], c.To)
	}
	return p
}

// The modules a unit uses, as the unit_uses view records them.
func (p *project) uses(id string) []string {
	u := p.units[id]
	if u == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, r := range u.Refs {
		if r.Module != "" && !seen[r.Module] {
			seen[r.Module] = true
			out = append(out, r.Module)
		}
	}
	return out
}

func (p *project) reaches(from, to string) bool {
	for _, t := range p.edges[from] {
		if t == to {
			return true
		}
	}
	return false
}

func TestServiceUnitsAndOwners(t *testing.T) {
	p := analyseProject(t, service)

	// Each layer's type, and its methods owned by the receiver.
	for _, want := range []struct{ id, kind, owner string }{
		{"internal/handlers.OrderHandler", unit.KindType, ""},
		{"internal/handlers.OrderHandler.Get", unit.KindFunction, "internal/handlers.OrderHandler"},
		{"internal/handlers.OrderHandler.Create", unit.KindFunction, "internal/handlers.OrderHandler"},
		{"internal/handlers.NewOrderHandler", unit.KindFunction, ""},
		{"internal/handlers.Register", unit.KindFunction, ""},
		{"internal/handlers.healthHandler", unit.KindFunction, ""},
		{"internal/middleware.RequestLogger", unit.KindFunction, ""},
		{"internal/service.OrderService", unit.KindType, ""},
		{"internal/service.OrderRepository", unit.KindType, ""},
		{"internal/service.OrderService.Find", unit.KindFunction, "internal/service.OrderService"},
		{"internal/store.OrderStore", unit.KindType, ""},
		{"internal/store.OrderStore.Get", unit.KindFunction, "internal/store.OrderStore"},
		{"internal/store.OrderStore.Ping", unit.KindFunction, "internal/store.OrderStore"},
		{"internal/models.Order", unit.KindType, ""},
		{"internal/models.Config", unit.KindType, ""},
		{"internal/grpc.Server", unit.KindType, ""},
		{"internal/grpc.Server.GetOrder", unit.KindFunction, "internal/grpc.Server"},
		{"cmd/server.main", unit.KindFunction, ""},
	} {
		u := p.units[want.id]
		require.NotNilf(t, u, "%s is not a unit", want.id)
		assert.Equal(t, want.kind, u.Kind, want.id)
		assert.Equal(t, want.owner, u.Owner, want.id)
	}

	// init is per file, so the store's test init and the server's do not fold.
	assert.Contains(t, p.units, "cmd/server.init@main.go")
	assert.Contains(t, p.units, "internal/store.init@orders_test.go")
	// A test file's functions are units like any other; the UI decides what a test is.
	assert.Contains(t, p.units, "internal/store.TestGet")
}

func TestServiceMarkers(t *testing.T) {
	p := analyseProject(t, service)

	order := p.units["internal/models.Order"]
	assert.ElementsMatch(t, []string{"id", "customer", "total"}, order.MarkerValues("json"))
	assert.ElementsMatch(t, []string{"id", "customer", "total"}, order.MarkerValues("db"))
	assert.True(t, order.HasMarker("Timestamps"), "an embedded struct is a supertype")
	for _, m := range order.Markers {
		if m.Key == "Timestamps" {
			assert.Equal(t, unit.SourceSupertype, m.Source)
		}
		if m.Key == "json" {
			assert.Equal(t, unit.SourceStructTag, m.Source)
		}
	}

	// The config struct carries json and yaml like a model does. Nothing in
	// the tags tells them apart; the name has to.
	cfg := p.units["internal/models.Config"]
	assert.True(t, cfg.HasMarker("json"))
	assert.True(t, cfg.HasMarker("yaml"))

	assert.True(t, p.units["internal/service.OrderRepository"].HasMarker("interface"))
	assert.False(t, p.units["internal/service.OrderService"].HasMarker("interface"))

	// The gRPC server embeds the generated Unimplemented type.
	assert.True(t, p.units["internal/grpc.Server"].HasMarker("UnimplementedOrdersServer"))

	// Handlers, services and stores carry no tags at all: their lane comes
	// from imports and names.
	for _, id := range []string{"internal/handlers.OrderHandler", "internal/service.OrderService", "internal/store.OrderStore"} {
		assert.Emptyf(t, p.units[id].Markers, "%s should carry no marker", id)
	}
}

// Raw imports are the file's; uses are each unit's. A store's methods are
// where database/sql is used, and the file's redis import is used by Ping
// alone.
func TestServiceImportsAndUses(t *testing.T) {
	p := analyseProject(t, service)

	assert.ElementsMatch(t, []string{"context", "database/sql", "github.com/acme/shop/internal/models", "github.com/redis/go-redis/v9"},
		p.raw["internal/store/orders.go"])

	assert.Contains(t, p.uses("internal/store.OrderStore"), "database/sql", "the field type names the package")
	assert.Contains(t, p.uses("internal/store.OrderStore"), "github.com/redis/go-redis/v9")
	assert.NotContains(t, p.uses("internal/store.OrderStore.Get"), "github.com/redis/go-redis/v9")
	assert.Contains(t, p.uses("internal/store.OrderStore.Get"), "github.com/acme/shop/internal/models")
	assert.Contains(t, p.uses("internal/store.OrderStore.Ping"), "context")

	assert.Contains(t, p.uses("internal/handlers.OrderHandler.Get"), "net/http")
	assert.Contains(t, p.uses("internal/handlers.OrderHandler.Get"), "github.com/gin-gonic/gin")
	// The closure's uses belong to the function that returns it.
	assert.Contains(t, p.uses("internal/handlers.healthHandler"), "net/http")
	assert.Contains(t, p.uses("internal/middleware.RequestLogger"), "github.com/gin-gonic/gin")
	assert.Contains(t, p.uses("internal/grpc.NewServer"), "google.golang.org/grpc")

	// Nothing in the models package uses anything but time and its own
	// package: a builtin like string is read as a same-package name, which
	// resolves to nothing.
	assert.Equal(t, []string{"time"}, p.uses("internal/models.Timestamps"))
	for _, id := range []string{"internal/models.Order", "internal/models.Config", "internal/models.CreateOrderRequest"} {
		for _, m := range p.uses(id) {
			assert.Equalf(t, "internal/models", m, "%s uses %s", id, m)
		}
	}
}

// The name half of `time.Time` is not an unqualified name. Read as one, a
// package that declares its own Time got an edge from every field of the
// standard library's.
func TestAQualifiedTypeIsNotALocalName(t *testing.T) {
	src := `package clock

import "time"

type Time struct{ n int }

type Stamp struct {
	At time.Time
}
`
	byID := unitsByID(analyse(t, "internal/clock/clock.go", src).Units)
	stamp := byID["internal/clock.Stamp"]
	require.NotNil(t, stamp)
	assert.Contains(t, stamp.Refs, unit.Ref{Module: "time", Name: "Time"})
	assert.NotContains(t, stamp.Refs, unit.Ref{Module: "internal/clock", Name: "Time", Exact: true})
	// An unqualified use of the package's own Time is still an edge.
	byID = unitsByID(analyse(t, "internal/clock/clock.go", src+"\nfunc Now() Time { return Time{} }\n").Units)
	assert.Contains(t, byID["internal/clock.Now"].Refs, unit.Ref{Module: "internal/clock", Name: "Time", Exact: true})
}

// A versioned import path is written by the element before the version:
// `github.com/redis/go-redis/v9` is `redis.Client`, `gopkg.in/yaml.v3` is
// `yaml.Unmarshal`, and `github.com/go-chi/chi/v5` is `chi.NewRouter`.
func TestVersionedImportPathsPairWithTheirSelectors(t *testing.T) {
	src := `package app

import (
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
	"github.com/mattn/go-sqlite3"
)

func Wire() {
	_ = chi.NewRouter()
	_ = redis.NewClient(nil)
	_ = yaml.Unmarshal(nil, nil)
	_ = sqlite3.Version()
}
`
	byID := unitsByID(analyse(t, "app/wire.go", src).Units)
	wire := byID["app.Wire"]
	require.NotNil(t, wire)
	var modules []string
	for _, r := range wire.Refs {
		if r.Module != "app" {
			modules = append(modules, r.Module)
		}
	}
	assert.ElementsMatch(t, []string{"github.com/go-chi/chi/v5", "github.com/redis/go-redis/v9", "gopkg.in/yaml.v3", "github.com/mattn/go-sqlite3"}, modules)
	// The package half of a selector is not a same-package name either.
	for _, r := range wire.Refs {
		assert.NotContains(t, []string{"chi", "redis", "yaml", "sqlite3"}, r.Name, "%v", r)
	}
}

// The edges the layers reading counts, members rolled up to their owners by
// the UI. Here they are as the engine records them.
func TestServiceEdgesRunDownTheLayers(t *testing.T) {
	p := analyseProject(t, service)

	// main -> handlers, store
	assert.True(t, p.reaches("cmd/server.main", "internal/handlers.Register"))
	assert.True(t, p.reaches("cmd/server.main", "internal/store.New"))
	// handlers -> service, models, middleware
	assert.True(t, p.reaches("internal/handlers.OrderHandler", "internal/service.OrderService"))
	assert.True(t, p.reaches("internal/handlers.OrderHandler.Get", "internal/handlers.OrderHandler"), "a method reaches its receiver")
	assert.True(t, p.reaches("internal/handlers.OrderHandler.Create", "internal/models.CreateOrderRequest"))
	assert.True(t, p.reaches("internal/handlers.Register", "internal/middleware.RequestLogger"))
	assert.True(t, p.reaches("internal/handlers.Register", "internal/service.NewOrderService"))
	// service -> store, models
	assert.True(t, p.reaches("internal/service.NewOrderService", "internal/store.OrderStore"))
	assert.True(t, p.reaches("internal/service.OrderService.Place", "internal/models.Order"))
	assert.True(t, p.reaches("internal/service.OrderService", "internal/service.OrderRepository"))
	// store -> models
	assert.True(t, p.reaches("internal/store.OrderStore.Get", "internal/models.Order"))
	// grpc -> service, generated pb
	assert.True(t, p.reaches("internal/grpc.Server", "internal/service.OrderService"))
	assert.True(t, p.reaches("internal/grpc.Server", "internal/grpc/pb.UnimplementedOrdersServer"))
	assert.True(t, p.reaches("internal/grpc.Server.GetOrder", "internal/grpc/pb.GetOrderRequest"))

	// Nothing runs back up.
	for from, tos := range p.edges {
		for _, to := range tos {
			if pkgOf(from) == "internal/models" {
				assert.Equal(t, "internal/models", pkgOf(to), "models reach %s", to)
			}
			if pkgOf(from) == "internal/store" {
				assert.NotContains(t, []string{"internal/service", "internal/handlers"}, pkgOf(to), "%s reaches %s", from, to)
			}
		}
	}

	// The same-package method call `h.svc.Find` is a field selector, not a
	// package one, so no edge is invented from the handler to the service's
	// method; the type edge carries the dependency.
	assert.False(t, p.reaches("internal/handlers.OrderHandler.Get", "internal/service.OrderService.Find"))
}

func pkgOf(id string) string {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '.' {
			return id[:i]
		}
	}
	return ""
}

func TestCliUnitsAndEdges(t *testing.T) {
	p := analyseProject(t, cli)

	assert.Contains(t, p.units, "main", "a root-level main has no package prefix")
	assert.Contains(t, p.units, "cmd.Execute")
	assert.Contains(t, p.units, "cmd.newRunCmd")
	assert.Contains(t, p.units, "cmd.init@root.go")
	assert.Contains(t, p.units, "internal/tidy.Run")
	assert.Contains(t, p.units, "internal/tidy.Dedupe", "a generic function is a function")
	assert.Contains(t, p.units, "internal/io.FileReader")
	assert.Contains(t, p.units, "internal/io.NewFileReader")
	assert.Contains(t, p.units, "internal/config.Config")
	assert.Contains(t, p.units, "internal/config.Load")

	// A method on a generic type belongs to that type, not to whichever
	// receiver the pack saw last.
	require.Contains(t, p.units, "internal/io.Buffer")
	require.Contains(t, p.units, "internal/io.Buffer.Push")
	assert.Equal(t, "internal/io.Buffer", p.units["internal/io.Buffer.Push"].Owner)
	assert.Equal(t, "internal/io.Buffer", p.units["internal/io.Buffer.Len"].Owner)
	assert.NotContains(t, p.units, "internal/io.FileWriter.Push")

	// Config carries the tags a CLI's config does.
	cfg := p.units["internal/config.Config"]
	assert.ElementsMatch(t, []string{"input", "output"}, cfg.MarkerValues("yaml"))
	assert.ElementsMatch(t, []string{"input", "output"}, cfg.MarkerValues("json"))

	// rootCmd is a package var, not a unit, so Execute reaches cobra only
	// through it and records no use of its own; newRunCmd names cobra directly.
	assert.NotContains(t, p.uses("cmd.Execute"), "github.com/spf13/cobra")
	assert.Contains(t, p.uses("cmd.newRunCmd"), "github.com/spf13/cobra")
	assert.Contains(t, p.uses("internal/config.Load"), "gopkg.in/yaml.v3")
	assert.Contains(t, p.uses("internal/io.FileReader.ReadAll"), "os")

	// main -> cmd -> tidy -> io, config
	assert.True(t, p.reaches("main", "cmd.Execute"))
	assert.True(t, p.reaches("cmd.newRunCmd", "internal/tidy.Run"))
	assert.True(t, p.reaches("cmd.newRunCmd", "internal/config.Load"))
	assert.True(t, p.reaches("internal/tidy.Run", "internal/tidy.NewRunner"))
	assert.True(t, p.reaches("internal/tidy.Runner.Run", "internal/io.NewFileReader"))
	assert.True(t, p.reaches("internal/tidy.Runner.Run", "internal/io.NewFileWriter"))
	assert.True(t, p.reaches("internal/tidy.Runner", "internal/config.Config"))
	assert.True(t, p.reaches("internal/tidy.Runner.Run", "internal/tidy.Dedupe"))
	assert.False(t, p.reaches("internal/io.FileReader.ReadAll", "internal/tidy.Runner"))
}
