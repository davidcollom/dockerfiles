package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// Helper function to calculate week of the year starting on Sunday
func getWeekOfYearSunday(t time.Time) int {
	// Find the first Sunday of the year
	yearStart := time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, t.Location())
	for yearStart.Weekday() != time.Sunday {
		yearStart = yearStart.AddDate(0, 0, 1)
	}

	// Calculate the difference in days
	dayOfYear := t.YearDay()
	dayOfYearStart := yearStart.YearDay()

	// Calculate the week number
	week := (dayOfYear-dayOfYearStart)/7 + 1
	return week
}

// handler for /version that redirects to /year-month-weekoftheyear (starting on Sunday)
func versionHandler(w http.ResponseWriter, r *http.Request) {
	currentTime := time.Now()
	year, month, _ := currentTime.Date()
	week := getWeekOfYearSunday(currentTime)

	redirectURI := fmt.Sprintf("/%d-%02d-%02d", year, int(month), week)
	http.Redirect(w, r, redirectURI, http.StatusFound)
}

func main() {
	http.HandleFunc("/version", versionHandler)

	port := ":8080"
	fmt.Printf("Listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
