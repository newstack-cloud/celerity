use std::io;

pub(crate) fn new_tokio_multi_thread() -> io::Result<tokio::runtime::Runtime> {
  tokio::runtime::Builder::new_multi_thread()
    .enable_io()
    .enable_time()
    .build()
}
