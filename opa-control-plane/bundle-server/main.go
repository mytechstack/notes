package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
	//dbURL := os.Getenv("DATABASE_URL")
	// if dbURL == "" {
	// 	dbURL = "postgres://postgres:password@postgres:5432/postgres?sslmode=disable"
	// }

	host := "localhost"
    port := 5432
    user := "postgres"      // Change to your username
    password := "password"  // Change to your password  
    dbname := "opa_policies"    // Change to your database name

    // Connection string with SSL disabled (for local testing)
    connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
        host, port, user, password, dbname)

	var db *sql.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = sql.Open("postgres", connStr)
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
		SELECT id, name, path, content, version 
		FROM policies 
		WHERE active = true
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
		err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Content, &p.Version)
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

func (bs *BundleServer) bundleHandler(w http.ResponseWriter, r *http.Request) {
	policies, err := bs.getPoliciesFromDB()
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Bundle server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}