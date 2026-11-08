"""Fake AWS endpoint for manually driving aws-tui without touching real AWS.

Route 53 has one hosted zone (example.com, Z1) whose records are kept in
memory, so they can be listed, created, edited and deleted. Every other
request gets a DescribeInstancesResponse holding two instances, so EC2 >
Instances shows data and other views get an error or an empty list.
Usage: python3 -I hack/fake_aws.py [port]   (default 18555)
"""
import sys
import re
import http.server
import xml.etree.ElementTree as ET
from xml.sax.saxutils import escape
BODY=b'''<?xml version="1.0" encoding="UTF-8"?>
<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><requestId>1</requestId><reservationSet><item><reservationId>r-1</reservationId><instancesSet>
<item><instanceId>i-0aaa</instanceId><instanceType>t3.micro</instanceType><instanceState><code>16</code><name>running</name></instanceState><ipAddress>1.2.3.4</ipAddress><tagSet><item><key>Name</key><value>web-1</value></item></tagSet></item>
<item><instanceId>i-0bbb</instanceId><instanceType>t3.large</instanceType><instanceState><code>80</code><name>stopped</name></instanceState><tagSet><item><key>Name</key><value>db-1</value></item></tagSet></item>
</instancesSet></item></reservationSet></DescribeInstancesResponse>'''

NS='https://route53.amazonaws.com/doc/2013-04-01/'
# (name, type, set identifier) -> {'ttl': int, 'values': [str]}
RECORDS={
    ('example.com.','NS',None): {'ttl':172800,'values':['ns-1.example.net.','ns-2.example.net.']},
    ('example.com.','SOA',None): {'ttl':900,'values':['ns-1.example.net. admin.example.com. 1 7200 900 1209600 86400']},
    ('example.com.','TXT',None): {'ttl':300,'values':['"v=spf1 -all"']},
    ('www.example.com.','A',None): {'ttl':300,'values':['10.0.0.1','10.0.0.2']},
    ('mail.example.com.','MX',None): {'ttl':3600,'values':['10 mx1.example.com.']},
}

def record_xml(key, r):
    name, typ, sid = key
    s = '<ResourceRecordSet><Name>%s</Name><Type>%s</Type>' % (escape(name), typ)
    if sid is not None:
        s += '<SetIdentifier>%s</SetIdentifier>' % escape(sid)
    s += '<TTL>%d</TTL><ResourceRecords>' % r['ttl']
    s += ''.join('<ResourceRecord><Value>%s</Value></ResourceRecord>' % escape(v) for v in r['values'])
    return s + '</ResourceRecords></ResourceRecordSet>'

def apply_changes(body):
    """Applies a change batch all or nothing; returns an error message or None."""
    new = dict(RECORDS)
    for change in ET.fromstring(body).iter('{%s}Change' % NS):
        rrs = change.find('{%s}ResourceRecordSet' % NS)
        get = lambda tag: rrs.findtext('{%s}%s' % (NS, tag))
        name = get('Name')
        key = (name if name.endswith('.') else name + '.', get('Type'), get('SetIdentifier'))
        r = {'ttl': int(get('TTL') or 0), 'values': [v.text for v in rrs.iter('{%s}Value' % NS)]}
        action = change.findtext('{%s}Action' % NS)
        if action == 'DELETE':
            if new.get(key) != r:
                return 'Tried to delete resource record set [name=%s, type=%s] but it was not found or the values do not match' % key[:2]
            del new[key]
        elif action == 'CREATE':
            if key in new:
                return 'Tried to create resource record set [name=%s, type=%s] but it already exists' % key[:2]
            new[key] = r
        else:
            new[key] = r
    RECORDS.clear(); RECORDS.update(new)
    return None

def route53(method, path, body):
    """Returns (status, body) for a Route 53 request, or None for other services."""
    if path == '/2013-04-01/hostedzone':
        return 200, '<ListHostedZonesResponse xmlns="%s"><HostedZones><HostedZone><Id>/hostedzone/Z1</Id><Name>example.com.</Name><CallerReference>1</CallerReference><Config><Comment>fake zone</Comment><PrivateZone>false</PrivateZone></Config><ResourceRecordSetCount>%d</ResourceRecordSetCount></HostedZone></HostedZones><IsTruncated>false</IsTruncated><MaxItems>100</MaxItems></ListHostedZonesResponse>' % (NS, len(RECORDS))
    if not re.fullmatch(r'/2013-04-01/hostedzone/Z1/rrset', path):
        return None
    if method == 'GET':
        keys = sorted(RECORDS, key=lambda k: (k[0].split('.')[::-1], k[1], k[2] or ''))
        items = ''.join(record_xml(k, RECORDS[k]) for k in keys)
        return 200, '<ListResourceRecordSetsResponse xmlns="%s"><ResourceRecordSets>%s</ResourceRecordSets><IsTruncated>false</IsTruncated><MaxItems>300</MaxItems></ListResourceRecordSetsResponse>' % (NS, items)
    err = apply_changes(body)
    if err:
        return 400, '<ErrorResponse xmlns="%s"><Error><Type>Sender</Type><Code>InvalidChangeBatch</Code><Message>%s</Message></Error><RequestId>1</RequestId></ErrorResponse>' % (NS, escape(err))
    return 200, '<ChangeResourceRecordSetsResponse xmlns="%s"><ChangeInfo><Id>/change/C1</Id><Status>PENDING</Status><SubmittedAt>2026-01-01T00:00:00Z</SubmittedAt></ChangeInfo></ChangeResourceRecordSetsResponse>' % NS

class H(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get('Content-Length',0)))
        status, out = route53(self.command, self.path.split('?')[0].rstrip('/'), body) or (200, BODY)
        if isinstance(out, str): out = out.encode()
        self.send_response(status); self.send_header('Content-Type','text/xml'); self.send_header('Content-Length',str(len(out))); self.end_headers(); self.wfile.write(out)
    do_GET=do_POST
    def log_message(self,*a): pass
http.server.HTTPServer(('127.0.0.1',int(sys.argv[1]) if len(sys.argv)>1 else 18555),H).serve_forever()
