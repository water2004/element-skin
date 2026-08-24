#!/usr/bin/env python3
"""Repair texture filenames and PostgreSQL references using the Yggdrasil pixel hash."""

from __future__ import annotations

import argparse
import hashlib
import os
import re
import struct
import sys
import uuid
from dataclasses import dataclass, field
from io import BytesIO
from pathlib import Path
from typing import Any, Sequence

from PIL import Image


HASH_PATTERN = re.compile(r"^[0-9a-fA-F]{64}$")
STAGING_PREFIX = ".texture-hash-repair-"


class RepairError(RuntimeError):
    """A repair precondition or operation failed."""


@dataclass(frozen=True)
class TextureFile:
    path: Path
    claimed_hash: str
    correct_hash: str
    raw_sha256: str
    size: int
    mtime_ns: int

    @property
    def canonical_path(self) -> Path:
        return self.path.parent / f"{self.correct_hash}.png"

    @property
    def needs_repair(self) -> bool:
        return self.path.name != f"{self.correct_hash}.png"


@dataclass(frozen=True)
class HashMapping:
    old_hash: str
    new_hash: str
    staged_hash: str


@dataclass
class StagedMove:
    texture: TextureFile
    staged_path: Path
    promoted: bool = False


@dataclass
class StagedFiles:
    moves: list[StagedMove] = field(default_factory=list)

    def rollback(self) -> None:
        errors: list[str] = []
        for move in reversed(self.moves):
            try:
                if move.promoted:
                    if not move.texture.canonical_path.exists():
                        raise RepairError(
                            f"cannot restore {move.texture.path}: canonical file disappeared"
                        )
                    os.replace(move.texture.canonical_path, move.staged_path)
                    move.promoted = False
            except Exception as exc:
                errors.append(str(exc))
        for move in reversed(self.moves):
            try:
                if move.staged_path.exists():
                    if move.texture.path.exists():
                        raise RepairError(
                            f"cannot restore {move.texture.path}: original path is occupied"
                        )
                    os.replace(move.staged_path, move.texture.path)
            except Exception as exc:
                errors.append(str(exc))
        if errors:
            raise RepairError("file rollback failed: " + "; ".join(errors))

    def finalize(self) -> int:
        removed = 0
        errors: list[str] = []
        for move in self.moves:
            if not move.staged_path.exists():
                continue
            try:
                move.staged_path.unlink()
                removed += 1
            except OSError as exc:
                errors.append(f"{move.staged_path}: {exc}")
        if errors:
            raise RepairError(
                "database repair committed, but duplicate staging files could not be removed: "
                + "; ".join(errors)
            )
        return removed


def yggdrasil_texture_hash(image_bytes: bytes) -> str:
    try:
        with Image.open(BytesIO(image_bytes)) as source:
            if source.format != "PNG":
                raise RepairError("file is not a PNG image")
            source.load()
            image = source.convert("RGBA")
    except RepairError:
        raise
    except Exception as exc:
        raise RepairError(f"failed to decode PNG: {exc}") from exc

    width, height = image.size
    buffer = bytearray(width * height * 4 + 8)
    struct.pack_into(">I", buffer, 0, width)
    struct.pack_into(">I", buffer, 4, height)
    pixels = image.load()
    position = 8
    for x in range(width):
        for y in range(height):
            red, green, blue, alpha = pixels[x, y]
            if alpha == 0:
                red = green = blue = 0
            buffer[position : position + 4] = bytes((alpha, red, green, blue))
            position += 4
    return hashlib.sha256(buffer).hexdigest()


def scan_texture_directory(textures_dir: Path) -> list[TextureFile]:
    directory = textures_dir.expanduser().resolve()
    if not directory.is_dir():
        raise RepairError(f"texture directory does not exist: {directory}")
    leftovers = sorted(directory.glob(f"{STAGING_PREFIX}*"))
    if leftovers:
        names = ", ".join(path.name for path in leftovers)
        raise RepairError(f"unfinished repair staging files exist: {names}")

    textures: list[TextureFile] = []
    claimed_paths: dict[str, Path] = {}
    for path in sorted(directory.iterdir(), key=lambda item: item.name):
        if not path.is_file() or path.suffix.lower() != ".png":
            continue
        if not HASH_PATTERN.fullmatch(path.stem):
            raise RepairError(
                f"texture filename is not a 64-character hexadecimal hash: {path.name}"
            )
        claimed_hash = path.stem.lower()
        if claimed_hash in claimed_paths:
            raise RepairError(
                f"duplicate texture filename after case normalization: "
                f"{claimed_paths[claimed_hash].name}, {path.name}"
            )
        claimed_paths[claimed_hash] = path

        before = path.stat()
        image_bytes = path.read_bytes()
        after = path.stat()
        if before.st_size != after.st_size or before.st_mtime_ns != after.st_mtime_ns:
            raise RepairError(f"texture changed while it was being scanned: {path}")
        try:
            correct_hash = yggdrasil_texture_hash(image_bytes)
        except RepairError as exc:
            raise RepairError(f"{path}: {exc}") from exc
        textures.append(
            TextureFile(
                path=path,
                claimed_hash=claimed_hash,
                correct_hash=correct_hash,
                raw_sha256=hashlib.sha256(image_bytes).hexdigest(),
                size=after.st_size,
                mtime_ns=after.st_mtime_ns,
            )
        )
    return textures


def verify_scanned_files(textures: Sequence[TextureFile]) -> None:
    for texture in textures:
        try:
            stat = texture.path.stat()
            raw_sha256 = hashlib.sha256(texture.path.read_bytes()).hexdigest()
        except OSError as exc:
            raise RepairError(f"cannot re-read {texture.path}: {exc}") from exc
        if (
            stat.st_size != texture.size
            or stat.st_mtime_ns != texture.mtime_ns
            or raw_sha256 != texture.raw_sha256
        ):
            raise RepairError(f"texture changed after scanning: {texture.path}")


def build_hash_mappings(
    textures: Sequence[TextureFile], run_id: str
) -> list[HashMapping]:
    return [
        HashMapping(
            old_hash=texture.claimed_hash,
            new_hash=texture.correct_hash,
            staged_hash=f"{STAGING_PREFIX}{run_id}-{texture.claimed_hash}",
        )
        for texture in textures
        if texture.claimed_hash != texture.correct_hash
    ]


def stage_texture_files(textures: Sequence[TextureFile], run_id: str) -> StagedFiles:
    state = StagedFiles()
    try:
        repair_files = [texture for texture in textures if texture.needs_repair]
        for index, texture in enumerate(repair_files):
            staged_path = (
                texture.path.parent / f"{STAGING_PREFIX}{run_id}-{index:08d}.png"
            )
            if staged_path.exists():
                raise RepairError(f"staging path already exists: {staged_path}")
            os.replace(texture.path, staged_path)
            state.moves.append(StagedMove(texture=texture, staged_path=staged_path))

        by_correct_hash: dict[str, list[StagedMove]] = {}
        for move in state.moves:
            by_correct_hash.setdefault(move.texture.correct_hash, []).append(move)
        for correct_hash, moves in sorted(by_correct_hash.items()):
            canonical_path = moves[0].texture.path.parent / f"{correct_hash}.png"
            if canonical_path.exists():
                continue
            chosen = min(moves, key=lambda move: move.texture.path.name)
            os.replace(chosen.staged_path, canonical_path)
            chosen.promoted = True
    except Exception:
        state.rollback()
        raise
    return state


def _cursor_scalar(cursor: Any, query: str) -> int:
    cursor.execute(query)
    row = cursor.fetchone()
    if row is None:
        raise RepairError("database query returned no result")
    return int(row[0])


def prepare_database(
    connection: Any,
    mappings: Sequence[HashMapping],
    lock_timeout_seconds: int,
) -> None:
    with connection.cursor() as cursor:
        cursor.execute(
            "SELECT set_config('lock_timeout', %s, true)",
            (f"{lock_timeout_seconds}s",),
        )
        cursor.execute(
            "LOCK TABLE users, profiles, user_textures, skin_library "
            "IN ACCESS EXCLUSIVE MODE"
        )
        cursor.execute(
            """
            CREATE TEMP TABLE texture_hash_repair_map (
                old_hash TEXT PRIMARY KEY,
                new_hash TEXT NOT NULL,
                staged_hash TEXT NOT NULL UNIQUE
            ) ON COMMIT DROP
            """
        )
        cursor.executemany(
            """
            INSERT INTO texture_hash_repair_map (old_hash, new_hash, staged_hash)
            VALUES (%s, %s, %s)
            """,
            [(item.old_hash, item.new_hash, item.staged_hash) for item in mappings],
        )


def analyze_database(connection: Any) -> dict[str, int]:
    with connection.cursor() as cursor:
        stats = {
            "user_avatars": _cursor_scalar(
                cursor,
                """
                SELECT COUNT(*) FROM users AS u
                JOIN texture_hash_repair_map AS m ON m.old_hash = u.avatar_hash
                """,
            ),
            "profile_skins": _cursor_scalar(
                cursor,
                """
                SELECT COUNT(*) FROM profiles AS p
                JOIN texture_hash_repair_map AS m ON m.old_hash = p.skin_hash
                """,
            ),
            "profile_capes": _cursor_scalar(
                cursor,
                """
                SELECT COUNT(*) FROM profiles AS p
                JOIN texture_hash_repair_map AS m ON m.old_hash = p.cape_hash
                """,
            ),
            "user_textures": _cursor_scalar(
                cursor,
                """
                SELECT COUNT(*) FROM user_textures AS ut
                JOIN texture_hash_repair_map AS m ON m.old_hash = ut.hash
                """,
            ),
            "library_textures": _cursor_scalar(
                cursor,
                """
                SELECT COUNT(*) FROM skin_library AS sl
                JOIN texture_hash_repair_map AS m ON m.old_hash = sl.skin_hash
                """,
            ),
        }
        stats["user_texture_conflicts"] = _cursor_scalar(
            cursor,
            """
            WITH candidates AS (
                SELECT
                    ut.user_id,
                    ut.texture_type,
                    m.old_hash,
                    m.new_hash,
                    EXISTS (
                        SELECT 1
                        FROM user_textures AS target
                        LEFT JOIN texture_hash_repair_map AS target_map
                          ON target_map.old_hash = target.hash
                        WHERE target.user_id = ut.user_id
                          AND target.hash = m.new_hash
                          AND target.texture_type = ut.texture_type
                          AND target_map.old_hash IS NULL
                    ) AS correct_exists,
                    ROW_NUMBER() OVER (
                        PARTITION BY ut.user_id, m.new_hash, ut.texture_type
                        ORDER BY ut.created_at ASC, m.old_hash ASC
                    ) AS candidate_rank
                FROM user_textures AS ut
                JOIN texture_hash_repair_map AS m ON m.old_hash = ut.hash
            )
            SELECT COUNT(*) FROM candidates
            WHERE correct_exists OR candidate_rank > 1
            """,
        )
        stats["library_conflicts"] = _cursor_scalar(
            cursor,
            """
            WITH candidates AS (
                SELECT
                    sl.texture_type,
                    m.old_hash,
                    m.new_hash,
                    EXISTS (
                        SELECT 1
                        FROM skin_library AS target
                        LEFT JOIN texture_hash_repair_map AS target_map
                          ON target_map.old_hash = target.skin_hash
                        WHERE target.skin_hash = m.new_hash
                          AND target.texture_type = sl.texture_type
                          AND target_map.old_hash IS NULL
                    ) AS correct_exists,
                    ROW_NUMBER() OVER (
                        PARTITION BY m.new_hash, sl.texture_type
                        ORDER BY sl.created_at ASC, m.old_hash ASC
                    ) AS candidate_rank
                FROM skin_library AS sl
                JOIN texture_hash_repair_map AS m ON m.old_hash = sl.skin_hash
            )
            SELECT COUNT(*) FROM candidates
            WHERE correct_exists OR candidate_rank > 1
            """,
        )
    return stats


def repair_database(connection: Any) -> dict[str, int]:
    stats: dict[str, int] = {}
    with connection.cursor() as cursor:
        cursor.execute(
            """
            UPDATE users AS u
            SET avatar_hash = m.new_hash
            FROM texture_hash_repair_map AS m
            WHERE u.avatar_hash = m.old_hash
            """
        )
        stats["user_avatars_updated"] = cursor.rowcount

        cursor.execute(
            """
            UPDATE profiles AS p
            SET skin_hash = m.new_hash
            FROM texture_hash_repair_map AS m
            WHERE p.skin_hash = m.old_hash
            """
        )
        stats["profile_skins_updated"] = cursor.rowcount
        cursor.execute(
            """
            UPDATE profiles AS p
            SET cape_hash = m.new_hash
            FROM texture_hash_repair_map AS m
            WHERE p.cape_hash = m.old_hash
            """
        )
        stats["profile_capes_updated"] = cursor.rowcount

        cursor.execute(
            """
            WITH candidates AS (
                SELECT
                    ut.ctid AS row_id,
                    m.old_hash,
                    m.new_hash,
                    EXISTS (
                        SELECT 1
                        FROM user_textures AS target
                        LEFT JOIN texture_hash_repair_map AS target_map
                          ON target_map.old_hash = target.hash
                        WHERE target.user_id = ut.user_id
                          AND target.hash = m.new_hash
                          AND target.texture_type = ut.texture_type
                          AND target_map.old_hash IS NULL
                    ) AS correct_exists,
                    ROW_NUMBER() OVER (
                        PARTITION BY ut.user_id, m.new_hash, ut.texture_type
                        ORDER BY ut.created_at ASC, m.old_hash ASC
                    ) AS candidate_rank
                FROM user_textures AS ut
                JOIN texture_hash_repair_map AS m ON m.old_hash = ut.hash
            )
            DELETE FROM user_textures AS ut
            USING candidates AS candidate
            WHERE ut.ctid = candidate.row_id
              AND (candidate.correct_exists OR candidate.candidate_rank > 1)
            """
        )
        stats["user_texture_conflicts_removed"] = cursor.rowcount

        cursor.execute(
            """
            WITH candidates AS (
                SELECT
                    sl.ctid AS row_id,
                    m.old_hash,
                    m.new_hash,
                    EXISTS (
                        SELECT 1
                        FROM skin_library AS target
                        LEFT JOIN texture_hash_repair_map AS target_map
                          ON target_map.old_hash = target.skin_hash
                        WHERE target.skin_hash = m.new_hash
                          AND target.texture_type = sl.texture_type
                          AND target_map.old_hash IS NULL
                    ) AS correct_exists,
                    ROW_NUMBER() OVER (
                        PARTITION BY m.new_hash, sl.texture_type
                        ORDER BY sl.created_at ASC, m.old_hash ASC
                    ) AS candidate_rank
                FROM skin_library AS sl
                JOIN texture_hash_repair_map AS m ON m.old_hash = sl.skin_hash
            )
            DELETE FROM skin_library AS sl
            USING candidates AS candidate
            WHERE sl.ctid = candidate.row_id
              AND (candidate.correct_exists OR candidate.candidate_rank > 1)
            """
        )
        stats["library_conflicts_removed"] = cursor.rowcount

        cursor.execute(
            """
            UPDATE user_textures AS ut
            SET hash = m.staged_hash
            FROM texture_hash_repair_map AS m
            WHERE ut.hash = m.old_hash
            """
        )
        stats["user_textures_staged"] = cursor.rowcount
        cursor.execute(
            """
            UPDATE skin_library AS sl
            SET skin_hash = m.staged_hash
            FROM texture_hash_repair_map AS m
            WHERE sl.skin_hash = m.old_hash
            """
        )
        stats["library_textures_staged"] = cursor.rowcount

        cursor.execute(
            """
            UPDATE user_textures AS ut
            SET hash = m.new_hash
            FROM texture_hash_repair_map AS m
            WHERE ut.hash = m.staged_hash
            """
        )
        stats["user_textures_updated"] = cursor.rowcount
        cursor.execute(
            """
            UPDATE skin_library AS sl
            SET skin_hash = m.new_hash
            FROM texture_hash_repair_map AS m
            WHERE sl.skin_hash = m.staged_hash
            """
        )
        stats["library_textures_updated"] = cursor.rowcount

        cursor.execute(
            """
            UPDATE skin_library AS sl
            SET usage_count = (
                SELECT COUNT(*)
                FROM user_textures AS ut
                WHERE ut.hash = sl.skin_hash
                  AND ut.texture_type = sl.texture_type
            )
            WHERE EXISTS (
                SELECT 1 FROM texture_hash_repair_map AS m
                WHERE m.new_hash = sl.skin_hash
            )
            """
        )
        stats["usage_counts_recalculated"] = cursor.rowcount

        staged_references = _cursor_scalar(
            cursor,
            """
            SELECT
                (SELECT COUNT(*) FROM user_textures AS ut
                 JOIN texture_hash_repair_map AS m ON m.staged_hash = ut.hash)
              + (SELECT COUNT(*) FROM skin_library AS sl
                 JOIN texture_hash_repair_map AS m ON m.staged_hash = sl.skin_hash)
            """,
        )
        if staged_references != 0:
            raise RepairError(
                f"database repair left {staged_references} staging hash references"
            )
    return stats


def load_postgres_driver() -> Any:
    try:
        import psycopg

        return psycopg
    except ImportError:
        try:
            import psycopg2

            return psycopg2
        except ImportError as exc:
            raise RepairError(
                "PostgreSQL driver is missing; install psycopg[binary] or psycopg2-binary"
            ) from exc


def print_scan(textures: Sequence[TextureFile]) -> None:
    mismatches = [texture for texture in textures if texture.needs_repair]
    print(f"Scanned {len(textures)} PNG texture files; {len(mismatches)} need repair.")
    for texture in mismatches:
        print(f"  {texture.path.name} -> {texture.correct_hash}.png")


def print_database_stats(prefix: str, stats: dict[str, int]) -> None:
    print(prefix)
    for key in sorted(stats):
        print(f"  {key}: {stats[key]}")


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Check and repair texture hashes using the Yggdrasil pixel-hash specification. "
            "Stop the backend and webhook worker before using --apply."
        )
    )
    parser.add_argument(
        "--textures-dir",
        type=Path,
        required=True,
        help="directory containing <texture-hash>.png files",
    )
    parser.add_argument(
        "--postgres-dsn",
        required=True,
        help="PostgreSQL DSN, for example postgresql://user:password@host:5432/database?sslmode=disable",
    )
    parser.add_argument(
        "--lock-timeout-seconds",
        type=int,
        default=5,
        help="seconds to wait for exclusive locks on affected tables (default: 5)",
    )
    parser.add_argument(
        "--apply",
        action="store_true",
        help="apply filesystem and database repairs; without this flag the script only checks",
    )
    args = parser.parse_args(argv)
    if args.lock_timeout_seconds <= 0:
        parser.error("--lock-timeout-seconds must be greater than zero")
    return args


def run(args: argparse.Namespace) -> int:
    textures = scan_texture_directory(args.textures_dir)
    print_scan(textures)
    repair_files = [texture for texture in textures if texture.needs_repair]
    if not repair_files:
        print("All texture filenames already match the Yggdrasil pixel hash.")
        return 0

    run_id = uuid.uuid4().hex
    mappings = build_hash_mappings(textures, run_id)
    connection = None
    if mappings:
        driver = load_postgres_driver()
        try:
            connection = driver.connect(args.postgres_dsn)
            prepare_database(connection, mappings, args.lock_timeout_seconds)
        except Exception as exc:
            if connection is not None:
                connection.rollback()
                connection.close()
            raise RepairError(
                f"failed to prepare PostgreSQL repair transaction: {exc}"
            ) from exc

    if not args.apply:
        try:
            if connection is not None:
                print_database_stats(
                    "Database references:", analyze_database(connection)
                )
        finally:
            if connection is not None:
                connection.rollback()
                connection.close()
        print(
            "Dry run only; use --apply after stopping the backend and webhook worker."
        )
        return 2

    staged_files = StagedFiles()
    try:
        verify_scanned_files(textures)
        staged_files = stage_texture_files(textures, run_id)
        database_stats: dict[str, int] = {}
        if connection is not None:
            database_stats = repair_database(connection)
            connection.commit()
    except Exception as exc:
        if connection is not None:
            connection.rollback()
        try:
            staged_files.rollback()
        except Exception as rollback_exc:
            raise RepairError(
                f"repair failed: {exc}; filesystem rollback also failed: {rollback_exc}"
            ) from rollback_exc
        raise RepairError(f"repair failed and was rolled back: {exc}") from exc
    finally:
        if connection is not None:
            connection.close()

    removed_duplicates = staged_files.finalize()

    if mappings:
        print_database_stats("Database changes:", database_stats)
    print(f"Repair complete; removed {removed_duplicates} duplicate texture files.")
    return 0


def main(argv: Sequence[str] | None = None) -> int:
    try:
        return run(parse_args(argv))
    except RepairError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
