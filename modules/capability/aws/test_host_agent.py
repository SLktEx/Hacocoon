import importlib.util
import io
import json
import unittest
from pathlib import Path
from unittest.mock import patch
import botocore.httpsession
from botocore.awsrequest import AWSResponse

spec = importlib.util.spec_from_file_location("haco_aws_agent", Path(__file__).with_name("host_agent.py"))
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)
ACCOUNT = "123456789012"
PRINCIPAL = "arn:aws:sts::123456789012:assumed-role/Developer/session"
CREDS = {"AccessKeyId": "ASIAFIXTUREONLY", "SecretAccessKey": "fixture-only-not-a-secret", "SessionToken": "fixture-session"}
STS = ("<GetCallerIdentityResponse xmlns=\"https://sts.amazonaws.com/doc/2011-06-15/\"><GetCallerIdentityResult>"
       "<Arn>" + PRINCIPAL + "</Arn><Account>" + ACCOUNT + "</Account><UserId>fixture</UserId>"
       "</GetCallerIdentityResult></GetCallerIdentityResponse>").encode()

class Raw:
    def __init__(self, data):
        self.data = data
    def stream(self, amt=None, decode_content=False):
        yield self.data

class AgentTest(unittest.TestCase):
    def setUp(self):
        self.requests = []
        self.s3 = [(200, {}, b'<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><IsTruncated>false</IsTruncated><Contents><Key>project/config.json</Key><Size>42</Size></Contents></ListBucketResult>')]
        def send(session, request):
            self.requests.append(request)
            if request.url.startswith("https://sts.ap-northeast-1.amazonaws.com"):
                return AWSResponse(request.url, 200, {}, Raw(STS))
            self.assertTrue(request.url.startswith("https://s3.ap-northeast-1.amazonaws.com/example-bucket?"))
            self.assertEqual(request.method, "GET")
            self.assertIn("list-type=2", request.url)
            self.assertEqual(request.headers["x-amz-expected-bucket-owner"], ACCOUNT.encode())
            auth = request.headers["Authorization"].decode()
            self.assertIn("Credential=ASIAFIXTUREONLY/", auth)
            self.assertIn("/ap-northeast-1/s3/aws4_request", auth)
            self.assertEqual(request.headers["X-Amz-Security-Token"], b"fixture-session")
            status, headers, data = self.s3.pop(0)
            return AWSResponse(request.url, status, headers, Raw(data))
        self.send = patch.object(botocore.httpsession.URLLib3Session, "send", send)
        self.send.start()
        self.addCleanup(self.send.stop)
        real_isfile = agent.os.path.isfile
        self.exists = patch.object(agent.os.path, "isfile", side_effect=lambda path: path == agent.AWS or real_isfile(path))
        self.exists.start()
        self.addCleanup(self.exists.stop)
        self.exports = []
        def export(args, env, payload=None):
            self.exports.append((args, env))
            self.assertEqual(args, ["configure", "export-credentials", "--profile", "default", "--format", "process"])
            self.assertNotIn("AWS_SECRET_ACCESS_KEY", env)
            return json.dumps(CREDS).encode()
        self.run = patch.object(agent, "run", export)
        self.run.start()
        self.addCleanup(self.run.stop)
    def request(self, **changes):
        req = {"mode":"list", "profile":"default", "region":"ap-northeast-1", "account":ACCOUNT,
               "principal":PRINCIPAL, "bucket":"example-bucket", "prefix":"project/"}
        req.update(changes)
        return req
    def test_preparation_does_not_list(self):
        result = agent.execute({"mode":"identity", "profile":"default", "region":"ap-northeast-1"})
        self.assertEqual(result["identity"]["account"], ACCOUNT)
        self.assertEqual(len(self.requests), 1)
        self.assertNotIn("fixture-session", json.dumps(result))
    def test_real_sdk_signing_uses_one_frozen_identity_and_exact_scope(self):
        result = agent.execute(self.request())
        self.assertEqual(result["objects"], [{"key":"project/config.json", "size":42}])
        self.assertEqual(len(self.exports), 1)
        self.assertEqual(len(self.requests), 2)
        self.assertNotIn("fixture-session", json.dumps(result))
    def test_identity_change_stops_before_s3(self):
        with self.assertRaisesRegex(agent.Failure, "identity_changed"):
            agent.execute(self.request(principal=PRINCIPAL + "-other"))
        self.assertEqual(len(self.requests), 1)
    def test_region_redirect_cannot_escape_reviewed_scope(self):
        self.s3 = [(301, {"x-amz-bucket-region":"eu-west-1"}, b"<Error><Code>PermanentRedirect</Code></Error>")]
        with self.assertRaises(agent.Failure):
            agent.execute(self.request())
        self.assertEqual(len(self.requests), 2)
    def test_region_discovery_head_request_is_not_implicitly_allowed(self):
        self.s3 = [(301, {}, b"<Error><Code>PermanentRedirect</Code></Error>")]
        with self.assertRaises(agent.Failure):
            agent.execute(self.request())
        self.assertEqual(len(self.requests), 2)
    def test_aws_denial_is_distinct_and_message_is_not_returned(self):
        self.s3 = [(403, {}, b"<Error><Code>AccessDenied</Code><Message>secret-upstream-body</Message></Error>")]
        with self.assertRaisesRegex(agent.Failure, "^aws_denied$"):
            agent.execute(self.request())
    def test_pagination_preserves_prefix_and_owner(self):
        self.s3 = [
            (200, {}, b"<ListBucketResult><IsTruncated>true</IsTruncated><NextContinuationToken>file://not-a-file</NextContinuationToken></ListBucketResult>"),
            (200, {}, b"<ListBucketResult><IsTruncated>false</IsTruncated></ListBucketResult>")]
        result = agent.execute(self.request())
        self.assertEqual(result["objects"], [])
        self.assertEqual(len(self.requests), 3)
        self.assertIn("continuation-token=file%3A%2F%2Fnot-a-file", self.requests[-1].url)
    def test_repeated_page_token_is_failure_not_partial_success(self):
        self.s3 = [(200, {}, b"<ListBucketResult><IsTruncated>true</IsTruncated><NextContinuationToken>same</NextContinuationToken></ListBucketResult>")] * 2
        with self.assertRaisesRegex(agent.Failure, "aws_failed"):
            agent.execute(self.request())

if __name__ == "__main__":
    unittest.main()
