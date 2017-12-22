package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"

	"github.com/spf13/viper"
)

// starts web server on configured port
func startWeb(port string) error {
	http.HandleFunc("/", index)
	http.HandleFunc("/api/stats", stats)
	return http.ListenAndServe(port, nil)
}

// route for stats api endpoint "/api/stats"
func stats(w http.ResponseWriter, r *http.Request) {
	data, err := retrieve("select count(*), sum(records.count), pg_size_pretty(pg_database_size('dmarcdb')) as dbsize from records;")
	if err != nil {
		log.Fatal(err)
	}

	j, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Add("Content-Type", "application/json")
	fmt.Fprint(w, string(j[:]))
}

// route for index "/"
func index(w http.ResponseWriter, r *http.Request) {
	tmpl := path.Join(viper.GetString("templates"), "index.html")
	err := template.Must(template.ParseFiles(tmpl)).Execute(w, nil)

	if err != nil {
		log.Fatal(err)
	}
}
