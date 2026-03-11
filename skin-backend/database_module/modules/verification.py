from ..core import BaseDB
import time

class VerificationModule:
    def __init__(self, db: BaseDB):
        self.db = db

    async def create_code(self, email: str, code: str, type: str, ttl: int):
        created_at = int(time.time() * 1000)
        expires_at = created_at + (ttl * 1000)
        # Upsert code
        await self.db.execute(
            "INSERT INTO verification_codes (email, code, type, created_at, expires_at) VALUES ($1, $2, $3, $4, $5) "
            "ON CONFLICT (email, type) DO UPDATE SET code = EXCLUDED.code, created_at = EXCLUDED.created_at, expires_at = EXCLUDED.expires_at",
            email, code, type, created_at, expires_at,
        )

    async def get_code(self, email: str, type: str):
        row = await self.db.fetchrow(
            "SELECT code, expires_at FROM verification_codes WHERE email=$1 AND type=$2",
            email, type,
        )
        return row

    async def delete_code(self, email: str, type: str):
        await self.db.execute(
            "DELETE FROM verification_codes WHERE email=$1 AND type=$2",
            email, type,
        )
