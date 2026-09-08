"""Trusted Host AWS adapter. Request data never becomes shell code or credentials."""
import json
import base64
import hashlib
import os
import re
import selectors
import signal
import subprocess
import sys
import time
from urllib.parse import urlsplit

AWS = "/usr/local/bin/aws"
LIMIT = 2 << 20
BASE = {"HOME": "/root", "PATH": "/usr/local/bin:/usr/bin:/bin", "LANG": "C.UTF-8",
        "AWS_CLI_AUTO_PROMPT": "off", "AWS_PAGER": "", "AWS_EC2_METADATA_DISABLED": "true"}
REGION = r"(af|ap|ca|eu|il|me|mx|sa|us)-(central|east|west|north|south|northeast|northwest|southeast|southwest)-[1-9][0-9]?"
class Failure(Exception):
    pass

def run(args, env, payload=None):
    process = subprocess.Popen([AWS] + args, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, cwd="/root", env=env, start_new_session=True)
    output = bytearray()
    error = bytearray()
    selector = selectors.DefaultSelector()
    try:
        process.stdin.write(payload or b"")
        process.stdin.close()
        selector.register(process.stdout, selectors.EVENT_READ, output)
        selector.register(process.stderr, selectors.EVENT_READ, error)
        deadline = time.monotonic() + 45
        while selector.get_map():
            if time.monotonic() > deadline:
                raise Failure("aws_failed")
            for key, _ in selector.select(0.1):
                data = os.read(key.fileobj.fileno(), 65536)
                if not data:
                    selector.unregister(key.fileobj)
                    continue
                key.data.extend(data)
                if len(output) + len(error) > LIMIT:
                    raise Failure("too_large")
        code = process.wait(timeout=1)
        if code:
            if b"AccessDenied" in error or b"Forbidden" in error:
                raise Failure("aws_denied")
            raise Failure("aws_failed")
        return bytes(output)
    finally:
        selector.close()
        # Covers credential_process descendants as well as the direct CLI process.
        if process.returncode is None:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        process.wait()
        process.stdout.close()
        process.stderr.close()

def api(service, action, region, env, fields, consume=None):
    # Signing and transport both remain bound to the reviewed region/endpoint.
    import botocore.session
    from botocore.config import Config
    from botocore.exceptions import ClientError
    host = service + "." + region + ".amazonaws.com"
    client = botocore.session.get_session().create_client(
        service, region_name=region, endpoint_url="https://" + host,
        aws_access_key_id=env["AWS_ACCESS_KEY_ID"],
        aws_secret_access_key=env["AWS_SECRET_ACCESS_KEY"],
        aws_session_token=env.get("AWS_SESSION_TOKEN"),
        config=Config(signature_version="s3v4" if service == "s3" else "v4",
                      retries={"total_max_attempts": 1}, connect_timeout=10, read_timeout=30,
                      s3={"addressing_style": "path", "use_arn_region": False}))
    def before_sign(request, region_name, **kwargs):
        if region_name != region or request.context.get("signing", {}).get("region", region) != region:
            raise Failure("identity_changed")
    def before_send(request, **kwargs):
        target = urlsplit(request.url)
        authorization = request.headers.get("Authorization", b"")
        if isinstance(authorization, bytes):
            authorization = authorization.decode("ascii")
        expected = (r"^AWS4-HMAC-SHA256 Credential=" + re.escape(env["AWS_ACCESS_KEY_ID"])
                    + r"/[0-9]{8}/" + re.escape(region) + "/" + service + r"/aws4_request,")
        if not re.match(expected, authorization):
            raise Failure("identity_changed")
        if target.scheme != "https" or target.netloc != host or (service == "s3" and request.method != "GET"):
            raise Failure("identity_changed")
    client.meta.events.register_first("before-sign." + service, before_sign)
    client.meta.events.register_first("before-send." + service, before_send)
    try:
        method = {"get-caller-identity": "get_caller_identity",
                  "list-objects-v2": "list_objects_v2", "get-object": "get_object"}[action]
        response = getattr(client, method)(**fields)
        return consume(response) if consume else response
    except ClientError as error:
        code = error.response.get("Error", {}).get("Code")
        if code in ("AccessDenied", "AccessDeniedException", "Forbidden"):
            raise Failure("aws_denied") from None
        raise Failure("aws_failed") from None
    finally:
        client.close()

def execute(req):
    allowed = {"mode", "profile", "region", "account", "principal", "bucket", "prefix", "key"}
    if not isinstance(req, dict) or set(req) - allowed or any(not isinstance(v, str) for v in req.values()):
        raise Failure("invalid")
    mode, profile, region = req.get("mode"), req.get("profile"), req.get("region", "")
    if mode not in ("identity", "list", "get") or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,63}", profile or ""):
        raise Failure("invalid")
    try:
        import botocore.session
    except ImportError:
        raise Failure("not_configured") from None
    if not os.path.isfile(AWS):
        raise Failure("not_configured")
    if not region:
        region = run(["configure", "get", "region", "--profile", profile], BASE).decode().strip()
    if not re.fullmatch(REGION, region):
        raise Failure("not_configured")
    # Resolve once per operation inside Host. Never return or persist these values.
    credentials = json.loads(run(["configure", "export-credentials", "--profile", profile, "--format", "process"], BASE))
    frozen = dict(BASE, AWS_CONFIG_FILE="/dev/null", AWS_SHARED_CREDENTIALS_FILE="/dev/null",
                  AWS_IGNORE_CONFIGURED_ENDPOINT_URLS="true", AWS_MAX_ATTEMPTS="1")
    for field, name in (("AccessKeyId", "AWS_ACCESS_KEY_ID"), ("SecretAccessKey", "AWS_SECRET_ACCESS_KEY"), ("SessionToken", "AWS_SESSION_TOKEN")):
        value = credentials.get(field, "")
        if not isinstance(value, str) or len(value) > 16384 or "\x00" in value:
            raise Failure("not_configured")
        if value:
            frozen[name] = value
    if not frozen.get("AWS_ACCESS_KEY_ID") or not frozen.get("AWS_SECRET_ACCESS_KEY"):
        raise Failure("not_configured")
    caller = api("sts", "get-caller-identity", region, frozen, {})
    identity = {"account": caller.get("Account"), "principal": caller.get("Arn"), "region": region}
    if mode == "identity":
        return {"identity": identity}
    if identity["account"] != req.get("account") or identity["principal"] != req.get("principal"):
        raise Failure("identity_changed")
    bucket, prefix = req.get("bucket", ""), req.get("prefix", "")
    if not re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", bucket) or ".." in bucket or bucket.endswith(("--x-s3", "-s3alias", "--ol-s3")) or bucket.startswith("xn--"):
        raise Failure("invalid")
    if len(prefix.encode()) > 1024 or not re.fullmatch(r"[0-9]{12}", req.get("account", "")):
        raise Failure("invalid")
    if mode == "get":
        key = req.get("key", "")
        if not key or len(key.encode())>1024 or any(part in (".", "..") for part in key.split("/")):
            raise Failure("invalid")
        def consume(response):
            body = response["Body"]
            try:
                length = response.get("ContentLength")
                if type(length) is not int or length<0 or response.get("ResponseMetadata",{}).get("HTTPStatusCode")!=200 or "ContentRange" in response:
                    raise Failure("aws_failed")
                digest = hashlib.sha256()
                received = 0
                while True:
                    chunk = body.read(65536)
                    if not chunk:
                        break
                    received += len(chunk)
                    if received > length:
                        raise Failure("aws_failed")
                    digest.update(chunk)
                    print(json.dumps({"data":base64.b64encode(chunk).decode("ascii")}), flush=True)
                if received != length:
                    raise Failure("aws_failed")
                return {"identity":identity,"receipt":{"bytes":received,"sha256":digest.hexdigest()}}
            finally:
                body.close()
        return api("s3", "get-object", region, frozen,
                   {"Bucket":bucket,"Key":key,"ExpectedBucketOwner":identity["account"]},consume)
    fields = {"Bucket": bucket, "Prefix": prefix, "ExpectedBucketOwner": identity["account"], "MaxKeys": 1000}
    objects, tokens = [], set()
    for _ in range(100):
        page = api("s3", "list-objects-v2", region, frozen, fields)
        content = page.get("Contents", [])
        if not isinstance(content, list):
            raise Failure("aws_failed")
        for item in content:
            if not isinstance(item, dict) or not isinstance(item.get("Key"), str) or type(item.get("Size")) is not int:
                raise Failure("aws_failed")
            objects.append({"key": item["Key"], "size": item["Size"]})
        if len(json.dumps(objects).encode()) > 1 << 20:
            raise Failure("too_large")
        if page.get("IsTruncated") is False:
            return {"identity": identity, "objects": objects}
        token = page.get("NextContinuationToken")
        if page.get("IsTruncated") is not True or not isinstance(token, str) or not token or len(token)>4096 or token in tokens:
            raise Failure("aws_failed")
        tokens.add(token)
        fields["ContinuationToken"] = token
    raise Failure("too_large")

def main():
    try:
        raw = sys.stdin.buffer.read(8193)
        if len(raw)>8192:
            raise Failure("invalid")
        result = execute(json.loads(raw))
    except Failure as error:
        result = {"error": str(error)}
    except Exception:
        result = {"error": "aws_failed"}
    print(json.dumps(result, ensure_ascii=True))
if __name__ == "__main__":
    main()
