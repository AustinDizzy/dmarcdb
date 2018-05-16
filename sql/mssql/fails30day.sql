SELECT        TOP (10) PERCENT org_name, source_ip, hostname, domain, location, spf_domain, SUM(count) AS sumOfCount, DATEADD(s, date_range_end, '1970-01-01') AS lastObserved
FROM            dbo.records
GROUP BY org_name, source_ip, hostname, domain, location, spf_domain, spf_result, dkim, date_range_end
HAVING        (spf_result <> 'pass') AND (dkim = 'fail') AND (date_range_end > DATEDIFF(s, CONVERT(DATETIME, '1970-01-01 00:00:00', 102), GETUTCDATE()) - 2628000)
ORDER BY sumOfCount DESC