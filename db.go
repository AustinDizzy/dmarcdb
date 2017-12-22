package main

import (
	"database/sql"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/denisenkom/go-mssqldb"

	"github.com/lib/pq"
	"github.com/spf13/viper"
	pb "gopkg.in/cheggaaa/pb.v1"
)

var (
	cols = []string{"org_name", "email", "contact_info", "date_range_begin", "date_range_end", "domain", "adkim", "aspf", "p", "pct", "location", "source_ip", "count", "disposition", "dkim", "spf", "reason_type", "comment", "envelope_to", "header_from", "dkim_domain", "dkim_result", "dkim_hresult", "spf_domain", "spf_result", "hostname"}
)

// MaxWorkers defines the maximum number of running workers (via goroutines)
const MaxWorkers = 1000

func (report *DMARCFeedback) store() error {
	// begin a transaction (i.e. all data inserted to db at once, all goes or nothing)
	txn, err := db.Begin()
	if err != nil {
		return err
	}

	// prepare the insert into the "records" table
	var query string
	switch strings.Split(viper.GetString("database"), "://")[0] {
	case "postgres":
		query = pq.CopyIn("records", cols...)
	case "mssql":
		opts := mssql.MssqlBulkOptions{}
		query = mssql.CopyIn("records", opts, cols...)
	}

	stmt, err := txn.Prepare(query)
	if err != nil {
		return err
	}

	var (
		// scale numWorkers linearly with respect to number of records to lookup
		numWorkers = (len(report.Records) + 30) / 15
		wg         sync.WaitGroup
	)

	// cap max workers at MaxWorkers
	if numWorkers > MaxWorkers {
		numWorkers = MaxWorkers
	}

	workers := make(chan bool, numWorkers)
	bar := pb.New(len(report.Records)).Prefix(fmt.Sprintf("Records (%d) ", numWorkers))
	bar.ShowTimeLeft = false
	bar.ShowSpeed = true
	bar.Start()

	for _, record := range report.Records {
		wg.Add(1)
		workers <- true
		go func(record DMARCRecord) {
			defer wg.Done()
			defer func() { <-workers }()
			insert(stmt, report, record)
			bar.Increment()
		}(record)
	}
	wg.Wait()
	bar.Finish()

	// exec once more to flush buffered data
	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	// close the connection
	err = stmt.Close()
	if err != nil {
		return err
	}

	// commit the transaction
	return txn.Commit()
}

// inserts a record into the datbase using the prepared stmt
func insert(stmt *sql.Stmt, report *DMARCFeedback, record DMARCRecord) error {
	var (
		host       = lookupHost(record.SourceIP)
		ip         = net.ParseIP(record.SourceIP)
		country, _ = geoCityDB.Country(ip)
		asn, _     = geoASNdb.ASN(ip)
		city, _    = geoCityDB.City(ip)
		loc        = "%s/%s"
		contact    string
		err        error
	)

	if len(city.Subdivisions) > 0 {
		loc = fmt.Sprintf(loc, city.Subdivisions[0].IsoCode, country.Country.IsoCode)
	} else {
		loc = country.Country.IsoCode
	}

	if asn != nil {
		contact = asn.AutonomousSystemOrganization
	}
	if report.Metadata.ExtraContactInfo != "NULL" {
		contact += report.Metadata.ExtraContactInfo
	}

	_, err = stmt.Exec(report.Metadata.OrgName, report.Metadata.Email, contact, report.Metadata.DateRangeBegin, report.Metadata.DateRangeEnd, report.Policy.Domain, report.Policy.ADKIM, report.Policy.ASPF, report.Policy.P, report.Policy.PCT, loc, record.SourceIP, record.Count, record.Disposition, record.DKIM, record.SPF, record.ReasonType, record.ReasonComment, record.EnvelopeTo, record.HeaderFrom, record.DKIMDomain, record.DKIMResult, record.DKIMHResult, record.SPFDomain, record.SPFResult, host)
	return err
}

func retrieve(query string) (map[string]interface{}, error) {

	var (
		rows, _ = db.Query(query)
		cols, _ = rows.Columns()
		data    = map[string]interface{}{
			"success": false,
			"count":   0,
		}
		err error
	)

	results := []map[string]interface{}{}

	for ; rows.Next(); data["count"] = data["count"].(int) + 1 {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err = rows.Scan(columnPointers...); err != nil {
			break
		}

		m := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			m[colName] = *val
		}

		results = append(results, m)
	}

	data["records"] = results
	data["success"] = (err == nil)

	return data, err
}
