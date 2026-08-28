use rusqlite::{params, Connection};
use std::path::Path;
use std::sync::Mutex;

#[derive(Debug, Clone, serde::Serialize)]
pub struct ObjectMeta {
    pub id: String,
    pub size: i64,
    pub content_type: String,
    pub etag: String,
    pub created_at: String,
    pub updated_at: String,
}

pub struct MetadataStore {
    conn: Mutex<Connection>,
}

impl MetadataStore {
    pub fn new(path: impl AsRef<Path>) -> rusqlite::Result<Self> {
        let conn = Connection::open(path)?;
        conn.execute_batch(
            "CREATE TABLE IF NOT EXISTS objects (
                id            TEXT PRIMARY KEY,
                size          INTEGER NOT NULL,
                content_type  TEXT NOT NULL DEFAULT 'application/octet-stream',
                etag          TEXT NOT NULL DEFAULT '',
                created_at    TEXT NOT NULL DEFAULT (datetime('now')),
                updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
            );",
        )?;

        // migrate old DBs that lack new columns
        let _ = conn.execute("ALTER TABLE objects ADD COLUMN content_type TEXT NOT NULL DEFAULT 'application/octet-stream'", []);
        let _ = conn.execute("ALTER TABLE objects ADD COLUMN etag TEXT NOT NULL DEFAULT ''", []);
        let _ = conn.execute("ALTER TABLE objects ADD COLUMN updated_at TEXT NOT NULL DEFAULT (datetime('now'))", []);

        Ok(Self {
            conn: Mutex::new(conn),
        })
    }

    pub fn insert(&self, id: &str, size: i64) -> rusqlite::Result<()> {
        let conn = self.conn.lock().expect("metadata db mutex poisoned");
        conn.execute(
            "INSERT OR REPLACE INTO objects (id, size) VALUES (?1, ?2)",
            params![id, size],
        )?;
        Ok(())
    }

    pub fn remove(&self, id: &str) -> rusqlite::Result<bool> {
        let conn = self.conn.lock().expect("metadata db mutex poisoned");
        let changed = conn.execute("DELETE FROM objects WHERE id = ?1", params![id])?;
        Ok(changed > 0)
    }

    pub fn get(&self, id: &str) -> rusqlite::Result<Option<ObjectMeta>> {
        let conn = self.conn.lock().expect("metadata db mutex poisoned");
        let mut stmt = conn.prepare(
            "SELECT id, size, content_type, etag, created_at, updated_at FROM objects WHERE id = ?1",
        )?;
        let mut rows = stmt.query(params![id])?;
        if let Some(row) = rows.next()? {
            Ok(Some(ObjectMeta {
                id: row.get(0)?,
                size: row.get(1)?,
                content_type: row.get(2)?,
                etag: row.get(3)?,
                created_at: row.get(4)?,
                updated_at: row.get(5)?,
            }))
        } else {
            Ok(None)
        }
    }


    pub fn list(&self) -> rusqlite::Result<Vec<ObjectMeta>> {
        let conn = self.conn.lock().expect("metadata db mutex poisoned");
        let mut stmt = conn.prepare(
            "SELECT id, size, content_type, etag, created_at, updated_at FROM objects ORDER BY updated_at DESC",
        )?;
        let rows = stmt.query_map([], |row| {
            Ok(ObjectMeta {
                id: row.get(0)?,
                size: row.get(1)?,
                content_type: row.get(2)?,
                etag: row.get(3)?,
                created_at: row.get(4)?,
                updated_at: row.get(5)?,
            })
        })?;
        rows.collect()
    }
}