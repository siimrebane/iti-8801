// ITI8801 course application: "Notes".
//
// One binary, two modes, one image. Students deploy it; they do not write it.
//
//	MODE=frontend  serves the built Vue page at /, forwards /api/* to API_URL,
//	               and answers the operational endpoints below for itself.
//	MODE=api       the backend: notes in PostgreSQL (DATABASE_URL), the same
//	               operational endpoints for itself.
//
// Operational endpoints, both modes:
//
//	GET /health    200 {"status":"ok"} or 503 when the mode's dependency is down
//	GET /version   {"version","commit","mode","hostname"}
//	GET /metrics   Prometheus text format: requests, latency, notes (api)
//	GET /report    the self-report the course validator reads (also at /hello):
//	               who I am, my private addresses, what I can and cannot reach.
//	               With ?nonce=<x> the answer is HMAC-signed with the build-time
//	               secret, exactly like the course beacon.
//
// API (MODE=api), and forwarded by the frontend:
//
//	GET    /api/notes        [{"id","text","created_at"}], oldest first
//	POST   /api/notes        {"text"} -> 201 with the created note
//	DELETE /api/notes/{id}   204, or 404
//	GET    /api/version      same as /version (so the page can show it)
//	GET    /api/report       same as /report (so the validator can read the backend through the frontend)
//
// Probes (both modes, every 30 s, reported in /report):
//
//	outbound       fixed internet targets; a locked-down subnet fails them all
//	DB_TARGET      host:port of the database; dialled after resolving, the
//	               address reported. The api tier must reach it; the frontend
//	               tier must NOT (it is given the same target to prove that)
//	PROBE_TARGETS  comma-separated host:port list, each dialled and reported
package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed all:dist
var dist embed.FS

// Set at build time: -ldflags "-X main.version=a4 -X main.commit=abc1234 -X main.secret=...".
var (
	version = "dev"
	commit  = "local"
	secret  = "dev-secret"
)

var (
	mode     = env("MODE", "api")
	instID   = newID()
	bootedAt = time.Now().UTC()
	hostname = func() string { h, _ := os.Hostname(); return h }()
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- self-report (the beacon, carried over) ----------

type probeResult struct {
	Target  string `json:"target"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Checked string `json:"checked"`
	Addr    string `json:"addr,omitempty"` // resolved IPv4 that was dialled (DB target)
}

type ifaceInfo struct {
	Name string   `json:"name"`
	MAC  string   `json:"mac,omitempty"`
	IPs  []string `json:"ips"`
}

type cloudIdentity struct {
	Provider  string `json:"provider"`
	VMID      string `json:"vm_id"`
	AccountID string `json:"account_id"`
	Region    string `json:"region"`
	Name      string `json:"name,omitempty"`
}

type report struct {
	App        string         `json:"app"` // "iti8801-notes"
	Mode       string         `json:"mode"`
	Version    string         `json:"version"`
	Commit     string         `json:"commit"`
	BeaconID   string         `json:"beacon_id"`
	BootedAt   string         `json:"booted_at"`
	UptimeSec  int64          `json:"uptime_sec"`
	Hostname   string         `json:"hostname"`
	Interfaces []ifaceInfo    `json:"interfaces"`
	Outbound   []probeResult  `json:"outbound"`
	DBCheck    *probeResult   `json:"db_check,omitempty"`
	Probes     []probeResult  `json:"probes,omitempty"`
	DBQuery    *bool          `json:"db_query,omitempty"` // api: SELECT 1 succeeded
	Cloud      *cloudIdentity `json:"cloud,omitempty"`
	Nonce      string         `json:"nonce,omitempty"`
	Sig        string         `json:"sig,omitempty"`
}

// canonical is the string both sides sign. The validator for this app
// mirrors it exactly: nonce | id | booted | mode | outbound… | db | dbaddr |
// probe:… | dbquery | cloud.
func canonical(r *report) string {
	parts := []string{r.Nonce, r.BeaconID, r.BootedAt, "mode=" + r.Mode}
	for _, o := range r.Outbound {
		parts = append(parts, fmt.Sprintf("%s=%v", o.Target, o.OK))
	}
	if r.DBCheck != nil {
		parts = append(parts, fmt.Sprintf("db=%v", r.DBCheck.OK))
		if r.DBCheck.Addr != "" {
			parts = append(parts, "dbaddr="+r.DBCheck.Addr)
		}
	}
	for _, p := range r.Probes {
		parts = append(parts, fmt.Sprintf("probe:%s=%v", p.Target, p.OK))
	}
	if r.DBQuery != nil {
		parts = append(parts, fmt.Sprintf("dbquery=%v", *r.DBQuery))
	}
	if r.Cloud != nil {
		parts = append(parts, fmt.Sprintf("cloud=%s/%s/%s", r.Cloud.Provider, r.Cloud.VMID, r.Cloud.AccountID))
	}
	return strings.Join(parts, "|")
}

func sign(msg string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

var outboundTargets = []string{"1.1.1.1:443", "8.8.8.8:53"}

var (
	probeMu    sync.RWMutex
	lastOut    []probeResult
	lastDB     *probeResult
	lastProbes []probeResult
	lastQuery  *bool
	cloudMu    sync.RWMutex
	cloud      *cloudIdentity
)

func dialCheck(target string) probeResult {
	res := probeResult{Target: target, Checked: time.Now().UTC().Format(time.RFC3339)}
	conn, err := net.DialTimeout("tcp", target, 3*time.Second)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	conn.Close()
	res.OK = true
	return res
}

// dialResolved resolves the host to an IPv4 first and reports the address it
// dialled: for a managed database that address proves which subnet it is in.
func dialResolved(target string) probeResult {
	res := probeResult{Target: target, Checked: time.Now().UTC().Format(time.RFC3339)}
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		res.Error = "resolve: " + err.Error()
		return res
	}
	var ip net.IP
	for _, a := range ips {
		if v4 := a.IP.To4(); v4 != nil {
			ip = v4
			break
		}
	}
	if ip == nil {
		res.Error = "resolve: no IPv4 address for " + host
		return res
	}
	res.Addr = ip.String()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip.String(), port), 3*time.Second)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	conn.Close()
	res.OK = true
	return res
}

func runProbes() {
	out := make([]probeResult, 0, 3)
	for _, t := range outboundTargets {
		out = append(out, dialCheck(t))
	}
	dnsRes := probeResult{Target: "dns:example.com", Checked: time.Now().UTC().Format(time.RFC3339)}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	if _, err := net.DefaultResolver.LookupHost(ctx, "example.com"); err != nil {
		dnsRes.Error = err.Error()
	} else {
		dnsRes.OK = true
	}
	cancel()
	out = append(out, dnsRes)

	var db *probeResult
	if t := os.Getenv("DB_TARGET"); t != "" {
		r := dialResolved(t)
		db = &r
	}
	var probes []probeResult
	for _, t := range strings.Split(os.Getenv("PROBE_TARGETS"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			probes = append(probes, dialResolved(t))
		}
	}
	var q *bool
	if mode == "api" {
		ok := dbPing()
		q = &ok
	}
	probeMu.Lock()
	lastOut, lastDB, lastProbes, lastQuery = out, db, probes, q
	probeMu.Unlock()
}

func interfaces() []ifaceInfo {
	var out []ifaceInfo
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		info := ifaceInfo{Name: iface.Name, MAC: iface.HardwareAddr.String()}
		for _, a := range addrs {
			info.IPs = append(info.IPs, a.String())
		}
		if len(info.IPs) > 0 {
			out = append(out, info)
		}
	}
	return out
}

func metaClient() *http.Client { return &http.Client{Timeout: 2 * time.Second} }

func fetchAzure() *cloudIdentity {
	req, _ := http.NewRequest("GET", "http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01", nil)
	req.Header.Set("Metadata", "true")
	resp, err := metaClient().Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	defer resp.Body.Close()
	var v struct {
		VMID string `json:"vmId"`
		Sub  string `json:"subscriptionId"`
		Loc  string `json:"location"`
		Name string `json:"name"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&v) != nil || v.VMID == "" {
		return nil
	}
	return &cloudIdentity{Provider: "azure", VMID: v.VMID, AccountID: v.Sub, Region: v.Loc, Name: v.Name}
}

func fetchAWS() *cloudIdentity {
	req, _ := http.NewRequest("PUT", "http://169.254.169.254/latest/api/token", nil)
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")
	resp, err := metaClient().Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	tok, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	req, _ = http.NewRequest("GET", "http://169.254.169.254/latest/dynamic/instance-identity/document", nil)
	req.Header.Set("X-aws-ec2-metadata-token", string(tok))
	resp, err = metaClient().Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	defer resp.Body.Close()
	var v struct {
		ID      string `json:"instanceId"`
		Account string `json:"accountId"`
		Region  string `json:"region"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&v) != nil || v.ID == "" {
		return nil
	}
	return &cloudIdentity{Provider: "aws", VMID: v.ID, AccountID: v.Account, Region: v.Region}
}

func fetchCloud() {
	cloudMu.RLock()
	have := cloud != nil
	cloudMu.RUnlock()
	if have {
		return
	}
	c := fetchAzure()
	if c == nil {
		c = fetchAWS()
	}
	if c != nil {
		cloudMu.Lock()
		cloud = c
		cloudMu.Unlock()
		log.Printf("cloud identity: %s vm=%s account=%s region=%s", c.Provider, c.VMID, c.AccountID, c.Region)
	}
}

func handleReport(w http.ResponseWriter, r *http.Request) {
	probeMu.RLock()
	rep := report{App: "iti8801-notes", Mode: mode, Version: version, Commit: commit, BeaconID: instID,
		BootedAt: bootedAt.Format(time.RFC3339), UptimeSec: int64(time.Since(bootedAt).Seconds()), Hostname: hostname,
		Interfaces: interfaces(), Outbound: lastOut, DBCheck: lastDB, Probes: lastProbes, DBQuery: lastQuery}
	probeMu.RUnlock()
	cloudMu.RLock()
	rep.Cloud = cloud
	cloudMu.RUnlock()
	if nonce := r.URL.Query().Get("nonce"); nonce != "" && len(nonce) <= 64 {
		rep.Nonce = nonce
		rep.Sig = sign(canonical(&rep))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rep)
}

// ---------- database (api mode) ----------

var (
	poolMu sync.RWMutex
	pool   *pgxpool.Pool
)

func getPool() *pgxpool.Pool {
	poolMu.RLock()
	defer poolMu.RUnlock()
	return pool
}

// connectDB keeps trying: the database may come up after the application,
// and a VM that boots before its database is normal in a fresh environment.
func connectDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Printf("WARNING: DATABASE_URL not set; the api runs without a database")
		return
	}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		p, err := pgxpool.New(ctx, dsn)
		if err == nil {
			_, err = p.Exec(ctx, `CREATE TABLE IF NOT EXISTS notes (
				id SERIAL PRIMARY KEY, text TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`)
		}
		cancel()
		if err == nil {
			poolMu.Lock()
			pool = p
			poolMu.Unlock()
			log.Printf("database connected")
			return
		}
		if p != nil {
			p.Close()
		}
		log.Printf("database not ready: %v (retrying in 5 s)", err)
		time.Sleep(5 * time.Second)
	}
}

func dbPing() bool {
	p := getPool()
	if p == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var one int
	return p.QueryRow(ctx, "SELECT 1").Scan(&one) == nil
}

type note struct {
	ID        int64     `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	jsonOut(w, status, map[string]string{"error": msg})
}

var noteIDRe = regexp.MustCompile(`^/api/notes/([0-9]{1,18})$`)

func handleNotes(w http.ResponseWriter, r *http.Request) {
	p := getPool()
	if p == nil {
		jsonErr(w, 503, "database not connected")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	switch {
	case r.URL.Path == "/api/notes" && r.Method == http.MethodGet:
		rows, err := p.Query(ctx, "SELECT id, text, created_at FROM notes ORDER BY id")
		if err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		defer rows.Close()
		notes := []note{}
		for rows.Next() {
			var n note
			if rows.Scan(&n.ID, &n.Text, &n.CreatedAt) == nil {
				notes = append(notes, n)
			}
		}
		jsonOut(w, 200, notes)
	case r.URL.Path == "/api/notes" && r.Method == http.MethodPost:
		var in struct {
			Text string `json:"text"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in) != nil || strings.TrimSpace(in.Text) == "" {
			jsonErr(w, 400, `body must be {"text": "..."}`)
			return
		}
		if len(in.Text) > 200 {
			jsonErr(w, 400, "text: at most 200 characters")
			return
		}
		var n note
		err := p.QueryRow(ctx, "INSERT INTO notes (text) VALUES ($1) RETURNING id, text, created_at", strings.TrimSpace(in.Text)).Scan(&n.ID, &n.Text, &n.CreatedAt)
		if err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		jsonOut(w, 201, n)
	case noteIDRe.MatchString(r.URL.Path) && r.Method == http.MethodDelete:
		id, _ := strconv.ParseInt(noteIDRe.FindStringSubmatch(r.URL.Path)[1], 10, 64)
		tag, err := p.Exec(ctx, "DELETE FROM notes WHERE id = $1", id)
		if err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		if tag.RowsAffected() == 0 {
			jsonErr(w, 404, "no such note")
			return
		}
		w.WriteHeader(204)
	case noteIDRe.MatchString(r.URL.Path) && r.Method == http.MethodGet:
		id, _ := strconv.ParseInt(noteIDRe.FindStringSubmatch(r.URL.Path)[1], 10, 64)
		var n note
		err := p.QueryRow(ctx, "SELECT id, text, created_at FROM notes WHERE id = $1", id).Scan(&n.ID, &n.Text, &n.CreatedAt)
		if err != nil {
			jsonErr(w, 404, "no such note")
			return
		}
		jsonOut(w, 200, n)
	default:
		jsonErr(w, 405, "method not allowed")
	}
}

// ---------- metrics (Prometheus text format, no library) ----------

type metricKey struct{ method, route, status string }

var (
	metMu       sync.Mutex
	reqCount    = map[metricKey]int64{}
	latBuckets  = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}
	latCounts   = map[string][]int64{} // route -> per-bucket cumulative counts (+Inf last)
	latSum      = map[string]float64{}
	latN        = map[string]int64{}
	routeIDRe   = regexp.MustCompile(`^/api/notes/[0-9]+$`)
	assetsRoute = regexp.MustCompile(`^/assets/`)
)

func routeOf(p string) string {
	switch {
	case routeIDRe.MatchString(p):
		return "/api/notes/{id}"
	case assetsRoute.MatchString(p):
		return "/assets/*"
	case strings.HasPrefix(p, "/api/") || p == "/" || p == "/health" || p == "/version" || p == "/metrics" || p == "/report" || p == "/hello":
		return p
	}
	return "other"
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(c int) { s.status = c; s.ResponseWriter.WriteHeader(c) }

func observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		if r.URL.Path == "/metrics" {
			return
		}
		d := time.Since(start).Seconds()
		route := routeOf(r.URL.Path)
		metMu.Lock()
		reqCount[metricKey{r.Method, route, strconv.Itoa(sw.status)}]++
		b := latCounts[route]
		if b == nil {
			b = make([]int64, len(latBuckets)+1)
			latCounts[route] = b
		}
		for i, le := range latBuckets {
			if d <= le {
				b[i]++
			}
		}
		b[len(latBuckets)]++
		latSum[route] += d
		latN[route]++
		metMu.Unlock()
	})
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP app_info Build information.\n# TYPE app_info gauge\napp_info{app=\"iti8801-notes\",mode=%q,version=%q,commit=%q} 1\n", mode, version, commit)
	fmt.Fprintf(w, "# HELP app_uptime_seconds Seconds since the process started.\n# TYPE app_uptime_seconds gauge\napp_uptime_seconds %d\n", int64(time.Since(bootedAt).Seconds()))
	metMu.Lock()
	keys := make([]metricKey, 0, len(reqCount))
	for k := range reqCount {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].route+keys[i].method+keys[i].status < keys[j].route+keys[j].method+keys[j].status
	})
	fmt.Fprintf(w, "# HELP app_http_requests_total Requests handled, by method, route and status.\n# TYPE app_http_requests_total counter\n")
	for _, k := range keys {
		fmt.Fprintf(w, "app_http_requests_total{method=%q,route=%q,status=%q} %d\n", k.method, k.route, k.status, reqCount[k])
	}
	routes := make([]string, 0, len(latCounts))
	for rt := range latCounts {
		routes = append(routes, rt)
	}
	sort.Strings(routes)
	fmt.Fprintf(w, "# HELP app_http_request_duration_seconds Request latency.\n# TYPE app_http_request_duration_seconds histogram\n")
	for _, rt := range routes {
		b := latCounts[rt]
		for i, le := range latBuckets {
			fmt.Fprintf(w, "app_http_request_duration_seconds_bucket{route=%q,le=\"%g\"} %d\n", rt, le, b[i])
		}
		fmt.Fprintf(w, "app_http_request_duration_seconds_bucket{route=%q,le=\"+Inf\"} %d\n", rt, b[len(latBuckets)])
		fmt.Fprintf(w, "app_http_request_duration_seconds_sum{route=%q} %f\n", rt, latSum[rt])
		fmt.Fprintf(w, "app_http_request_duration_seconds_count{route=%q} %d\n", rt, latN[rt])
	}
	metMu.Unlock()
	if mode == "api" {
		if p := getPool(); p != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			var n int64
			if p.QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&n) == nil {
				fmt.Fprintf(w, "# HELP app_notes_total Notes in the database.\n# TYPE app_notes_total gauge\napp_notes_total %d\n", n)
			}
			cancel()
		}
		up := 0
		if dbPing() {
			up = 1
		}
		fmt.Fprintf(w, "# HELP app_db_up 1 when the database answers.\n# TYPE app_db_up gauge\napp_db_up %d\n", up)
	}
}

// ---------- health and version ----------

func handleHealth(w http.ResponseWriter, r *http.Request) {
	switch mode {
	case "api":
		if dbPing() {
			jsonOut(w, 200, map[string]string{"status": "ok", "mode": mode, "db": "ok"})
		} else {
			jsonOut(w, 503, map[string]string{"status": "degraded", "mode": mode, "db": "unreachable"})
		}
	default:
		ok := apiReachable()
		if ok {
			jsonOut(w, 200, map[string]string{"status": "ok", "mode": mode, "api": "ok"})
		} else {
			jsonOut(w, 503, map[string]string{"status": "degraded", "mode": mode, "api": "unreachable"})
		}
	}
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, 200, map[string]string{"version": version, "commit": commit, "mode": mode, "hostname": hostname})
}

// ---------- frontend mode ----------

var apiURL *url.URL

func apiReachable() bool {
	if apiURL == nil {
		return false
	}
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(apiURL.String() + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func spaHandler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		log.Fatalf("frontend files missing from the binary: %v", err)
	}
	files := http.FS(sub)
	fileServer := http.FileServer(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean(r.URL.Path)
		if f, err := files.Open(p); err == nil {
			f.Close()
			if p != "/" {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		// Anything else is a page route: serve the app shell.
		r.URL.Path = "/"
		w.Header().Set("Cache-Control", "no-store")
		fileServer.ServeHTTP(w, r)
	})
}

func main() {
	port := env("PORT", map[string]string{"api": "8080", "frontend": "80"}[mode])
	if mode != "api" && mode != "frontend" {
		log.Fatalf("MODE must be api or frontend, got %q", mode)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/version", handleVersion)
	mux.HandleFunc("/metrics", handleMetrics)
	mux.HandleFunc("/report", handleReport)
	mux.HandleFunc("/hello", handleReport)

	switch mode {
	case "api":
		go connectDB()
		mux.HandleFunc("/api/notes", handleNotes)
		mux.HandleFunc("/api/notes/", handleNotes)
		mux.HandleFunc("/api/version", handleVersion)
		mux.HandleFunc("/api/health", handleHealth)
		mux.HandleFunc("/api/report", handleReport) // reachable through the frontend's /api proxy: the validator reads the backend tier's report this way
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				jsonOut(w, 200, map[string]string{"app": "iti8801-notes", "mode": "api", "see": "/api/notes, /health, /version, /metrics, /report"})
				return
			}
			jsonErr(w, 404, "not found")
		})
	case "frontend":
		raw := os.Getenv("API_URL")
		if raw == "" {
			log.Fatalf("MODE=frontend needs API_URL, e.g. http://10.60.3.10:8080")
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			log.Fatalf("API_URL: %q is not a URL", raw)
		}
		apiURL = u
		proxy := httputil.NewSingleHostReverseProxy(u)
		proxy.Transport = &http.Transport{DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext, ResponseHeaderTimeout: 10 * time.Second}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			var ne net.Error
			if errors.As(err, &ne) || err != nil {
				jsonErr(w, 502, "backend unreachable: "+err.Error())
			}
		}
		mux.Handle("/api/", proxy)
		mux.Handle("/", spaHandler())
	}

	go fetchCloud()
	runProbes()
	go func() {
		for {
			time.Sleep(30 * time.Second)
			fetchCloud()
			runProbes()
		}
	}()
	log.Printf("iti8801-notes %s (%s) mode=%s id=%s listening on :%s", version, commit, mode, instID, port)
	log.Fatal(http.ListenAndServe(":"+port, observe(mux)))
}
