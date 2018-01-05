# DMARC.db
A (still WIP) tool to populate and browse a SQL database with records from incoming DMARC aggregate reports. The idea is to make detecting misconfigured mail hosts, malicious mail servers, and monitoring outgoing mail traffic easier through constantly monitoring reported mail traffic from DMARC aggregate reports [(as described in RFC 7489 section 7.2)](https://www.rfc-editor.org/rfc/rfc7489.txt) in the form of easy-to-run reports and an (optional) interactive web interface.

This tool was built based on requirements needed or requested by West Virginia University's Information Security Services. This project is open-source licensed by the GNU General Public License (v3) found in the [LICENSE](./LICENSE) file in this directory.

## Usage
**Configuration**: A single `config.yml` (see: [`config-example.yml`](./config-example.yml)) or similar static configuration file named `config` (i.e. `config.json`, `config.xml`, `config.toml`, etc.) is required for basic runtime configuration in the same directory as the dmarcdb binary (i.e. "beside" the binary).

**Prerequisite**: MaxMind's [City](http://geolite.maxmind.com/download/geoip/database/GeoLite2-City.tar.gz) and [ASN](http://geolite.maxmind.com/download/geoip/database/GeoLite2-ASN.tar.gz) GeoLite2 databases in an accessible folder on the machine and for the user running dmarcdb, with locations configured in the config file.

For stable configuration, logging, and accessibility purposes, it'd be best to just have a singular folder for dmarcdb and it's accompanying files alone (i.e. `C:\Program Files\DMARCDB\`).

**Commands**:

For each command listed below (and also without any commands) on startup, if `web` is set in the config to `true`, it will also begin to serve a web browseable interface (data read-only) over the configured `port`.


* `./dmarcdb build` - Begins the process of building the database with records populated from the mail folder configured as `mailFolder`

* `./dmarcdb logs` - Prints any error logs received while attempting to processed malformed DMARC aggregate reports or malformed emails

* `./dmarcdb flush <fails|hosts>` - Without parameters, deletes both logged errors and cached hostname lookups. With either extra parameter `flush` or `hosts`, will only flush respective option.

## Third-Party Technologies
The following (nonexhaustive) list of third-party technolgies were used in this project:
* [PostgreSQL](https://www.postgresql.org/) / [MSSQL](http://www.microsoft.com/sqlserver/)
* [Go](https://golang.org) (>= 1.8)
    * [Bolt](https://github.com/boltdb/bolt) - "A fast key/value store inspired by [Howard Chu's LMDB project](https://symas.com/products/lightning-memory-mapped-database/)."
    * [Viper](https://github.com/spf13/viper) - A library to make accepting client configurations in Go easier.
    * Uses the [Win32 API](https://msdn.microsoft.com/en-us/library/aa271855(v=vs.60).aspx) (via [go-ole](https://github.com/go-ole/go-ole)) to browse mail items from a [Microsoft Outlook](https://products.office.com/en-us/outlook/email-and-calendar-software-microsoft-outlook) folder. In future versions it should probably just use IMAP to read mail for cross-platform purposes. However, only Windows compatibility was initially required by the requesting party and reading cached emails from an already functioning desktop mail client seemed less resource intensive.
    * [and a handful of others](https://godoc.org/github.com/AustinDizzy/dmarcdb?imports)
* [MaxMind's GeoIP](http://dev.maxmind.com/geoip/)
