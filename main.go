// +build windows

package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/oschwald/geoip2-golang"

	"github.com/kardianos/osext"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"

	"github.com/boltdb/bolt"
)

var (
	bdb *bolt.DB
	db  *sql.DB

	geoCityDB *geoip2.Reader
	geoASNdb  *geoip2.Reader

	buildStart = flag.Int("startAt", -1, "start index for processing DMARC report emails")
)

func readConfig() error {
	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/dmarcdb/")
	viper.AddConfigPath("$HOME/.dmarcdb")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("DMARCDB")
	return viper.ReadInConfig()
}

func dbConnect() error {
	var err error
	bdb, err = bolt.Open("dmarc.db", 0666, nil)
	if err != nil {
		return err
	}

	geoCityDB, err = geoip2.Open(viper.GetString("geocitydb"))
	if err != nil {
		return err
	}

	geoASNdb, err = geoip2.Open(viper.GetString("geoasndb"))
	if err != nil {
		return err
	}

	err = bdb.Update(func(tx *bolt.Tx) (err error) {
		_, err = tx.CreateBucketIfNotExists([]byte("processed-mail"))
		if err != nil {
			return
		}
		_, err = tx.CreateBucketIfNotExists([]byte("processed-fail"))
		if err != nil {
			return
		}
		_, err = tx.CreateBucketIfNotExists([]byte("hosts-cache"))
		return
	})
	if err != nil {
		return err
	}

	var proto = strings.Split(viper.GetString("database"), "://")[0]
	switch proto {
	case "postgres":
		db, err = sql.Open(proto, viper.GetString("database"))
	default:
		db, err = sql.Open("sqlserver", viper.GetString("database"))
	}

	return err
}

func init() {
	if err := readConfig(); err != nil {
		log.Fatal(err)
	}

	if err := dbConnect(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	flag.Parse()
	progPath, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal(err)
	}

	viper.SetDefault("geocitydb", "GeoLite2-City.mmdb")
	viper.SetDefault("geoasndb", "GeoLite2-ASN.mmdb")
	viper.SetDefault("environment", "prod")
	viper.SetDefault("duplicates", false)
	viper.SetDefault("stopOnError", false)
	viper.SetDefault("cacheHosts", true)
	viper.SetDefault("web", false)
	viper.SetDefault("port", ":8080")
	viper.SetDefault("templates", path.Join(progPath, "templates"))

	if viper.GetBool("web") {
		go func() {
			if err := startWeb(viper.GetString("port")); err != nil {
				log.Fatal(err)
			}
		}()
	}

	if flag.NArg() >= 1 {
		switch flag.Arg(0) {
		// i.e. `dmarcdb build`
		case "build":
			// i.e. `dmarcdb build /path/to/folder` for spidering specific Outlook folder
			if flag.NArg() >= 2 {
				viper.Set("mailFolder", flag.Arg(1))
			}
			err = build(strings.Split(viper.GetString("mailFolder"), "/")...)
		case "config":
			log.Println("Loaded configuration: ")
			for k, v := range viper.AllSettings() {
				fmt.Println(k, ": ", v)
			}
		case "logs":
			err = bdb.View(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("processed-fail"))
				fmt.Println("Fetching logs")
				if b != nil {
					m := map[string]int{}
					err = b.ForEach(func(k, v []byte) error {
						if val, ok := m[string(v[:])]; !ok {
							m[string(v[:])] = 1
						} else {
							m[string(v[:])] = val + 1
						}
						return nil
					})
					if err != nil {
						return err
					}
					for txt, num := range m {
						fmt.Printf("(%d) %s \n", num, txt)
					}
				}
				return nil
			})
		case "flush":
			delBucket := func(key string) error {
				return bdb.Update(func(tx *bolt.Tx) error {
					return tx.DeleteBucket([]byte(key))
				})
			}
			if flag.NArg() > 2 {
				for _, opt := range flag.Args()[1:] {
					fmt.Printf("Flushing all %s\n", strings.Join(flag.Args()[1:], " & "))
					switch opt {
					case "fails":
						err = delBucket("processed-fail")
					case "hosts":
						err = delBucket("hosts-cache")
					}
				}
			} else {
				fmt.Printf("Flushing all fails and hosts")
				for _, b := range []string{"processed-fail", "hosts-cache"} {
					if err = delBucket(b); err != nil {
						break
					}
				}
			}
		default:
			log.Printf("The option \"%v\" is not yet available", flag.Arg(0))
		}
	} else {
		fmt.Println("No command line arguments given")
	}

	if err != nil {
		log.Fatal(err)
	}
}

func devLogger(msg string) {
	if viper.GetString("environment") == "dev" {
		fmt.Println(msg)
	}
}
