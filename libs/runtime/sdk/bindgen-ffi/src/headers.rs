pub struct HttpHeaders {}

/// # Safety
/// The caller must ensure the returned pointer is properly managed and eventually deallocated using http_headers_destroy.
pub unsafe fn http_headers_create() -> *mut HttpHeaders {
    let headers = Box::new(HttpHeaders {});
    Box::into_raw(headers)
}

/// # Safety
/// The caller must ensure that `headers` is a valid pointer to HttpHeaders and has not already been deallocated.
pub unsafe fn http_headers_destroy(headers: *mut HttpHeaders) {
    if !headers.is_null() {
        drop(Box::from_raw(headers));
    };
}
