package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

// VPC represents a virtual private cloud.
type VPC struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CIDR      string `json:"cidr"`
	CreatedAt string `json:"created_at"`
}

// Subnet belongs to a VPC.
type Subnet struct {
	ID        string `json:"id"`
	VPCID     string `json:"vpc_id"`
	Name      string `json:"name"`
	CIDR      string `json:"cidr"`
	CreatedAt string `json:"created_at"`
}

// RouteTable routes traffic for a subnet.
type RouteTable struct {
	ID        string `json:"id"`
	SubnetID  string `json:"subnet_id"`
	Dest      string `json:"destination"`
	Target    string `json:"target"`
	CreatedAt string `json:"created_at"`
}

// SecurityGroup holds allow/deny rules.
type SecurityGroup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	VPCID     string `json:"vpc_id"`
	CreatedAt string `json:"created_at"`
}

// SGRule is one rule in a security group.
type SGRule struct {
	ID              string `json:"id"`
	SecurityGroupID string `json:"sg_id"`
	Direction       string `json:"direction"` // inbound | outbound
	Action          string `json:"action"`    // allow | deny
	Protocol        string `json:"protocol"`  // tcp | udp | icmp | *
	Port            int    `json:"port"`      // 0 = all
	CIDR            string `json:"cidr"`
}

var (
	db     *sql.DB
	seq    uint64
)

func main() {
	dbPath := getenv("NETWORKING_DB", "networking.db")
	listenAddr := getenv("NETWORKING_ADDR", ":9005")

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS vpcs (
			id TEXT PRIMARY KEY, name TEXT, cidr TEXT, created_at TEXT
		);
		CREATE TABLE IF NOT EXISTS subnets (
			id TEXT PRIMARY KEY, vpc_id TEXT, name TEXT, cidr TEXT, created_at TEXT
		);
		CREATE TABLE IF NOT EXISTS route_tables (
			id TEXT PRIMARY KEY, subnet_id TEXT, destination TEXT, target TEXT, created_at TEXT
		);
		CREATE TABLE IF NOT EXISTS security_groups (
			id TEXT PRIMARY KEY, name TEXT, vpc_id TEXT, created_at TEXT
		);
		CREATE TABLE IF NOT EXISTS sg_rules (
			id TEXT PRIMARY KEY, sg_id TEXT, direction TEXT, action TEXT,
			protocol TEXT, port INTEGER, cidr TEXT
		);
		CREATE TABLE IF NOT EXISTS instance_subnets (
			instance_id TEXT PRIMARY KEY, subnet_id TEXT
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("GET /health", handleHealth)

	// VPC
	http.HandleFunc("POST /vpcs", handleVPCCreate)
	http.HandleFunc("GET /vpcs", handleVPCList)
	http.HandleFunc("GET /vpcs/{id}", handleVPCGet)

	// Subnet
	http.HandleFunc("POST /subnets", handleSubnetCreate)
	http.HandleFunc("GET /subnets", handleSubnetList)

	// Route table
	http.HandleFunc("POST /route-tables", handleRouteCreate)
	http.HandleFunc("GET /route-tables", handleRouteList)

	// Security groups
	http.HandleFunc("POST /security-groups", handleSGCreate)
	http.HandleFunc("GET /security-groups", handleSGList)
	http.HandleFunc("POST /security-groups/{id}/rules", handleSGRuleAdd)
	http.HandleFunc("GET /security-groups/{id}/rules", handleSGRuleList)

	// F8: assign instance to subnet
	http.HandleFunc("PUT /instances/{id}/subnet", handleInstanceSubnet)
	http.HandleFunc("GET /instances/{id}/subnet", handleInstanceSubnetGet)

	log.Printf("networking service listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"healthy","service":"networking"}`)
}

func nextID(prefix string) string {
	n := atomic.AddUint64(&seq, 1)
	return fmt.Sprintf("%s-%d", prefix, n)
}

func handleVPCCreate(w http.ResponseWriter, r *http.Request) {
	var v VPC
	json.NewDecoder(r.Body).Decode(&v)
	if v.Name == "" || v.CIDR == "" {
		http.Error(w, "name and cidr required", http.StatusBadRequest)
		return
	}
	v.ID = nextID("vpc")
	v.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO vpcs VALUES (?,?,?,?)`, v.ID, v.Name, v.CIDR, v.CreatedAt)
	log.Printf("POST /vpcs - %s (%s)", v.Name, v.CIDR)
	writeJSON(w, http.StatusCreated, v)
}

func handleVPCList(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query(`SELECT id, name, cidr, created_at FROM vpcs`)
	defer rows.Close()
	out := []VPC{}
	for rows.Next() {
		var v VPC
		rows.Scan(&v.ID, &v.Name, &v.CIDR, &v.CreatedAt)
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

func handleVPCGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var v VPC
	err := db.QueryRow(`SELECT id, name, cidr, created_at FROM vpcs WHERE id=?`, id).
		Scan(&v.ID, &v.Name, &v.CIDR, &v.CreatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func handleSubnetCreate(w http.ResponseWriter, r *http.Request) {
	var s Subnet
	json.NewDecoder(r.Body).Decode(&s)
	if s.VPCID == "" || s.CIDR == "" {
		http.Error(w, "vpc_id and cidr required", http.StatusBadRequest)
		return
	}
	s.ID = nextID("subnet")
	s.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO subnets VALUES (?,?,?,?,?)`, s.ID, s.VPCID, s.Name, s.CIDR, s.CreatedAt)
	log.Printf("POST /subnets - %s in %s (%s)", s.Name, s.VPCID, s.CIDR)
	writeJSON(w, http.StatusCreated, s)
}

func handleSubnetList(w http.ResponseWriter, r *http.Request) {
	vpcID := r.URL.Query().Get("vpc_id")
	var rows *sql.Rows
	if vpcID != "" {
		rows, _ = db.Query(`SELECT id, vpc_id, name, cidr, created_at FROM subnets WHERE vpc_id=?`, vpcID)
	} else {
		rows, _ = db.Query(`SELECT id, vpc_id, name, cidr, created_at FROM subnets`)
	}
	defer rows.Close()
	out := []Subnet{}
	for rows.Next() {
		var s Subnet
		rows.Scan(&s.ID, &s.VPCID, &s.Name, &s.CIDR, &s.CreatedAt)
		out = append(out, s)
	}
	writeJSON(w, http.StatusOK, out)
}

func handleRouteCreate(w http.ResponseWriter, r *http.Request) {
	var rt RouteTable
	json.NewDecoder(r.Body).Decode(&rt)
	if rt.SubnetID == "" || rt.Dest == "" || rt.Target == "" {
		http.Error(w, "subnet_id, destination, target required", http.StatusBadRequest)
		return
	}
	rt.ID = nextID("rt")
	rt.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO route_tables VALUES (?,?,?,?,?)`, rt.ID, rt.SubnetID, rt.Dest, rt.Target, rt.CreatedAt)
	writeJSON(w, http.StatusCreated, rt)
}

func handleRouteList(w http.ResponseWriter, r *http.Request) {
	subnetID := r.URL.Query().Get("subnet_id")
	var rows *sql.Rows
	if subnetID != "" {
		rows, _ = db.Query(`SELECT id, subnet_id, destination, target, created_at FROM route_tables WHERE subnet_id=?`, subnetID)
	} else {
		rows, _ = db.Query(`SELECT id, subnet_id, destination, target, created_at FROM route_tables`)
	}
	defer rows.Close()
	out := []RouteTable{}
	for rows.Next() {
		var rt RouteTable
		rows.Scan(&rt.ID, &rt.SubnetID, &rt.Dest, &rt.Target, &rt.CreatedAt)
		out = append(out, rt)
	}
	writeJSON(w, http.StatusOK, out)
}

func handleSGCreate(w http.ResponseWriter, r *http.Request) {
	var sg SecurityGroup
	json.NewDecoder(r.Body).Decode(&sg)
	if sg.Name == "" || sg.VPCID == "" {
		http.Error(w, "name and vpc_id required", http.StatusBadRequest)
		return
	}
	sg.ID = nextID("sg")
	sg.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO security_groups VALUES (?,?,?,?)`, sg.ID, sg.Name, sg.VPCID, sg.CreatedAt)
	writeJSON(w, http.StatusCreated, sg)
}

func handleSGList(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query(`SELECT id, name, vpc_id, created_at FROM security_groups`)
	defer rows.Close()
	out := []SecurityGroup{}
	for rows.Next() {
		var sg SecurityGroup
		rows.Scan(&sg.ID, &sg.Name, &sg.VPCID, &sg.CreatedAt)
		out = append(out, sg)
	}
	writeJSON(w, http.StatusOK, out)
}

func handleSGRuleAdd(w http.ResponseWriter, r *http.Request) {
	sgID := r.PathValue("id")
	var rule SGRule
	json.NewDecoder(r.Body).Decode(&rule)
	if rule.Direction == "" || rule.Action == "" {
		http.Error(w, "direction and action required", http.StatusBadRequest)
		return
	}
	rule.ID = nextID("rule")
	rule.SecurityGroupID = sgID
	if rule.Protocol == "" {
		rule.Protocol = "*"
	}
	if rule.CIDR == "" {
		rule.CIDR = "0.0.0.0/0"
	}
	db.Exec(`INSERT INTO sg_rules VALUES (?,?,?,?,?,?,?)`,
		rule.ID, rule.SecurityGroupID, rule.Direction, rule.Action, rule.Protocol, rule.Port, rule.CIDR)
	writeJSON(w, http.StatusCreated, rule)
}

func handleSGRuleList(w http.ResponseWriter, r *http.Request) {
	sgID := r.PathValue("id")
	rows, _ := db.Query(`SELECT id, sg_id, direction, action, protocol, port, cidr FROM sg_rules WHERE sg_id=?`, sgID)
	defer rows.Close()
	out := []SGRule{}
	for rows.Next() {
		var rule SGRule
		rows.Scan(&rule.ID, &rule.SecurityGroupID, &rule.Direction, &rule.Action, &rule.Protocol, &rule.Port, &rule.CIDR)
		out = append(out, rule)
	}
	writeJSON(w, http.StatusOK, out)
}

// F8: PUT /instances/{id}/subnet — assign instance to subnet.
func handleInstanceSubnet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		SubnetID string `json:"subnet_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.SubnetID == "" {
		http.Error(w, "subnet_id required", http.StatusBadRequest)
		return
	}
	db.Exec(`INSERT INTO instance_subnets VALUES (?,?) ON CONFLICT(instance_id) DO UPDATE SET subnet_id=excluded.subnet_id`,
		id, req.SubnetID)
	log.Printf("PUT /instances/%s/subnet - %s", id, req.SubnetID)
	w.WriteHeader(http.StatusNoContent)
}

func handleInstanceSubnetGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var subnetID string
	err := db.QueryRow(`SELECT subnet_id FROM instance_subnets WHERE instance_id=?`, id).Scan(&subnetID)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"instance_id": id, "subnet_id": subnetID})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
