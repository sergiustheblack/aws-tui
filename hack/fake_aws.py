"""Fake AWS endpoint for manually driving aws-tui without touching real AWS.

Answers every request with a DescribeInstancesResponse holding two instances,
so EC2 > Instances shows data and other views get an error or an empty list.
Usage: python3 -I hack/fake_aws.py [port]   (default 18555)
"""
import sys
import http.server
BODY=b'''<?xml version="1.0" encoding="UTF-8"?>
<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><requestId>1</requestId><reservationSet><item><reservationId>r-1</reservationId><instancesSet>
<item><instanceId>i-0aaa</instanceId><instanceType>t3.micro</instanceType><instanceState><code>16</code><name>running</name></instanceState><ipAddress>1.2.3.4</ipAddress><tagSet><item><key>Name</key><value>web-1</value></item></tagSet></item>
<item><instanceId>i-0bbb</instanceId><instanceType>t3.large</instanceType><instanceState><code>80</code><name>stopped</name></instanceState><tagSet><item><key>Name</key><value>db-1</value></item></tagSet></item>
</instancesSet></item></reservationSet></DescribeInstancesResponse>'''
class H(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        self.rfile.read(int(self.headers.get('Content-Length',0)))
        self.send_response(200); self.send_header('Content-Type','text/xml'); self.send_header('Content-Length',str(len(BODY))); self.end_headers(); self.wfile.write(BODY)
    do_GET=do_POST
    def log_message(self,*a): pass
http.server.HTTPServer(('127.0.0.1',int(sys.argv[1]) if len(sys.argv)>1 else 18555),H).serve_forever()
