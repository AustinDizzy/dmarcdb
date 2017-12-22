package main

import (
	"time"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/spf13/viper"

	"github.com/boltdb/bolt"

	"github.com/go-ole/go-ole"

	"github.com/go-ole/go-ole/oleutil"
)

var (
	errDuplicateRecord = errors.New("record was already processed")
	defaultResoler     = dnsResolver()
)

// build db file
func build(folderpath ...string) error {
	var (
		ns     = oleutil.MustCallMethod(outlook, "GetNamespace", "MAPI").ToIDispatch()
		folder = getFolder(ns, folderpath...)
		msgs   = oleutil.MustGetProperty(folder, "Items").ToIDispatch()
		count  = oleutil.MustGetProperty(msgs, "Count").Value().(int32)
		dupes = 0
		start = time.Now()
		startIndex  = *buildStart
	)

	if startIndex < 0 {
		startIndex = int(count)
	} else {
		startIndex = int(count) - *buildStart
	}

	for i := startIndex; i >= 1; i-- {
		err := processMail(oleutil.MustCallMethod(msgs, "Item", i))
		if err != nil && err != errDuplicateRecord {
			return err
		}

		if err != errDuplicateRecord {
			fmt.Printf("Processed %d / %d reports\n", (int(count) - i), count - 1)
		} else {
			count--
			dupes++
		}

		if viper.GetString("environment") == "dev" {
			break
		}
	}

	if !viper.GetBool("duplicates") {
		fmt.Printf("Processed %d reports, skipped %d duplicates, took %s", count, dupes, time.Since(start))
	} else {
		fmt.Printf("Processed %d reports, took %s", count, time.Since(start))
	}

	return nil
}

func dnsResolver() *net.Resolver {
	if !viper.IsSet("dns") {
		return net.DefaultResolver
	}
	dialer := func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{}
		return d.DialContext(ctx, "udp", viper.GetString("dns"))
	}
	return &net.Resolver{
		PreferGo: true,
		Dial:     dialer,
	}
}

// lookup hostname from IP address and cache it if configured to
func lookupHost(ip string) string {
	var host = func(ip string) string {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		addr, _ := defaultResoler.LookupAddr(ctx, ip)
		return strings.Join(addr, ",")
	}

	if !viper.GetBool("cacheHosts") {
		return host(ip)
	}

	var result string
	bdb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("hosts-cache"))
		v := b.Get([]byte(ip))
		if v == nil {
			result = host(ip)
			b.Put([]byte(ip), []byte(result))
		} else {
			result = string(v[:])
		}
		return nil
	})
	return result
}

func processMail(val *ole.VARIANT) error {
	var (
		message     = val.ToIDispatch()
		id          = oleutil.MustGetProperty(message, "EntryID").ToString()
		attachments = oleutil.MustGetProperty(message, "Attachments").ToIDispatch()
		numAtt      = oleutil.MustGetProperty(attachments, "Count").Value().(int32)
		isDupe      = false
	)

	// check if mail has already been processed
	bdb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("processed-mail")).Get([]byte(id))
		if b != nil && bytes.Equal(b, []byte{1}) {
			isDupe = true
		}
		return nil
	})

	// if it's a dupe and we're configured to skip dupes, then skip it
	if !viper.GetBool("duplicates") && isDupe {
		return errDuplicateRecord
	}

	// for each attachment on the mail
	for i := 1; int32(i) <= numAtt; i++ {
		var (
			attachment = oleutil.MustGetProperty(message, "Attachments", i).ToIDispatch()
			r, err     = openAttachment(attachment)
		)

		if err != nil {
			return err
		}

		// parse the XML file as a DMARC aggregate report
		report, err := parseDMARC(r)
		if err != nil {
			// if we've configured to stop processing on any broken DMARC report, return an error
			if viper.GetBool("stopOnError") {
				return err
			}
			// else, log the broken DMARC report so we can later harass the offending aggregate report sender for why their reports are broken
			err = bdb.Update(func(tx *bolt.Tx) error {
				return tx.Bucket([]byte("processed-fail")).Put([]byte(id), []byte(err.Error()))
			})
			// if there's an error logging the report so we can later harass the offending aggregate report sender,
			// return an error and harass ourselves
			if err != nil {
				return err
			}
			return nil
		}

		// store each report in the database
		if err = report.store(); err != nil {
			return err
		}
	}

	// if we've gotten this far, the mail is done processing and we should flag it as such
	return bdb.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("processed-mail")).Put([]byte(id), []byte{1})
	})
}

// trims everything from a str past the found cutset
func trimFrom(str, cutset string) string {
	if idx := strings.Index(str, cutset); idx != -1 {
		return str[:idx]
	}
	return str
}
