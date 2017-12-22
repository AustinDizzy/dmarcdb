package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/antchfx/xquery/xml"
)

type DMARCMetadata struct {
	OrgName          string `xml:"org_name"`
	Email            string `xml:"email"`
	ExtraContactInfo string `xml:"extra_contact_info"`
	ReportID         string `xml:"report_id"`
	DateRangeBegin   int64  `xml:"date_range>begin"`
	DateRangeEnd     int64  `xml:"date_range>end"`
}

type DMARCPolicy struct {
	Domain string `xml:"domain"`
	ADKIM  string `xml:"adkim"`
	ASPF   string `xml:"aspf"`
	P      string `xml:"p"`
	PCT    int    `xml:"pct"`
}

type DMARCRecord struct {
	SourceIP      string `xml:"row>source_ip"`
	Count         int    `xml:"row>count"`
	Disposition   string `xml:"row>policy_evaluated>disposition"`
	DKIM          string `xml:"row>policy_evaluated>dkim"`
	SPF           string `xml:"row>policy_evaluated>spf"`
	ReasonType    string `xml:"row>policy_evaluated>reason>type"`
	ReasonComment string `xml:"row>policy_evaluated>reason>comment"`
	EnvelopeTo    string `xml:"identifiers>envelope_to"`
	HeaderFrom    string `xml:"identifiers>header_from"`
	DKIMDomain    string `xml:"auth_results>dkim>domain"`
	DKIMResult    string `xml:"auth_results>dkim>result"`
	DKIMHResult   string `xml:"auth_results>dkim>human_result"`
	SPFDomain     string `xml:"auth_results>spf>domain"`
	SPFResult     string `xml:"auth_results>spf>result"`
}

type DMARCFeedback struct {
	Metadata DMARCMetadata `xml:"report_metadata"`
	Policy   DMARCPolicy   `xml:"policy_published"`
	Records  []DMARCRecord `xml:"record"`
}

func parseDMARC(r io.Reader) (*DMARCFeedback, error) {
	doc, err := xmlquery.Parse(r)
	if err != nil {
		return nil, err
	}

	meta, err := getNodeElm(doc, "feedback/report_metadata")
	if err != nil {
		return nil, err
	}

	policy, err := getNodeElm(doc, "feedback/policy_published")
	if err != nil {
		return nil, err
	}

	dateBegin, err := strconv.ParseInt(getNodeVal(meta, "date_range/begin"), 10, 64)
	if err != nil {
		return nil, err
	}

	dateEnd, err := strconv.ParseInt(getNodeVal(meta, "date_range/end"), 10, 64)
	if err != nil {
		return nil, err
	}

	pct, err := strconv.Atoi(policy.SelectElement("pct").InnerText())
	if err != nil {
		return nil, err
	}

	records := doc.SelectElements("feedback/record")
	feedback := &DMARCFeedback{
		Metadata: DMARCMetadata{
			OrgName:          getNodeVal(meta, "org_name"),
			Email:            getNodeVal(meta, "email"),
			ExtraContactInfo: getNodeVal(meta, "extra_contact_info"),
			ReportID:         getNodeVal(meta, "report_id"),
			DateRangeBegin:   dateBegin,
			DateRangeEnd:     dateEnd,
		},
		Policy: DMARCPolicy{
			Domain: getNodeVal(policy, "domain"),
			ADKIM:  getNodeVal(policy, "adkim"),
			ASPF:   getNodeVal(policy, "aspf"),
			P:      getNodeVal(policy, "p"),
			PCT:    pct,
		},
	}
	feedback.Records = make([]DMARCRecord, len(records))
	for i, node := range records {
		feedback.Records[i].SourceIP = node.SelectElement("row/source_ip").InnerText()
		feedback.Records[i].Count, _ = strconv.Atoi(node.SelectElement("row/count").InnerText())
		feedback.Records[i].Disposition = getNodeVal(node, "row/policy_evaluated/disposition")
		feedback.Records[i].DKIM = getNodeVal(node, "row/policy_evaluated/dkim")
		feedback.Records[i].SPF = getNodeVal(node, "row/policy_evaluated/spf")
		feedback.Records[i].ReasonType = getNodeVal(node, "row/policy_evaluated/reason/type")
		feedback.Records[i].ReasonComment = getNodeVal(node, "row/policy_evaluated/reason/comment")
		feedback.Records[i].EnvelopeTo = getNodeVal(node, "identifiers/envelope_to")
		feedback.Records[i].HeaderFrom = getNodeVal(node, "identifiers/header_from")
		feedback.Records[i].DKIMDomain = getNodeVal(node, "auth_results/dkim/domain")
		feedback.Records[i].DKIMResult = getNodeVal(node, "auth_results/dkim/result")
		feedback.Records[i].DKIMHResult = getNodeVal(node, "auth_results/dkim/human_result")
		feedback.Records[i].SPFDomain = getNodeVal(node, "auth_results/spf/domain")
		feedback.Records[i].SPFResult = getNodeVal(node, "auth_results/spf/result")
	}

	return feedback, nil
}

func getNodeElm(n *xmlquery.Node, path string) (node *xmlquery.Node, err error) {
	node = n.SelectElement(path)
	if node == nil {
		err = fmt.Errorf("Report doesn't contain required \"%s\"", path)
	}
	return
}

func getNodeVal(n *xmlquery.Node, path string, def ...string) string {
	node, err := getNodeElm(n, path)
	if err != nil {
		if len(def) > 0 {
			return def[0]
		}
		return "NULL"
	}
	return strings.TrimSpace(node.InnerText())
}
