SELECT records.org_name, records.source_ip, records.hostname, records.domain, records.location, records.spf_domain, Sum(records.Count) AS SumOfcount, to_timestamp(max(records.date_range_begin)) AS lastObserved
FROM records
WHERE (to_timestamp(records.date_range_begin) > NOW() - INTERVAL '30 days')
GROUP BY records.org_name, records.source_ip, records.hostname, records.domain, records.location, records.spf_domain, records.spf_result, records.dkim
HAVING (((records.spf_result)<>'pass') AND ((records.dkim)='fail'))
ORDER BY Sum(records.Count) DESC;