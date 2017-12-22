SELECT records.org_name, records.source_ip, records.hostname, records.domain, records.location, records.spf_domain, Sum(records.Count) AS SumOfcount, to_timestamp(max(records.date_range_end)) AS lastObserved
FROM records
GROUP BY records.org_name, records.source_ip, records.hostname, records.domain, records.location, records.spf_domain, records.spf_result, records.dkim
HAVING (((records.spf_result)<>'pass') AND ((records.dkim)='fail'))
ORDER BY Sum(records.Count) DESC;