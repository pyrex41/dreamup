package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Report structure matching the reporter package
type Report struct {
	Evidence struct {
		Screenshots []struct {
			Filepath string `json:"filepath"`
		} `json:"screenshots"`
	} `json:"evidence"`
	VideoPath string `json:"videoPath,omitempty"`
}

func main() {
	// Open database
	db, err := sql.Open("sqlite3", "./data/dreamup.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Query all tests with report data
	rows, err := db.Query("SELECT id, report_data FROM tests WHERE report_data IS NOT NULL AND report_data != ''")
	if err != nil {
		log.Fatalf("Failed to query tests: %v", err)
	}
	defer rows.Close()

	updated := 0
	errors := 0

	for rows.Next() {
		var id, reportDataStr string
		if err := rows.Scan(&id, &reportDataStr); err != nil {
			log.Printf("Error scanning row: %v", err)
			errors++
			continue
		}

		// Parse report JSON
		var report Report
		if err := json.Unmarshal([]byte(reportDataStr), &report); err != nil {
			log.Printf("Error parsing report for test %s: %v", id, err)
			errors++
			continue
		}

		modified := false

		// Update screenshot paths
		for i := range report.Evidence.Screenshots {
			if strings.HasPrefix(report.Evidence.Screenshots[i].Filepath, "data/media/") {
				// Remove "data/media/" prefix
				report.Evidence.Screenshots[i].Filepath = strings.TrimPrefix(report.Evidence.Screenshots[i].Filepath, "data/media/")
				modified = true
			} else if strings.HasPrefix(report.Evidence.Screenshots[i].Filepath, "./data/media/") {
				// Remove "./data/media/" prefix
				report.Evidence.Screenshots[i].Filepath = strings.TrimPrefix(report.Evidence.Screenshots[i].Filepath, "./data/media/")
				modified = true
			}
		}

		// Update video path
		if report.VideoPath != "" {
			if strings.HasPrefix(report.VideoPath, "data/media/") {
				report.VideoPath = strings.TrimPrefix(report.VideoPath, "data/media/")
				modified = true
			} else if strings.HasPrefix(report.VideoPath, "./data/media/") {
				report.VideoPath = strings.TrimPrefix(report.VideoPath, "./data/media/")
				modified = true
			}
		}

		// Update database if modified
		if modified {
			// Marshal back to JSON
			updatedData, err := json.Marshal(&report)
			if err != nil {
				log.Printf("Error marshaling updated report for test %s: %v", id, err)
				errors++
				continue
			}

			// Update database
			_, err = db.Exec("UPDATE tests SET report_data = ? WHERE id = ?", string(updatedData), id)
			if err != nil {
				log.Printf("Error updating test %s: %v", id, err)
				errors++
				continue
			}

			updated++
			log.Printf("Updated media paths for test %s", id)
		}
	}

	fmt.Printf("\n✅ Migration complete!\n")
	fmt.Printf("   Updated: %d test reports\n", updated)
	if errors > 0 {
		fmt.Printf("   Errors: %d\n", errors)
	}
}
