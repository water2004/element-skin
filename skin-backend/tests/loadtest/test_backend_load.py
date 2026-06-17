import asyncio
import math
import os
import re
import socket
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Callable

import aiohttp
import asyncpg
import pytest
import uvicorn

from routes_reference import app, config, db, rate_limiter, texture_storage
from utils.password_utils import hash_password
from utils.typing import InviteCode, PlayerProfile, Session, Token, User
from utils.uuid_utils import generate_random_uuid


@dataclass
class LoadScenario:
    area: str
    name: str
    method: str
    path: str
    body: str = ""
    cookie: str = ""
    prepare: Callable[[], object] | None = None


@dataclass
class StepSummary:
    concurrency: int
    total: int
    success: int
    failed: int
    failure_pct: float
    rps: float
    success_rps: float
    avg: float
    p50: float
    p95: float
    p99: float
    statuses: dict[int, int]
    wall: float
    first_error: str = ""


@dataclass
class ScenarioResult:
    scenario: LoadScenario
    concurrency: int
    summary: StepSummary


@dataclass
class LoadSeed:
    user: User
    admin: User
    ygg_user: User
    profile_id: str
    profile_name: str
    texture_hash: str
    ygg_access_token: str
    ygg_client_token: str
    ygg_server_id: str


@dataclass
class LoadConfig:
    duration: float
    max_db_connections: int


@pytest.mark.asyncio
async def test_real_backend_load():
    if os.getenv("LOADTEST_ENABLE") != "1":
        pytest.skip("set LOADTEST_ENABLE=1 to run the real test-backend load test")

    load_cfg = load_test_config()
    concurrency = load_test_concurrency()
    db_name = f"elementskin_py_test_{os.getpid()}_{int(time.time() * 1000)}"
    test_dsn = f"postgresql://postgres:12345678@localhost:5432/{db_name}?sslmode=disable"
    old_dsn = db.dsn
    old_max_connections = db._max_connections
    old_db_config = config._data.get("database", {}).copy()
    old_textures_dir = texture_storage.textures_dir
    old_carousel_dir = config.get("carousel.directory", "carousel")

    temp_root = Path(os.getenv("LOADTEST_TMPDIR", Path.cwd() / ".loadtest-tmp")).resolve()
    textures_dir = temp_root / db_name / "textures"
    carousel_dir = temp_root / db_name / "carousel"
    textures_dir.mkdir(parents=True, exist_ok=True)
    carousel_dir.mkdir(parents=True, exist_ok=True)

    await ensure_database(db_name)
    server = None
    try:
        db.dsn = test_dsn
        db._max_connections = load_cfg.max_db_connections
        config._data.setdefault("database", {})
        config._data["database"]["dsn"] = test_dsn
        config._data["database"]["max_connections"] = load_cfg.max_db_connections
        config._data.setdefault("textures", {})["directory"] = str(textures_dir)
        config._data.setdefault("carousel", {})["directory"] = str(carousel_dir)
        texture_storage.textures_dir = str(textures_dir)

        await db.connect()
        await reset_public_schema()
        await db.init()
        load_cfg.max_db_connections = db._max_connections
        await db.setting.set("rate_limit_enabled", "false")
        rate_limiter._attempts.clear()

        seed = await seed_load_test_data()
        server = UvicornTestServer(app)
        await server.start()
        base_url = server.url

        user_cookie = await login(base_url, seed.user.email, "Password123")
        admin_cookie = await login(base_url, seed.admin.email, "Password123")
        scenarios = default_load_scenarios(seed, user_cookie, admin_cookie)
        scenarios = filter_scenarios(scenarios, os.getenv("LOADTEST_SCENARIOS", ""))

        results: list[ScenarioResult] = []
        for scenario in scenarios:
            if scenario.prepare:
                maybe_coro = scenario.prepare()
                if asyncio.iscoroutine(maybe_coro):
                    await maybe_coro
            summary = await run_step(base_url, scenario, concurrency, load_cfg.duration)
            results.append(ScenarioResult(scenario, concurrency, summary))
            print(
                "loadtest "
                f"scenario={scenario.name} concurrency={summary.concurrency} "
                f"requests={summary.total} ok={summary.success} fail={summary.failed} "
                f"fail_pct={summary.failure_pct:.2f} success_rps={summary.success_rps:.1f} "
                f"total_rps={summary.rps:.1f} avg={format_duration(summary.avg)} "
                f"p50={format_duration(summary.p50)} p95={format_duration(summary.p95)} "
                f"p99={format_duration(summary.p99)} status={format_statuses(summary.statuses)}"
            )
            if summary.first_error:
                print(f"loadtest scenario={scenario.name} first_error={summary.first_error}")

        await write_load_test_report(report_path(), load_cfg, concurrency, results)
    finally:
        if server:
            await server.close()
        await db.close()
        db.dsn = old_dsn
        db._max_connections = old_max_connections
        config._data["database"] = old_db_config
        config._data.setdefault("textures", {})["directory"] = old_textures_dir
        config._data.setdefault("carousel", {})["directory"] = old_carousel_dir
        texture_storage.textures_dir = old_textures_dir
        await drop_database(db_name)


class UvicornTestServer:
    def __init__(self, asgi_app):
        self.app = asgi_app
        self.server: uvicorn.Server | None = None
        self.task: asyncio.Task | None = None
        self.socket: socket.socket | None = None
        self.url = ""

    async def start(self):
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.bind(("127.0.0.1", 0))
        sock.listen(2048)
        sock.setblocking(False)
        host, port = sock.getsockname()[:2]
        self.url = f"http://{host}:{port}"
        self.socket = sock
        cfg = uvicorn.Config(
            self.app,
            host=host,
            port=port,
            lifespan="off",
            log_level="warning",
            access_log=False,
        )
        self.server = uvicorn.Server(cfg)
        self.task = asyncio.create_task(self.server.serve(sockets=[sock]))
        for _ in range(200):
            if self.server.started:
                return
            await asyncio.sleep(0.01)
        raise RuntimeError("load-test HTTP server did not start")

    async def close(self):
        if self.server:
            self.server.should_exit = True
        if self.task:
            await asyncio.wait_for(self.task, timeout=10)
        self.server = None
        self.task = None
        self.socket = None


async def ensure_database(db_name: str):
    admin_dsn = admin_database_dsn()
    conn = await asyncpg.connect(admin_dsn)
    try:
        exists = await conn.fetchval("SELECT 1 FROM pg_database WHERE datname = $1", db_name)
        if not exists:
            await conn.execute(f"CREATE DATABASE {quote_ident(db_name)}")
    finally:
        await conn.close()


async def drop_database(db_name: str):
    conn = await asyncpg.connect(admin_database_dsn())
    try:
        await conn.execute(
            """
            SELECT pg_terminate_backend(pid)
            FROM pg_stat_activity
            WHERE datname = $1 AND pid <> pg_backend_pid()
            """,
            db_name,
        )
        await conn.execute(f"DROP DATABASE IF EXISTS {quote_ident(db_name)}")
    finally:
        await conn.close()


async def reset_public_schema():
    async with db.get_conn() as conn:
        await conn.execute("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
        await conn.execute("GRANT ALL ON SCHEMA public TO public;")
        await conn.execute("GRANT ALL ON SCHEMA public TO postgres;")


def admin_database_dsn() -> str:
    return os.getenv(
        "ADMIN_DATABASE_DSN",
        "postgresql://postgres:12345678@localhost:5432/postgres?sslmode=disable",
    )


def quote_ident(name: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9_]+", name):
        raise ValueError(f"unsafe database name: {name!r}")
    return '"' + name.replace('"', '""') + '"'


async def seed_load_test_data() -> LoadSeed:
    seed_user = None
    seed_admin = None
    seed_ygg_user = None
    seed_profile_id = ""
    seed_profile_name = ""
    seed_texture_hash = ""

    for i in range(100):
        email = f"load-user-{i:03d}@example.com"
        user = User(
            generate_random_uuid(),
            email,
            hash_password("Password123"),
            i == 0,
            "zh_CN",
            f"LoadUser{i:03d}",
        )
        await db.user.create(user)
        if i == 0:
            seed_admin = user
        if i == 1:
            seed_user = user
        if i == 2:
            seed_ygg_user = user

        for p in range(3):
            profile = PlayerProfile(generate_random_uuid(), user.id, f"LoadProfile{i:03d}_{p}")
            await db.user.create_profile(profile)
            if i == 2 and p == 0:
                seed_profile_id = profile.id
                seed_profile_name = profile.name

        for n in range(5):
            texture_hash = f"load_texture_{i:03d}_{n:03d}"
            texture_type = "cape" if n % 2 else "skin"
            model = "slim" if n % 3 == 0 else "default"
            await db.texture.add_to_library(
                user.id,
                texture_hash,
                texture_type,
                note=f"Load Texture {i}-{n}",
                is_public=n % 4 != 0,
                model=model,
            )
            if i == 1 and n == 0:
                seed_texture_hash = texture_hash

    if not seed_user or not seed_admin or not seed_ygg_user or not seed_profile_id:
        raise RuntimeError("load-test seed was not initialized")

    access_token = "load_ygg_access_token"
    client_token = "load_ygg_client_token"
    server_id = "load_ygg_server"
    now = now_ms()
    await db.user.add_token(Token(access_token, client_token, seed_ygg_user.id, seed_profile_id, now))
    await refresh_ygg_load_session(access_token, server_id)
    for i in range(50):
        await db.user.create_invite(InviteCode(f"LOAD_INVITE_{i:03d}", now_ms(), total_uses=10, note="Load invite"))

    return LoadSeed(
        user=seed_user,
        admin=seed_admin,
        ygg_user=seed_ygg_user,
        profile_id=seed_profile_id,
        profile_name=seed_profile_name,
        texture_hash=seed_texture_hash,
        ygg_access_token=access_token,
        ygg_client_token=client_token,
        ygg_server_id=server_id,
    )


async def refresh_ygg_load_session(access_token: str, server_id: str):
    await db.user.delete_session(server_id)
    await db.user.add_session(Session(server_id, access_token, "127.0.0.1", now_ms()))


def default_load_scenarios(seed: LoadSeed, user_cookie: str, admin_cookie: str) -> list[LoadScenario]:
    return [
        LoadScenario("Public home", "public-settings", "GET", "/public/settings"),
        LoadScenario("Public home", "public-carousel", "GET", "/public/carousel"),
        LoadScenario("Public library", "public-library-search", "GET", "/public/skin-library?limit=20&q=Load"),
        LoadScenario("Authentication", "site-login", "POST", "/site-login", f'{{"email":"{seed.user.email}","password":"Password123"}}'),
        LoadScenario("Yggdrasil", "ygg-metadata", "GET", "/"),
        LoadScenario("Yggdrasil", "ygg-authenticate", "POST", "/authserver/authenticate", f'{{"username":"{seed.user.email}","password":"Password123","requestUser":true}}'),
        LoadScenario("Yggdrasil", "ygg-validate", "POST", "/authserver/validate", f'{{"accessToken":"{seed.ygg_access_token}","clientToken":"{seed.ygg_client_token}"}}'),
        LoadScenario("Yggdrasil", "ygg-profile", "GET", "/sessionserver/session/minecraft/profile/" + seed.profile_id),
        LoadScenario("Yggdrasil", "ygg-lookup-name", "GET", "/api/users/profiles/minecraft/" + seed.profile_name),
        LoadScenario(
            "Yggdrasil",
            "ygg-has-joined",
            "GET",
            "/sessionserver/session/minecraft/hasJoined?username=" + seed.profile_name + "&serverId=" + seed.ygg_server_id,
            prepare=lambda: refresh_ygg_load_session(seed.ygg_access_token, seed.ygg_server_id),
        ),
        LoadScenario("User center", "me", "GET", "/me", cookie=user_cookie),
        LoadScenario("User center", "my-profiles", "GET", "/me/profiles?limit=20", cookie=user_cookie),
        LoadScenario("User center", "my-textures", "GET", "/me/textures?limit=20", cookie=user_cookie),
        LoadScenario("User center", "texture-detail", "GET", "/me/textures/" + seed.texture_hash + "/skin", cookie=user_cookie),
        LoadScenario("Admin console", "admin-users", "GET", "/admin/users?limit=20&q=Load", cookie=admin_cookie),
        LoadScenario("Admin console", "admin-user-detail", "GET", "/admin/users/" + seed.user.id, cookie=admin_cookie),
        LoadScenario("Admin console", "admin-user-profiles", "GET", "/admin/users/" + seed.user.id + "/profiles?limit=20", cookie=admin_cookie),
        LoadScenario("Admin console", "admin-profiles", "GET", "/admin/profiles?limit=20", cookie=admin_cookie),
        LoadScenario("Admin console", "admin-textures", "GET", "/admin/textures?limit=20", cookie=admin_cookie),
        LoadScenario("Admin console", "admin-invites", "GET", "/admin/invites?limit=20", cookie=admin_cookie),
        LoadScenario("Admin console", "admin-settings-site", "GET", "/admin/settings/site", cookie=admin_cookie),
    ]


def filter_scenarios(scenarios: list[LoadScenario], raw: str) -> list[LoadScenario]:
    if not raw.strip():
        return scenarios
    allowed = {part.strip() for part in raw.split(",") if part.strip()}
    return [scenario for scenario in scenarios if scenario.name in allowed]


async def login(base_url: str, email: str, password: str) -> str:
    async with aiohttp.ClientSession(cookie_jar=aiohttp.CookieJar(unsafe=True)) as session:
        async with session.post(
            base_url + "/site-login",
            json={"email": email, "password": password},
            timeout=aiohttp.ClientTimeout(total=5),
        ) as response:
            await response.read()
            if response.status < 200 or response.status >= 300:
                raise RuntimeError(f"login returned status {response.status}")
            cookies = response.cookies
            if not cookies:
                raise RuntimeError("login succeeded without cookies")
            return "; ".join(f"{key}={morsel.value}" for key, morsel in cookies.items())


async def run_step(base_url: str, scenario: LoadScenario, concurrency: int, duration: float) -> StepSummary:
    connector = aiohttp.TCPConnector(limit=concurrency * 4, limit_per_host=concurrency)
    timeout = aiohttp.ClientTimeout(total=5)
    async with aiohttp.ClientSession(connector=connector, timeout=timeout, cookie_jar=aiohttp.DummyCookieJar()) as session:
        end_at = time.perf_counter() + duration
        results: list[tuple[int, float, str]] = []
        lock = asyncio.Lock()

        async def worker():
            local_results = []
            while time.perf_counter() < end_at:
                local_results.append(await do_request(session, base_url, scenario))
            async with lock:
                results.extend(local_results)

        started = time.perf_counter()
        await asyncio.gather(*(worker() for _ in range(concurrency)))
        wall = time.perf_counter() - started
    return summarize(concurrency, results, wall)


async def do_request(session: aiohttp.ClientSession, base_url: str, scenario: LoadScenario) -> tuple[int, float, str]:
    headers = {}
    data = None
    if scenario.body:
        headers["Content-Type"] = "application/json"
        data = scenario.body
    if scenario.cookie:
        headers["Cookie"] = scenario.cookie

    started = time.perf_counter()
    try:
        async with session.request(scenario.method, base_url + scenario.path, data=data, headers=headers) as response:
            body = await response.content.read(512)
            response.release()
            latency = time.perf_counter() - started
            detail = ""
            if response.status < 200 or response.status >= 400:
                detail = body.decode("utf-8", errors="replace").strip()
            return response.status, latency, detail
    except Exception as exc:
        return 0, time.perf_counter() - started, str(exc)


def summarize(concurrency: int, results: list[tuple[int, float, str]], wall: float) -> StepSummary:
    statuses: dict[int, int] = {}
    latencies = []
    success = 0
    first_error = ""
    for status, latency, detail in results:
        latencies.append(latency)
        if status:
            statuses[status] = statuses.get(status, 0) + 1
        if 200 <= status < 400:
            success += 1
        elif not first_error:
            first_error = detail or (f"status {status}" if status else "transport error")

    latencies.sort()
    total = len(results)
    failed = total - success
    return StepSummary(
        concurrency=concurrency,
        total=total,
        success=success,
        failed=failed,
        failure_pct=(failed * 100 / total) if total else 0,
        rps=(total / wall) if wall else 0,
        success_rps=(success / wall) if wall else 0,
        avg=(sum(latencies) / total) if total else 0,
        p50=percentile(latencies, 50),
        p95=percentile(latencies, 95),
        p99=percentile(latencies, 99),
        statuses=statuses,
        wall=wall,
        first_error=first_error,
    )


def percentile(sorted_values: list[float], pct: float) -> float:
    if not sorted_values:
        return 0
    idx = math.ceil((pct / 100) * len(sorted_values)) - 1
    idx = min(max(idx, 0), len(sorted_values) - 1)
    return sorted_values[idx]


def load_test_config() -> LoadConfig:
    return LoadConfig(duration=load_test_duration(), max_db_connections=load_test_max_db_connections())


def load_test_concurrency() -> int:
    raw = os.getenv("LOADTEST_CONCURRENCY", "200").strip()
    try:
        value = int(raw)
    except ValueError as exc:
        raise ValueError(f"invalid LOADTEST_CONCURRENCY {raw!r}") from exc
    if value <= 0:
        raise ValueError(f"invalid LOADTEST_CONCURRENCY {raw!r}")
    return value


def load_test_duration() -> float:
    raw = os.getenv("LOADTEST_DURATION", "1s").strip()
    match = re.fullmatch(r"(\d+(?:\.\d+)?)(ms|s|m)?", raw)
    if not match:
        return 1.0
    value = float(match.group(1))
    unit = match.group(2) or "s"
    if unit == "ms":
        return value / 1000
    if unit == "m":
        return value * 60
    return value


def load_test_max_db_connections() -> int:
    raw = os.getenv("LOADTEST_DB_MAX_CONNECTIONS", "20").strip()
    try:
        value = int(raw)
    except ValueError:
        return 20
    return value if value > 0 else 20


def report_path() -> Path:
    raw = os.getenv("LOADTEST_REPORT")
    if raw:
        return Path(raw)
    return Path(__file__).resolve().parents[3] / "reports" / "python-concurrency-load-test.md"


async def write_load_test_report(path: Path, cfg: LoadConfig, concurrency: int, results: list[ScenarioResult]):
    path.parent.mkdir(parents=True, exist_ok=True)
    now = time.strftime("%Y-%m-%dT%H:%M:%S%z")
    lines = [
        "# Python Backend Concurrency Load Test Report",
        "",
        f"- Generated at: `{now}`",
        "- Harness: `LOADTEST_ENABLE=1 pytest tests/loadtest/test_backend_load.py -q -s`",
        "- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites, 1 pre-joined Yggdrasil session",
        f"- Fixed concurrency: `{concurrency}`",
        f"- Duration per level: `{format_seconds(cfg.duration)}`",
        f"- Backend database pool used by harness: `{cfg.max_db_connections}` max connections",
        "- Test database: isolated `elementskin_py_test_*`, dropped by test cleanup",
        "- HTTP server: real local Uvicorn server, closed by test cleanup",
        "- Auth rate limiting: disabled for load-test login scenario to measure login throughput instead of 429 policy",
        "",
        "## Scenario Coverage",
        "",
        "| Area | Scenario | Method | Path |",
        "| --- | --- | --- | --- |",
    ]
    seen = set()
    for result in results:
        scenario = result.scenario
        if scenario.name in seen:
            continue
        seen.add(scenario.name)
        lines.append(f"| {scenario.area} | `{scenario.name}` | `{scenario.method}` | `{scenario.path}` |")

    lines.extend(
        [
            "",
            f"## Fixed-{concurrency} One-Second Results",
            "",
            "| Area | Scenario | Concurrency | Requests | OK | Fail | Fail % | Successful req/s | Total req/s | Avg | P50 | P95 | P99 | Status | First Error |",
            "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |",
        ]
    )
    for result in results:
        summary = result.summary
        lines.append(
            f"| {result.scenario.area} | `{result.scenario.name}` | {result.concurrency} | "
            f"{summary.total} | {summary.success} | {summary.failed} | {summary.failure_pct:.2f} | "
            f"{summary.success_rps:.1f} | {summary.rps:.1f} | {format_duration(summary.avg)} | "
            f"{format_duration(summary.p50)} | {format_duration(summary.p95)} | {format_duration(summary.p99)} | "
            f"`{format_statuses(summary.statuses)}` | `{escape_table(summary.first_error)}` |"
        )

    lines.extend(
        [
            "",
            "## Notes",
            "",
            "- Every scenario is measured once at the same fixed concurrency, default `200`, for a one-second window.",
            "- `Successful req/s` is the useful per-second throughput under that fixed concurrency.",
            "- This report covers public, site, admin, and common Yggdrasil client endpoints; destructive write endpoints are intentionally excluded from high-concurrency runs.",
            "- A failure is any request with a transport error or non-2xx/3xx response.",
            "- The test harness closes the local HTTP server, closes the database pool, terminates leftover database sessions, and drops the temporary PostgreSQL database during cleanup.",
        ]
    )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def now_ms() -> int:
    return int(time.time() * 1000)


def format_duration(seconds: float) -> str:
    if not seconds:
        return "-"
    if seconds < 1:
        return f"{seconds * 1000:.1f}ms"
    return f"{seconds:.2f}s"


def format_seconds(seconds: float) -> str:
    if seconds == int(seconds):
        return f"{int(seconds)}s"
    return f"{seconds}s"


def format_statuses(statuses: dict[int, int]) -> str:
    if not statuses:
        return "errors-only"
    return ",".join(f"{status}:{statuses[status]}" for status in sorted(statuses))


def escape_table(value: str) -> str:
    return value.replace("\n", " ").replace("|", "\\|")
