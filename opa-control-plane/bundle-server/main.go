package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type Policy struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
	Version int    `json:"version"`
	Active  bool   `json:"active"`
}

type BundleManifest struct {
	Revision string            `json:"revision"`
	Roots    []string          `json:"roots,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type BundleServer struct {
	db *sql.DB
}

func NewBundleServer() (*BundleServer, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@postgres:5432/opa_policies?sslmode=disable"
	}

	var db *sql.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = sql.Open("postgres", dbURL)
		if err == nil {
			err = db.Ping()
			if err == nil {
				break
			}
		}
		log.Printf("Database connection attempt %d failed: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	log.Println("Connected to database successfully")
	return &BundleServer{db: db}, nil
}

func (bs *BundleServer) getPoliciesFromDB() ([]Policy, error) {
	query := `
		SELECT id, name, path, content, version, active
		FROM policies
		ORDER BY path
	`

	rows, err := bs.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query policies: %v", err)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		var p Policy
		err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Content, &p.Version, &p.Active)
		if err != nil {
			return nil, fmt.Errorf("failed to scan policy: %v", err)
		}
		policies = append(policies, p)
	}

	return policies, nil
}

func (bs *BundleServer) createBundle(policies []Policy) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Create manifest
	revision := fmt.Sprintf("%d", time.Now().Unix())
	if len(policies) > 0 {
		maxVersion := 0
		for _, p := range policies {
			if p.Version > maxVersion {
				maxVersion = p.Version
			}
		}
		revision = fmt.Sprintf("%d", maxVersion)
	}

	manifest := BundleManifest{
		Revision: revision,
		Metadata: map[string]string{
			"generated_at": time.Now().Format(time.RFC3339),
			"policy_count": fmt.Sprintf("%d", len(policies)),
		},
	}

	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")

	// Add manifest
	manifestHeader := &tar.Header{
		Name: ".manifest",
		Mode: 0644,
		Size: int64(len(manifestJSON)),
	}
	tw.WriteHeader(manifestHeader)
	tw.Write(manifestJSON)

	// Add policies
	for _, policy := range policies {
		header := &tar.Header{
			Name: policy.Path,
			Mode: 0644,
			Size: int64(len(policy.Content)),
		}
		tw.WriteHeader(header)
		tw.Write([]byte(policy.Content))
	}

	tw.Close()
	gw.Close()
	return buf.Bytes(), nil
}

func (bs *BundleServer) getActivePolicies() ([]Policy, error) {
	all, err := bs.getPoliciesFromDB()
	if err != nil {
		return nil, err
	}
	var active []Policy
	for _, p := range all {
		if p.Active {
			active = append(active, p)
		}
	}
	return active, nil
}

func (bs *BundleServer) bundleHandler(w http.ResponseWriter, r *http.Request) {
	policies, err := bs.getActivePolicies()
	if err != nil {
		log.Printf("Error getting policies: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	bundle, err := bs.createBundle(policies)
	if err != nil {
		log.Printf("Error creating bundle: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Write(bundle)
}

func (bs *BundleServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	policies, err := bs.getPoliciesFromDB()
	if err != nil {
		http.Error(w, "Failed to get policies", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"policy_count": len(policies),
		"timestamp":    time.Now().Format(time.RFC3339),
		"policies":     make([]map[string]interface{}, 0),
	}

	for _, p := range policies {
		response["policies"] = append(response["policies"].([]map[string]interface{}), map[string]interface{}{
			"name":    p.Name,
			"path":    p.Path,
			"version": p.Version,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (bs *BundleServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (bs *BundleServer) listPoliciesHandler(w http.ResponseWriter, r *http.Request) {
	policies, err := bs.getPoliciesFromDB()
	if err != nil {
		log.Printf("Error listing policies: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(policies)
}

func (bs *BundleServer) createPolicyHandler(w http.ResponseWriter, r *http.Request) {
	var p Policy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	err := bs.db.QueryRow(
		`INSERT INTO policies (name, path, content, active) VALUES ($1, $2, $3, $4) RETURNING id, version`,
		p.Name, p.Path, p.Content, p.Active,
	).Scan(&p.ID, &p.Version)
	if err != nil {
		log.Printf("Error creating policy: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func (bs *BundleServer) updatePolicyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	var p Policy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	_, err := bs.db.Exec(
		`UPDATE policies SET name=$1, path=$2, content=$3, active=$4, version=version+1, updated_at=NOW() WHERE id=$5`,
		p.Name, p.Path, p.Content, p.Active, id,
	)
	if err != nil {
		log.Printf("Error updating policy: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (bs *BundleServer) deletePolicyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	_, err := bs.db.Exec(`DELETE FROM policies WHERE id=$1`, id)
	if err != nil {
		log.Printf("Error deleting policy: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusNoContent)
}

func (bs *BundleServer) validatePolicyHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
		Path    string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if strings.TrimSpace(req.Content) == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": true})
		return
	}

	opaURL := os.Getenv("OPA_URL")
	if opaURL == "" {
		opaURL = "http://opa:8181"
	}

	tmpID := fmt.Sprintf("tmp_val_%d", time.Now().UnixNano())
	putURL := fmt.Sprintf("%s/v1/policies/%s", opaURL, tmpID)

	putReq, err := http.NewRequest(http.MethodPut, putURL, strings.NewReader(req.Content))
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	putReq.Header.Set("Content-Type", "text/plain")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(putReq)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":  false,
			"errors": []map[string]interface{}{{"message": "OPA server unreachable"}},
		})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		// Valid — clean up the temporary policy
		delReq, _ := http.NewRequest(http.MethodDelete, putURL, nil)
		client.Do(delReq) //nolint best-effort
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": true})
		return
	}

	// Parse OPA's error response and forward it
	var opaResp map[string]interface{}
	if err := json.Unmarshal(body, &opaResp); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":  false,
			"errors": []map[string]interface{}{{"message": string(body)}},
		})
		return
	}
	opaResp["valid"] = false
	json.NewEncoder(w).Encode(opaResp)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	bs, err := NewBundleServer()
	if err != nil {
		log.Fatalf("Failed to create bundle server: %v", err)
	}
	defer bs.db.Close()

	r := mux.NewRouter()
	r.HandleFunc("/bundles/policies", bs.bundleHandler)
	r.HandleFunc("/status", bs.statusHandler)
	r.HandleFunc("/health", bs.healthHandler)
	r.HandleFunc("/policies", bs.listPoliciesHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/policies", bs.createPolicyHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/policies/{id}", bs.updatePolicyHandler).Methods("PUT", "OPTIONS")
	r.HandleFunc("/policies/{id}", bs.deletePolicyHandler).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/validate", bs.validatePolicyHandler).Methods("POST", "OPTIONS")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Bundle server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, corsMiddleware(r)))
}