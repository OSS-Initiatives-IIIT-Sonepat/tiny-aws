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

        Self::migrate_schema(&conn)?;

        Ok(Self {
            conn: Mutex::new(conn),
        })
    }

    fn migrate_schema(conn: &Connection) -> rusqlite::Result<()> {
        let mut stmt = conn.prepare("PRAGMA table_info(objects)")?;
        let columns = stmt.query_map([], |row| row.get::<_, String>(1))?;
        let mut existing = std::collections::HashSet::new();
        for col in columns {
            existing.insert(col?);
        }

        if !existing.contains("content_type") {
            conn.execute(
                "ALTER TABLE objects ADD COLUMN content_type TEXT NOT NULL DEFAULT 'application/octet-stream'",
                [],
            )?;
        }
        if !existing.contains("etag") {
            conn.execute(
                "ALTER TABLE objects ADD COLUMN etag TEXT NOT NULL DEFAULT ''",
                [],
            )?;
        }
        if !existing.contains("updated_at") {
            conn.execute(
                "ALTER TABLE objects ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''",
                [],
            )?;
            conn.execute(
                "UPDATE objects SET updated_at = datetime('now') WHERE updated_at = ''",
                [],
            )?;
        }

        Ok(())
    }

    pub fn upsert(
        &self,
        id: &str,
        size: i64,
        content_type: &str,
        etag: &str,
    ) -> rusqlite::Result<()> {
        let conn = self.conn.lock().expect("metadata db mutex poisoned");
        conn.execute(
            "INSERT INTO objects (id, size, content_type, etag)
             VALUES (?1, ?2, ?3, ?4)
             ON CONFLICT(id) DO UPDATE SET
                size = excluded.size,
                content_type = excluded.content_type,
                etag = excluded.etag,
                updated_at = datetime('now')",
            params![id, size, content_type, etag],
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