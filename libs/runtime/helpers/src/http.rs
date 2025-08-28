use std::{collections::HashMap, fmt::Display};

use tokio::sync::RwLock;

/// ResourceStore provides functionality to fetch resources from a given URL.
/// The resource store will cache results for a configured duration.
#[derive(Debug)]
pub struct ResourceStore {
    client: reqwest::Client,
    resources: RwLock<HashMap<String, CacheEntry>>,
    cache_entry_ttl: i64,
}

#[derive(Debug)]
struct CacheEntry {
    value: String,
    expires_at: i64,
}

#[derive(Debug)]
pub enum FetchResourceError {
    FailedToGetResource(reqwest::Error),
}

impl Display for FetchResourceError {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        match self {
            FetchResourceError::FailedToGetResource(e) => write!(f, "Failed to get resource: {e}"),
        }
    }
}

impl From<reqwest::Error> for FetchResourceError {
    fn from(e: reqwest::Error) -> Self {
        FetchResourceError::FailedToGetResource(e)
    }
}

impl ResourceStore {
    /// Create a new resource store with the given client.
    /// The cache entry TTL is the duration in seconds for which the resource will be cached.
    pub fn new(client: reqwest::Client, cache_entry_ttl: i64) -> Self {
        Self {
            client,
            resources: RwLock::new(HashMap::new()),
            cache_entry_ttl,
        }
    }

    /// Get the resource from the cache or fetch it from the given URL.
    /// This will return the response body as a string to be deserialised by the caller
    /// in which ever way it sees fit.
    pub async fn get(&self, url: &str) -> Result<String, FetchResourceError> {
        {
            let read_guard = self.resources.read().await;
            let cache_entry = read_guard.get(url);
            if let Some(cache_entry) = cache_entry {
                if cache_entry.expires_at > chrono::Utc::now().timestamp() {
                    return Ok(cache_entry.value.clone());
                } else {
                    drop(read_guard);
                    self.resources.write().await.remove(url);
                }
            }
        }

        let response = self.client.get(url).send().await?;
        let value = response.text().await?;
        self.resources.write().await.insert(
            url.to_string(),
            CacheEntry {
                value: value.clone(),
                expires_at: chrono::Utc::now().timestamp() + self.cache_entry_ttl,
            },
        );

        Ok(value)
    }

    /// Clean up expired cache entries.
    /// This should be called periodically to ensure that the cache does not grow indefinitely.
    pub async fn clean_expired_cache_entries(&self) {
        let mut write_guard = self.resources.write().await;
        let now = chrono::Utc::now().timestamp();
        write_guard.retain(|_, entry| entry.expires_at > now);
    }
}
