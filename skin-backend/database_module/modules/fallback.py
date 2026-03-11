from ..core import BaseDB
import time

class FallbackModule:
    def __init__(self, db: BaseDB):
        self.db = db
        self._endpoints_cache = []
        self._domains_cache = []
        self._whitelist_cache = {} # {endpoint_id: set(usernames)}

    async def init(self):
        """Initialize all fallback related caches"""
        await self.refresh_endpoints_cache()
        await self.refresh_whitelist_cache()

    async def refresh_endpoints_cache(self):
        self._endpoints_cache = await self._list_endpoints_from_db()
        # Pre-parse domains
        domains = []
        for ep in self._endpoints_cache:
            raw = ep.get("skin_domains")
            if raw:
                parts = [part.strip() for part in raw.split(",") if part.strip()]
                domains.extend(parts)
        self._domains_cache = list(set(domains))

    async def refresh_whitelist_cache(self):
        rows = await self.db.fetch("SELECT username, endpoint_id FROM whitelisted_users")
        new_cache = {}
        for username, ep_id in rows:
            if ep_id not in new_cache:
                new_cache[ep_id] = set()
            new_cache[ep_id].add(username.lower())
        self._whitelist_cache = new_cache

    async def list_endpoints(self) -> list[dict]:
        return self._endpoints_cache

    async def _list_endpoints_from_db(self) -> list[dict]:
        rows = await self.db.fetch(
            """
            SELECT id, priority, session_url, account_url, services_url, cache_ttl, skin_domains,
                   enable_profile, enable_hasjoined, enable_whitelist, note
            FROM fallback_endpoints
            ORDER BY priority ASC, id ASC
            """
        )
        return [
            {
                "id": r[0],
                "priority": r[1],
                "session_url": r[2],
                "account_url": r[3],
                "services_url": r[4],
                "cache_ttl": r[5],
                "skin_domains": r[6],
                "enable_profile": bool(r[7]),
                "enable_hasjoined": bool(r[8]),
                "enable_whitelist": bool(r[9]),
                "note": r[10],
            }
            for r in rows
        ]

    async def get_primary_endpoint(self) -> dict | None:
        return self._endpoints_cache[0] if self._endpoints_cache else None

    async def save_endpoints(self, fallbacks: list[dict]):
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                # 获取现有 ID
                existing_rows = await conn.fetch("SELECT id FROM fallback_endpoints")
                existing_ids = {row[0] for row in existing_rows}

                incoming_ids = {
                    entry["id"] for entry in fallbacks if entry.get("id") is not None
                }
                
                # 删除不在传入列表中的端点
                for endpoint_id in existing_ids - incoming_ids:
                    await conn.execute(
                        "DELETE FROM fallback_endpoints WHERE id=$1", endpoint_id
                    )

                for idx, entry in enumerate(fallbacks, start=1):
                    priority = idx
                    session_url = entry["session_url"]
                    account_url = entry["account_url"]
                    services_url = entry["services_url"]
                    cache_ttl = entry["cache_ttl"]
                    skin_domains = entry.get("skin_domains", "")
                    enable_profile = bool(entry.get("enable_profile"))
                    enable_hasjoined = bool(entry.get("enable_hasjoined"))
                    enable_whitelist = bool(entry.get("enable_whitelist"))
                    note = entry.get("note", "")

                    if entry.get("id") is not None:
                        await conn.execute(
                            """
                            UPDATE fallback_endpoints
                            SET priority=$1, session_url=$2, account_url=$3, services_url=$4, cache_ttl=$5, skin_domains=$6,
                                enable_profile=$7, enable_hasjoined=$8, enable_whitelist=$9, note=$10
                            WHERE id=$11
                            """,
                            priority, session_url, account_url, services_url, cache_ttl, skin_domains,
                            enable_profile, enable_hasjoined, enable_whitelist, note, entry["id"],
                        )
                    else:
                        await conn.execute(
                            """
                            INSERT INTO fallback_endpoints (
                                priority, session_url, account_url, services_url, cache_ttl, skin_domains,
                                enable_profile, enable_hasjoined, enable_whitelist, note
                            )
                            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
                            """,
                            priority, session_url, account_url, services_url, cache_ttl, skin_domains,
                            enable_profile, enable_hasjoined, enable_whitelist, note,
                        )
        await self.refresh_endpoints_cache()
            
    async def collect_skin_domains(self) -> list[str]:
        return self._domains_cache
            
    # ========== Fallback Whitelist ==========

    async def add_whitelist_user(self, username: str, endpoint_id: int):
        created_at = int(time.time() * 1000)
        await self.db.execute(
            """
            INSERT INTO whitelisted_users (username, endpoint_id, created_at)
            VALUES ($1, $2, $3) ON CONFLICT DO NOTHING
            """,
            username, endpoint_id, created_at,
        )
        
        # Update cache
        if endpoint_id not in self._whitelist_cache:
            self._whitelist_cache[endpoint_id] = set()
        self._whitelist_cache[endpoint_id].add(username.lower())

    async def remove_whitelist_user(
        self, username: str, endpoint_id: int
    ):
        await self.db.execute(
            "DELETE FROM whitelisted_users WHERE username=$1 AND endpoint_id=$2",
            username, endpoint_id,
        )
        
        # Update cache
        if endpoint_id in self._whitelist_cache:
            self._whitelist_cache[endpoint_id].discard(username.lower())

    async def is_user_in_whitelist(
        self, username: str, endpoint_id: int
    ) -> bool:
        """High-performance cache check"""
        if endpoint_id not in self._whitelist_cache:
            return False
        return username.lower() in self._whitelist_cache[endpoint_id]

    async def list_whitelist_users(
        self, endpoint_id: int
    ) -> list[dict]:
        """Keep DB query for list method to get timestamps, but usually used in Admin UI only"""
        rows = await self.db.fetch(
            "SELECT username, created_at FROM whitelisted_users WHERE endpoint_id=$1 ORDER BY created_at DESC",
            endpoint_id,
        )
        return [{"username": r[0], "created_at": r[1]} for r in rows]
