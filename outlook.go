package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/mholt/archiver"
)

var (
	app     *ole.IUnknown
	outlook *ole.IDispatch
)

func init() {
	ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	app, err := oleutil.CreateObject("Outlook.Application")
	outlook, err = app.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		log.Fatal(err)
	}
}

func getFolder(namespace *ole.IDispatch, path ...string) *ole.IDispatch {
	var folder *ole.IDispatch
	if len(path) >= 1 {
		folder = oleutil.MustCallMethod(namespace, "Folders", path[0]).ToIDispatch()
	}
	for i := 1; i < len(path); i++ {
		folder = oleutil.MustCallMethod(folder, "Folders", path[i]).ToIDispatch()
	}
	return folder
}

func openAttachment(attachment *ole.IDispatch) (io.Reader, error) {
	var (
		// create a temporary directory to save attachment(s) to
		dir, err = ioutil.TempDir("", "dmarc")
		filename = oleutil.MustGetProperty(attachment, "FileName").Value().(string)
		saveTo   = filepath.Join(dir, filename)
		arx      = archiver.MatchingFormat(filename)
		r        io.Reader
	)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Opening %s\n", filename)
	// save mail attachment to temporary directory
	oleutil.MustCallMethod(attachment, "SaveAsFile", saveTo)

	// get name of the XML file to open
	var (
		isXML   = func(n string) bool { return strings.HasSuffix(n, ".xml") }
		xmlFile = filepath.Join(dir, trimFrom(filename, ".xml"))
	)
	if strings.Contains(filename, ".xml") {
		xmlFile += ".xml"
	} else if strings.HasSuffix(filename, ".zip") {
		xmlFile = strings.Replace(xmlFile, ".zip", ".xml", 1)
	}

	// unarchive the attachment in the respective way (if required)
	if strings.HasSuffix(filename, ".xml.gz") {
		// file is simple XML gzip'd
		att, err := os.Open(saveTo)
		if err != nil {
			return nil, err
		}

		return gzip.NewReader(att)
	} else if arx != nil {
		// file is a supported type by archiver tool
		err = arx.Open(saveTo, dir)
		if err != nil {
			return nil, err
		}

		r, err = os.Open(xmlFile)
		// if the file in the archive is named differently than expected
		if os.IsNotExist(err) {
			xmlFile = ""
			devLogger("XML file not defaultly named, searching " + dir)
			// walk through the directory which we unarchived it to
			err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if xmlFile != "" {
					return filepath.SkipDir
				}

				// and find the XML file and set its path to xmlFile
				if isXML(path) || isXML(info.Name()) {
					xmlFile = path
				}
				return nil
			})
			if err != nil {
				return nil, err
			}

			// so we can open it properly
			return os.Open(xmlFile)
		}
	} else if !isXML(filename) {
		// file type not gzip or recognized by archiver tool
		return nil, fmt.Errorf("File type \"%s\" not yet supported", filename)
	} else {
		r, err = os.Open(xmlFile)
	}

	// file is a regular XML file
	return r, err
}
