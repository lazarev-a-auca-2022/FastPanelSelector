// Package domain holds the shared types for the plan-catalog pipeline.
package domain

// Plan mirrors the record shape the frontend already expects (see
// plans-data.js at the repo root) so the HTTP API can serialize it as-is.
type Plan struct {
	Location string   `json:"location"`
	City     string   `json:"city"`
	ID       string   `json:"id"`
	Package  string   `json:"package"`
	Arch     string   `json:"arch"`
	CPUType  string   `json:"cpuType"`
	Cores    int      `json:"cores"`
	RAM      int      `json:"ram"`
	Disk     int      `json:"disk"`
	Enabled  bool     `json:"enabled"`
	Price    *float64 `json:"price"`
}
