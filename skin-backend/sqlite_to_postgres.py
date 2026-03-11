import sqlite3
import psycopg2
import sys

# 配置
SQLITE_DB = "yggdrasil.db"
POSTGRES_DSN = "postgresql://yggdrasil:password@localhost:5432/yggdrasil"

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
    try:
        sl_conn = sqlite3.connect(SQLITE_DB)
        sl_cur = sl_conn.cursor()
        
        pg_conn = psycopg2.connect(POSTGRES_DSN)
        pg_cur = pg_conn.cursor()
        
        print("Starting migration...")
        
        for table in TABLES:
            print(f"Migrating table: {table}")
            
            # 获取数据
            sl_cur.execute(f"SELECT * FROM {table}")
            rows = sl_cur.fetchall()
            
            if not rows:
                print(f"Table {table} is empty, skipping.")
                continue
                
            # 获取列名
            sl_cur.execute(f"PRAGMA table_info({table})")
            cols = [col[1] for col in sl_cur.fetchall()]
            cols_str = ", ".join(cols)
            placeholders = ", ".join(["%s"] * len(cols))
            
            # 清理目标表并插入
            pg_cur.execute(f"TRUNCATE TABLE {table} CASCADE")
            insert_query = f"INSERT INTO {table} ({cols_str}) VALUES ({placeholders})"
            
            pg_cur.executemany(insert_query, rows)
            print(f"Successfully migrated {len(rows)} rows for {table}.")

        pg_conn.commit()
        print("Migration finished successfully!")
        
    except Exception as e:
        print(f"Error during migration: {e}")
        if 'pg_conn' in locals():
            pg_conn.rollback()
    finally:
        if 'sl_conn' in locals():
            sl_conn.close()
        if 'pg_conn' in locals():
            pg_conn.close()

if __name__ == "__main__":
    migrate()
