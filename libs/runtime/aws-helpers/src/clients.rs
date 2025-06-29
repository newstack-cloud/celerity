/// Creates a new SQS client with an optional custom endpoint.
/// This client builder is useful for VPC endpoints and emulators
/// such as LocalStack.
pub fn sqs_client(
    conf: &aws_types::SdkConfig,
    endpoint_opt: Option<String>,
) -> aws_sdk_sqs::Client {
    let mut sqs_config_builder = aws_sdk_sqs::config::Builder::from(conf);
    if let Some(endpoint) = endpoint_opt {
        sqs_config_builder = sqs_config_builder.endpoint_url(endpoint)
    }
    aws_sdk_sqs::Client::from_conf(sqs_config_builder.build())
}
