use aws_config::{default_provider::credentials, provider_config::ProviderConfig};
use aws_sdk_account::config::SharedCredentialsProvider;
use aws_types::region::Region;

/// Produces a default credentials provider from the current
/// environment.
pub async fn default_credentials_provider(region: Option<Region>) -> SharedCredentialsProvider {
    let mut builder =
        credentials::DefaultCredentialsChain::builder().configure(ProviderConfig::default());
    builder.set_region(region);
    SharedCredentialsProvider::new(builder.build().await)
}
