use rusqlite::{params, Connection};
use std::path::Path;
use std::sync::Mutex;

#[derive(Debug, Clone, serde::Serialize)]
pub struct ObjectMeta {
    pub id: String,
    pub size: i64,
    pub created_at: String,
}

pub struct MetadataStore {
    conn: Mutex<Connection>,
}

impl MetadataStore {
    pub fn new(path: impl AsRef<Path>) -> rusqlite::Result<Self> {
        let conn = Connection::open(path)?;
        conn.execute_batch(
            "CREATE TABLE IF NOT EXISTS objects (
                id          TEXT PRIMARY KEY,
                size        INTEGER NOT NULL,
                created_at  TEXT NOT NULL DEFAULT (datetime('now'))
            );",
        )?;

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
            "SELECT id, size, created_at FROM objects WHERE id = ?1",
        )?;

        let mut rows = stmt.query(params![id])?;
        if let Some(row) = rows.next()? {
            Ok(Some(ObjectMeta {
                id: row.get(0)?,
                size: row.get(1)?,
                created_at: row.get(2)?,
            }))
        } else {
            Ok(None)
        }
    }

    pub fn list(&self) -> rusqlite::Result<Vec<ObjectMeta>> {
        let conn = self.conn.lock().expect("metadata db mutex poisoned");
        let mut stmt = conn.prepare(
            "SELECT id, size, created_at FROM objects ORDER BY created_at DESC",
        )?;

        let rows = stmt.query_map([], |row| {
            Ok(ObjectMeta {
                id: row.get(0)?,
                size: row.get(1)?,
                created_at: row.get(2)?,
            })
        })?;

        rows.collect()
    }
}
