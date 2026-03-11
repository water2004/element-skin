import sqlite3
import psycopg2
import os

# ================= 配置区 =================
# 请在此处手动配置你的数据库连接信息
SQLITE_DB = "yggdrasil.db"
POSTGRES_DSN = "postgresql://postgres:12345678@localhost:5432/elementskin?sslmode=disable"
# ==========================================

TABLES = [
    "users",
    "profiles",
    "tokens",
    "sessions",
    "invites",
    "settings",
    "user_textures",
    "skin_library",
    "fallback_endpoints",
    "whitelisted_users",
    "verification_codes"
]

def migrate():
    if not os.path.exists(SQLITE_DB):
        print(f"❌ 错误: 找不到 SQLite 数据库文件 '{SQLITE_DB}'")
        return

    sl_conn = None
    pg_conn = None
    try:
        # 1. 连接数据库
        sl_conn = sqlite3.connect(SQLITE_DB)
        sl_cur = sl_conn.cursor()
        
        pg_conn = psycopg2.connect(POSTGRES_DSN)
        pg_cur = pg_conn.cursor()
        
        print("--- 开始数据迁移 (SQLite -> PostgreSQL) ---")
        
        # 2. 清理已有数据 (TRUNCATE)
        # 使用 CASCADE 确保可以清理被外键引用的表
        print("正在清空 PostgreSQL 中的旧数据...")
        for table in reversed(TABLES):
            pg_cur.execute(f"TRUNCATE TABLE {table} CASCADE")
        
        # 3. 逐表迁移数据
        for table in TABLES:
            print(f"正在同步表: {table}")
            
            # 获取 SQLite 数据
            sl_cur.execute(f"SELECT * FROM {table}")
            rows = sl_cur.fetchall()
            
            # 获取列名（确保 INSERT 语句列顺序正确）
            sl_cur.execute(f"PRAGMA table_info({table})")
            cols = [col[1] for col in sl_cur.fetchall()]
            
            if not rows:
                print(f"  - 表 {table} 为空，跳过。")
                continue
            
            # --- 类型转换处理 ---
            # 定义需要从 0/1 转换为 True/False 的布尔字段名
            bool_cols = {"is_admin", "enable_profile", "enable_hasjoined", "enable_whitelist"}
            
            processed_rows = []
            for row in rows:
                new_row = list(row)
                for i, col_name in enumerate(cols):
                    if col_name in bool_cols:
                        if new_row[i] is not None:
                            new_row[i] = bool(new_row[i]) # 0 -> False, 1 -> True
                processed_rows.append(tuple(new_row))
            # --------------------

            cols_str = ", ".join([f'"{c}"' for c in cols])
            placeholders = ", ".join(["%s"] * len(cols))
            
            insert_query = f"INSERT INTO {table} ({cols_str}) VALUES ({placeholders})"
            pg_cur.executemany(insert_query, processed_rows)
            print(f"  ✅ 成功迁移 {len(rows)} 条记录。")

            # 4. 重置自增序列 (仅针对有 ID 序列的表)
            if "id" in cols and table in ["fallback_endpoints", "whitelisted_users"]:
                pg_cur.execute(f"SELECT setval(pg_get_serial_sequence('{table}', 'id'), coalesce(max(id), 1)) FROM {table}")

        pg_conn.commit()
        print("\n✨ 数据迁移成功！")
        
    except Exception as e:
        print(f"\n❌ 迁移失败: {e}")
        if pg_conn:
            pg_conn.rollback()
    finally:
        if sl_conn:
            sl_conn.close()
        if pg_conn:
            pg_conn.close()

if __name__ == "__main__":
    migrate()
