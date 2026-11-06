"""Verify the real Nginx and Caddy configuration against a synthetic backend.

Run with Python 3 and a running Docker engine; no database or credentials needed.
"""

import concurrent.futures
import json
import subprocess
import tempfile
import time
import uuid
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, build_opener, ProxyHandler


DEPLOY = Path(__file__).resolve().parents[1]
OPENER = build_opener(ProxyHandler({}))


def docker(*args, check=True):
    return subprocess.run(
        ["docker", *args], check=check, capture_output=True, text=True
    ).stdout.strip()


def mount(source, target):
    return f"type=bind,source={source},target={target},readonly"


def request(base, path, headers=None, method="GET"):
    req = Request(
        base + path,
        headers={"Host": "cache.test", **(headers or {})},
        method=method,
        data=b"fixture" if method in ("POST", "PATCH") else None,
    )
    try:
        response = OPENER.open(req, timeout=10)
    except HTTPError as error:
        response = error
    with response:
        body = response.read()
        return response.status, response.headers, json.loads(body) if body else None


def main():
    prefix = f"petrolida-cache-test-{uuid.uuid4().hex[:10]}"
    network = prefix + "-net"
    containers = [prefix + suffix for suffix in ("-backend", "-nginx", "-caddy")]
    network_created = False
    checks = 0

    def passed(label):
        nonlocal checks
        checks += 1
        print(f"PASS {label}", flush=True)

    try:
        docker("info", "--format", "{{.ServerVersion}}")
        docker("network", "create", network)
        network_created = True
        with tempfile.TemporaryDirectory(prefix=prefix) as temp_dir:
            temp = Path(temp_dir)
            (temp / "staging").mkdir()
            # Test the actual routes without issuing production certificates.
            (temp / "Caddyfile").write_text(
                "{\n    auto_https off\n}\n" + (DEPLOY / "Caddyfile").read_text(),
                encoding="utf-8",
            )
            docker(
                "run", "-d", "--name", containers[0], "--network", network,
                "--network-alias", "backend", "--network-alias", "frontend",
                "--mount", mount(DEPLOY / "tests" / "mock_backend.py", "/srv/mock.py"),
                "python:3.12-alpine", "python", "/srv/mock.py",
            )
            docker(
                "run", "-d", "--name", containers[1], "--network", network,
                "--network-alias", "nginx-cache", "-p", "127.0.0.1::8080",
                "--mount", mount(DEPLOY / "nginx-cache.conf", "/etc/nginx/conf.d/default.conf"),
                "nginx:stable-alpine",
            )
            docker("exec", containers[1], "nginx", "-t")
            docker(
                "run", "-d", "--name", containers[2], "--network", network,
                "-e", "SITE_ADDRESS=http://cache.test", "-p", "127.0.0.1::80",
                "--mount", mount(temp / "Caddyfile", "/etc/caddy/Caddyfile"),
                "--mount", mount(temp / "staging", "/etc/caddy/staging"),
                "caddy:2-alpine",
            )
            docker("exec", containers[2], "caddy", "validate", "--config", "/etc/caddy/Caddyfile")
            port = docker("port", containers[2], "80/tcp").rsplit(":", 1)[1]
            base = f"http://127.0.0.1:{port}"
            for _ in range(40):
                try:
                    # The health route goes straight to the backend. Wait for
                    # the Nginx route too before testing cold cache entries.
                    # This synthetic path is not on the cache allowlist.
                    if (request(base, "/api/v1/health")[0] == 200 and
                            request(base, "/api/v1/master/cache-readiness")[0] == 200):
                        break
                except (URLError, TimeoutError):
                    pass
                time.sleep(0.25)
            else:
                raise AssertionError("Proxy did not become ready")

            routes = (
                "division", "division/sub", "department", "faculty", "divisions",
                "subdivisions", "departments", "faculties",
            )
            for route in routes:
                path = "/api/v1/master/" + route
                first = request(base, path)
                second = request(base, path)
                assert first[0] == second[0] == 200, (path, first, second)
                assert first[1].get("X-Cache-Status") == "MISS", first
                assert second[1].get("X-Cache-Status") == "HIT", second
                assert first[2] == second[2]
                assert first[1]["X-Request-ID"] != second[1]["X-Request-ID"]
            passed("all eight public list routes MISS then HIT; request IDs stay unique")

            path = "/api/v1/master/faculties"
            for headers in ({"Cookie": "session=fixture"}, {"Authorization": "Bearer fixture"}, {"Cookie": "0"}):
                first, second = request(base, path, headers), request(base, path, headers)
                assert first[1].get("X-Cache-Status") == second[1].get("X-Cache-Status") == "BYPASS"
                assert first[2]["count"] < second[2]["count"]
                assert request(base, path)[2]["count"] == 1
            passed("cookie and Authorization bypass reads and writes, preserving anonymous cache")

            custom_id = request(base, path, {"X-Request-ID": "fixture-request-id"})
            assert custom_id[1]["X-Request-ID"] == "fixture-request-id"
            head = request(base, path, method="HEAD")
            assert head[0] == 200 and head[1].get("X-Cache-Status") == "HIT" and head[2] is None
            passed("HEAD shares the GET cache; incoming request IDs are preserved")

            for method in ("POST", "PATCH", "OPTIONS"):
                first = request(base, path, method=method)
                second = request(base, path, method=method)
                assert first[1].get("X-Cache-Status") != "HIT"
                assert second[1].get("X-Cache-Status") != "HIT"
                assert first[2]["count"] < second[2]["count"]
            passed("POST, PATCH and OPTIONS never use the cache")

            for query in ("version=one", "version=two"):
                path = "/api/v1/master/faculties?" + query
                assert request(base, path)[1].get("X-Cache-Status") == "MISS"
                assert request(base, path)[1].get("X-Cache-Status") == "HIT"
            passed("query strings have separate cache entries")

            path = "/api/v1/master/faculties?cors=test"
            no_origin = request(base, path)
            allowed = request(base, path, {"Origin": "https://allowed.example"})
            assert allowed[1].get("X-Cache-Status") == "MISS"
            allowed_hit = request(base, path, {"Origin": "https://allowed.example"})
            assert allowed_hit[1].get("X-Cache-Status") == "HIT"
            assert allowed_hit[1]["Access-Control-Allow-Origin"] == "https://allowed.example"
            assert not no_origin[1].get("Access-Control-Allow-Origin")
            denied = request(base, path, {"Origin": "https://denied.example"})
            assert denied[1].get("X-Cache-Status") == "MISS"
            assert not denied[1].get("Access-Control-Allow-Origin")
            assert not request(base, path)[1].get("Access-Control-Allow-Origin")
            passed("CORS responses stay isolated by Origin, including requests without Origin")

            for query in ("policy=no-store", "policy=private", "policy=no-cache", "set_cookie=1", "status=404", "status=500", "status=302"):
                path = "/api/v1/master/faculties?" + query
                first, second = request(base, path), request(base, path)
                assert second[1].get("X-Cache-Status") != "HIT", query
                assert first[2]["count"] < second[2]["count"], query
            passed("no-store/private/no-cache, Set-Cookie, errors and redirects are not cached")

            for path in (
                "/api/v1/master/faculty/00000000-0000-0000-0000-000000000001",
                "/api/v1/master/unknown", "/api/v1/oprec-staff/config",
                "/api/v1/announcements/config", "/api/v1/auth/me",
                "/api/v1/oprec-staff", "/api/v1/oprec-staff/count",
                "/api/v1/oprec-staff/id/documents/id", "/go/fixture", "/",
            ):
                first, second = request(base, path), request(base, path)
                assert not second[1].get("X-Cache-Status"), path
                assert first[2]["count"] < second[2]["count"], path
            passed("detail, unknown, config, admin, documents, shortlinks and frontend stay uncached")

            path = "/api/v1/master/faculties?burst=test&delay=0.3"
            with concurrent.futures.ThreadPoolExecutor(max_workers=12) as pool:
                responses = list(pool.map(lambda _: request(base, path), range(12)))
            assert all(response[0] == 200 and response[2]["count"] == 1 for response in responses)
            assert len({response[1]["X-Request-ID"] for response in responses}) == 12
            passed("12 concurrent cold requests produce one upstream request")
            print(f"All {checks} proxy integration checks passed.", flush=True)
    except Exception:
        for container in containers:
            logs = subprocess.run(
                ["docker", "logs", "--tail", "20", container],
                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
            ).stdout.strip()
            if logs:
                print(f"Logs for {container}:\n{logs}", flush=True)
        raise
    finally:
        docker("rm", "-f", *containers, check=False)
        if network_created:
            docker("network", "rm", network, check=False)


if __name__ == "__main__":
    main()
