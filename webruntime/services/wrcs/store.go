package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Versioned, file-backed manifest storage.
//
// Layout on disk (DATA_DIR, default /data):
//   {tenant}/{env}/versions/{n}.json   immutable manifest snapshot
//   {tenant}/{env}/pointers.json       {"stable": n, "candidate": n}  (0 = unset)

type Pointers struct {
	Stable    int `json:"stable"`
	Candidate int `json:"candidate"`
}

func dataDir() string {
	if d := os.Getenv("DATA_DIR"); d != "" {
		return d
	}
	return "/data"
}

func tenantEnvDir(tenant, env string) string {
	return filepath.Join(dataDir(), tenant, env)
}

func versionsDir(tenant, env string) string {
	return filepath.Join(tenantEnvDir(tenant, env), "versions")
}

func pointersFile(tenant, env string) string {
	return filepath.Join(tenantEnvDir(tenant, env), "pointers.json")
}

func listVersions(tenant, env string) ([]int, error) {
	dir := versionsDir(tenant, env)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var versions []int
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		n, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		versions = append(versions, n)
	}
	sort.Ints(versions)
	return versions, nil
}

func nextVersion(tenant, env string) (int, error) {
	versions, err := listVersions(tenant, env)
	if err != nil {
		return 0, err
	}
	if len(versions) == 0 {
		return 1, nil
	}
	return versions[len(versions)-1] + 1, nil
}

func writeVersion(tenant, env string, version int, data []byte) error {
	dir := versionsDir(tenant, env)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var pretty map[string]interface{}
	if err := json.Unmarshal(data, &pretty); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.json", version)), out, 0o644)
}

func readVersion(tenant, env string, version int) ([]byte, error) {
	path := filepath.Join(versionsDir(tenant, env), fmt.Sprintf("%d.json", version))
	return os.ReadFile(path)
}

func readPointers(tenant, env string) (Pointers, error) {
	var p Pointers
	data, err := os.ReadFile(pointersFile(tenant, env))
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	return p, nil
}

func writePointers(tenant, env string, p Pointers) error {
	dir := tenantEnvDir(tenant, env)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pointersFile(tenant, env), data, 0o644)
}

func listTenantsEnvs() ([][2]string, error) {
	root := dataDir()
	tenants, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out [][2]string
	for _, t := range tenants {
		if !t.IsDir() {
			continue
		}
		envs, err := os.ReadDir(filepath.Join(root, t.Name()))
		if err != nil {
			continue
		}
		for _, e := range envs {
			if e.IsDir() {
				out = append(out, [2]string{t.Name(), e.Name()})
			}
		}
	}
	return out, nil
}
