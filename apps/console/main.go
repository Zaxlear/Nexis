package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nexis/packages/configlite"
)

func main() {
	cfg := loadConfig()
	st := newStore(cfg.ReverseStart)
	if cfg.DatabaseDSN != "" {
		dbStore, err := openPostgresStore(cfg)
		if err != nil {
			log.Fatalf("Nexis Console database failed: %v", err)
		}
		st.db = dbStore
		if err := st.loadFromDatabase(); err != nil {
			log.Fatalf("load database state failed: %v", err)
		}
		log.Printf("Nexis Console database: PostgreSQL enabled")
	} else {
		log.Printf("Nexis Console database: in-memory mode")
	}
	if cfg.SeedDemo && !st.hasPersistedDemoData() {
		st.seedDemoData()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go st.expireLoop(ctx.Done(), cfg)
	go st.schedulerLoop(ctx, cfg)

	webServer := &http.Server{Addr: cfg.WebAddr, Handler: staticWebHandler(cfg.WebDir)}
	apiServer := &http.Server{Addr: cfg.APIAddr, Handler: apiHandler(st)}
	controlServer := &http.Server{Addr: cfg.ControlAddr, Handler: controlHandler(st, cfg)}
	metricsServer := &http.Server{Addr: cfg.MetricsAddr, Handler: debugHandler(st)}

	go serve("Nexis Console Web", webServer)
	go serve("Nexis Console API", apiServer)
	go serve("Nexis Console Control", controlServer)
	go serve("Nexis Console Metrics", metricsServer)

	log.Printf("Nexis Console Web:     http://127.0.0.1%s", cfg.WebAddr)
	log.Printf("Nexis Console API:     http://127.0.0.1%s/api/v1/nodes", cfg.APIAddr)
	log.Printf("Nexis Console Control: ws://127.0.0.1%s/ws", cfg.ControlAddr)
	log.Printf("Nexis Console Metrics: http://127.0.0.1%s/metrics", cfg.MetricsAddr)
	select {}
}

func loadConfig() config {
	defaultWebDir := "apps/web"
	if _, err := os.Stat(defaultWebDir); err != nil {
		defaultWebDir = filepath.Join("..", "web")
	}
	configPath := env("NEXIS_CONFIG", "")
	if argPath := configlite.PathFromArgs(os.Args[1:], "config"); argPath != "" {
		configPath = argPath
	}
	fileValues, err := configlite.Load(configPath)
	if err != nil {
		log.Fatalf("load config %s failed: %v", configPath, err)
	}

	cfg := config{
		ConfigPath:          configPath,
		WebAddr:             env("NEXIS_WEB_ADDR", addrFromPort(fileValues.Int("console.web_port", 47131))),
		APIAddr:             env("NEXIS_API_ADDR", addrFromPort(fileValues.Int("console.api_port", 47141))),
		ControlAddr:         env("NEXIS_CONTROL_ADDR", addrFromPort(fileValues.Int("console.control_port", 47151))),
		MetricsAddr:         env("NEXIS_METRICS_ADDR", addrFromPort(fileValues.Int("console.metrics_port", 47191))),
		WebDir:              env("NEXIS_WEB_DIR", fileValues.String("console.web_dir", defaultWebDir)),
		PublicHost:          env("NEXIS_PUBLIC_HOST", fileValues.String("console.public_host", "127.0.0.1")),
		ReverseStart:        envInt("NEXIS_REVERSE_START", fileValues.Int("console.reverse_port_start", 47300)),
		ReverseEnd:          envInt("NEXIS_REVERSE_END", fileValues.Int("console.reverse_port_end", 47999)),
		HeartbeatInterval:   time.Duration(envInt("NEXIS_HEARTBEAT_INTERVAL_SEC", fileValues.Int("agent.heartbeat_interval_sec", 15))) * time.Second,
		HeartbeatTimeout:    time.Duration(envInt("NEXIS_HEARTBEAT_TIMEOUT_SEC", fileValues.Int("agent.heartbeat_timeout_sec", 45))) * time.Second,
		LeaseTTL:            time.Duration(envInt("NEXIS_LEASE_TTL_SEC", fileValues.Int("agent.lease_ttl_sec", 90))) * time.Second,
		ScheduleInterval:    time.Duration(envInt("NEXIS_SCHEDULE_INTERVAL_SEC", fileValues.Int("scheduler.forward_probe_interval_sec", 30))) * time.Second,
		SeedDemo:            envBool("NEXIS_SEED_DEMO", fileValues.Bool("console.seed_demo", true)),
		AgentTokenHashes:    parseAgentTokens(env("NEXIS_AGENT_TOKENS", fileValues.String("agent.tokens", ""))),
		DatabaseDSN:         env("NEXIS_DATABASE_DSN", fileValues.String("database.dsn", "")),
		DatabaseAutoMigrate: envBool("NEXIS_DATABASE_AUTO_MIGRATE", fileValues.Bool("database.auto_migrate", true)),
	}

	flag.StringVar(&cfg.ConfigPath, "config", cfg.ConfigPath, "YAML config file path")
	flag.StringVar(&cfg.WebAddr, "web-addr", cfg.WebAddr, "Nexis Console Web listen address")
	flag.StringVar(&cfg.APIAddr, "api-addr", cfg.APIAddr, "Nexis Console REST API listen address")
	flag.StringVar(&cfg.ControlAddr, "control-addr", cfg.ControlAddr, "Nexis Console WebSocket control listen address")
	flag.StringVar(&cfg.MetricsAddr, "metrics-addr", cfg.MetricsAddr, "Nexis Console metrics/debug listen address")
	flag.StringVar(&cfg.WebDir, "web-dir", cfg.WebDir, "static web directory")
	flag.StringVar(&cfg.PublicHost, "public-host", cfg.PublicHost, "host that agents use for reverse logical TCP targets")
	flag.IntVar(&cfg.ReverseStart, "reverse-start", cfg.ReverseStart, "first reverse tunnel port")
	flag.IntVar(&cfg.ReverseEnd, "reverse-end", cfg.ReverseEnd, "last reverse tunnel port")
	flag.BoolVar(&cfg.SeedDemo, "seed-demo", cfg.SeedDemo, "seed demo data for the web console")
	flag.StringVar(&cfg.DatabaseDSN, "database-dsn", cfg.DatabaseDSN, "PostgreSQL DSN; empty keeps in-memory storage only")
	flag.BoolVar(&cfg.DatabaseAutoMigrate, "database-auto-migrate", cfg.DatabaseAutoMigrate, "create/update PostgreSQL tables on startup")
	flag.Parse()

	if cfg.ReverseEnd < cfg.ReverseStart {
		log.Fatalf("invalid reverse port range: %d-%d", cfg.ReverseStart, cfg.ReverseEnd)
	}
	return cfg
}

func serve(name string, server *http.Server) {
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("%s failed: %v", name, err)
	}
}

func staticWebHandler(webDir string) http.Handler {
	fs := http.FileServer(http.Dir(webDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func addrFromPort(port int) string {
	return ":" + strconv.Itoa(port)
}

func parseAgentTokens(raw string) map[string]string {
	tokens := map[string]string{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			parts = strings.SplitN(item, ":", 2)
		}
		if len(parts) != 2 {
			continue
		}
		id := strings.TrimSpace(parts[0])
		token := strings.TrimSpace(parts[1])
		if id != "" && token != "" {
			tokens[id] = tokenHash(token)
		}
	}
	return tokens
}

func newID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
